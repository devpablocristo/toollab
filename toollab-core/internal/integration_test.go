package internal_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"

	artifactHandler "toollab-core/internal/artifact/handler"
	artifactRepo "toollab-core/internal/artifact/repository"
	artifactUC "toollab-core/internal/artifact/usecases"
	discoveryUC "toollab-core/internal/discovery/usecases"
	evidenceUC "toollab-core/internal/evidence/usecases"
	runHandler "toollab-core/internal/run/handler"
	runRepo "toollab-core/internal/run/repository"
	runUC "toollab-core/internal/run/usecases"
	runnerUC "toollab-core/internal/runner/usecases"
	"toollab-core/internal/shared"
	targetHandler "toollab-core/internal/target/handler"
	targetRepo "toollab-core/internal/target/repository"
	targetUC "toollab-core/internal/target/usecases"
)

func setupServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")

	db, err := shared.OpenDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	migSQL, _ := os.ReadFile("../migrations/001_init.sql")
	if err := shared.Migrate(db, []string{string(migSQL)}); err != nil {
		t.Fatal(err)
	}

	tRepo := targetRepo.NewSQLite(db)
	rRepo := runRepo.NewSQLite(db)
	aIdxRepo := artifactRepo.NewSQLite(db)
	aStorage := artifactRepo.NewFSStorage(tmp)

	tSvc := targetUC.NewService(tRepo)
	rSvc := runUC.NewService(rRepo, tRepo)
	aSvc := artifactUC.NewService(aIdxRepo, aStorage, rRepo)

	runner := runnerUC.NewHTTPRunner()
	artPutter := evidenceUC.NewArtifactPutter(aSvc)
	ingestor := evidenceUC.NewFSIngestor(aStorage, artPutter)
	executor := runUC.NewExecutor(rRepo, tRepo, aSvc, runner, ingestor)

	chiAnalyzer := discoveryUC.NewChiAnalyzer()
	dSvc := discoveryUC.NewService(chiAnalyzer, aSvc, tRepo)

	tH := targetHandler.New(tSvc)
	rH := runHandler.New(rSvc, executor, aSvc, aStorage, dSvc)
	aH := artifactHandler.New(aSvc)

	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/targets", func(r chi.Router) {
			r.Mount("/", tH.Routes())
			r.Route("/{target_id}/runs", func(r chi.Router) {
				r.Mount("/", rH.TargetRoutes())
			})
		})
		r.Route("/runs", func(r chi.Router) {
			r.Mount("/", rH.RunRoutes())
			r.Route("/{run_id}/artifacts", func(r chi.Router) {
				r.Mount("/", aH.Routes())
			})
		})
	})

	return httptest.NewServer(r), tmp
}

func TestFullLoop(t *testing.T) {
	ts, _ := setupServer(t)
	defer ts.Close()

	targetBody := `{"name":"nexus","source":{"type":"path","value":"/workspace/nexus"},"runtime_hint":{"base_url":"http://localhost:3000"}}`
	resp := doReq(t, ts, "POST", "/api/v1/targets", targetBody)
	assertStatus(t, resp, http.StatusCreated)
	var target map[string]any
	readJSON(t, resp, &target)
	targetID := target["id"].(string)
	if targetID == "" {
		t.Fatal("target id empty")
	}

	resp = doReq(t, ts, "GET", "/api/v1/targets", "")
	assertStatus(t, resp, http.StatusOK)
	var tl map[string]any
	readJSON(t, resp, &tl)
	if len(tl["items"].([]any)) != 1 {
		t.Fatal("expected 1 target")
	}

	resp = doReq(t, ts, "GET", "/api/v1/targets/"+targetID, "")
	assertStatus(t, resp, http.StatusOK)

	resp = doReq(t, ts, "POST", "/api/v1/targets/"+targetID+"/runs", `{"seed":"s","notes":"n"}`)
	assertStatus(t, resp, http.StatusCreated)
	var run map[string]any
	readJSON(t, resp, &run)
	runID := run["id"].(string)

	resp = doReq(t, ts, "GET", "/api/v1/targets/"+targetID+"/runs", "")
	assertStatus(t, resp, http.StatusOK)

	resp = doReq(t, ts, "GET", "/api/v1/runs/"+runID, "")
	assertStatus(t, resp, http.StatusOK)

	plan1 := `{"schema_version":"v1","cases":[{"id":"c1","endpoint_method":"GET","endpoint_path":"/health","category":"happy","request":{"method":"GET","url":"/health"}}]}`
	resp = doReq(t, ts, "PUT", "/api/v1/runs/"+runID+"/artifacts/scenario_plan", plan1)
	assertStatus(t, resp, http.StatusCreated)
	var pr map[string]any
	readJSON(t, resp, &pr)
	if int(pr["revision"].(float64)) != 1 {
		t.Fatalf("expected rev 1, got %v", pr["revision"])
	}

	plan2 := `{"schema_version":"v1","cases":[{"id":"c1","endpoint_method":"GET","endpoint_path":"/health","category":"happy","request":{"method":"GET","url":"/health"}},{"id":"c2","endpoint_method":"POST","endpoint_path":"/users","category":"invalid","request":{"method":"POST","url":"/users"}}]}`
	resp = doReq(t, ts, "PUT", "/api/v1/runs/"+runID+"/artifacts/scenario_plan", plan2)
	assertStatus(t, resp, http.StatusCreated)
	readJSON(t, resp, &pr)
	if int(pr["revision"].(float64)) != 2 {
		t.Fatalf("expected rev 2, got %v", pr["revision"])
	}

	resp = doReq(t, ts, "GET", "/api/v1/runs/"+runID+"/artifacts/scenario_plan", "")
	assertStatus(t, resp, http.StatusOK)
	body := readBody(t, resp)
	if !bytes.Contains(body, []byte("c2")) {
		t.Fatal("latest should contain c2")
	}

	resp = doReq(t, ts, "GET", "/api/v1/runs/"+runID+"/artifacts/scenario_plan/v/1", "")
	assertStatus(t, resp, http.StatusOK)
	body = readBody(t, resp)
	if bytes.Contains(body, []byte("c2")) {
		t.Fatal("rev 1 should NOT contain c2")
	}

	resp = doReq(t, ts, "GET", "/api/v1/runs/"+runID+"/artifacts/scenario_plan/meta", "")
	assertStatus(t, resp, http.StatusOK)
	var meta map[string]any
	readJSON(t, resp, &meta)
	if int(meta["revision"].(float64)) != 2 {
		t.Fatalf("meta should show rev 2")
	}

	resp = doReq(t, ts, "GET", "/api/v1/runs/"+runID+"/artifacts/scenario_plan/revisions", "")
	assertStatus(t, resp, http.StatusOK)
	var rl map[string]any
	readJSON(t, resp, &rl)
	if len(rl["items"].([]any)) != 2 {
		t.Fatal("expected 2 revisions")
	}

	resp = doReq(t, ts, "GET", "/api/v1/runs/"+runID+"/artifacts", "")
	assertStatus(t, resp, http.StatusOK)

	resp = doReq(t, ts, "PUT", "/api/v1/runs/"+runID+"/artifacts/bogus_type", `{}`)
	assertStatus(t, resp, http.StatusBadRequest)

	resp = doReq(t, ts, "GET", "/api/v1/runs/nonexistent", "")
	assertStatus(t, resp, http.StatusNotFound)

	resp = doReq(t, ts, "PUT", "/api/v1/runs/"+runID+"/artifacts/scenario_plan", "not json")
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestExecuteRunLoop(t *testing.T) {
	fakeSvc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/ok":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"healthy"}`))
		case r.Method == "POST" && r.URL.Path == "/echo":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			body, _ := io.ReadAll(r.Body)
			_, _ = w.Write(body)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer fakeSvc.Close()

	ts, dataDir := setupServer(t)
	defer ts.Close()

	// 1) Create target with base_url pointing to fake service
	targetBody := fmt.Sprintf(
		`{"name":"test-svc","source":{"type":"path","value":"/test"},"runtime_hint":{"base_url":"%s"}}`,
		fakeSvc.URL,
	)
	resp := doReq(t, ts, "POST", "/api/v1/targets", targetBody)
	assertStatus(t, resp, http.StatusCreated)
	var target map[string]any
	readJSON(t, resp, &target)
	targetID := target["id"].(string)

	// 2) Create run
	resp = doReq(t, ts, "POST", "/api/v1/targets/"+targetID+"/runs", `{"seed":"test-seed"}`)
	assertStatus(t, resp, http.StatusCreated)
	var run map[string]any
	readJSON(t, resp, &run)
	runID := run["id"].(string)

	// 3) Upload scenario_plan with 2 cases
	plan := fmt.Sprintf(`{
		"plan_id": "plan-1",
		"run_id": "%s",
		"schema_version": "v1",
		"cases": [
			{
				"case_id": "case-get-ok",
				"name": "GET /ok returns 200",
				"enabled": true,
				"tags": ["smoke"],
				"request": {"method": "GET", "path": "/ok"}
			},
			{
				"case_id": "case-post-echo",
				"name": "POST /echo returns 201",
				"enabled": true,
				"tags": ["smoke"],
				"request": {
					"method": "POST",
					"path": "/echo",
					"headers": {"X-Custom": "value"},
					"body_json": {"msg": "hello"}
				}
			}
		]
	}`, runID)
	resp = doReq(t, ts, "PUT", "/api/v1/runs/"+runID+"/scenario-plan", plan)
	assertStatus(t, resp, http.StatusCreated)
	var putRes map[string]any
	readJSON(t, resp, &putRes)
	if int(putRes["revision"].(float64)) != 1 {
		t.Fatalf("expected scenario_plan rev 1, got %v", putRes["revision"])
	}

	// 4) Execute run
	resp = doReq(t, ts, "POST", "/api/v1/runs/"+runID+"/execute", `{}`)
	assertStatus(t, resp, http.StatusOK)
	var execResp map[string]any
	readJSON(t, resp, &execResp)
	if execResp["status"] != "completed" {
		t.Fatalf("expected status completed, got %v", execResp["status"])
	}
	if int(execResp["evidence_pack_revision"].(float64)) != 1 {
		t.Fatalf("expected evidence_pack rev 1, got %v", execResp["evidence_pack_revision"])
	}
	packID := execResp["pack_id"].(string)
	if packID == "" {
		t.Fatal("pack_id is empty")
	}

	// 5) Verify run status is completed
	resp = doReq(t, ts, "GET", "/api/v1/runs/"+runID, "")
	assertStatus(t, resp, http.StatusOK)
	readJSON(t, resp, &run)
	if run["status"] != "completed" {
		t.Fatalf("run status should be completed, got %v", run["status"])
	}

	// 6) Get evidence_pack artifact and validate body_ref, hashes, raw files
	resp = doReq(t, ts, "GET", "/api/v1/runs/"+runID+"/evidence", "")
	assertStatus(t, resp, http.StatusOK)
	var pack map[string]any
	readJSON(t, resp, &pack)
	items := pack["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("expected 2 evidence items, got %d", len(items))
	}

	var postEvidenceID string
	for i, raw := range items {
		item := raw.(map[string]any)
		evidenceID := item["evidence_id"].(string)
		if evidenceID == "" {
			t.Fatalf("item %d: evidence_id empty", i)
		}
		caseID := item["case_id"].(string)
		if item["kind"] != "http_exchange" {
			t.Fatalf("item %d: expected kind http_exchange, got %v", i, item["kind"])
		}

		reqMap := item["request"].(map[string]any)
		if reqMap["method"] == "" {
			t.Fatalf("item %d: request.method empty", i)
		}

		respMap := item["response"].(map[string]any)
		if respMap == nil {
			t.Fatalf("item %d: response is nil", i)
		}

		if caseID == "case-post-echo" {
			postEvidenceID = evidenceID

			if reqMap["body_ref"] == nil || reqMap["body_ref"] == "" {
				t.Fatalf("POST case: request.body_ref should be set")
			}
			if reqMap["body_inline_truncated"] == nil || reqMap["body_inline_truncated"] == "" {
				t.Fatalf("POST case: request.body_inline_truncated should be set")
			}
			if respMap["body_ref"] == nil || respMap["body_ref"] == "" {
				t.Fatalf("POST case: response.body_ref should be set")
			}

			hashes := item["hashes"].(map[string]any)
			if hashes["sha256_request_body"] == nil || hashes["sha256_request_body"] == "" {
				t.Fatalf("POST case: sha256_request_body should be set")
			}
			if hashes["sha256_response_body"] == nil || hashes["sha256_response_body"] == "" {
				t.Fatalf("POST case: sha256_response_body should be set")
			}

			reqRef := reqMap["body_ref"].(string)
			rawReqPath := filepath.Join(dataDir, reqRef)
			if _, err := os.Stat(rawReqPath); os.IsNotExist(err) {
				t.Fatalf("raw request body file does not exist: %s", rawReqPath)
			}

			respRef := respMap["body_ref"].(string)
			rawRespPath := filepath.Join(dataDir, respRef)
			if _, err := os.Stat(rawRespPath); os.IsNotExist(err) {
				t.Fatalf("raw response body file does not exist: %s", rawRespPath)
			}
		}
	}

	// 6b) GET /evidence/items/{evidence_id} for the POST case
	if postEvidenceID == "" {
		t.Fatal("POST case evidence_id not found")
	}
	resp = doReq(t, ts, "GET", "/api/v1/runs/"+runID+"/evidence/items/"+postEvidenceID, "")
	assertStatus(t, resp, http.StatusOK)
	var itemDetail map[string]any
	readJSON(t, resp, &itemDetail)
	if itemDetail["evidence_id"] != postEvidenceID {
		t.Fatalf("evidence item: expected id %s, got %v", postEvidenceID, itemDetail["evidence_id"])
	}
	if itemDetail["request_body_full"] == nil || itemDetail["request_body_full"] == "" {
		t.Fatal("evidence item: request_body_full should contain the full request body")
	}
	if itemDetail["response_body_full"] == nil || itemDetail["response_body_full"] == "" {
		t.Fatal("evidence item: response_body_full should contain the full response body")
	}

	// 6c) GET /evidence/items/nonexistent should 404
	resp = doReq(t, ts, "GET", "/api/v1/runs/"+runID+"/evidence/items/nonexistent", "")
	assertStatus(t, resp, http.StatusNotFound)

	// 7) Verify evidence_pack as artifact (via artifact API)
	resp = doReq(t, ts, "GET", "/api/v1/runs/"+runID+"/artifacts/evidence_pack/meta", "")
	assertStatus(t, resp, http.StatusOK)
	var artMeta map[string]any
	readJSON(t, resp, &artMeta)
	if int(artMeta["revision"].(float64)) != 1 {
		t.Fatalf("expected artifact rev 1, got %v", artMeta["revision"])
	}

	// 8) Re-execute run: evidence_pack revision should increment (append-only)
	resp = doReq(t, ts, "POST", "/api/v1/runs/"+runID+"/execute", `{}`)
	assertStatus(t, resp, http.StatusOK)
	readJSON(t, resp, &execResp)
	if int(execResp["evidence_pack_revision"].(float64)) != 2 {
		t.Fatalf("expected evidence_pack rev 2 after re-execute, got %v", execResp["evidence_pack_revision"])
	}

	// 9) Upload new scenario_plan and verify revision increments
	plan2 := fmt.Sprintf(`{
		"plan_id": "plan-2",
		"run_id": "%s",
		"schema_version": "v1",
		"cases": [
			{
				"case_id": "case-get-ok",
				"name": "GET /ok returns 200",
				"enabled": true,
				"request": {"method": "GET", "path": "/ok"}
			}
		]
	}`, runID)
	resp = doReq(t, ts, "PUT", "/api/v1/runs/"+runID+"/scenario-plan", plan2)
	assertStatus(t, resp, http.StatusCreated)
	readJSON(t, resp, &putRes)
	if int(putRes["revision"].(float64)) != 2 {
		t.Fatalf("expected scenario_plan rev 2, got %v", putRes["revision"])
	}

	// 10) Verify GET scenario-plan returns latest
	resp = doReq(t, ts, "GET", "/api/v1/runs/"+runID+"/scenario-plan", "")
	assertStatus(t, resp, http.StatusOK)
	var latestPlan map[string]any
	readJSON(t, resp, &latestPlan)
	cases := latestPlan["cases"].([]any)
	if len(cases) != 1 {
		t.Fatalf("latest plan should have 1 case, got %d", len(cases))
	}
}

func TestDiscoverFlow(t *testing.T) {
	ts, _ := setupServer(t)
	defer ts.Close()

	testdataPath, err := filepath.Abs("../testdata/sample-chi-app")
	if err != nil {
		t.Fatal(err)
	}

	// 1) Create target pointing to the testdata fixture
	targetBody := fmt.Sprintf(
		`{"name":"sample-chi","source":{"type":"path","value":"%s"},"runtime_hint":{"base_url":"http://localhost:3000"}}`,
		testdataPath,
	)
	resp := doReq(t, ts, "POST", "/api/v1/targets", targetBody)
	assertStatus(t, resp, http.StatusCreated)
	var target map[string]any
	readJSON(t, resp, &target)
	targetID := target["id"].(string)

	// 2) Create run
	resp = doReq(t, ts, "POST", "/api/v1/targets/"+targetID+"/runs", `{"seed":"discover-test"}`)
	assertStatus(t, resp, http.StatusCreated)
	var run map[string]any
	readJSON(t, resp, &run)
	runID := run["id"].(string)

	// 3) POST discover with generate_scenario_plan=true
	resp = doReq(t, ts, "POST", "/api/v1/runs/"+runID+"/discover",
		`{"framework_hint":"chi","generate_scenario_plan":true}`)
	assertStatus(t, resp, http.StatusOK)
	var discoverResp map[string]any
	readJSON(t, resp, &discoverResp)

	if discoverResp["run_id"] != runID {
		t.Fatalf("expected run_id %s, got %v", runID, discoverResp["run_id"])
	}
	if int(discoverResp["service_model_revision"].(float64)) != 1 {
		t.Fatalf("expected service_model rev 1, got %v", discoverResp["service_model_revision"])
	}
	if int(discoverResp["model_report_revision"].(float64)) != 1 {
		t.Fatalf("expected model_report rev 1, got %v", discoverResp["model_report_revision"])
	}
	endpointsCount := int(discoverResp["endpoints_count"].(float64))
	if endpointsCount < 4 {
		t.Fatalf("expected at least 4 endpoints, got %d", endpointsCount)
	}
	confidence := discoverResp["confidence"].(float64)
	if confidence <= 0 || confidence > 1 {
		t.Fatalf("confidence should be between 0 and 1, got %f", confidence)
	}
	t.Logf("Discovery found %d endpoints with confidence %.2f", endpointsCount, confidence)

	if gaps, ok := discoverResp["gaps"].([]any); ok {
		for _, g := range gaps {
			t.Logf("  gap: %v", g)
		}
	}

	// scenario_plan should have been generated
	scenarioPlanRev := int(discoverResp["scenario_plan_revision"].(float64))
	if scenarioPlanRev < 1 {
		t.Fatal("expected scenario_plan_revision >= 1")
	}

	// 4) Verify service_model artifact exists
	resp = doReq(t, ts, "GET", "/api/v1/runs/"+runID+"/artifacts/service_model", "")
	assertStatus(t, resp, http.StatusOK)
	var serviceModel map[string]any
	readJSON(t, resp, &serviceModel)
	endpoints := serviceModel["endpoints"].([]any)
	if len(endpoints) != endpointsCount {
		t.Fatalf("service_model endpoints mismatch: got %d, expected %d", len(endpoints), endpointsCount)
	}

	foundMethods := make(map[string]bool)
	for _, raw := range endpoints {
		ep := raw.(map[string]any)
		method := ep["method"].(string)
		path := ep["path"].(string)
		t.Logf("  endpoint: %s %s", method, path)
		foundMethods[method] = true

		if ref, ok := ep["ref"].(map[string]any); ok {
			if ref["file"] == nil || ref["file"] == "" {
				t.Fatalf("endpoint ref should have file")
			}
			if ref["line"] == nil {
				t.Fatalf("endpoint ref should have line")
			}
		}
	}
	if !foundMethods["GET"] {
		t.Fatal("should have found GET endpoints")
	}
	if !foundMethods["POST"] {
		t.Fatal("should have found POST endpoints")
	}

	// 5) Verify model_report artifact
	resp = doReq(t, ts, "GET", "/api/v1/runs/"+runID+"/artifacts/model_report", "")
	assertStatus(t, resp, http.StatusOK)
	var modelReport map[string]any
	readJSON(t, resp, &modelReport)
	if int(modelReport["endpoints_count"].(float64)) != endpointsCount {
		t.Fatal("model_report endpoints_count mismatch")
	}

	// 6) Verify scenario_plan was generated
	resp = doReq(t, ts, "GET", "/api/v1/runs/"+runID+"/scenario-plan", "")
	assertStatus(t, resp, http.StatusOK)
	var plan map[string]any
	readJSON(t, resp, &plan)
	cases := plan["cases"].([]any)
	if len(cases) < 4 {
		t.Fatalf("generated scenario_plan should have at least 4 cases, got %d", len(cases))
	}
	t.Logf("Generated scenario plan with %d cases", len(cases))

	for _, raw := range cases {
		c := raw.(map[string]any)
		if c["case_id"] == "" {
			t.Fatal("case should have case_id")
		}
		if c["enabled"] != true {
			t.Fatal("generated cases should be enabled")
		}
		req := c["request"].(map[string]any)
		if req["method"] == "" {
			t.Fatal("case request should have method")
		}
		if req["path"] == "" {
			t.Fatal("case request should have path")
		}
	}

	// 7) Re-discover should create new revisions (append-only)
	resp = doReq(t, ts, "POST", "/api/v1/runs/"+runID+"/discover",
		`{"framework_hint":"chi","generate_scenario_plan":true}`)
	assertStatus(t, resp, http.StatusOK)
	readJSON(t, resp, &discoverResp)
	if int(discoverResp["service_model_revision"].(float64)) != 2 {
		t.Fatalf("expected service_model rev 2 on re-discover, got %v", discoverResp["service_model_revision"])
	}
}

func doReq(t *testing.T, ts *httptest.Server, method, path, body string) *http.Response {
	t.Helper()
	var br io.Reader
	if body != "" {
		br = bytes.NewBufferString(body)
	}
	req, err := http.NewRequest(method, ts.URL+path, br)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func assertStatus(t *testing.T, resp *http.Response, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected %d, got %d: %s", expected, resp.StatusCode, string(b))
	}
}

func readJSON(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatal(err)
	}
}

func readBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
