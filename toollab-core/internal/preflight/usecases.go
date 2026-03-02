package preflight

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	d "toollab-core/internal/pipeline/usecases/domain"
	"toollab-core/internal/pipeline"
	"toollab-core/internal/shared"
)

type Step struct{}

func New() *Step { return &Step{} }

func (s *Step) Name() d.PipelineStep { return d.StepPreflight }

func (s *Step) Run(ctx context.Context, state *pipeline.PipelineState) d.StepResult {
	start := time.Now()

	baseURL := state.Target.RuntimeHint.BaseURL
	if baseURL == "" {
		return d.StepResult{
			Step:       d.StepPreflight,
			Status:     "failed",
			DurationMs: ms(start),
			Error:      "target has no base_url configured",
		}
	}

	baseURL = shared.RewriteHost(baseURL)

	profile := &d.TargetProfile{
		SchemaVersion: "v2",
		BaseURL:       strings.TrimRight(baseURL, "/"),
		TimeoutDefaults: d.TimeoutDefaults{
			ConnectMs: 5000,
			ReadMs:    10000,
		},
	}

	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 0 {
			profile.Redirects = append(profile.Redirects, req.URL.String())
		}
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	}}

	budgetUsed := 0

	probeURLs := []string{baseURL, baseURL + "/", baseURL + "/healthz", baseURL + "/health", baseURL + "/api", baseURL + "/api/v1"}
	for _, u := range probeURLs {
		if !state.Budget.CanRequest("_preflight", "preflight") {
			break
		}

		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Accept", "application/json, text/html, */*")
		req.Header.Set("Accept-Encoding", "gzip, deflate")

		t0 := time.Now()
		resp, err := client.Do(req)
		latency := time.Since(t0).Milliseconds()
		state.Budget.Record("_preflight", "preflight")
		budgetUsed++

		if err != nil {
			state.Evidence.Add("_preflight", d.CatPreflight, []string{"preflight", "probe"},
				d.EvidenceRequest{Method: "GET", URL: u, Path: extractPath(u)},
				nil, d.EvidenceTiming{LatencyMs: latency}, err.Error())
			continue
		}

		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()

		ct := resp.Header.Get("Content-Type")
		if ct != "" && !containsStr(profile.ObservedContentTypes.Produces, ct) {
			profile.ObservedContentTypes.Produces = append(profile.ObservedContentTypes.Produces, ct)
		}
		if strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") {
			profile.SupportsGzip = true
		}
		if resp.Header.Get("Set-Cookie") != "" {
			profile.SupportsCookies = true
		}

		for _, h := range []string{"Www-Authenticate", "X-Api-Key", "Authorization"} {
			if v := resp.Header.Get(h); v != "" {
				profile.AuthHints.HeadersSeen = appendUnique(profile.AuthHints.HeadersSeen, h)
			}
		}

		path := extractPath(u)
		if resp.StatusCode < 400 && path != "/" && path != "" {
			profile.BasePaths = appendUnique(profile.BasePaths, path)
		}

		if resp.StatusCode == 200 && (strings.HasSuffix(u, "/healthz") || strings.HasSuffix(u, "/health")) {
			profile.HealthEndpoints = appendUnique(profile.HealthEndpoints, extractPath(u))
		}

		if strings.Contains(path, "/v1") || strings.Contains(path, "/v2") || strings.Contains(path, "/v3") {
			parts := strings.Split(path, "/")
			for _, p := range parts {
				if len(p) >= 2 && p[0] == 'v' && p[1] >= '0' && p[1] <= '9' {
					profile.VersioningHint = p
					break
				}
			}
		}

		evResp := &d.EvidenceResponse{
			Status:      resp.StatusCode,
			Headers:     headerMap(resp.Header),
			BodySnippet: string(body),
			ContentType: ct,
			Size:        int64(len(body)),
		}
		state.Evidence.Add("_preflight", d.CatPreflight, []string{"preflight", "probe"},
			d.EvidenceRequest{Method: "GET", URL: u, Path: path},
			evResp, d.EvidenceTiming{LatencyMs: latency}, "")
	}

	if len(state.Target.RuntimeHint.AuthHeaders) > 0 {
		for k := range state.Target.RuntimeHint.AuthHeaders {
			profile.AuthHints.HeadersSeen = appendUnique(profile.AuthHints.HeadersSeen, k)
			if strings.ToLower(k) == "authorization" {
				profile.AuthHints.Mechanisms = appendUnique(profile.AuthHints.Mechanisms, "bearer")
			}
		}
	}

	state.TargetProfile = profile

	return d.StepResult{
		Step:       d.StepPreflight,
		Status:     "ok",
		DurationMs: ms(start),
		BudgetUsed: budgetUsed,
	}
}

func ms(start time.Time) int64 { return time.Since(start).Milliseconds() }

func extractPath(u string) string {
	idx := strings.Index(u, "://")
	if idx >= 0 {
		u = u[idx+3:]
	}
	idx = strings.Index(u, "/")
	if idx >= 0 {
		return u[idx:]
	}
	return "/"
}

func headerMap(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k := range h {
		out[k] = h.Get(k)
	}
	return out
}

func appendUnique(ss []string, s string) []string {
	for _, v := range ss {
		if v == s {
			return ss
		}
	}
	return append(ss, s)
}

func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
