package comprehension

type Report struct {
	SchemaVersion  int                  `json:"schema_version"`
	RunID          string               `json:"run_id"`
	DataSource     string               `json:"data_source"`
	Identity       Identity             `json:"identity"`
	Architecture   Architecture         `json:"architecture"`
	Dependencies   []ExternalDependency `json:"dependencies,omitempty"`
	Models         []Model              `json:"models"`
	AllFlows       []FlowDetail         `json:"all_flows"`
	Behavior       Behavior             `json:"behavior"`
	Performance    Performance          `json:"performance"`
	Security       SecuritySummary      `json:"security"`
	ContractQuality ContractQuality     `json:"contract_quality"`
	Verdict        Verdict              `json:"verdict"`
	MaturityScore  int                  `json:"maturity_score"`
	MaturityGrade  string               `json:"maturity_grade"`
}

type Identity struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Purpose     string `json:"purpose"`
	Domain      string `json:"domain"`
	APIType     string `json:"api_type"`
	Consumers   string `json:"consumers"`
}

type Architecture struct {
	Type           string `json:"type"`
	AuthType       string `json:"auth_type"`
	DataFormat     string `json:"data_format"`
	HasVersioning  bool   `json:"has_versioning"`
	HasOpenAPI     bool   `json:"has_openapi"`
	TotalEndpoints int    `json:"total_endpoints"`
	ResourceCount  int    `json:"resource_count"`
}

type Model struct {
	Name        string          `json:"name"`
	Kind        string          `json:"kind"`
	Description string          `json:"description,omitempty"`
	Fields      []ModelField    `json:"fields"`
	Relations   []ModelRelation `json:"relations,omitempty"`
	Operations  []string        `json:"operations"`
}

type ModelField struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description,omitempty"`
	Example     string `json:"example,omitempty"`
}

type ModelRelation struct {
	Target      string `json:"target"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

type ExternalDependency struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
}

type FlowDetail struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	Steps       []FlowStep     `json:"steps"`
	Payload     *PayloadDetail `json:"payload,omitempty"`
	Response    *PayloadDetail `json:"response,omitempty"`
	StatusCodes []int          `json:"observed_status_codes"`
	AvgLatency  int            `json:"avg_latency_ms"`
	ErrorRate   float64        `json:"error_rate"`
}

type FlowStep struct {
	Order       int    `json:"order"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	Description string `json:"description"`
}

type PayloadDetail struct {
	ContentType string            `json:"content_type"`
	Example     string            `json:"example"`
	Fields      map[string]string `json:"fields,omitempty"`
}

type Behavior struct {
	InvalidInput     string `json:"invalid_input"`
	MissingAuth      string `json:"missing_auth"`
	NotFound         string `json:"not_found"`
	Duplicates       string `json:"duplicates"`
	ErrorConsistency string `json:"error_consistency"`
	Idempotency      string `json:"idempotency"`
}

type Performance struct {
	OverallP50    int               `json:"overall_p50_ms"`
	OverallP95    int               `json:"overall_p95_ms"`
	OverallP99    int               `json:"overall_p99_ms"`
	FastEndpoints []EndpointPerf    `json:"fast_endpoints"`
	SlowEndpoints []EndpointPerf    `json:"slow_endpoints"`
	Bottlenecks   []string          `json:"bottlenecks"`
}

type EndpointPerf struct {
	Endpoint  string `json:"endpoint"`
	AvgMs     int    `json:"avg_ms"`
	Requests  int    `json:"requests"`
}

type SecuritySummary struct {
	Grade    string   `json:"grade"`
	Score    int      `json:"score"`
	Risks    []string `json:"risks"`
	Strengths []string `json:"strengths"`
}

type ContractQuality struct {
	ComplianceRate float64  `json:"compliance_rate"`
	Issues         []string `json:"issues"`
	Strengths      []string `json:"strengths"`
}

type Verdict struct {
	ProductionReady bool     `json:"production_ready"`
	Confidence      string   `json:"confidence"`
	Risks           []string `json:"risks"`
	Improvements    []string `json:"improvements"`
	MissingFeatures []string `json:"missing_features"`
}
