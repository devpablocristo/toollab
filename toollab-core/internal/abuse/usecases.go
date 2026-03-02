package abuse

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	d "toollab-core/internal/pipeline/usecases/domain"
	"toollab-core/internal/pipeline"
)

type Step struct{}

func New() *Step { return &Step{} }

func (s *Step) Name() d.PipelineStep { return d.StepAbuse }

func (s *Step) Run(ctx context.Context, state *pipeline.PipelineState) d.StepResult {
	start := time.Now()

	if state.Catalog == nil || len(state.Catalog.Endpoints) == 0 {
		return d.StepResult{Step: d.StepAbuse, Status: "skipped", DurationMs: ms(start)}
	}

	baseURL := ""
	if state.TargetProfile != nil {
		baseURL = state.TargetProfile.BaseURL
	}
	if baseURL == "" {
		baseURL = state.Target.RuntimeHint.BaseURL
	}

	budgetUsed := 0
	var results []d.AbuseResult
	resilience := &d.ResilienceMetrics{}

	// Select representative endpoints (max 5 for abuse testing)
	targets := selectAbuseTargets(state.Catalog.Endpoints, 5)

	for _, ep := range targets {
		if ctx.Err() != nil || !state.Budget.CanRequest(ep.EndpointID, "abuse") {
			break
		}

		path := replaceParams(ep.Path)
		fullURL := strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")

		// Rate limiting burst test
		burstSizes := []int{20, 50}
		for _, burst := range burstSizes {
			if !state.Budget.CanRequest(ep.EndpointID, "abuse") {
				break
			}

			r := s.burstTest(ctx, state, ep, fullURL, burst)
			results = append(results, r)
			budgetUsed += burst

			if r.Got429 {
				resilience.RateLimitObserved = true
			}
			if r.Degraded {
				resilience.DegradationOnBurst = true
			}
		}

		// Large payload test (for POST/PUT endpoints)
		if (ep.Method == "POST" || ep.Method == "PUT") && state.Budget.CanRequest(ep.EndpointID, "abuse") {
			r := s.largePayloadTest(ctx, state, ep, fullURL)
			results = append(results, r)
			budgetUsed++
			if !r.Crashed {
				resilience.LargePayloadHandled = true
			}
		}

		state.Emit(pipeline.ProgressEvent{
			Step:    d.StepAbuse,
			Phase:   "exec",
			Message: fmt.Sprintf("[Abuse] %s %s — burst + payload tests", ep.Method, ep.Path),
		})
	}

	state.AbuseResults = results
	state.ResilienceMetrics = resilience

	// Check for rate limit headers in any evidence
	for _, sample := range state.Evidence.Samples() {
		if sample.Response != nil {
			for k := range sample.Response.Headers {
				kl := strings.ToLower(k)
				if strings.Contains(kl, "ratelimit") || strings.Contains(kl, "rate-limit") || strings.Contains(kl, "x-ratelimit") || strings.Contains(kl, "retry-after") {
					resilience.RateLimitHeaders = true
					break
				}
			}
		}
	}

	if !resilience.RateLimitObserved && len(targets) > 0 {
		var eids []string
		for _, r := range results {
			eids = append(eids, r.EvidenceRefs...)
		}
		state.AddFindings(d.FindingRaw{
			FindingID:      d.FindingID(string(d.TaxAbuseNoRateLimit), "_global", eids),
			TaxonomyID:     d.TaxAbuseNoRateLimit,
			Severity:       d.SeverityMedium,
			Category:       d.FindCatRateLimit,
			Title:          "No rate limiting observed under burst testing",
			Description:    "Burst tests (20-50 requests) no provocaron 429 ni headers de rate limit en ningún endpoint.",
			EvidenceRefs:   eids,
			Confidence:     0.7,
			Classification: d.ClassCandidate,
		})
	}

	state.Emit(pipeline.ProgressEvent{
		Step:    d.StepAbuse,
		Phase:   "results",
		Message: fmt.Sprintf("Abuse: %d tests, rate_limit=%v, degradation=%v", len(results), resilience.RateLimitObserved, resilience.DegradationOnBurst),
	})

	return d.StepResult{Step: d.StepAbuse, Status: "ok", DurationMs: ms(start), BudgetUsed: budgetUsed}
}

func (s *Step) burstTest(ctx context.Context, state *pipeline.PipelineState, ep d.EndpointEntry, url string, burst int) d.AbuseResult {
	client := &http.Client{Timeout: 5 * time.Second}
	var mu sync.Mutex
	var eids []string
	got429 := false
	errCount := 0
	var latencies []int64

	var wg sync.WaitGroup
	sem := make(chan struct{}, 10) // max 10 concurrent

	for i := 0; i < burst; i++ {
		state.Budget.Record(ep.EndpointID, "abuse")
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			req, _ := http.NewRequestWithContext(ctx, ep.Method, url, nil)
			req.Header.Set("Accept", "application/json")
			for k, v := range state.Target.RuntimeHint.AuthHeaders {
				req.Header.Set(k, v)
			}

			t0 := time.Now()
			resp, err := client.Do(req)
			lat := time.Since(t0).Milliseconds()

			evReq := d.EvidenceRequest{Method: ep.Method, URL: url, Path: ep.Path}

			mu.Lock()
			defer mu.Unlock()

			latencies = append(latencies, lat)

			if err != nil {
				eid := state.Evidence.Add(ep.EndpointID, d.CatAbuse, []string{"abuse", "burst"}, evReq, nil, d.EvidenceTiming{LatencyMs: lat}, err.Error())
				eids = append(eids, eid)
				errCount++
				return
			}

			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			resp.Body.Close()

			evResp := &d.EvidenceResponse{Status: resp.StatusCode, BodySnippet: string(body), ContentType: resp.Header.Get("Content-Type"), Size: int64(len(body))}
			eid := state.Evidence.Add(ep.EndpointID, d.CatAbuse, []string{"abuse", "burst"}, evReq, evResp, d.EvidenceTiming{LatencyMs: lat}, "")
			eids = append(eids, eid)

			if resp.StatusCode == 429 {
				got429 = true
			}
			if resp.StatusCode >= 500 {
				errCount++
			}
		}()
	}

	wg.Wait()

	degraded := false
	if len(latencies) > 5 {
		avgFirst := avg(latencies[:5])
		avgLast := avg(latencies[len(latencies)-5:])
		if avgLast > avgFirst*3 {
			degraded = true
		}
	}

	return d.AbuseResult{
		EndpointID:   ep.EndpointID,
		TestType:     "rate_limit",
		BurstSize:    burst,
		Got429:       got429,
		Degraded:     degraded,
		Crashed:      errCount > burst/2,
		EvidenceRefs: limitRefs(eids, 5),
	}
}

func (s *Step) largePayloadTest(ctx context.Context, state *pipeline.PipelineState, ep d.EndpointEntry, url string) d.AbuseResult {
	state.Budget.Record(ep.EndpointID, "abuse")

	bigBody := `{"data":"` + strings.Repeat("X", 1024*100) + `"}`
	client := &http.Client{Timeout: 15 * time.Second}

	req, _ := http.NewRequestWithContext(ctx, ep.Method, url, bytes.NewReader([]byte(bigBody)))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range state.Target.RuntimeHint.AuthHeaders {
		req.Header.Set(k, v)
	}

	t0 := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(t0).Milliseconds()

	evReq := d.EvidenceRequest{Method: ep.Method, URL: url, Path: ep.Path, ContentType: "application/json", Size: int64(len(bigBody))}

	if err != nil {
		eid := state.Evidence.Add(ep.EndpointID, d.CatAbuse, []string{"abuse", "large_payload"}, evReq, nil, d.EvidenceTiming{LatencyMs: latency}, err.Error())
		return d.AbuseResult{EndpointID: ep.EndpointID, TestType: "large_payload", Crashed: true, EvidenceRefs: []string{eid}}
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	resp.Body.Close()

	evResp := &d.EvidenceResponse{Status: resp.StatusCode, BodySnippet: string(body), ContentType: resp.Header.Get("Content-Type"), Size: int64(len(body))}
	eid := state.Evidence.Add(ep.EndpointID, d.CatAbuse, []string{"abuse", "large_payload"}, evReq, evResp, d.EvidenceTiming{LatencyMs: latency}, "")

	crashed := resp.StatusCode >= 500
	if crashed {
		state.ErrSigBuilder.Observe(resp.StatusCode, resp.Header.Get("Content-Type"), string(body), ep.EndpointID, eid)
		state.AddFindings(d.FindingRaw{
			FindingID:      d.FindingID(string(d.TaxRobLargePayloadCrash), ep.EndpointID, []string{eid}),
			TaxonomyID:     d.TaxRobLargePayloadCrash,
			Severity:       d.SeverityHigh,
			Category:       d.FindCatRobust,
			EndpointID:     ep.EndpointID,
			Title:          fmt.Sprintf("Server crash (%d) with large payload on %s %s", resp.StatusCode, ep.Method, ep.Path),
			EvidenceRefs:   []string{eid},
			Confidence:     0.8,
			Classification: d.ClassCandidate,
		})
	}

	return d.AbuseResult{
		EndpointID:   ep.EndpointID,
		TestType:     "large_payload",
		Crashed:      crashed,
		EvidenceRefs: []string{eid},
	}
}

func selectAbuseTargets(endpoints []d.EndpointEntry, max int) []d.EndpointEntry {
	if len(endpoints) <= max {
		return endpoints
	}
	// Prioritize write endpoints, then reads
	var writes, reads []d.EndpointEntry
	for _, ep := range endpoints {
		if ep.Method == "POST" || ep.Method == "PUT" || ep.Method == "DELETE" {
			writes = append(writes, ep)
		} else {
			reads = append(reads, ep)
		}
	}
	out := writes
	out = append(out, reads...)
	if len(out) > max {
		out = out[:max]
	}
	return out
}

func avg(vals []int64) int64 {
	if len(vals) == 0 {
		return 0
	}
	var sum int64
	for _, v := range vals {
		sum += v
	}
	return sum / int64(len(vals))
}

func limitRefs(refs []string, max int) []string {
	if len(refs) <= max {
		return refs
	}
	return refs[:max]
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

func ms(start time.Time) int64 { return time.Since(start).Milliseconds() }
