package smoke

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	d "toollab-core/internal/pipeline/usecases/domain"
	"toollab-core/internal/pipeline"
)

type Step struct{}

func New() *Step { return &Step{} }

func (s *Step) Name() d.PipelineStep { return d.StepSmoke }

func (s *Step) Run(ctx context.Context, state *pipeline.PipelineState) d.StepResult {
	start := time.Now()

	if state.Catalog == nil || len(state.Catalog.Endpoints) == 0 {
		return d.StepResult{Step: d.StepSmoke, Status: "skipped", DurationMs: ms(start), Error: "no endpoints"}
	}

	baseURL := ""
	if state.TargetProfile != nil {
		baseURL = state.TargetProfile.BaseURL
	}
	if baseURL == "" {
		baseURL = state.Target.RuntimeHint.BaseURL
	}
	if baseURL == "" {
		return d.StepResult{Step: d.StepSmoke, Status: "failed", DurationMs: ms(start), Error: "no base_url"}
	}

	client := &http.Client{Timeout: 10 * time.Second}
	budgetUsed := 0
	var results []d.SmokeResult

	for _, ep := range state.Catalog.Endpoints {
		if ctx.Err() != nil {
			break
		}
		if !state.Budget.CanRequest(ep.EndpointID, "smoke") {
			break
		}

		path := ep.Path
		path = replacePathParams(path)
		fullURL := strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")

		var bodyReader io.Reader
		var ct string
		if ep.Method == "POST" || ep.Method == "PUT" || ep.Method == "PATCH" {
			bodyReader = bytes.NewReader([]byte("{}"))
			ct = "application/json"
		}

		req, err := http.NewRequestWithContext(ctx, ep.Method, fullURL, bodyReader)
		if err != nil {
			results = append(results, d.SmokeResult{
				EndpointID:  ep.EndpointID,
				Method:      ep.Method,
				Path:        ep.Path,
				Passed:      false,
				BlockReason: err.Error(),
			})
			continue
		}

		if ct != "" {
			req.Header.Set("Content-Type", ct)
		}
		for k, v := range state.Target.RuntimeHint.AuthHeaders {
			req.Header.Set(k, v)
		}
		req.Header.Set("Accept", "application/json")

		t0 := time.Now()
		resp, err := client.Do(req)
		latency := time.Since(t0).Milliseconds()
		state.Budget.Record(ep.EndpointID, "smoke")
		budgetUsed++

		evReq := d.EvidenceRequest{
			Method:      ep.Method,
			URL:         fullURL,
			Path:        ep.Path,
			Headers:     headerMap(req.Header),
			ContentType: ct,
		}

		if err != nil {
			eid := state.Evidence.Add(ep.EndpointID, d.CatSmoke, []string{"smoke"}, evReq, nil, d.EvidenceTiming{LatencyMs: latency}, err.Error())
			results = append(results, d.SmokeResult{
				EndpointID:   ep.EndpointID,
				Method:       ep.Method,
				Path:         ep.Path,
				Passed:       false,
				EvidenceRefs: []string{eid},
				BlockReason:  err.Error(),
			})
			continue
		}

		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		resp.Body.Close()

		evResp := &d.EvidenceResponse{
			Status:      resp.StatusCode,
			Headers:     headerMap(resp.Header),
			BodySnippet: string(body),
			ContentType: resp.Header.Get("Content-Type"),
			Size:        int64(len(body)),
		}

		eid := state.Evidence.Add(ep.EndpointID, d.CatSmoke, []string{"smoke"}, evReq, evResp, d.EvidenceTiming{LatencyMs: latency}, "")

		passed := resp.StatusCode >= 200 && resp.StatusCode < 500
		results = append(results, d.SmokeResult{
			EndpointID:   ep.EndpointID,
			Method:       ep.Method,
			Path:         ep.Path,
			Passed:       passed,
			StatusCode:   resp.StatusCode,
			EvidenceRefs: []string{eid},
		})

		state.Emit(pipeline.ProgressEvent{
			Step:    d.StepSmoke,
			Phase:   "exec",
			Message: fmt.Sprintf("[Smoke] %s %s → %d (%dms)", ep.Method, ep.Path, resp.StatusCode, latency),
			Current: budgetUsed,
			Total:   len(state.Catalog.Endpoints),
		})
	}

	state.SmokeResults = results

	passed := 0
	for _, r := range results {
		if r.Passed {
			passed++
		}
	}

	state.Emit(pipeline.ProgressEvent{
		Step:    d.StepSmoke,
		Phase:   "results",
		Message: fmt.Sprintf("Smoke: %d/%d passed", passed, len(results)),
	})

	return d.StepResult{
		Step:       d.StepSmoke,
		Status:     "ok",
		DurationMs: ms(start),
		BudgetUsed: budgetUsed,
	}
}

func replacePathParams(path string) string {
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
