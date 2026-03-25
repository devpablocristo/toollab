package playground

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/devpablocristo/core/http/go/httpjson"

	artifactUC "toollab-core/internal/artifact"
	artDomain "toollab-core/internal/artifact/usecases/domain"
	d "toollab-core/internal/pipeline/usecases/domain"
)

type Handler struct {
	artifactSvc *artifactUC.Service
	authStore   *AuthStore
	httpClient  *http.Client
}

func NewHandler(artifactSvc *artifactUC.Service) *Handler {
	return &Handler{
		artifactSvc: artifactSvc,
		authStore:   NewAuthStore(),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
	}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/send", h.send)
	r.Post("/replay", h.replay)
	r.Get("/auth-profiles", h.listAuthProfiles)
	r.Post("/auth-profiles", h.createAuthProfile)
	r.Delete("/auth-profiles/{profile_id}", h.deleteAuthProfile)
	return r
}

func (h *Handler) send(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "run_id")

	var req d.PlaygroundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body: "+err.Error())
		return
	}

	allowedHost, err := h.getAllowedHost(runID)
	if err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL", "cannot determine allowed host")
		return
	}

	if err := validateSSRF(req.URL, allowedHost); err != nil {
		httpjson.WriteError(w, http.StatusForbidden, "FORBIDDEN", "SSRF blocked: "+err.Error())
		return
	}

	if req.AuthProfileID != "" {
		profile, ok := h.authStore.Get(runID, req.AuthProfileID)
		if ok {
			if req.Headers == nil {
				req.Headers = make(map[string]string)
			}
			switch profile.Mechanism {
			case "bearer":
				req.Headers["Authorization"] = "Bearer " + profile.Value
			case "api_key":
				key := profile.HeaderKey
				if key == "" {
					key = "X-API-Key"
				}
				req.Headers[key] = profile.Value
			case "cookie":
				req.Headers["Cookie"] = profile.Value
			}
		}
	}

	timeout := 10 * time.Second
	if req.TimeoutMs > 0 && req.TimeoutMs <= 30000 {
		timeout = time.Duration(req.TimeoutMs) * time.Millisecond
	}

	start := time.Now()
	resp, evidenceSample, err := h.executeRequest(req, timeout, runID)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		pgResp := d.PlaygroundResponse{
			EvidenceID: evidenceSample.EvidenceID,
			Error:      err.Error(),
			LatencyMs:  latency,
		}
		h.saveEvidence(runID, evidenceSample)
		httpjson.WriteJSON(w, http.StatusOK, pgResp)
		return
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	bodyStr := string(bodyBytes)

	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		respHeaders[k] = strings.Join(v, ", ")
	}

	evidenceSample.Response = &d.EvidenceResponse{
		Status:      resp.StatusCode,
		Headers:     respHeaders,
		BodySnippet: truncateStr(bodyStr, 8192),
		ContentType: resp.Header.Get("Content-Type"),
		Size:        int64(len(bodyBytes)),
	}
	evidenceSample.Timing.LatencyMs = latency

	h.saveEvidence(runID, evidenceSample)

	pgResp := d.PlaygroundResponse{
		EvidenceID:  evidenceSample.EvidenceID,
		Status:      resp.StatusCode,
		Headers:     respHeaders,
		Body:        bodyStr,
		BodySnippet: truncateStr(bodyStr, 2048),
		ContentType: resp.Header.Get("Content-Type"),
		LatencyMs:   latency,
		Size:        int64(len(bodyBytes)),
	}

	httpjson.WriteJSON(w, http.StatusOK, pgResp)
}

func (h *Handler) replay(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "run_id")

	var body struct {
		EvidenceID string `json:"evidence_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid body")
		return
	}

	data, _, err := h.artifactSvc.GetLatest(runID, artDomain.ArtifactRawEvidence)
	if err != nil {
		httpjson.WriteError(w, http.StatusNotFound, "NOT_FOUND", "evidence not found")
		return
	}

	var rawEvidence d.RawEvidence
	if err := json.Unmarshal(data, &rawEvidence); err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL", "internal error")
		return
	}

	var original *d.EvidenceSample
	for i := range rawEvidence.Samples {
		if rawEvidence.Samples[i].EvidenceID == body.EvidenceID {
			original = &rawEvidence.Samples[i]
			break
		}
	}

	if original == nil {
		httpjson.WriteError(w, http.StatusNotFound, "NOT_FOUND", "evidence sample not found")
		return
	}

	replayReq := d.PlaygroundRequest{
		EndpointID: original.EndpointID,
		Method:     original.Request.Method,
		URL:        original.Request.URL,
		Headers:    original.Request.Headers,
		Query:      original.Request.Query,
		Body:       original.Request.Body,
	}

	allowedHost, _ := h.getAllowedHost(runID)
	if err := validateSSRF(replayReq.URL, allowedHost); err != nil {
		httpjson.WriteError(w, http.StatusForbidden, "FORBIDDEN", "SSRF blocked: "+err.Error())
		return
	}

	start := time.Now()
	resp, evidenceSample, err := h.executeRequest(replayReq, 10*time.Second, runID)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		pgResp := d.PlaygroundResponse{
			EvidenceID: evidenceSample.EvidenceID,
			Error:      err.Error(),
			LatencyMs:  latency,
		}
		h.saveEvidence(runID, evidenceSample)
		httpjson.WriteJSON(w, http.StatusOK, pgResp)
		return
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	bodyStr := string(bodyBytes)

	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		respHeaders[k] = strings.Join(v, ", ")
	}

	evidenceSample.Response = &d.EvidenceResponse{
		Status:      resp.StatusCode,
		Headers:     respHeaders,
		BodySnippet: truncateStr(bodyStr, 8192),
		ContentType: resp.Header.Get("Content-Type"),
		Size:        int64(len(bodyBytes)),
	}
	evidenceSample.Timing.LatencyMs = latency
	evidenceSample.CorrelationIDs = []string{body.EvidenceID}

	h.saveEvidence(runID, evidenceSample)

	pgResp := d.PlaygroundResponse{
		EvidenceID:  evidenceSample.EvidenceID,
		Status:      resp.StatusCode,
		Headers:     respHeaders,
		Body:        bodyStr,
		BodySnippet: truncateStr(bodyStr, 2048),
		ContentType: resp.Header.Get("Content-Type"),
		LatencyMs:   latency,
		Size:        int64(len(bodyBytes)),
	}

	httpjson.WriteJSON(w, http.StatusOK, pgResp)
}

func (h *Handler) listAuthProfiles(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "run_id")
	profiles := h.authStore.List(runID)
	masked := make([]d.AuthProfileMasked, 0, len(profiles))
	for _, p := range profiles {
		masked = append(masked, p.ToMasked())
	}
	httpjson.WriteJSON(w, http.StatusOK, map[string]interface{}{"profiles": masked})
}

func (h *Handler) createAuthProfile(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "run_id")

	var profile d.AuthProfile
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid body")
		return
	}

	profile.ID = uuid.New().String()
	profile.RunID = runID
	profile.CreatedAt = time.Now().UTC().Format(time.RFC3339)

	h.authStore.Put(profile)
	httpjson.WriteJSON(w, http.StatusCreated, profile.ToMasked())
}

func (h *Handler) deleteAuthProfile(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "run_id")
	profileID := chi.URLParam(r, "profile_id")
	h.authStore.Delete(runID, profileID)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) executeRequest(req d.PlaygroundRequest, timeout time.Duration, runID string) (*http.Response, d.EvidenceSample, error) {
	evidenceID := d.EvidenceID(runID, req.EndpointID, d.CatManual, int(time.Now().UnixNano()%100000))
	sample := d.EvidenceSample{
		EvidenceID: evidenceID,
		EndpointID: req.EndpointID,
		Category:   d.CatManual,
		Tags:       []string{"manual_playground"},
		Request: d.EvidenceRequest{
			Method:  req.Method,
			URL:     req.URL,
			Path:    extractPath(req.URL),
			Headers: req.Headers,
			Query:   req.Query,
			Body:    req.Body,
		},
	}

	if req.Body != "" {
		sample.Request.Size = int64(len(req.Body))
		if ct, ok := req.Headers["Content-Type"]; ok {
			sample.Request.ContentType = ct
		}
	}

	httpReq, err := http.NewRequest(req.Method, req.URL, strings.NewReader(req.Body))
	if err != nil {
		sample.Error = err.Error()
		return nil, sample, err
	}

	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	q := httpReq.URL.Query()
	for k, v := range req.Query {
		q.Set(k, v)
	}
	httpReq.URL.RawQuery = q.Encode()

	client := *h.httpClient
	client.Timeout = timeout

	resp, err := client.Do(httpReq)
	if err != nil {
		sample.Error = err.Error()
		return nil, sample, err
	}

	return resp, sample, nil
}

func (h *Handler) saveEvidence(runID string, sample d.EvidenceSample) {
	data, err := json.Marshal(sample)
	if err != nil {
		return
	}
	h.artifactSvc.Put(runID, artDomain.ArtifactType("playground_evidence_"+sample.EvidenceID), data)
}

func (h *Handler) getAllowedHost(runID string) (string, error) {
	data, _, err := h.artifactSvc.GetLatest(runID, artDomain.ArtifactTargetProfile)
	if err != nil {
		return "", err
	}

	var profile d.TargetProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return "", err
	}

	parsed, err := url.Parse(profile.BaseURL)
	if err != nil {
		return "", err
	}
	return parsed.Host, nil
}

func validateSSRF(rawURL, allowedHost string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if parsed.Scheme == "file" || parsed.Scheme == "ftp" || parsed.Scheme == "gopher" {
		return fmt.Errorf("scheme %q not allowed", parsed.Scheme)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("scheme %q not allowed; only http/https", parsed.Scheme)
	}

	host := parsed.Hostname()

	if host == "169.254.169.254" || host == "metadata.google.internal" {
		return fmt.Errorf("cloud metadata endpoint blocked")
	}

	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
			if allowedHost != "" {
				allowedIP := net.ParseIP(allowedHostname(allowedHost))
				if allowedIP != nil && allowedIP.Equal(ip) {
					return nil
				}
				if host == allowedHostname(allowedHost) {
					return nil
				}
			}
		}
	}

	if allowedHost != "" && parsed.Host != allowedHost {
		return fmt.Errorf("host %q not in allowlist (allowed: %s)", parsed.Host, allowedHost)
	}

	return nil
}

func allowedHostname(hostport string) string {
	h, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport
	}
	return h
}

func extractPath(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return parsed.Path
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
