package domain

// DocsMiniDossier is the minimal dossier sent to the LLM for documentation generation.
type DocsMiniDossier struct {
	SchemaVersion string                `json:"schema_version"`
	RunID         string                `json:"run_id"`
	RunMode       RunMode               `json:"run_mode"`
	Service       DocsMiniService       `json:"service"`
	Endpoints     []DocsMiniEndpoint    `json:"endpoints"`
	Auth          DocsMiniAuth          `json:"auth"`
	CommonErrors  []DocsMiniCommonError `json:"common_errors,omitempty"`
	Gaps          DocsMiniGaps          `json:"gaps"`
	Stats         DocsMiniStats         `json:"stats"`
}

// DocsMiniService is the service identity block.
type DocsMiniService struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Framework   string `json:"framework"`
	BaseURL     string `json:"base_url"`
}

// DocsMiniEndpoint is a single API endpoint with minimal docs-relevant info.
type DocsMiniEndpoint struct {
	Method        string   `json:"method"`
	Path          string   `json:"path"`
	Domain        string   `json:"domain"`
	RequestFields []string `json:"request_fields,omitempty"`
	ResponseKeys  []string `json:"response_keys,omitempty"`
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

// DocsMiniGaps summarizes what is still unknown.
type DocsMiniGaps struct {
	UnconfirmedEndpoints int `json:"unconfirmed_endpoints"`
	EndpointsNoShape     int `json:"endpoints_no_shape"`
	EndpointsAuthUnknown int `json:"endpoints_auth_unknown"`
}

type DocsMiniStats struct {
	EndpointsTotal     int `json:"endpoints_total"`
	EndpointsConfirmed int `json:"endpoints_confirmed"`
	DomainsCount       int `json:"domains_count"`
}
