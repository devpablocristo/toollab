package domain

// DocsMiniDossier is the curated, minimal dossier sent to the LLM for narrative documentation.
// The LLM generates ONLY the overview/narrative sections. Endpoint catalog and DTOs
// are already covered by the Endpoints tab (EndpointIntelligence) and don't need LLM generation.
type DocsMiniDossier struct {
	SchemaVersion  string                  `json:"schema_version"`
	RunID          string                  `json:"run_id"`
	RunMode        RunMode                 `json:"run_mode"`
	Service        DocsMiniService         `json:"service"`
	Domains        []DocsMiniDomain        `json:"domains"`
	RouteSummary   []DocsMiniRouteGroup    `json:"route_summary"`
	ResponseShapes []DocsMiniResponseShape `json:"response_shapes,omitempty"`
	AuthSummary    DocsMiniAuth            `json:"auth_summary"`
	AuthObserved   DocsMiniAuthObserved    `json:"auth_observed"`
	CommonErrors   []DocsMiniCommonError   `json:"common_errors,omitempty"`
	Findings       DocsMiniFindings        `json:"findings"`
	Metrics        DocsMiniMetrics         `json:"metrics"`
	Stats          DocsMiniStats           `json:"stats"`
}

// DocsMiniRouteGroup is a lightweight summary of routes per domain.
type DocsMiniRouteGroup struct {
	Domain string   `json:"domain"`
	Routes []string `json:"routes"`
}

// DocsMiniResponseShape shows top-level JSON keys from a representative response for a route.
type DocsMiniResponseShape struct {
	Route  string   `json:"route"`
	Status int      `json:"status"`
	Keys   []string `json:"keys"`
}

// DocsMiniService is the service identity block.
type DocsMiniService struct {
	Name            string       `json:"name"`
	Description     string       `json:"description,omitempty"`
	SourcePath      string       `json:"source_path,omitempty"`
	Framework       string       `json:"framework"`
	BaseURL         string       `json:"base_url"`
	BasePaths       []string     `json:"base_paths,omitempty"`
	VersioningHint  string       `json:"versioning_hint,omitempty"`
	HealthEndpoints []string     `json:"health_endpoints,omitempty"`
	ContentTypes    ContentTypes `json:"content_types"`
}

// DocsMiniDomain represents a code package/domain grouping.
type DocsMiniDomain struct {
	Package       string   `json:"package"`
	EndpointCount int      `json:"endpoint_count"`
	Handlers      []string `json:"handlers,omitempty"`
}

// AuthClassification is the proven auth status for an endpoint.
type AuthClassification string

const (
	AuthProvenRequired    AuthClassification = "PROVEN_REQUIRED"
	AuthProvenNotRequired AuthClassification = "PROVEN_NOT_REQUIRED"
	AuthClassUnknown      AuthClassification = "UNKNOWN"
)

// DocsMiniAuth is the aggregate auth summary.
type DocsMiniAuth struct {
	Mechanisms          []string              `json:"mechanisms"`
	ProvenRequired      int                   `json:"proven_required"`
	ProvenNotRequired   int                   `json:"proven_not_required"`
	Unknown             int                   `json:"unknown"`
	DiscrepancyCount    int                   `json:"discrepancy_count"`
	DiscrepancyExamples []DocsMiniDiscrepancy `json:"discrepancy_examples,omitempty"`
}

// DocsMiniDiscrepancy is a simplified AST-vs-runtime mismatch.
type DocsMiniDiscrepancy struct {
	EndpointID  string `json:"endpoint_id"`
	Description string `json:"description"`
	ASTSays     string `json:"ast_says"`
	RuntimeSays string `json:"runtime_says"`
}

// DocsMiniAuthObserved holds auth evidence extracted from runtime samples.
type DocsMiniAuthObserved struct {
	HeadersSeen       []DocsMiniAuthHeader           `json:"headers_seen,omitempty"`
	ErrorFingerprints []DocsMiniAuthErrorFingerprint `json:"error_fingerprints,omitempty"`
}

// DocsMiniAuthHeader is an auth-relevant header observed in successful requests.
type DocsMiniAuthHeader struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// DocsMiniAuthErrorFingerprint is a deduplicated auth error pattern.
type DocsMiniAuthErrorFingerprint struct {
	Status            int    `json:"status"`
	Body              string `json:"body"`
	Count             int    `json:"count"`
	ExampleEvidenceID string `json:"example_evidence_id,omitempty"`
}

// DocsMiniFindings is a compact findings summary for docs.
type DocsMiniFindings struct {
	Total      int                    `json:"total"`
	BySeverity map[FindingSeverity]int `json:"by_severity"`
	ByCategory map[FindingCategory]int `json:"by_category"`
	Highlights []DocsMiniHighlight     `json:"highlights"`
}

// DocsMiniHighlight is a top finding for docs context.
type DocsMiniHighlight struct {
	Title        string          `json:"title"`
	Severity     FindingSeverity `json:"severity"`
	Category     FindingCategory `json:"category"`
	Description  string          `json:"description"`
	EvidenceRefs []string        `json:"evidence_refs"`
}

// DocsMiniCommonError is a deduplicated error pattern observed across endpoints.
type DocsMiniCommonError struct {
	Status            int    `json:"status"`
	ErrorCode         string `json:"error_code,omitempty"`
	Message           string `json:"message"`
	Count             int    `json:"count"`
	ExampleEvidenceID string `json:"example_evidence_id,omitempty"`
}

// DocsMiniMetrics is a compact metrics summary.
type DocsMiniMetrics struct {
	TotalRequests   int     `json:"total_requests"`
	SuccessRate     float64 `json:"success_rate"`
	P50Ms           int64   `json:"p50_ms,omitempty"`
	P95Ms           int64   `json:"p95_ms,omitempty"`
	EndpointsTested int     `json:"endpoints_tested"`
	EndpointsTotal  int     `json:"endpoints_total"`
	CoveragePct     float64 `json:"coverage_pct"`
}

// DocsMiniStats tracks the dossier payload metadata.
type DocsMiniStats struct {
	EndpointsConfirmed int `json:"endpoints_confirmed"`
	EndpointsCatalog   int `json:"endpoints_catalog"`
	DomainsCount       int `json:"domains_count"`
}
