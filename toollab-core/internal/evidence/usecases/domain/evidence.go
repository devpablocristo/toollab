package domain

import "time"

type EvidencePack struct {
	PackID    string         `json:"pack_id"`
	RunID     string         `json:"run_id"`
	CreatedAt time.Time      `json:"created_at"`
	Items     []EvidenceItem `json:"items"`
}

type EvidenceItem struct {
	EvidenceID string            `json:"evidence_id"`
	CaseID     string            `json:"case_id"`
	Kind       string            `json:"kind"` // http_exchange
	Tags       []string          `json:"tags,omitempty"`
	Request    EvidenceRequest   `json:"request"`
	Response   *EvidenceResponse `json:"response,omitempty"`
	TimingMs   int64             `json:"timing_ms"`
	Error      string            `json:"error,omitempty"`
	Hashes     *EvidenceHashes   `json:"hashes,omitempty"`
}

type EvidenceRequest struct {
	Method              string            `json:"method"`
	URL                 string            `json:"url"`
	Headers             map[string]string `json:"headers,omitempty"`
	BodyRef             string            `json:"body_ref,omitempty"`
	BodyInlineTruncated string            `json:"body_inline_truncated,omitempty"`
}

type EvidenceResponse struct {
	Status              int               `json:"status"`
	Headers             map[string]string `json:"headers,omitempty"`
	BodyRef             string            `json:"body_ref,omitempty"`
	BodyInlineTruncated string            `json:"body_inline_truncated,omitempty"`
}

type EvidenceHashes struct {
	SHA256RequestBody  string `json:"sha256_request_body,omitempty"`
	SHA256ResponseBody string `json:"sha256_response_body,omitempty"`
}

type Ingestor interface {
	Ingest(runID string, exec ExecutionResult) (EvidencePack, int, error)
}

type ExecutionResult struct {
	RunID      string       `json:"run_id"`
	StartedAt  time.Time    `json:"started_at"`
	FinishedAt time.Time    `json:"finished_at"`
	Cases      []CaseResult `json:"cases"`
}

type CaseResult struct {
	CaseID     string            `json:"case_id"`
	EvidenceID string            `json:"evidence_id"`
	Tags       []string          `json:"tags,omitempty"`
	ReqFinal   CaseResultReq     `json:"request_final"`
	Response   *CaseResultResp   `json:"response,omitempty"`
	TimingMs   int64             `json:"timing_ms"`
	Error      string            `json:"error,omitempty"`
}

type CaseResultReq struct {
	Method   string            `json:"method"`
	URL      string            `json:"url"`
	Headers  map[string]string `json:"headers,omitempty"`
	BodySize int64             `json:"body_size"`
	BodyRaw  []byte            `json:"-"`
}

type CaseResultResp struct {
	Status   int               `json:"status"`
	Headers  map[string]string `json:"headers,omitempty"`
	BodySize int64             `json:"body_size"`
	BodyRaw  []byte            `json:"-"`
}
