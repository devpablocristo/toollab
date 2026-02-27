package model

import "time"

type RunStatus string

const (
	RunQueued    RunStatus = "queued"
	RunRunning   RunStatus = "running"
	RunSucceeded RunStatus = "succeeded"
	RunFailed    RunStatus = "failed"
)

type Run struct {
	ID           string    `json:"id"`
	Status       RunStatus `json:"status"`
	SourceType   string    `json:"source_type"`
	SourceRef    string    `json:"source_ref"`
	CreatedAt    time.Time `json:"created_at"`
	StartedAt    time.Time `json:"started_at,omitempty"`
	FinishedAt   time.Time `json:"finished_at,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

type EvidenceRef struct {
	File      string `json:"file"`
	LineStart int    `json:"line_start"`
	LineEnd   int    `json:"line_end"`
	Symbol    string `json:"symbol,omitempty"`
}

type Endpoint struct {
	ID              string      `json:"id"`
	Method          string      `json:"method"`
	Path            string      `json:"path"`
	HandlerPkg      string      `json:"handler_pkg"`
	HandlerName     string      `json:"handler_name"`
	HandlerReceiver string      `json:"handler_receiver,omitempty"`
	MiddlewareChain []string    `json:"middleware_chain"`
	Evidence        EvidenceRef `json:"evidence"`
}

type TypeField struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	JSONTag    string `json:"json_tag,omitempty"`
	Validate   string `json:"validate,omitempty"`
	Binding    string `json:"binding,omitempty"`
	IsRequired bool   `json:"is_required"`
}

type ModelType struct {
	Name     string      `json:"name"`
	Kind     string      `json:"kind"`
	Fields   []TypeField `json:"fields"`
	Evidence EvidenceRef `json:"evidence"`
}

type Dependency struct {
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	Scope    string      `json:"scope"`
	Evidence EvidenceRef `json:"evidence"`
}

type FlowStep struct {
	Order  int    `json:"order"`
	From   string `json:"from"`
	To     string `json:"to"`
	Kind   string `json:"kind"`
	Symbol string `json:"symbol,omitempty"`
}

type Flow struct {
	ID         string      `json:"id"`
	EndpointID string      `json:"endpoint_id"`
	Steps      []FlowStep  `json:"steps"`
	Evidence   EvidenceRef `json:"evidence"`
}

type DomainGroup struct {
	Name        string   `json:"name"`
	EndpointIDs []string `json:"endpoint_ids"`
}

type ServiceModel struct {
	ModelVersion      string        `json:"model_version"`
	SnapshotID        string        `json:"snapshot_id"`
	HashTree          string        `json:"hash_tree"`
	ServiceName       string        `json:"service_name"`
	LanguageDetected  string        `json:"language_detected"`
	FrameworkDetected string        `json:"framework_detected"`
	Endpoints         []Endpoint    `json:"endpoints"`
	Types             []ModelType   `json:"types"`
	Dependencies      []Dependency  `json:"dependencies"`
	Flows             []Flow        `json:"flows"`
	DomainGroups      []DomainGroup `json:"domain_groups"`
	Fingerprint       string        `json:"fingerprint"`
}

type Summary struct {
	ServiceName      string `json:"service_name"`
	EndpointCount    int    `json:"endpoint_count"`
	DependencyCount  int    `json:"dependency_count"`
	TypeCount        int    `json:"type_count"`
	ComplexEndpoints []struct {
		EndpointID string `json:"endpoint_id"`
		Reason     string `json:"reason"`
	} `json:"complex_endpoints"`
	TopDependencies []string `json:"top_dependencies"`
}

type Finding struct {
	ID             string      `json:"id"`
	Severity       string      `json:"severity"`
	Category       string      `json:"category"`
	Title          string      `json:"title"`
	Description    string      `json:"description"`
	Recommendation string      `json:"recommendation"`
	EndpointID     string      `json:"endpoint_id,omitempty"`
	Evidence       EvidenceRef `json:"evidence"`
}

type AuditReport struct {
	ModelFingerprint string    `json:"model_fingerprint"`
	GeneratedAt      time.Time `json:"generated_at"`
	Findings         []Finding `json:"findings"`
}

type Scenario struct {
	ID              string `json:"id"`
	EndpointID      string `json:"endpoint_id"`
	Method          string `json:"method"`
	Path            string `json:"path"`
	PayloadTemplate any    `json:"payload_template,omitempty"`
	ExpectedStatus  int    `json:"expected_status"`
	RiskCategory    string `json:"risk_category"`
	Notes           string `json:"notes"`
}

type RepoFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type RepoSnapshot struct {
	SnapshotID        string     `json:"snapshot_id"`
	SourceType        string     `json:"source_type"`
	RepoName          string     `json:"repo_name"`
	CommitSHA         string     `json:"commit_sha,omitempty"`
	HashTree          string     `json:"hash_tree"`
	LanguageDetected  string     `json:"language_detected"`
	FrameworkDetected string     `json:"framework_detected"`
	CreatedAt         time.Time  `json:"created_at"`
	Files             []RepoFile `json:"files"`
	ResolvedLocalPath string     `json:"-"`
}

type LLMInterpretation struct {
	ModelFingerprint       string   `json:"model_fingerprint"`
	Provider               string   `json:"provider"`
	Model                  string   `json:"model"`
	FunctionalSummary      string   `json:"functional_summary"`
	DomainGroups           []string `json:"domain_groups"`
	RiskHypotheses         []string `json:"risk_hypotheses"`
	SuggestedTestScenarios []string `json:"suggested_test_scenarios"`
	Raw                    string   `json:"raw,omitempty"`
}
