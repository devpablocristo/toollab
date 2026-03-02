package domain

// DocsMiniDossier is the curated dossier sent to the LLM for documentation generation.
// Optimized for documentation: rich AST data (endpoints, DTOs) + selected runtime evidence.
type DocsMiniDossier struct {
	SchemaVersion string                `json:"schema_version"`
	RunID         string                `json:"run_id"`
	RunMode       RunMode               `json:"run_mode"`
	Service       DocsMiniService       `json:"service"`
	Endpoints     []DocsMiniEndpoint    `json:"endpoints"`
	DTOs          []DocsMiniDTO         `json:"dtos,omitempty"`
	Auth          DocsMiniAuth          `json:"auth"`
	CommonErrors  []DocsMiniCommonError `json:"common_errors,omitempty"`
	Findings      DocsMiniFindings      `json:"findings"`
	Stats         DocsMiniStats         `json:"stats"`
}

// DocsMiniService is the service identity block.
type DocsMiniService struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Framework   string `json:"framework"`
	BaseURL     string `json:"base_url"`
}

// DocsMiniEndpoint is a single API endpoint with all docs-relevant info.
type DocsMiniEndpoint struct {
	Method             string   `json:"method"`
	Path               string   `json:"path"`
	Handler            string   `json:"handler"`
	Domain             string   `json:"domain"`
	Middlewares        []string `json:"middlewares,omitempty"`
	RequestFields      []string `json:"request_fields,omitempty"`
	ResponseFields     []string `json:"response_fields,omitempty"`
	ContractConfidence float64  `json:"contract_confidence,omitempty"`
	ResponseKeys       []string `json:"response_keys,omitempty"`
	ResponseStatus     int      `json:"response_status,omitempty"`
}

// DocsMiniDTO is a data transfer object discovered in the code.
type DocsMiniDTO struct {
	Name   string   `json:"name"`
	Domain string   `json:"domain"`
	Fields []string `json:"fields"`
}

// AuthClassification is the proven auth status for an endpoint.
type AuthClassification string

const (
	AuthProvenRequired    AuthClassification = "PROVEN_REQUIRED"
	AuthProvenNotRequired AuthClassification = "PROVEN_NOT_REQUIRED"
	AuthClassUnknown      AuthClassification = "UNKNOWN"
)

// DocsMiniAuth combines auth summary and observed evidence.
type DocsMiniAuth struct {
	HeadersSeen       []DocsMiniAuthHeader           `json:"headers_seen,omitempty"`
	ErrorFingerprints []DocsMiniAuthErrorFingerprint `json:"error_fingerprints,omitempty"`
	ProvenRequired    int                            `json:"proven_required"`
	ProvenNotRequired int                            `json:"proven_not_required"`
	Unknown           int                            `json:"unknown"`
	DiscrepancyCount  int                            `json:"discrepancy_count"`
	Discrepancies     []DocsMiniDiscrepancy          `json:"discrepancies,omitempty"`
}

type DocsMiniAuthHeader struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type DocsMiniAuthErrorFingerprint struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
	Count  int    `json:"count"`
}

type DocsMiniDiscrepancy struct {
	Endpoint    string `json:"endpoint"`
	Description string `json:"description"`
}

// DocsMiniCommonError is a deduplicated error pattern.
type DocsMiniCommonError struct {
	Status    int    `json:"status"`
	ErrorCode string `json:"error_code,omitempty"`
	Message   string `json:"message"`
	Count     int    `json:"count"`
}

// DocsMiniFindings is a compact findings summary.
type DocsMiniFindings struct {
	Total      int                 `json:"total"`
	Highlights []DocsMiniHighlight `json:"highlights,omitempty"`
}

type DocsMiniHighlight struct {
	Title       string `json:"title"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
}

type DocsMiniStats struct {
	EndpointsTotal     int `json:"endpoints_total"`
	EndpointsConfirmed int `json:"endpoints_confirmed"`
	DTOsTotal          int `json:"dtos_total"`
	DomainsCount       int `json:"domains_count"`
}
