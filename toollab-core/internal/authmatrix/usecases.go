package authmatrix

import (
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

func (s *Step) Name() d.PipelineStep { return d.StepAuth }

func (s *Step) Run(ctx context.Context, state *pipeline.PipelineState) d.StepResult {
	start := time.Now()

	if state.Catalog == nil || len(state.Catalog.Endpoints) == 0 {
		return d.StepResult{Step: d.StepAuth, Status: "skipped", DurationMs: ms(start)}
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
	var entries []d.AuthMatrixEntry
	var discrepancies []d.AuthDiscrepancy

	for _, ep := range state.Catalog.Endpoints {
		if ctx.Err() != nil {
			break
		}
		if !state.Budget.CanRequest(ep.EndpointID, "auth") {
			break
		}

		path := replaceParams(ep.Path)
		fullURL := strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")

		entry := d.AuthMatrixEntry{
			EndpointID: ep.EndpointID,
			Method:     ep.Method,
			Path:       ep.Path,
			NoAuth:     d.AuthUnknown,
			InvalidAuth: d.AuthUnknown,
		}

		// Test (a) no auth
		noAuthStatus, noAuthEID := s.probe(ctx, client, state, ep, fullURL, nil, "noauth")
		budgetUsed++

		// Test (b) invalid auth
		invalidHeaders := map[string]string{"Authorization": "Bearer invalid_token_garbage_12345"}
		invalidStatus, invalidEID := s.probe(ctx, client, state, ep, fullURL, invalidHeaders, "auth_invalid")
		budgetUsed++

		entry.EvidenceRefs = []string{noAuthEID, invalidEID}

		if noAuthStatus >= 200 && noAuthStatus < 400 {
			entry.NoAuth = d.AuthAllowed
		} else if noAuthStatus == 401 || noAuthStatus == 403 {
			entry.NoAuth = d.AuthDenied
		}

		if invalidStatus >= 200 && invalidStatus < 400 {
			entry.InvalidAuth = d.AuthAllowed
		} else if invalidStatus == 401 || invalidStatus == 403 {
			entry.InvalidAuth = d.AuthDenied
		}

		// Test (c) valid auth if available
		if len(state.Target.RuntimeHint.AuthHeaders) > 0 {
			if state.Budget.CanRequest(ep.EndpointID, "auth") {
				validStatus, validEID := s.probe(ctx, client, state, ep, fullURL, state.Target.RuntimeHint.AuthHeaders, "auth_valid")
				budgetUsed++
				entry.EvidenceRefs = append(entry.EvidenceRefs, validEID)

				if validStatus >= 200 && validStatus < 400 {
					entry.ValidAuth = d.AuthAllowed
				} else if validStatus == 401 || validStatus == 403 {
					entry.ValidAuth = d.AuthDenied
				}
			}
		}

		// Detect AST↔runtime discrepancies
		hasAuthMiddleware := false
		for _, mw := range ep.Middlewares {
			name := strings.ToLower(mw.Label)
			if strings.Contains(name, "auth") || strings.Contains(name, "jwt") {
				hasAuthMiddleware = true
				break
			}
		}

		if hasAuthMiddleware && entry.NoAuth == d.AuthAllowed {
			disc := d.AuthDiscrepancy{
				EndpointID:  ep.EndpointID,
				Method:      ep.Method,
				Path:        ep.Path,
				Description: fmt.Sprintf("AST muestra auth middleware pero runtime permite acceso sin auth a %s %s", ep.Method, ep.Path),
				ASTSays:     "requires_auth (middleware detected)",
				RuntimeSays: "allowed_without_auth",
				Risk:        "critical",
				EvidenceRefs: entry.EvidenceRefs,
				ASTRefs:     ep.Middlewares,
			}
			discrepancies = append(discrepancies, disc)

			state.AddFindings(d.FindingRaw{
				FindingID:      d.FindingID(string(d.TaxAuthASTDiscrepancy), ep.EndpointID, entry.EvidenceRefs),
				TaxonomyID:     d.TaxAuthASTDiscrepancy,
				Severity:       d.SeverityCritical,
				Category:       d.FindCatAuth,
				EndpointID:     ep.EndpointID,
				Title:          fmt.Sprintf("Auth bypass: %s %s accessible without auth despite auth middleware in AST", ep.Method, ep.Path),
				Description:    disc.Description,
				EvidenceRefs:   entry.EvidenceRefs,
				ASTRefs:        ep.Middlewares,
				Confidence:     0.9,
				Classification: d.ClassCandidate,
			})
		}

		if !hasAuthMiddleware && entry.NoAuth == d.AuthDenied {
			discrepancies = append(discrepancies, d.AuthDiscrepancy{
				EndpointID:  ep.EndpointID,
				Method:      ep.Method,
				Path:        ep.Path,
				Description: fmt.Sprintf("No auth middleware en AST pero runtime deniega acceso a %s %s", ep.Method, ep.Path),
				ASTSays:     "no_auth_middleware",
				RuntimeSays: "denied",
				Risk:        "info",
				EvidenceRefs: entry.EvidenceRefs,
			})
		}

		entries = append(entries, entry)

		state.Emit(pipeline.ProgressEvent{
			Step:    d.StepAuth,
			Phase:   "exec",
			Message: fmt.Sprintf("[Auth] %s %s → noauth=%s invalid=%s", ep.Method, ep.Path, entry.NoAuth, entry.InvalidAuth),
			Current: len(entries),
			Total:   len(state.Catalog.Endpoints),
		})
	}

	state.AuthMatrix = &d.AuthMatrix{
		SchemaVersion: "v2",
		Entries:       entries,
		Discrepancies: discrepancies,
	}

	state.Emit(pipeline.ProgressEvent{
		Step:    d.StepAuth,
		Phase:   "results",
		Message: fmt.Sprintf("Auth matrix: %d entries, %d discrepancies", len(entries), len(discrepancies)),
	})

	return d.StepResult{
		Step:       d.StepAuth,
		Status:     "ok",
		DurationMs: ms(start),
		BudgetUsed: budgetUsed,
	}
}

func (s *Step) probe(ctx context.Context, client *http.Client, state *pipeline.PipelineState, ep d.EndpointEntry, fullURL string, headers map[string]string, tag string) (int, string) {
	state.Budget.Record(ep.EndpointID, "auth")

	req, err := http.NewRequestWithContext(ctx, ep.Method, fullURL, nil)
	if err != nil {
		eid := state.Evidence.Add(ep.EndpointID, d.CatAuth, []string{"auth", tag},
			d.EvidenceRequest{Method: ep.Method, URL: fullURL, Path: ep.Path},
			nil, d.EvidenceTiming{}, err.Error())
		return 0, eid
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	t0 := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(t0).Milliseconds()

	evReq := d.EvidenceRequest{
		Method:  ep.Method,
		URL:     fullURL,
		Path:    ep.Path,
		Headers: headerMap(req.Header),
	}

	if err != nil {
		eid := state.Evidence.Add(ep.EndpointID, d.CatAuth, []string{"auth", tag},
			evReq, nil, d.EvidenceTiming{LatencyMs: latency}, err.Error())
		return 0, eid
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	resp.Body.Close()

	evResp := &d.EvidenceResponse{
		Status:      resp.StatusCode,
		Headers:     headerMap(resp.Header),
		BodySnippet: string(body),
		ContentType: resp.Header.Get("Content-Type"),
		Size:        int64(len(body)),
	}

	eid := state.Evidence.Add(ep.EndpointID, d.CatAuth, []string{"auth", tag},
		evReq, evResp, d.EvidenceTiming{LatencyMs: latency}, "")

	return resp.StatusCode, eid
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
