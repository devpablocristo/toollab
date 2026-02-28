package domain

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"toollab-core/internal/shared"
)

// --- Dossier (input to LLM; curated and compact) ---

type Dossier struct {
	SchemaVersion   string            `json:"schema_version"`
	RunID           string            `json:"run_id"`
	GeneratedAt     time.Time         `json:"generated_at"`
	ServiceOverview ServiceOverview   `json:"service_overview"`
	EndpointsTop    []EndpointTop     `json:"endpoints_top"`
	AuditHighlights []AuditHighlight  `json:"audit_highlights"`
	EvidenceSamples []EvidenceSample  `json:"evidence_samples"`
	AnalysisSummary json.RawMessage   `json:"analysis_summary,omitempty"`
	Constraints     []string          `json:"constraints"`
	Knobs           DossierKnobs      `json:"knobs"`
}

type ServiceOverview struct {
	Framework      string   `json:"framework"`
	EndpointsCount int      `json:"endpoints_count"`
	Confidence     float64  `json:"confidence"`
	Gaps           []string `json:"gaps,omitempty"`
}

type EndpointTop struct {
	EndpointKey string          `json:"endpoint_key"`
	Method      string          `json:"method"`
	Path        string          `json:"path"`
	HandlerName string          `json:"handler_name,omitempty"`
	ModelRef    *shared.ModelRef `json:"model_ref,omitempty"`
}

type AuditHighlight struct {
	FindingID    string           `json:"finding_id"`
	RuleID       string           `json:"rule_id"`
	Severity     string           `json:"severity"`
	Category     string           `json:"category"`
	Title        string           `json:"title"`
	Anomaly      bool             `json:"anomaly"`
	EvidenceRefs []string         `json:"evidence_refs,omitempty"`
	ModelRefs    []shared.ModelRef `json:"model_refs,omitempty"`
}

type EvidenceSample struct {
	EvidenceID       string            `json:"evidence_id"`
	EndpointKey      string            `json:"endpoint_key"`
	RequestSignature string            `json:"request_signature"`
	RequestSummary   RequestSummary    `json:"request_summary"`
	ResponseSummary  *ResponseSummary  `json:"response_summary,omitempty"`
	TimingMs         int64             `json:"timing_ms"`
	Error            string            `json:"error,omitempty"`
}

type RequestSummary struct {
	Method        string            `json:"method"`
	URL           string            `json:"url"`
	HeadersMasked map[string]string `json:"headers_masked,omitempty"`
	BodySnippet   string            `json:"body_snippet,omitempty"`
}

type ResponseSummary struct {
	Status        int               `json:"status"`
	HeadersMasked map[string]string `json:"headers_masked,omitempty"`
	BodySnippet   string            `json:"body_snippet,omitempty"`
}

type DossierKnobs struct {
	MaxSnippetBytes int `json:"max_snippet_bytes"`
	TopEndpoints    int `json:"top_endpoints"`
	TopFindings     int `json:"top_findings"`
}

// --- LLMInterpretation (artifact output) ---

type LLMInterpretation struct {
	SchemaVersion       string               `json:"schema_version"`
	RunID               string               `json:"run_id"`
	CreatedAt           time.Time            `json:"created_at"`
	Overview            *ServiceDoc          `json:"overview,omitempty"`
	DataModels          []DataModelDoc       `json:"data_models,omitempty"`
	Flows               []Flow               `json:"flows,omitempty"`
	SecurityAssessment  *SecurityAssessment  `json:"security_assessment,omitempty"`
	BehaviorAssessment  *BehaviorAssessment  `json:"behavior_assessment,omitempty"`
	Facts               []Fact               `json:"facts"`
	Inferences          []Inference          `json:"inferences"`
	Improvements        []Improvement        `json:"improvements,omitempty"`
	Tests               []TestSuggestion     `json:"tests,omitempty"`
	OpenQuestions       []OpenQuestion       `json:"open_questions"`
	GuidedTour          []GuidedStep         `json:"guided_tour"`
	ScenarioSuggestions []ScenarioSuggestion `json:"scenario_suggestions"`
	Stats               Stats                `json:"stats"`
}

type DataModelDoc struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Fields      []string `json:"fields"`
	UsedBy      []string `json:"used_by"`
}

type SecurityAssessment struct {
	OverallRisk      string   `json:"overall_risk"`
	Summary          string   `json:"summary"`
	CriticalFindings []string `json:"critical_findings"`
	PositiveFindings []string `json:"positive_findings"`
	AttackSurface    string   `json:"attack_surface"`
}

type BehaviorAssessment struct {
	InputValidation string `json:"input_validation"`
	AuthEnforcement string `json:"auth_enforcement"`
	ErrorHandling   string `json:"error_handling"`
	Robustness      string `json:"robustness"`
}

type ServiceDoc struct {
	ServiceName       string `json:"service_name"`
	Description       string `json:"description"`
	Framework         string `json:"framework"`
	TotalEndpoints    int    `json:"total_endpoints"`
	ArchitectureNotes string `json:"architecture_notes"`
}

type Flow struct {
	Name            string           `json:"name"`
	Description     string           `json:"description"`
	Importance      string           `json:"importance"`
	Endpoints       []string         `json:"endpoints"`
	Sequence        string           `json:"sequence"`
	ExampleRequests []FlowExample    `json:"example_requests,omitempty"`
	EvidenceRefs    []string         `json:"evidence_refs,omitempty"`
}

type FlowExample struct {
	Step                    string         `json:"step"`
	Method                  string         `json:"method"`
	Path                    string         `json:"path"`
	Headers                 map[string]string `json:"headers,omitempty"`
	Body                    json.RawMessage   `json:"body,omitempty"`
	ExpectedStatus          int            `json:"expected_status"`
	ExpectedResponseSnippet json.RawMessage `json:"expected_response_snippet,omitempty"`
	Notes                   string         `json:"notes,omitempty"`
}

type Improvement struct {
	Title        string   `json:"title"`
	Severity     string   `json:"severity"`
	Category     string   `json:"category"`
	Description  string   `json:"description"`
	Remediation  string   `json:"remediation"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
	FindingRefs  []string `json:"finding_refs,omitempty"`
}

type TestSuggestion struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Flow        string       `json:"flow"`
	Importance  string       `json:"importance"`
	Request     TestRequest  `json:"request"`
	Expected    TestExpected `json:"expected"`
	EvidenceRefs []string    `json:"evidence_refs,omitempty"`
}

type TestRequest struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    json.RawMessage   `json:"body,omitempty"`
}

type TestExpected struct {
	Status       int      `json:"status"`
	BodyContains []string `json:"body_contains,omitempty"`
	Description  string   `json:"description"`
}

type Fact struct {
	ID           string           `json:"id"`
	Text         string           `json:"text"`
	EvidenceRefs []string         `json:"evidence_refs,omitempty"`
	FindingRefs  []string         `json:"finding_refs,omitempty"`
	ModelRefs    []shared.ModelRef `json:"model_refs,omitempty"`
	Confidence   float64          `json:"confidence"`
}

type Inference struct {
	ID               string           `json:"id"`
	Text             string           `json:"text"`
	RuleOfInference  string           `json:"rule_of_inference"`
	EvidenceRefs     []string         `json:"evidence_refs,omitempty"`
	FindingRefs      []string         `json:"finding_refs,omitempty"`
	ModelRefs        []shared.ModelRef `json:"model_refs,omitempty"`
	Confidence       float64          `json:"confidence"`
}

type OpenQuestion struct {
	ID             string              `json:"id"`
	Question       string              `json:"question"`
	WhyMissing     string              `json:"why_missing,omitempty"`
	SuggestedProbe *ScenarioSuggestion `json:"suggested_probe,omitempty"`
}

type GuidedStep struct {
	ID     string    `json:"id"`
	Title  string    `json:"title"`
	Action string    `json:"action"`
	Target StepTarget `json:"target"`
	Refs   StepRefs  `json:"refs"`
}

type StepTarget struct {
	EndpointKey string `json:"endpoint_key,omitempty"`
	FindingID   string `json:"finding_id,omitempty"`
	EvidenceID  string `json:"evidence_id,omitempty"`
}

type StepRefs struct {
	EvidenceRefs []string         `json:"evidence_refs,omitempty"`
	FindingRefs  []string         `json:"finding_refs,omitempty"`
	ModelRefs    []shared.ModelRef `json:"model_refs,omitempty"`
}

type ScenarioSuggestion struct {
	ID                  string            `json:"id"`
	Name                string            `json:"name"`
	TargetEndpointKey   string            `json:"target_endpoint_key"`
	RequestPatch        RequestPatch      `json:"request_patch"`
	ExpectedObservation string            `json:"expected_observation"`
	Refs                StepRefs          `json:"refs"`
}

type RequestPatch struct {
	Headers    map[string]string `json:"headers,omitempty"`
	Query      map[string]string `json:"query,omitempty"`
	PathParams map[string]string `json:"path_params,omitempty"`
	BodyJSON   json.RawMessage   `json:"body_json,omitempty"`
}

type Stats struct {
	FactsCount          int    `json:"facts_count"`
	InferencesCount     int    `json:"inferences_count"`
	QuestionsCount      int    `json:"questions_count"`
	RejectedClaimsCount int    `json:"rejected_claims_count"`
	ProviderName        string `json:"provider_name"`
	ValidationMode      string `json:"validation_mode"`
}

// --- Helpers for deterministic IDs ---

func DeterministicID(text string, evidenceRefs, findingRefs []string, modelRefs []shared.ModelRef) string {
	parts := []string{text}
	erefs := make([]string, len(evidenceRefs))
	copy(erefs, evidenceRefs)
	sort.Strings(erefs)
	parts = append(parts, erefs...)

	frefs := make([]string, len(findingRefs))
	copy(frefs, findingRefs)
	sort.Strings(frefs)
	parts = append(parts, frefs...)

	for _, mr := range modelRefs {
		parts = append(parts, mr.Kind+":"+mr.ID+":"+mr.File)
	}
	return shared.SHA256Bytes([]byte(strings.Join(parts, "|")))[:16]
}

func HasAnyRef(evidenceRefs, findingRefs []string, modelRefs []shared.ModelRef) bool {
	return len(evidenceRefs) > 0 || len(findingRefs) > 0 || len(modelRefs) > 0
}
