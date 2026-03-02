package domain

// DocsMiniDossier is the curated, minimal dossier sent to the LLM for documentation.
// Design principle: the LLM explains facts, it does not discover them.
type DocsMiniDossier struct {
	SchemaVersion string              `json:"schema_version"`
	RunID         string              `json:"run_id"`
	RunMode       RunMode             `json:"run_mode"`
	Service       DocsMiniService     `json:"service"`
	Domains       []DocsMiniDomain    `json:"domains"`
	Endpoints     []DocsMiniEndpoint  `json:"endpoints"`
	DTOs          []DocsMiniDTO       `json:"dtos,omitempty"`
	AuthSummary   DocsMiniAuth        `json:"auth_summary"`
	Middlewares   []DocsMiniMiddleware `json:"middlewares"`
	Findings      DocsMiniFindings    `json:"findings"`
	Metrics       DocsMiniMetrics     `json:"metrics"`
	Stats         DocsMiniStats       `json:"stats"`
}

// DocsMiniService is the service identity block.
type DocsMiniService struct {
	Name             string   `json:"name"`
	SourcePath       string   `json:"source_path,omitempty"`
	Framework        string   `json:"framework"`
	BaseURL          string   `json:"base_url"`
	BasePaths        []string `json:"base_paths,omitempty"`
	VersioningHint   string   `json:"versioning_hint,omitempty"`
	HealthEndpoints  []string `json:"health_endpoints,omitempty"`
	ContentTypes     ContentTypes `json:"content_types"`
}

// DocsMiniDomain represents a code package/domain grouping.
type DocsMiniDomain struct {
	Package       string   `json:"package"`
	EndpointCount int      `json:"endpoint_count"`
	Handlers      []string `json:"handlers,omitempty"`
}

// DocsMiniDTO is a data transfer object discovered from AST.
type DocsMiniDTO struct {
	Name    string   `json:"name"`
	Package string   `json:"package,omitempty"`
	File    string   `json:"file,omitempty"`
	Fields  []string `json:"fields"`
}

// DocsMiniEndpoint is a single endpoint with curated evidence.
type DocsMiniEndpoint struct {
	EndpointID      string           `json:"endpoint_id"`
	Method          string           `json:"method"`
	Path            string           `json:"path"`
	HandlerSymbol   string           `json:"handler_symbol,omitempty"`
	HandlerFile     string           `json:"handler_file,omitempty"`
	HandlerLine     int              `json:"handler_line,omitempty"`
	HandlerPackage  string           `json:"handler_package,omitempty"`
	MiddlewareChain []string         `json:"middleware_chain,omitempty"`
	GroupPrefix     string           `json:"group_prefix,omitempty"`
	GroupLabel      string           `json:"group_label,omitempty"`
	Auth            AuthClassification `json:"auth"`
	HappySample     *DocsMiniSample  `json:"happy_sample,omitempty"`
	ErrorSample     *DocsMiniSample  `json:"error_sample,omitempty"`
	StatusesSeen    []int            `json:"statuses_seen,omitempty"`
}

// AuthClassification is the proven auth status for an endpoint.
type AuthClassification string

const (
	AuthProvenRequired    AuthClassification = "PROVEN_REQUIRED"
	AuthProvenNotRequired AuthClassification = "PROVEN_NOT_REQUIRED"
	AuthClassUnknown      AuthClassification = "UNKNOWN"
)

// DocsMiniSample is a compact request/response pair.
type DocsMiniSample struct {
	EvidenceID  string            `json:"evidence_id"`
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	ReqHeaders  map[string]string `json:"req_headers,omitempty"`
	ReqBody     string            `json:"req_body,omitempty"`
	Status      int               `json:"status"`
	RespSnippet string            `json:"resp_snippet,omitempty"`
	LatencyMs   int64             `json:"latency_ms"`
}

// DocsMiniAuth is the aggregate auth summary.
type DocsMiniAuth struct {
	Mechanisms       []string `json:"mechanisms"`
	ProvenRequired   int      `json:"proven_required"`
	ProvenNotRequired int     `json:"proven_not_required"`
	Unknown          int      `json:"unknown"`
	Discrepancies    []DocsMiniDiscrepancy `json:"discrepancies,omitempty"`
}

// DocsMiniDiscrepancy is a simplified AST-vs-runtime mismatch.
type DocsMiniDiscrepancy struct {
	EndpointID  string `json:"endpoint_id"`
	Description string `json:"description"`
	ASTSays     string `json:"ast_says"`
	RuntimeSays string `json:"runtime_says"`
}

// DocsMiniMiddleware is a flat middleware entry.
type DocsMiniMiddleware struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	SourceFile string `json:"source_file,omitempty"`
	SourceLine int    `json:"source_line,omitempty"`
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

// DocsMiniMetrics is a compact metrics summary.
type DocsMiniMetrics struct {
	TotalRequests   int     `json:"total_requests"`
	SuccessRate     float64 `json:"success_rate"`
	P50Ms           int64   `json:"p50_ms"`
	P95Ms           int64   `json:"p95_ms"`
	EndpointsTested int     `json:"endpoints_tested"`
	EndpointsTotal  int     `json:"endpoints_total"`
	CoveragePct     float64 `json:"coverage_pct"`
}

// DocsMiniStats tracks the dossier payload metadata.
type DocsMiniStats struct {
	EndpointsCount int `json:"endpoints_count"`
	SamplesCount   int `json:"samples_count"`
	MiddlewareCount int `json:"middleware_count"`
}
