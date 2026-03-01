package fuzz

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	d "toollab-core/internal/domain"
	"toollab-core/internal/pipeline"
)

type Step struct{}

func New() *Step { return &Step{} }

func (s *Step) Name() d.PipelineStep { return d.StepFuzz }

func (s *Step) Run(ctx context.Context, state *pipeline.PipelineState) d.StepResult {
	start := time.Now()

	if state.Catalog == nil || len(state.Catalog.Endpoints) == 0 {
		return d.StepResult{Step: d.StepFuzz, Status: "skipped", DurationMs: ms(start)}
	}

	baseURL := ""
	if state.TargetProfile != nil {
		baseURL = state.TargetProfile.BaseURL
	}
	if baseURL == "" {
		baseURL = state.Target.RuntimeHint.BaseURL
	}

	client := &http.Client{Timeout: 15 * time.Second}
	budgetUsed := 0
	var results []d.FuzzResult

	contractMap := make(map[string]d.InferredContract)
	for _, c := range state.Contracts {
		contractMap[c.EndpointID] = c
	}

	for _, ep := range state.Catalog.Endpoints {
		if ctx.Err() != nil || !state.Budget.CanRequest(ep.EndpointID, "guided_fuzz") {
			break
		}

		contract := contractMap[ep.EndpointID]
		cases := generateFuzzCases(ep, contract)

		for _, fc := range cases {
			if !state.Budget.CanRequest(ep.EndpointID, "guided_fuzz") {
				break
			}

			path := replaceParams(ep.Path)
			fullURL := strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")

			var bodyReader io.Reader
			var bodyStr string
			if fc.body != "" {
				bodyReader = bytes.NewReader([]byte(fc.body))
				bodyStr = fc.body
			}

			req, err := http.NewRequestWithContext(ctx, fc.method, fullURL, bodyReader)
			if err != nil {
				continue
			}
			for k, v := range fc.headers {
				req.Header.Set(k, v)
			}
			for k, v := range state.Target.RuntimeHint.AuthHeaders {
				req.Header.Set(k, v)
			}

			t0 := time.Now()
			resp, err := client.Do(req)
			latency := time.Since(t0).Milliseconds()
			state.Budget.Record(ep.EndpointID, "guided_fuzz")
			budgetUsed++

			evReq := d.EvidenceRequest{
				Method:      fc.method,
				URL:         fullURL,
				Path:        ep.Path,
				Headers:     headerMap(req.Header),
				Body:        bodyStr,
				ContentType: fc.headers["Content-Type"],
			}

			fr := d.FuzzResult{
				EndpointID:  ep.EndpointID,
				Category:    fc.category,
				SubCategory: fc.subCategory,
				InputDesc:   fc.desc,
			}

			if err != nil {
				fr.Timeout = strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline")
				eid := state.Evidence.Add(ep.EndpointID, d.CatFuzz, []string{"fuzz", fc.category, fc.subCategory}, evReq, nil, d.EvidenceTiming{LatencyMs: latency}, err.Error())
				fr.EvidenceRef = eid
			} else {
				body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
				resp.Body.Close()

				evResp := &d.EvidenceResponse{
					Status:      resp.StatusCode,
					Headers:     headerMap(resp.Header),
					BodySnippet: string(body),
					ContentType: resp.Header.Get("Content-Type"),
					Size:        int64(len(body)),
				}

				eid := state.Evidence.Add(ep.EndpointID, d.CatFuzz, []string{"fuzz", fc.category, fc.subCategory}, evReq, evResp, d.EvidenceTiming{LatencyMs: latency}, "")
				fr.EvidenceRef = eid
				fr.Status = resp.StatusCode
				fr.Crashed = resp.StatusCode >= 500

				if fr.Crashed {
					state.ErrSigBuilder.Observe(resp.StatusCode, resp.Header.Get("Content-Type"), string(body), ep.EndpointID, eid)
					s.recordFinding(state, ep, fc, eid, resp.StatusCode, string(body))
				}

				if fc.category == "injection" && resp.StatusCode == 200 {
					s.checkInjectionSuccess(state, ep, fc, eid, string(body))
				}
			}

			results = append(results, fr)
		}

		state.Emit(pipeline.ProgressEvent{
			Step:    d.StepFuzz,
			Phase:   "exec",
			Message: fmt.Sprintf("[Fuzz] %s %s — %d cases", ep.Method, ep.Path, len(cases)),
		})
	}

	state.FuzzResults = results

	crashes := 0
	for _, r := range results {
		if r.Crashed {
			crashes++
		}
	}

	state.Emit(pipeline.ProgressEvent{
		Step:    d.StepFuzz,
		Phase:   "results",
		Message: fmt.Sprintf("Fuzz: %d tests, %d crashes", len(results), crashes),
	})

	return d.StepResult{Step: d.StepFuzz, Status: "ok", DurationMs: ms(start), BudgetUsed: budgetUsed}
}

type fuzzCase struct {
	method      string
	category    string
	subCategory string
	desc        string
	body        string
	headers     map[string]string
}

func generateFuzzCases(ep d.EndpointEntry, contract d.InferredContract) []fuzzCase {
	var cases []fuzzCase

	// Injection payloads (only for string fields in body)
	if ep.Method == "POST" || ep.Method == "PUT" || ep.Method == "PATCH" {
		stringFields := getStringFields(contract)
		if len(stringFields) == 0 {
			stringFields = []string{"name", "value", "input", "query", "search"}
		}
		for _, field := range stringFields[:min(len(stringFields), 3)] {
			for _, payload := range injectionPayloads {
				body, _ := json.Marshal(map[string]string{field: payload.value})
				cases = append(cases, fuzzCase{
					method:      ep.Method,
					category:    "injection",
					subCategory: payload.subType,
					desc:        fmt.Sprintf("%s in field '%s': %s", payload.subType, field, truncStr(payload.value, 40)),
					body:        string(body),
					headers:     map[string]string{"Content-Type": "application/json"},
				})
			}
		}
	}

	// Boundary values
	if ep.Method == "POST" || ep.Method == "PUT" || ep.Method == "PATCH" {
		for _, bv := range boundaryPayloads {
			cases = append(cases, fuzzCase{
				method:      ep.Method,
				category:    "boundary",
				subCategory: bv.subType,
				desc:        bv.desc,
				body:        bv.body,
				headers:     map[string]string{"Content-Type": "application/json"},
			})
		}
	}

	// Malformed inputs
	for _, mf := range malformedPayloads {
		cases = append(cases, fuzzCase{
			method:      ep.Method,
			category:    "malformed",
			subCategory: mf.subType,
			desc:        mf.desc,
			body:        mf.body,
			headers:     mf.headers,
		})
	}

	// Content-type mismatch
	if ep.Method == "POST" || ep.Method == "PUT" {
		cases = append(cases, fuzzCase{
			method:      ep.Method,
			category:    "content_type",
			subCategory: "xml_as_json",
			desc:        "Send XML with application/json content-type",
			body:        "<root><test>value</test></root>",
			headers:     map[string]string{"Content-Type": "application/json"},
		})
		cases = append(cases, fuzzCase{
			method:      ep.Method,
			category:    "content_type",
			subCategory: "plain_as_json",
			desc:        "Send plain text with application/json content-type",
			body:        "just a plain string",
			headers:     map[string]string{"Content-Type": "application/json"},
		})
	}

	// Method tamper
	unexpected := "DELETE"
	if ep.Method == "DELETE" {
		unexpected = "PATCH"
	}
	cases = append(cases, fuzzCase{
		method:      unexpected,
		category:    "method_tamper",
		subCategory: "wrong_method",
		desc:        fmt.Sprintf("Tamper: send %s instead of %s", unexpected, ep.Method),
		headers:     map[string]string{"Accept": "application/json"},
	})

	return cases
}

func (s *Step) recordFinding(state *pipeline.PipelineState, ep d.EndpointEntry, fc fuzzCase, eid string, status int, body string) {
	var tax d.FindingTaxonomy
	var severity d.FindingSeverity

	switch fc.category {
	case "malformed":
		tax = d.TaxRobMalformedCrash
		severity = d.SeverityHigh
	case "boundary":
		tax = d.TaxRobBoundaryCrash
		severity = d.SeverityHigh
	case "injection":
		tax = d.TaxRobMalformedCrash
		severity = d.SeverityMedium
	default:
		tax = d.TaxRobMalformedCrash
		severity = d.SeverityMedium
	}

	state.AddFindings(d.FindingRaw{
		FindingID:      d.FindingID(string(tax), ep.EndpointID, []string{eid}),
		TaxonomyID:     tax,
		Severity:       severity,
		Category:       d.FindCatRobust,
		EndpointID:     ep.EndpointID,
		Title:          fmt.Sprintf("Server crash (%d) on %s %s with %s input", status, ep.Method, ep.Path, fc.category),
		Description:    fmt.Sprintf("El servidor retornó %d al recibir input %s: %s. Body: %s", status, fc.category, fc.desc, truncStr(body, 200)),
		EvidenceRefs:   []string{eid},
		Confidence:     0.8,
		Classification: d.ClassCandidate,
	})
}

func (s *Step) checkInjectionSuccess(state *pipeline.PipelineState, ep d.EndpointEntry, fc fuzzCase, eid, body string) {
	markers := []struct {
		tax     d.FindingTaxonomy
		markers []string
	}{
		{d.TaxSecSQLi, []string{"sql", "syntax", "mysql", "postgres", "sqlite", "error in your SQL"}},
		{d.TaxSecXSSReflected, []string{"<script>", "alert(", "onerror="}},
		{d.TaxSecPathTraversal, []string{"root:", "/etc/passwd", "boot.ini"}},
		{d.TaxSecCMDi, []string{"uid=", "root:", "command not found"}},
	}

	bodyLower := strings.ToLower(body)
	for _, m := range markers {
		for _, marker := range m.markers {
			if strings.Contains(bodyLower, marker) {
				state.AddFindings(d.FindingRaw{
					FindingID:      d.FindingID(string(m.tax), ep.EndpointID, []string{eid}),
					TaxonomyID:     m.tax,
					Severity:       d.SeverityCritical,
					Category:       d.FindCatInjection,
					EndpointID:     ep.EndpointID,
					Title:          fmt.Sprintf("Possible %s on %s %s", m.tax, ep.Method, ep.Path),
					Description:    fmt.Sprintf("Response 200 con marker '%s' al enviar payload %s. Requiere confirmación.", marker, fc.subCategory),
					EvidenceRefs:   []string{eid},
					Confidence:     0.6,
					Classification: d.ClassCandidate,
				})
				return
			}
		}
	}
}

type payloadDef struct {
	subType string
	value   string
}

var injectionPayloads = []payloadDef{
	{"sqli", "' OR '1'='1' --"},
	{"sqli", "1; DROP TABLE users;--"},
	{"sqli", "' UNION SELECT null,null,null--"},
	{"xss", "<script>alert('xss')</script>"},
	{"xss", "\"><img src=x onerror=alert(1)>"},
	{"path_traversal", "../../etc/passwd"},
	{"path_traversal", "..\\..\\windows\\system32\\config\\sam"},
	{"cmdi", "; ls -la"},
	{"cmdi", "| cat /etc/passwd"},
}

type bodyPayloadDef struct {
	subType string
	desc    string
	body    string
	headers map[string]string
}

var boundaryPayloads = []bodyPayloadDef{
	{"empty_object", "Empty JSON object", "{}", nil},
	{"negative_number", "Negative number values", `{"id":-1,"amount":-999999}`, nil},
	{"zero_values", "All zeros", `{"id":0,"count":0,"amount":0.0}`, nil},
	{"max_int", "Maximum integer", `{"id":9999999999999999}`, nil},
	{"long_string", "Very long string (10KB)", `{"name":"` + strings.Repeat("A", 10000) + `"}`, nil},
	{"unicode", "Unicode special chars", `{"name":"用户名🎉\u0000\uffff"}`, nil},
	{"null_fields", "Null fields", `{"id":null,"name":null,"status":null}`, nil},
	{"array_instead", "Array instead of object", `[{"id":1}]`, nil},
	{"deeply_nested", "Deeply nested object", buildNestedJSON(20), nil},
}

var malformedPayloads = []bodyPayloadDef{
	{"broken_json", "Broken JSON", `{"name": "test"`, map[string]string{"Content-Type": "application/json"}},
	{"empty_body", "Empty body with JSON content-type", "", map[string]string{"Content-Type": "application/json"}},
	{"number_as_body", "Raw number as body", "42", map[string]string{"Content-Type": "application/json"}},
	{"missing_ct", "JSON body without Content-Type", `{"test": true}`, map[string]string{}},
}

func getStringFields(contract d.InferredContract) []string {
	var fields []string
	if contract.RequestSchema != nil {
		for _, f := range contract.RequestSchema.Fields {
			if f.Type == "string" {
				fields = append(fields, f.Name)
			}
		}
	}
	return fields
}

func buildNestedJSON(depth int) string {
	s := ""
	for i := 0; i < depth; i++ {
		s += `{"nested":`
	}
	s += `"value"`
	for i := 0; i < depth; i++ {
		s += "}"
	}
	return s
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

func truncStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func ms(start time.Time) int64 { return time.Since(start).Milliseconds() }
