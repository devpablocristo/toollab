package domain

import "time"

type Analysis struct {
	TargetID   string    `json:"target_id"`
	TargetName string    `json:"target_name"`
	CreatedAt  time.Time `json:"created_at"`
	RunID      string    `json:"run_id"`

	Discovery   DiscoverySummary   `json:"discovery"`
	Performance PerformanceMetrics `json:"performance"`
	Security    SecurityReport     `json:"security"`
	Contract    ContractReport     `json:"contract"`
	Coverage    CoverageReport     `json:"coverage"`
	Behavior    BehaviorAnalysis   `json:"behavior"`

	ProbesSummary ProbesSummary `json:"probes_summary"`

	Score int    `json:"score"`
	Grade string `json:"grade"`
}

type ProbesSummary struct {
	TotalProbes       int `json:"total_probes"`
	InjectionProbes   int `json:"injection_probes"`
	MalformedProbes   int `json:"malformed_probes"`
	BoundaryProbes    int `json:"boundary_probes"`
	MethodTamperProbes int `json:"method_tamper_probes"`
	HiddenEndpointProbes int `json:"hidden_endpoint_probes"`
	LargePayloadProbes int `json:"large_payload_probes"`
	ContentTypeProbes  int `json:"content_type_probes"`
	NoAuthProbes       int `json:"no_auth_probes"`
}

type BehaviorAnalysis struct {
	InvalidInput     BehaviorObservation        `json:"invalid_input"`
	MissingAuth      BehaviorObservation        `json:"missing_auth"`
	NotFound         BehaviorObservation        `json:"not_found"`
	ErrorConsistency BehaviorObservation        `json:"error_consistency"`
	InferredModels   []InferredModel            `json:"inferred_models"`
	EndpointBehavior []EndpointBehaviorSummary  `json:"endpoint_behavior"`
}

type BehaviorObservation struct {
	Quality string `json:"quality"` // good, mixed, poor, unknown
	Summary string `json:"summary"`
	Tested  int    `json:"tested"`
}

type InferredModel struct {
	Name     string          `json:"name"`
	Fields   []InferredField `json:"fields"`
	SeenFrom []string        `json:"seen_from"`
}

type InferredField struct {
	Name     string `json:"name"`
	JSONType string `json:"json_type"`
	Example  string `json:"example,omitempty"`
}

type EndpointBehaviorSummary struct {
	Endpoint     string         `json:"endpoint"`
	Method       string         `json:"method"`
	Path         string         `json:"path"`
	StatusCodes  map[int]int    `json:"status_codes"`
	RequestCount int            `json:"request_count"`
	AvgLatencyMs int64          `json:"avg_latency_ms"`
	ErrorCount   int            `json:"error_count"`
	RequiresAuth bool           `json:"requires_auth"`
}

type DiscoverySummary struct {
	Framework      string `json:"framework"`
	EndpointsCount int    `json:"endpoints_count"`
	Confidence     float64 `json:"confidence"`
	Gaps           []string `json:"gaps"`
}

type PerformanceMetrics struct {
	TotalRequests   int            `json:"total_requests"`
	SuccessRate     float64        `json:"success_rate"`
	ErrorRate       float64        `json:"error_rate"`
	P50Ms           int            `json:"p50_ms"`
	P95Ms           int            `json:"p95_ms"`
	P99Ms           int            `json:"p99_ms"`
	StatusHistogram map[string]int `json:"status_histogram"`
	SlowestEndpoints []EndpointTiming `json:"slowest_endpoints"`
}

type EndpointTiming struct {
	Method   string `json:"method"`
	Path     string `json:"path"`
	TimingMs int64  `json:"timing_ms"`
	Status   int    `json:"status"`
}

type SecurityReport struct {
	Score    int               `json:"score"`
	Grade    string            `json:"grade"`
	Findings []SecurityFinding `json:"findings"`
	Summary  SeveritySummary   `json:"summary"`
}

type SecurityFinding struct {
	ID          string `json:"id"`
	Category    string `json:"category"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Endpoint    string `json:"endpoint,omitempty"`
	Remediation string `json:"remediation"`
}

type SeveritySummary struct {
	Total    int `json:"total"`
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
}

type ContractReport struct {
	Compliant       bool                `json:"compliant"`
	ComplianceRate  float64             `json:"compliance_rate"`
	TotalChecks     int                 `json:"total_checks"`
	TotalViolations int                 `json:"total_violations"`
	Violations      []ContractViolation `json:"violations"`
}

type ContractViolation struct {
	Endpoint    string `json:"endpoint"`
	StatusCode  int    `json:"status_code"`
	Field       string `json:"field"`
	Expected    string `json:"expected"`
	Actual      string `json:"actual"`
	Description string `json:"description"`
}

type CoverageReport struct {
	TotalEndpoints  int              `json:"total_endpoints"`
	TestedEndpoints int              `json:"tested_endpoints"`
	CoverageRate    float64          `json:"coverage_rate"`
	ByMethod        []MethodCoverage `json:"by_method"`
	StatusCodes     []StatusCodeObs  `json:"status_codes_observed"`
	Untested        []EndpointRef    `json:"untested"`
}

type MethodCoverage struct {
	Method string  `json:"method"`
	Total  int     `json:"total"`
	Tested int     `json:"tested"`
	Rate   float64 `json:"rate"`
}

type StatusCodeObs struct {
	Code     int  `json:"code"`
	Observed bool `json:"observed"`
}

type EndpointRef struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}
