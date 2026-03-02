package confirm

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

func (s *Step) Name() d.PipelineStep { return d.StepConfirm }

func (s *Step) Run(ctx context.Context, state *pipeline.PipelineState) d.StepResult {
	start := time.Now()

	if len(state.FindingsRaw) == 0 {
		return d.StepResult{Step: d.StepConfirm, Status: "skipped", DurationMs: ms(start)}
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
	var confirmations []d.Confirmation

	sampleMap := make(map[string]d.EvidenceSample)
	for _, s := range state.Evidence.Samples() {
		sampleMap[s.EvidenceID] = s
	}

	for i := range state.FindingsRaw {
		f := &state.FindingsRaw[i]
		if ctx.Err() != nil || !state.Budget.CanRequest(f.EndpointID, "confirm") {
			break
		}

		if len(f.EvidenceRefs) == 0 {
			continue
		}

		origEID := f.EvidenceRefs[0]
		origSample, ok := sampleMap[origEID]
		if !ok {
			continue
		}

		// Replay: exact same request
		replayStatus, replayBody, replayEID := s.replay(ctx, client, state, origSample, baseURL, "confirm_replay")
		budgetUsed++

		// Variation: slightly modified request
		var variationEID string
		if state.Budget.CanRequest(f.EndpointID, "confirm") {
			_, _, variationEID = s.replayVariation(ctx, client, state, origSample, baseURL)
			budgetUsed++
		}

		conf := d.Confirmation{
			FindingID:        f.FindingID,
			OriginalEvidence: origEID,
			ReplayEvidence:   replayEID,
		}
		if variationEID != "" {
			conf.VariationEvidence = variationEID
		}

		if origSample.Response != nil && replayStatus == origSample.Response.Status {
			if isSimilarBody(origSample.Response.BodySnippet, replayBody) {
				conf.Classification = d.ClassConfirmed
				conf.Notes = "Replay produjo mismo status y body similar."
				f.Classification = d.ClassConfirmed
				f.Confidence = clamp(f.Confidence+0.2, 0, 1)
			} else {
				conf.Classification = d.ClassAnomaly
				conf.Notes = "Replay produjo mismo status pero body diferente."
				f.Classification = d.ClassAnomaly
				f.Confidence = clamp(f.Confidence-0.2, 0, 1)
			}
		} else {
			conf.Classification = d.ClassInconclusive
			conf.Notes = fmt.Sprintf("Replay produjo status %d vs original %d.", replayStatus, safeStatus(origSample))
			f.Classification = d.ClassInconclusive
			f.Confidence = clamp(f.Confidence-0.3, 0, 1)
		}

		confirmations = append(confirmations, conf)
		f.EvidenceRefs = append(f.EvidenceRefs, replayEID)
		if variationEID != "" {
			f.EvidenceRefs = append(f.EvidenceRefs, variationEID)
		}

		state.Emit(pipeline.ProgressEvent{
			Step:    d.StepConfirm,
			Phase:   "exec",
			Message: fmt.Sprintf("[Confirm] %s → %s", f.Title, conf.Classification),
			Current: len(confirmations),
			Total:   len(state.FindingsRaw),
		})
	}

	state.Confirmations = confirmations

	confirmed := 0
	anomalies := 0
	for _, c := range confirmations {
		switch c.Classification {
		case d.ClassConfirmed:
			confirmed++
		case d.ClassAnomaly:
			anomalies++
		}
	}

	state.Emit(pipeline.ProgressEvent{
		Step:    d.StepConfirm,
		Phase:   "results",
		Message: fmt.Sprintf("Confirmations: %d confirmed, %d anomalies, %d inconclusive", confirmed, anomalies, len(confirmations)-confirmed-anomalies),
	})

	return d.StepResult{Step: d.StepConfirm, Status: "ok", DurationMs: ms(start), BudgetUsed: budgetUsed}
}

func (s *Step) replay(ctx context.Context, client *http.Client, state *pipeline.PipelineState, orig d.EvidenceSample, baseURL, tag string) (int, string, string) {
	state.Budget.Record(orig.EndpointID, "confirm")

	var bodyReader io.Reader
	if orig.Request.Body != "" {
		bodyReader = bytes.NewReader([]byte(orig.Request.Body))
	}

	req, err := http.NewRequestWithContext(ctx, orig.Request.Method, orig.Request.URL, bodyReader)
	if err != nil {
		eid := state.Evidence.Add(orig.EndpointID, d.CatConfirm, []string{"confirm", tag}, orig.Request, nil, d.EvidenceTiming{}, err.Error())
		return 0, "", eid
	}

	for k, v := range orig.Request.Headers {
		req.Header.Set(k, v)
	}

	t0 := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(t0).Milliseconds()

	if err != nil {
		eid := state.Evidence.Add(orig.EndpointID, d.CatConfirm, []string{"confirm", tag}, orig.Request, nil, d.EvidenceTiming{LatencyMs: latency}, err.Error())
		return 0, "", eid
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

	eid := state.Evidence.Add(orig.EndpointID, d.CatConfirm, []string{"confirm", tag}, orig.Request, evResp, d.EvidenceTiming{LatencyMs: latency}, "")
	return resp.StatusCode, string(body), eid
}

func (s *Step) replayVariation(ctx context.Context, client *http.Client, state *pipeline.PipelineState, orig d.EvidenceSample, baseURL string) (int, string, string) {
	// Slight variation: change a header
	modReq := orig.Request
	if modReq.Headers == nil {
		modReq.Headers = make(map[string]string)
	}
	modReq.Headers["X-Toollab-Variation"] = "true"

	// Modify body slightly if present
	if modReq.Body != "" {
		modReq.Body = strings.Replace(modReq.Body, "test", "test2", 1)
	}

	state.Budget.Record(orig.EndpointID, "confirm")

	var bodyReader io.Reader
	if modReq.Body != "" {
		bodyReader = bytes.NewReader([]byte(modReq.Body))
	}

	req, err := http.NewRequestWithContext(ctx, modReq.Method, modReq.URL, bodyReader)
	if err != nil {
		eid := state.Evidence.Add(orig.EndpointID, d.CatConfirm, []string{"confirm", "variation"}, modReq, nil, d.EvidenceTiming{}, err.Error())
		return 0, "", eid
	}

	for k, v := range modReq.Headers {
		req.Header.Set(k, v)
	}

	t0 := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(t0).Milliseconds()

	if err != nil {
		eid := state.Evidence.Add(orig.EndpointID, d.CatConfirm, []string{"confirm", "variation"}, modReq, nil, d.EvidenceTiming{LatencyMs: latency}, err.Error())
		return 0, "", eid
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

	eid := state.Evidence.Add(orig.EndpointID, d.CatConfirm, []string{"confirm", "variation"}, modReq, evResp, d.EvidenceTiming{LatencyMs: latency}, "")
	return resp.StatusCode, string(body), eid
}

func isSimilarBody(a, b string) bool {
	if a == b {
		return true
	}
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	// Simple similarity: same first 100 chars or length within 20%
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	if float64(minLen)/float64(maxLen) > 0.8 {
		return true
	}
	return false
}

func safeStatus(s d.EvidenceSample) int {
	if s.Response != nil {
		return s.Response.Status
	}
	return 0
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func headerMap(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k := range h {
		out[k] = h.Get(k)
	}
	return out
}

func ms(start time.Time) int64 { return time.Since(start).Milliseconds() }
