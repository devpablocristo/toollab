package domain

// InferredContract is a schema for one endpoint's request/response.
type InferredContract struct {
	EndpointID      string           `json:"endpoint_id"`
	Method          string           `json:"method"`
	Path            string           `json:"path"`
	RequestSchema   *RequestSchema   `json:"request_schema,omitempty"`
	ResponseSchemas []ResponseSchema `json:"response_schemas,omitempty"`
	Confidence      float64          `json:"confidence"`
	EvidenceRefs    []string         `json:"evidence_refs,omitempty"`
}

type RequestSchema struct {
	ContentType string        `json:"content_type,omitempty"`
	SchemaRef   string        `json:"schema_ref"`
	Fields      []SchemaField `json:"fields,omitempty"`
	Headers     []string      `json:"headers,omitempty"`
	QueryParams []string      `json:"query_params,omitempty"`
}

type ResponseSchema struct {
	Status      int           `json:"status"`
	ContentType string        `json:"content_type,omitempty"`
	SchemaRef   string        `json:"schema_ref"`
	Fields      []SchemaField `json:"fields,omitempty"`
	ExampleRef  string        `json:"example_ref,omitempty"`
}

type SchemaField struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Optional bool   `json:"optional,omitempty"`
	Example  string `json:"example,omitempty"`
}

// SchemaRegistry is the catalog of all inferred schemas.
type SchemaRegistry struct {
	SchemaVersion string            `json:"schema_version"`
	Schemas       map[string]Schema `json:"schemas"`
}

type Schema struct {
	SchemaRef   string        `json:"schema_ref"`
	ContentType string        `json:"content_type"`
	Fields      []SchemaField `json:"fields"`
	Confidence  float64       `json:"confidence"`
}

// SemanticAnnotation tags fields with semantic meaning.
type SemanticAnnotation struct {
	EndpointID string            `json:"endpoint_id"`
	Fields     []FieldAnnotation `json:"fields"`
}

type FieldAnnotation struct {
	FieldPath    string   `json:"field_path"`
	Tag          string   `json:"tag"` // id_field, owner_field, status_field, etc.
	Confidence   float64  `json:"confidence"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

// SmokeResult is the output of Step 3.
type SmokeResult struct {
	EndpointID   string   `json:"endpoint_id"`
	Method       string   `json:"method"`
	Path         string   `json:"path"`
	Passed       bool     `json:"passed"`
	StatusCode   int      `json:"status_code,omitempty"`
	EvidenceRefs []string `json:"evidence_refs"`
	BlockReason  string   `json:"block_reason,omitempty"`
}

// AuthMatrix is the output of Step 4.
type AuthMatrix struct {
	SchemaVersion string            `json:"schema_version"`
	Entries       []AuthMatrixEntry `json:"entries"`
	Discrepancies []AuthDiscrepancy `json:"discrepancies,omitempty"`
}

type AuthStatus string

const (
	AuthAllowed      AuthStatus = "allowed"
	AuthDenied       AuthStatus = "denied"
	AuthUnknown      AuthStatus = "unknown"
	AuthInconsistent AuthStatus = "inconsistent"
)

type AuthMatrixEntry struct {
	EndpointID   string     `json:"endpoint_id"`
	Method       string     `json:"method"`
	Path         string     `json:"path"`
	NoAuth       AuthStatus `json:"no_auth"`
	InvalidAuth  AuthStatus `json:"invalid_auth"`
	ValidAuth    AuthStatus `json:"valid_auth,omitempty"`
	EvidenceRefs []string   `json:"evidence_refs"`
}

type AuthDiscrepancy struct {
	EndpointID   string   `json:"endpoint_id"`
	Method       string   `json:"method"`
	Path         string   `json:"path"`
	Description  string   `json:"description"`
	ASTSays      string   `json:"ast_says"`
	RuntimeSays  string   `json:"runtime_says"`
	Risk         string   `json:"risk"`
	EvidenceRefs []string `json:"evidence_refs"`
	ASTRefs      []ASTRef `json:"ast_refs,omitempty"`
}

// FuzzResult is a single fuzz test outcome.
type FuzzResult struct {
	EndpointID  string `json:"endpoint_id"`
	Category    string `json:"category"` // injection, boundary, malformed, content_type, method_tamper
	SubCategory string `json:"sub_category,omitempty"`
	InputDesc   string `json:"input_desc"`
	Status      int    `json:"status,omitempty"`
	Crashed     bool   `json:"crashed"`
	Timeout     bool   `json:"timeout"`
	EvidenceRef string `json:"evidence_ref"`
}

// LogicResult is a business logic test outcome.
type LogicResult struct {
	EndpointID   string   `json:"endpoint_id"`
	TestType     string   `json:"test_type"` // idor, duplicate, state_transition, idempotency, concurrency
	Description  string   `json:"description"`
	Passed       bool     `json:"passed"`
	Anomaly      bool     `json:"anomaly"`
	EvidenceRefs []string `json:"evidence_refs"`
}

// AbuseResult is an abuse/resilience test outcome.
type AbuseResult struct {
	EndpointID   string   `json:"endpoint_id"`
	TestType     string   `json:"test_type"` // rate_limit, timeout, large_payload
	BurstSize    int      `json:"burst_size,omitempty"`
	Got429       bool     `json:"got_429"`
	Degraded     bool     `json:"degraded"`
	Crashed      bool     `json:"crashed"`
	EvidenceRefs []string `json:"evidence_refs"`
}

// ResilienceMetrics summarizes abuse/resilience testing.
type ResilienceMetrics struct {
	RateLimitObserved   bool `json:"rate_limit_observed"`
	RateLimitHeaders    bool `json:"rate_limit_headers"`
	DegradationOnBurst  bool `json:"degradation_on_burst"`
	LargePayloadHandled bool `json:"large_payload_handled"`
}
