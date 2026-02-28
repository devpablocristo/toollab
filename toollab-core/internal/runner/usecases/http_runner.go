package usecases

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	evidenceDomain "toollab-core/internal/evidence/usecases/domain"
	"toollab-core/internal/runner/usecases/domain"
	scenarioDomain "toollab-core/internal/scenario/usecases/domain"
	"toollab-core/internal/shared"
)

var blockedSchemes = map[string]bool{
	"file":   true,
	"gopher": true,
	"ftp":    true,
	"data":   true,
}

type HTTPRunner struct{}

func NewHTTPRunner() *HTTPRunner { return &HTTPRunner{} }

type CaseProgressFn func(idx int, c scenarioDomain.ScenarioCase, cr evidenceDomain.CaseResult)

func (r *HTTPRunner) Run(baseURL string, plan scenarioDomain.ScenarioPlan, opts domain.Options) evidenceDomain.ExecutionResult {
	return r.RunWithProgress(baseURL, plan, opts, nil)
}

func (r *HTTPRunner) RunWithProgress(baseURL string, plan scenarioDomain.ScenarioPlan, opts domain.Options, onCase CaseProgressFn) evidenceDomain.ExecutionResult {
	if opts.TimeoutMs <= 0 {
		opts.TimeoutMs = 10000
	}
	if opts.MaxBodyBytes <= 0 {
		opts.MaxBodyBytes = 1024 * 1024
	}

	subsetSet := toSet(opts.SubsetIDs)
	tagSet := toSet(opts.Tags)

	start := shared.Now()
	var results []evidenceDomain.CaseResult
	idx := 0

	for _, c := range plan.Cases {
		if !c.Enabled {
			continue
		}
		if len(subsetSet) > 0 && !subsetSet[c.CaseID] {
			continue
		}
		if len(tagSet) > 0 && !hasAnyTag(c.Tags, tagSet) {
			continue
		}
		cr := r.executeCase(baseURL, c, opts)
		results = append(results, cr)
		if onCase != nil {
			onCase(idx, c, cr)
		}
		idx++
	}

	return evidenceDomain.ExecutionResult{
		StartedAt:  start,
		FinishedAt: shared.Now(),
		Cases:      results,
	}
}

func (r *HTTPRunner) executeCase(baseURL string, c scenarioDomain.ScenarioCase, opts domain.Options) evidenceDomain.CaseResult {
	evidenceID := shared.NewID()
	cr := evidenceDomain.CaseResult{
		CaseID:     c.CaseID,
		EvidenceID: evidenceID,
		Tags:       c.Tags,
	}

	fullURL, err := buildURL(baseURL, c.Request)
	if err != nil {
		cr.Error = err.Error()
		return cr
	}

	if err := validateScheme(fullURL); err != nil {
		cr.Error = err.Error()
		return cr
	}

	var bodyReader io.Reader
	var bodySize int64
	if len(c.Request.BodyJSON) > 0 {
		bodyReader = bytes.NewReader(c.Request.BodyJSON)
		bodySize = int64(len(c.Request.BodyJSON))
	}

	req, err := http.NewRequest(c.Request.Method, fullURL, bodyReader)
	if err != nil {
		cr.Error = fmt.Sprintf("building request: %v", err)
		return cr
	}

	skipAuth := c.Auth != nil && c.Auth.Mode == "none"
	if !skipAuth {
		for k, v := range opts.AuthHeaders {
			req.Header.Set(k, v)
		}
	}
	for k, v := range c.Request.Headers {
		req.Header.Set(k, v)
	}
	if bodySize > 0 && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if !skipAuth {
		applyAuth(req, c.Auth)
	}

	var reqBodyRaw []byte
	if len(c.Request.BodyJSON) > 0 {
		reqBodyRaw = make([]byte, len(c.Request.BodyJSON))
		copy(reqBodyRaw, c.Request.BodyJSON)
	}

	cr.ReqFinal = evidenceDomain.CaseResultReq{
		Method:   req.Method,
		URL:      req.URL.String(),
		Headers:  headerMap(req.Header),
		BodySize: bodySize,
		BodyRaw:  reqBodyRaw,
	}

	client := &http.Client{Timeout: time.Duration(opts.TimeoutMs) * time.Millisecond}
	t0 := time.Now()
	resp, err := client.Do(req)
	cr.TimingMs = time.Since(t0).Milliseconds()
	if err != nil {
		cr.Error = err.Error()
		return cr
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, opts.MaxBodyBytes))
	cr.Response = &evidenceDomain.CaseResultResp{
		Status:   resp.StatusCode,
		Headers:  headerMap(resp.Header),
		BodySize: int64(len(respBody)),
		BodyRaw:  respBody,
	}
	return cr
}

func buildURL(baseURL string, req scenarioDomain.CaseRequest) (string, error) {
	path := req.Path
	for k, v := range req.PathParams {
		path = strings.ReplaceAll(path, "{"+k+"}", v)
	}
	u, err := url.Parse(strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/"))
	if err != nil {
		return "", err
	}
	if len(req.Query) > 0 {
		q := u.Query()
		for k, v := range req.Query {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
}

func validateScheme(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if blockedSchemes[u.Scheme] {
		return fmt.Errorf("blocked scheme: %s", u.Scheme)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}
	return nil
}

func applyAuth(req *http.Request, auth *scenarioDomain.CaseAuth) {
	if auth == nil || auth.Mode == "none" || auth.Mode == "" {
		return
	}
	switch auth.Mode {
	case "bearer_token":
		req.Header.Set("Authorization", "Bearer "+auth.Token)
	case "api_key":
		name := auth.HeaderName
		if name == "" {
			name = "X-Api-Key"
		}
		req.Header.Set(name, auth.Value)
	}
}

func headerMap(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k := range h {
		out[k] = h.Get(k)
	}
	return out
}

func toSet(items []string) map[string]bool {
	if len(items) == 0 {
		return nil
	}
	s := make(map[string]bool, len(items))
	for _, it := range items {
		s[it] = true
	}
	return s
}

func hasAnyTag(tags []string, tagSet map[string]bool) bool {
	for _, t := range tags {
		if tagSet[t] {
			return true
		}
	}
	return false
}
