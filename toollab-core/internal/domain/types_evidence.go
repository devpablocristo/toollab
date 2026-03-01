package domain

import (
	"strings"
	"time"
)

// TestCategory identifies the pipeline step that produced the evidence.
type TestCategory string

const (
	CatPreflight TestCategory = "preflight"
	CatSmoke     TestCategory = "smoke"
	CatAuth      TestCategory = "auth"
	CatFuzz      TestCategory = "fuzz"
	CatLogic     TestCategory = "logic"
	CatAbuse     TestCategory = "abuse"
	CatConfirm   TestCategory = "confirm"
	CatManual    TestCategory = "manual"
)

// EvidenceSample is a single request/response pair with stable ID and rich metadata.
type EvidenceSample struct {
	EvidenceID       string            `json:"evidence_id"`
	EndpointID       string            `json:"endpoint_id,omitempty"`
	Category         TestCategory      `json:"category"`
	Tags             []string          `json:"tags,omitempty"`
	Request          EvidenceRequest   `json:"request"`
	Response         *EvidenceResponse `json:"response,omitempty"`
	Timing           EvidenceTiming    `json:"timing"`
	Error            string            `json:"error,omitempty"`
	ErrorSignatureID string            `json:"error_signature_id,omitempty"`
	CorrelationIDs   []string          `json:"correlation_ids,omitempty"`
}

type EvidenceRequest struct {
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	Path        string            `json:"path"`
	Headers     map[string]string `json:"headers,omitempty"`
	Query       map[string]string `json:"query,omitempty"`
	Body        string            `json:"body,omitempty"`
	ContentType string            `json:"content_type,omitempty"`
	Size        int64             `json:"size"`
}

type EvidenceResponse struct {
	Status      int               `json:"status"`
	Headers     map[string]string `json:"headers,omitempty"`
	BodySnippet string            `json:"body_snippet,omitempty"`
	ContentType string            `json:"content_type,omitempty"`
	Size        int64             `json:"size"`
}

type EvidenceTiming struct {
	LatencyMs int64 `json:"latency_ms"`
}

// EvidenceStore accumulates evidence samples during a run with sequence tracking.
type EvidenceStore struct {
	RunSeed  string
	samples  []EvidenceSample
	counters map[string]int // key = endpointID+category
}

func NewEvidenceStore(runSeed string) *EvidenceStore {
	return &EvidenceStore{
		RunSeed:  runSeed,
		counters: make(map[string]int),
	}
}

func (s *EvidenceStore) Add(endpointID string, cat TestCategory, tags []string, req EvidenceRequest, resp *EvidenceResponse, timing EvidenceTiming, errMsg string) string {
	key := endpointID + string(cat)
	seq := s.counters[key]
	s.counters[key] = seq + 1

	eid := EvidenceID(s.RunSeed, endpointID, cat, seq)
	sample := EvidenceSample{
		EvidenceID: eid,
		EndpointID: endpointID,
		Category:   cat,
		Tags:       tags,
		Request:    req,
		Response:   resp,
		Timing:     timing,
		Error:      errMsg,
	}
	s.samples = append(s.samples, sample)
	return eid
}

func (s *EvidenceStore) Samples() []EvidenceSample { return s.samples }
func (s *EvidenceStore) Count() int                { return len(s.samples) }

// ErrorSignature groups recurring error patterns.
type ErrorSignature struct {
	SignatureID        string   `json:"signature_id"`
	Pattern            string   `json:"pattern"`
	Status             int      `json:"status"`
	ContentType        string   `json:"content_type"`
	NormalizedMarkers  []string `json:"normalized_markers,omitempty"`
	Count              int      `json:"count"`
	EndpointsAffected  []string `json:"endpoints_affected,omitempty"`
	SampleEvidenceRefs []string `json:"sample_evidence_refs,omitempty"`
}

// ErrorSignatureBuilder accumulates error observations and produces signatures.
type ErrorSignatureBuilder struct {
	byID map[string]*ErrorSignature
}

func NewErrorSignatureBuilder() *ErrorSignatureBuilder {
	return &ErrorSignatureBuilder{byID: make(map[string]*ErrorSignature)}
}

func (b *ErrorSignatureBuilder) Observe(status int, contentType, bodySnippet, endpointID, evidenceID string) {
	norm := NormalizeErrorPattern(bodySnippet)
	sid := SignatureID(status, contentType, norm)

	sig, ok := b.byID[sid]
	if !ok {
		sig = &ErrorSignature{
			SignatureID: sid,
			Status:      status,
			ContentType: contentType,
			Pattern:     truncate(norm, 200),
		}
		sig.NormalizedMarkers = extractMarkers(norm)
		b.byID[sid] = sig
	}

	sig.Count++
	if !contains(sig.EndpointsAffected, endpointID) && len(sig.EndpointsAffected) < 50 {
		sig.EndpointsAffected = append(sig.EndpointsAffected, endpointID)
	}
	if len(sig.SampleEvidenceRefs) < 3 {
		sig.SampleEvidenceRefs = append(sig.SampleEvidenceRefs, evidenceID)
	}
}

func (b *ErrorSignatureBuilder) Build() []ErrorSignature {
	out := make([]ErrorSignature, 0, len(b.byID))
	for _, s := range b.byID {
		out = append(out, *s)
	}
	return out
}

// RawEvidence contains all evidence and error signatures from a run.
type RawEvidence struct {
	SchemaVersion   string           `json:"schema_version"`
	RunID           string           `json:"run_id"`
	CreatedAt       time.Time        `json:"created_at"`
	Samples         []EvidenceSample `json:"samples"`
	ErrorSignatures []ErrorSignature `json:"error_signatures"`
	TotalCount      int              `json:"total_count"`
}

func extractMarkers(norm string) []string {
	markers := []string{}
	keywords := []string{"panic", "stack trace", "runtime error", "sql", "syntax error",
		"internal server error", "null pointer", "segfault", "timeout", "connection refused",
		"unauthorized", "forbidden", "not found", "bad request", "validation"}
	for _, kw := range keywords {
		if strings.Contains(norm, kw) {
			markers = append(markers, kw)
		}
	}
	return markers
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

// PlaygroundRequest is the input for a manual Try-It request.
type PlaygroundRequest struct {
	EndpointID    string            `json:"endpoint_id"`
	Method        string            `json:"method"`
	URL           string            `json:"url"`
	PathParams    map[string]string `json:"path_params,omitempty"`
	Query         map[string]string `json:"query,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	Body          string            `json:"body,omitempty"`
	AuthProfileID string            `json:"auth_profile_id,omitempty"`
	TimeoutMs     int               `json:"timeout_ms,omitempty"`
}

// PlaygroundResponse is the output of a manual Try-It request.
type PlaygroundResponse struct {
	EvidenceID       string            `json:"evidence_id"`
	Status           int               `json:"status,omitempty"`
	Headers          map[string]string `json:"headers,omitempty"`
	Body             string            `json:"body,omitempty"`
	BodySnippet      string            `json:"body_snippet,omitempty"`
	ContentType      string            `json:"content_type,omitempty"`
	LatencyMs        int64             `json:"latency_ms"`
	Size             int64             `json:"size"`
	Error            string            `json:"error,omitempty"`
	ErrorSignatureID string            `json:"error_signature_id,omitempty"`
}

// AuthProfile stores auth credentials for playground use.
type AuthProfile struct {
	ID        string `json:"id"`
	RunID     string `json:"run_id"`
	Name      string `json:"name"`
	Mechanism string `json:"mechanism"` // bearer, api_key, cookie, none
	HeaderKey string `json:"header_key,omitempty"`
	Value     string `json:"value"`
	Env       string `json:"env,omitempty"` // dev, staging, local
	CreatedAt string `json:"created_at"`
}

// AuthProfileMasked is a safe-for-UI view of an auth profile.
type AuthProfileMasked struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Mechanism string `json:"mechanism"`
	HeaderKey string `json:"header_key,omitempty"`
	Masked    string `json:"masked_value"`
	Env       string `json:"env,omitempty"`
}

func (p AuthProfile) ToMasked() AuthProfileMasked {
	masked := "****"
	if len(p.Value) > 8 {
		masked = p.Value[:4] + "****" + p.Value[len(p.Value)-4:]
	}
	return AuthProfileMasked{
		ID:        p.ID,
		Name:      p.Name,
		Mechanism: p.Mechanism,
		HeaderKey: p.HeaderKey,
		Masked:    masked,
		Env:       p.Env,
	}
}
