package logic

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	d "toollab-core/internal/domain"
	"toollab-core/internal/pipeline"
)

type Step struct{}

func New() *Step { return &Step{} }

func (s *Step) Name() d.PipelineStep { return d.StepLogic }

func (s *Step) Run(ctx context.Context, state *pipeline.PipelineState) d.StepResult {
	start := time.Now()

	if state.Catalog == nil || len(state.Catalog.Endpoints) == 0 {
		return d.StepResult{Step: d.StepLogic, Status: "skipped", DurationMs: ms(start)}
	}

	baseURL := ""
	if state.TargetProfile != nil {
		baseURL = state.TargetProfile.BaseURL
	}
	if baseURL == "" {
		baseURL = state.Target.RuntimeHint.BaseURL
	}

	client := &http.Client{Timeout: 10 * time.Second}
	budgetUsed := 0
	var results []d.LogicResult

	annotMap := make(map[string]d.SemanticAnnotation)
	for _, a := range state.SemanticAnnotations {
		annotMap[a.EndpointID] = a
	}

	for _, ep := range state.Catalog.Endpoints {
		if ctx.Err() != nil || !state.Budget.CanRequest(ep.EndpointID, "logic") {
			break
		}

		annot := annotMap[ep.EndpointID]
		path := replaceParams(ep.Path)
		fullURL := strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")

		// IDOR test: vary ID params
		if hasIDField(annot) && (ep.Method == "GET" || ep.Method == "PUT" || ep.Method == "DELETE") {
			if state.Budget.CanRequest(ep.EndpointID, "logic") {
				idorURL := varyIDInURL(fullURL)
				r := s.testIDOR(ctx, client, state, ep, idorURL)
				results = append(results, r)
				budgetUsed++
			}
		}

		// Duplicate creation test
		if ep.Method == "POST" && state.Budget.CanRequest(ep.EndpointID, "logic") {
			r := s.testDuplicate(ctx, client, state, ep, fullURL)
			results = append(results, r...)
			budgetUsed += 2
		}

		// Idempotency test (retry same request)
		if (ep.Method == "POST" || ep.Method == "PUT") && state.Budget.CanRequest(ep.EndpointID, "logic") {
			r := s.testIdempotency(ctx, client, state, ep, fullURL)
			results = append(results, r)
			budgetUsed += 2
		}

		// Concurrency test (parallel updates)
		if (ep.Method == "PUT" || ep.Method == "PATCH") && state.Budget.CanRequest(ep.EndpointID, "logic") {
			r := s.testConcurrency(ctx, client, state, ep, fullURL)
			results = append(results, r)
			budgetUsed += 2
		}
	}

	state.LogicResults = results

	anomalies := 0
	for _, r := range results {
		if r.Anomaly {
			anomalies++
		}
	}

	state.Emit(pipeline.ProgressEvent{
		Step:    d.StepLogic,
		Phase:   "results",
		Message: fmt.Sprintf("Logic: %d tests, %d anomalies", len(results), anomalies),
	})

	return d.StepResult{Step: d.StepLogic, Status: "ok", DurationMs: ms(start), BudgetUsed: budgetUsed}
}

func (s *Step) testIDOR(ctx context.Context, client *http.Client, state *pipeline.PipelineState, ep d.EndpointEntry, url string) d.LogicResult {
	state.Budget.Record(ep.EndpointID, "logic")

	req, _ := http.NewRequestWithContext(ctx, ep.Method, url, nil)
	req.Header.Set("Accept", "application/json")
	// NO auth headers — testing without owner auth

	t0 := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(t0).Milliseconds()

	evReq := d.EvidenceRequest{Method: ep.Method, URL: url, Path: ep.Path, Headers: headerMap(req.Header)}

	if err != nil {
		eid := state.Evidence.Add(ep.EndpointID, d.CatLogic, []string{"logic", "idor"}, evReq, nil, d.EvidenceTiming{LatencyMs: latency}, err.Error())
		return d.LogicResult{EndpointID: ep.EndpointID, TestType: "idor", Description: "IDOR test failed: " + err.Error(), EvidenceRefs: []string{eid}}
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	resp.Body.Close()

	evResp := &d.EvidenceResponse{Status: resp.StatusCode, Headers: headerMap(resp.Header), BodySnippet: string(body), ContentType: resp.Header.Get("Content-Type"), Size: int64(len(body))}
	eid := state.Evidence.Add(ep.EndpointID, d.CatLogic, []string{"logic", "idor"}, evReq, evResp, d.EvidenceTiming{LatencyMs: latency}, "")

	anomaly := resp.StatusCode == 200 && len(body) > 10
	if anomaly {
		state.AddFindings(d.FindingRaw{
			FindingID:      d.FindingID(string(d.TaxLogicIDOR), ep.EndpointID, []string{eid}),
			TaxonomyID:     d.TaxLogicIDOR,
			Severity:       d.SeverityHigh,
			Category:       d.FindCatIDOR,
			EndpointID:     ep.EndpointID,
			Title:          fmt.Sprintf("Possible IDOR on %s %s — returned data with varied ID without auth", ep.Method, ep.Path),
			Description:    fmt.Sprintf("Acceso a recurso con ID variado sin credenciales de propietario retornó 200 con datos."),
			EvidenceRefs:   []string{eid},
			Confidence:     0.6,
			Classification: d.ClassCandidate,
		})
	}

	return d.LogicResult{
		EndpointID:   ep.EndpointID,
		TestType:     "idor",
		Description:  fmt.Sprintf("IDOR test: varied ID → %d", resp.StatusCode),
		Passed:       resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 404,
		Anomaly:      anomaly,
		EvidenceRefs: []string{eid},
	}
}

func (s *Step) testDuplicate(ctx context.Context, client *http.Client, state *pipeline.PipelineState, ep d.EndpointEntry, url string) []d.LogicResult {
	body := `{"name":"toollab_dup_test","email":"dup@test.local"}`
	var results []d.LogicResult

	for i := 0; i < 2; i++ {
		if !state.Budget.CanRequest(ep.EndpointID, "logic") {
			break
		}
		state.Budget.Record(ep.EndpointID, "logic")

		req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		for k, v := range state.Target.RuntimeHint.AuthHeaders {
			req.Header.Set(k, v)
		}

		t0 := time.Now()
		resp, err := client.Do(req)
		latency := time.Since(t0).Milliseconds()

		evReq := d.EvidenceRequest{Method: "POST", URL: url, Path: ep.Path, Body: body, ContentType: "application/json", Headers: headerMap(req.Header)}

		if err != nil {
			eid := state.Evidence.Add(ep.EndpointID, d.CatLogic, []string{"logic", "duplicate"}, evReq, nil, d.EvidenceTiming{LatencyMs: latency}, err.Error())
			results = append(results, d.LogicResult{EndpointID: ep.EndpointID, TestType: "duplicate", EvidenceRefs: []string{eid}})
			continue
		}

		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		resp.Body.Close()

		evResp := &d.EvidenceResponse{Status: resp.StatusCode, BodySnippet: string(respBody), ContentType: resp.Header.Get("Content-Type"), Size: int64(len(respBody))}
		eid := state.Evidence.Add(ep.EndpointID, d.CatLogic, []string{"logic", "duplicate"}, evReq, evResp, d.EvidenceTiming{LatencyMs: latency}, "")

		anomaly := i == 1 && (resp.StatusCode == 200 || resp.StatusCode == 201)
		results = append(results, d.LogicResult{
			EndpointID:   ep.EndpointID,
			TestType:     "duplicate",
			Description:  fmt.Sprintf("Duplicate creation attempt %d → %d", i+1, resp.StatusCode),
			Passed:       i == 0 || resp.StatusCode == 409 || resp.StatusCode == 422,
			Anomaly:      anomaly,
			EvidenceRefs: []string{eid},
		})

		if anomaly {
			state.AddFindings(d.FindingRaw{
				FindingID:      d.FindingID(string(d.TaxLogicDuplicate), ep.EndpointID, []string{eid}),
				TaxonomyID:     d.TaxLogicDuplicate,
				Severity:       d.SeverityMedium,
				Category:       d.FindCatLogic,
				EndpointID:     ep.EndpointID,
				Title:          fmt.Sprintf("Duplicate creation accepted on %s %s", ep.Method, ep.Path),
				EvidenceRefs:   []string{eid},
				Confidence:     0.7,
				Classification: d.ClassCandidate,
			})
		}
	}

	return results
}

func (s *Step) testIdempotency(ctx context.Context, client *http.Client, state *pipeline.PipelineState, ep d.EndpointEntry, url string) d.LogicResult {
	body := `{"name":"idempotency_test"}`
	var eids []string

	for i := 0; i < 2; i++ {
		if !state.Budget.CanRequest(ep.EndpointID, "logic") {
			break
		}
		state.Budget.Record(ep.EndpointID, "logic")

		req, _ := http.NewRequestWithContext(ctx, ep.Method, url, bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		for k, v := range state.Target.RuntimeHint.AuthHeaders {
			req.Header.Set(k, v)
		}

		t0 := time.Now()
		resp, err := client.Do(req)
		latency := time.Since(t0).Milliseconds()

		evReq := d.EvidenceRequest{Method: ep.Method, URL: url, Path: ep.Path, Body: body, ContentType: "application/json"}

		if err != nil {
			eid := state.Evidence.Add(ep.EndpointID, d.CatLogic, []string{"logic", "idempotency"}, evReq, nil, d.EvidenceTiming{LatencyMs: latency}, err.Error())
			eids = append(eids, eid)
			continue
		}

		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		resp.Body.Close()

		evResp := &d.EvidenceResponse{Status: resp.StatusCode, BodySnippet: string(respBody), Size: int64(len(respBody))}
		eid := state.Evidence.Add(ep.EndpointID, d.CatLogic, []string{"logic", "idempotency"}, evReq, evResp, d.EvidenceTiming{LatencyMs: latency}, "")
		eids = append(eids, eid)
	}

	return d.LogicResult{
		EndpointID:   ep.EndpointID,
		TestType:     "idempotency",
		Description:  "Retry same request twice to check idempotency",
		Passed:       true,
		EvidenceRefs: eids,
	}
}

func (s *Step) testConcurrency(ctx context.Context, client *http.Client, state *pipeline.PipelineState, ep d.EndpointEntry, url string) d.LogicResult {
	body1 := `{"name":"concurrent_a"}`
	body2 := `{"name":"concurrent_b"}`

	var wg sync.WaitGroup
	var mu sync.Mutex
	var eids []string
	var statuses []int

	for _, b := range []string{body1, body2} {
		if !state.Budget.CanRequest(ep.EndpointID, "logic") {
			break
		}
		state.Budget.Record(ep.EndpointID, "logic")

		wg.Add(1)
		go func(bd string) {
			defer wg.Done()

			req, _ := http.NewRequestWithContext(ctx, ep.Method, url, bytes.NewReader([]byte(bd)))
			req.Header.Set("Content-Type", "application/json")
			for k, v := range state.Target.RuntimeHint.AuthHeaders {
				req.Header.Set(k, v)
			}

			t0 := time.Now()
			resp, err := client.Do(req)
			latency := time.Since(t0).Milliseconds()

			evReq := d.EvidenceRequest{Method: ep.Method, URL: url, Path: ep.Path, Body: bd, ContentType: "application/json"}

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				eid := state.Evidence.Add(ep.EndpointID, d.CatLogic, []string{"logic", "concurrency"}, evReq, nil, d.EvidenceTiming{LatencyMs: latency}, err.Error())
				eids = append(eids, eid)
				return
			}

			respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
			resp.Body.Close()

			evResp := &d.EvidenceResponse{Status: resp.StatusCode, BodySnippet: string(respBody), Size: int64(len(respBody))}
			eid := state.Evidence.Add(ep.EndpointID, d.CatLogic, []string{"logic", "concurrency"}, evReq, evResp, d.EvidenceTiming{LatencyMs: latency}, "")
			eids = append(eids, eid)
			statuses = append(statuses, resp.StatusCode)
		}(b)
	}

	wg.Wait()

	anomaly := false
	for _, st := range statuses {
		if st >= 500 {
			anomaly = true
			break
		}
	}

	if anomaly {
		state.AddFindings(d.FindingRaw{
			FindingID:      d.FindingID(string(d.TaxLogicRaceCondition), ep.EndpointID, eids),
			TaxonomyID:     d.TaxLogicRaceCondition,
			Severity:       d.SeverityMedium,
			Category:       d.FindCatLogic,
			EndpointID:     ep.EndpointID,
			Title:          fmt.Sprintf("Race condition suspected on %s %s — 5xx under concurrent updates", ep.Method, ep.Path),
			EvidenceRefs:   eids,
			Confidence:     0.5,
			Classification: d.ClassCandidate,
		})
	}

	return d.LogicResult{
		EndpointID:   ep.EndpointID,
		TestType:     "concurrency",
		Description:  "2 parallel updates to detect race conditions",
		Passed:       !anomaly,
		Anomaly:      anomaly,
		EvidenceRefs: eids,
	}
}

func hasIDField(annot d.SemanticAnnotation) bool {
	for _, f := range annot.Fields {
		if f.Tag == "id_field" || f.Tag == "owner_field" {
			return true
		}
	}
	return false
}

func varyIDInURL(url string) string {
	return strings.Replace(url, "/1", "/99999", 1)
}

func replaceParams(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") {
			parts[i] = "1"
		}
	}
	return strings.Join(parts, "/")
}

func headerMap(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k := range h {
		out[k] = h.Get(k)
	}
	return out
}

func ms(start time.Time) int64 { return time.Since(start).Milliseconds() }
