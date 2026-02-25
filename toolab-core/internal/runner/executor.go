package runner

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"toolab-core/internal/chaos"
	"toolab-core/internal/scenario"
	"toolab-core/pkg/utils"
)

type BaseExecutor struct {
	Scenario    *scenario.Scenario
	Plan        *Plan
	ChaosEngine *chaos.Engine
	Client      *http.Client
}

func NewBaseExecutor(s *scenario.Scenario, p *Plan, chaosEngine *chaos.Engine) *BaseExecutor {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
	}
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return &BaseExecutor{Scenario: s, Plan: p, ChaosEngine: chaosEngine, Client: client}
}

func (e *BaseExecutor) Execute(ctx context.Context) ([]Outcome, error) {
	if e.Scenario == nil || e.Plan == nil {
		return nil, fmt.Errorf("executor requires scenario and plan")
	}
	concurrency := e.Plan.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}

	jobs := make(chan PlannedRequest)
	results := make(chan Outcome, len(e.Plan.PlannedRequests))
	workerCount := concurrency
	if workerCount > len(e.Plan.PlannedRequests) {
		workerCount = len(e.Plan.PlannedRequests)
	}
	if workerCount == 0 {
		return []Outcome{}, nil
	}

	var wg sync.WaitGroup
	wg.Add(workerCount)
	for i := 0; i < workerCount; i++ {
		go func() {
			defer wg.Done()
			for p := range jobs {
				out := e.executeOne(ctx, p)
				results <- out
			}
		}()
	}

	for _, p := range e.Plan.PlannedRequests {
		jobs <- p
	}
	close(jobs)
	wg.Wait()
	close(results)

	outcomes := make([]Outcome, 0, len(e.Plan.PlannedRequests))
	for out := range results {
		outcomes = append(outcomes, out)
	}
	sort.Slice(outcomes, func(i, j int) bool { return outcomes[i].Seq < outcomes[j].Seq })
	return outcomes, nil
}

func (e *BaseExecutor) executeOne(ctx context.Context, p PlannedRequest) Outcome {
	reqSpec := e.Scenario.Workload.Requests[p.RequestIdx]
	bodyBytes := requestBody(reqSpec)
	chaosDecision := chaos.Decision{DriftMutations: []string{}}
	if e.ChaosEngine != nil {
		chaosDecision, bodyBytes = e.ChaosEngine.Apply(reqSpec, p.Seq, bodyBytes)
	}
	requestURL := buildURL(e.Scenario.Target.BaseURL, reqSpec.Path, reqSpec.Query)
	requestHeaders := mergeHeaders(e.Scenario.Target.Headers, reqSpec.Headers)

	idempotencyKey := ""
	if reqSpec.IdempotencyKey != "" {
		idempotencyKey = strings.ReplaceAll(reqSpec.IdempotencyKey, "{{request_id}}", reqSpec.ID)
		idempotencyKey = strings.ReplaceAll(idempotencyKey, "{{request_seq}}", fmt.Sprintf("%d", p.Seq))
		requestHeaders["Idempotency-Key"] = idempotencyKey
	}

	applyAuth(requestHeaders, e.Scenario.Target.Auth, &requestURL)

	start := time.Now()
	if chaosDecision.LatencyMS > 0 {
		time.Sleep(time.Duration(chaosDecision.LatencyMS) * time.Millisecond)
	}
	out := Outcome{
		Seq:             p.Seq,
		RequestID:       reqSpec.ID,
		Method:          reqSpec.Method,
		Path:            reqSpec.Path,
		ErrorKind:       "none",
		RequestURL:      requestURL,
		RequestHeaders:  requestHeaders,
		RequestBody:     bodyBytes,
		ResponseHash:    utils.SHA256Hex([]byte{}),
		ResponseHeaders: map[string]string{},
		IdempotencyKey:  idempotencyKey,
		ChaosApplied: ChaosApplied{
			LatencyInjectedMS: chaosDecision.LatencyMS,
			ErrorInjected:     chaosDecision.ErrorInjected,
			ErrorMode:         "abort",
			PayloadDrift:      chaosDecision.DriftApplied,
			PayloadMutations:  append([]string(nil), chaosDecision.DriftMutations...),
		},
	}

	if chaosDecision.Abort {
		out.ErrorKind = "chaos_abort"
		out.LatencyMS = int(time.Since(start).Milliseconds())
		out.OccurredAt = time.Now().UTC()
		return out
	}

	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(reqSpec.TimeoutMS)*time.Millisecond)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(reqCtx, reqSpec.Method, requestURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		out.ErrorKind = "conn"
		out.LatencyMS = int(time.Since(start).Milliseconds())
		out.OccurredAt = time.Now().UTC()
		return out
	}
	for k, v := range requestHeaders {
		httpReq.Header.Set(k, v)
	}

	resp, err := e.Client.Do(httpReq)
	if err != nil {
		out.ErrorKind = classifyError(err)
		out.LatencyMS = int(time.Since(start).Milliseconds())
		out.OccurredAt = time.Now().UTC()
		return out
	}
	defer resp.Body.Close()

	statusCode := resp.StatusCode
	out.StatusCode = &statusCode
	if resp.StatusCode >= 400 {
		out.ErrorKind = "http_status_error"
	}
	respBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		out.ErrorKind = "read"
		out.ResponseHash = utils.SHA256Hex([]byte{})
	} else {
		out.ResponseBody = respBytes
		out.ResponseHash = utils.SHA256Hex(respBytes)
	}
	for k, vals := range resp.Header {
		if len(vals) > 0 {
			out.ResponseHeaders[k] = vals[0]
		}
	}
	out.LatencyMS = int(time.Since(start).Milliseconds())
	out.OccurredAt = time.Now().UTC()
	return out
}

func requestBody(req scenario.RequestSpec) []byte {
	if req.Body != nil {
		return []byte(*req.Body)
	}
	if len(req.JSONBody) > 0 {
		var v any
		if err := json.Unmarshal(req.JSONBody, &v); err == nil {
			if canonical, err := utils.CanonicalJSON(v); err == nil {
				return canonical
			}
		}
		return req.JSONBody
	}
	return []byte{}
}

func buildURL(baseURL, path string, query map[string]string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return baseURL + path
	}
	u.Path = strings.TrimRight(u.Path, "/") + path
	if len(query) > 0 {
		q := u.Query()
		for k, v := range query {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
	}
	return u.String()
}

func mergeHeaders(base, req map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range req {
		out[k] = v
	}
	return out
}

func applyAuth(headers map[string]string, auth scenario.Auth, requestURL *string) {
	switch auth.Type {
	case "bearer":
		token := os.Getenv(auth.BearerTokenEnv)
		if token != "" {
			headers["Authorization"] = "Bearer " + token
		}
	case "basic":
		u := os.Getenv(auth.UsernameEnv)
		p := os.Getenv(auth.PasswordEnv)
		if u != "" || p != "" {
			headers["Authorization"] = "Basic " + basicToken(u, p)
		}
	case "api_key":
		key := os.Getenv(auth.APIKeyEnv)
		if key == "" {
			return
		}
		if auth.In == "header" {
			headers[auth.Name] = key
			return
		}
		parsed, err := url.Parse(*requestURL)
		if err != nil {
			return
		}
		q := parsed.Query()
		q.Set(auth.Name, key)
		parsed.RawQuery = q.Encode()
		*requestURL = parsed.String()
	}
}

func basicToken(user, pass string) string {
	return base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
}

func classifyError(err error) string {
	if err == nil {
		return "none"
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "context deadline exceeded") {
		return "timeout"
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return "dns"
	}
	if strings.Contains(msg, "tls") {
		return "tls"
	}
	if strings.Contains(msg, "dial") || strings.Contains(msg, "connect") {
		return "conn"
	}
	return "read"
}
