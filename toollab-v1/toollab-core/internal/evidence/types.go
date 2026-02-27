package evidence

import "time"

type Bundle struct {
	SchemaVersion            int                 `json:"schema_version"`
	Metadata                 Metadata            `json:"metadata"`
	ScenarioFingerprint      ScenarioFingerprint `json:"scenario_fingerprint"`
	Execution                Execution           `json:"execution"`
	Stats                    Stats               `json:"stats"`
	Outcomes                 []Outcome           `json:"outcomes"`
	Samples                  []Sample            `json:"samples"`
	Observability            *Observability      `json:"observability,omitempty"`
	Assertions               Assertions          `json:"assertions"`
	Unknowns                 []string            `json:"unknowns"`
	RedactionSummary         RedactionSummary    `json:"redaction_summary"`
	Repro                    Repro               `json:"repro"`
	Environment              *Environment        `json:"environment,omitempty"`
	DeterministicFingerprint string              `json:"deterministic_fingerprint"`
}

type Metadata struct {
	ToollabVersion   string `json:"toollab_version"`
	Mode            string `json:"mode"`
	RunID           string `json:"run_id"`
	RunSeed         string `json:"run_seed"`
	ChaosSeed       string `json:"chaos_seed"`
	StartedUTC      string `json:"started_utc,omitempty"`
	FinishedUTC     string `json:"finished_utc,omitempty"`
	DBSeedReference string `json:"db_seed_reference,omitempty"`
}

type ScenarioFingerprint struct {
	ScenarioPath          string `json:"scenario_path"`
	ScenarioSHA256        string `json:"scenario_sha256"`
	ScenarioSchemaVersion int    `json:"scenario_schema_version"`
}

type Execution struct {
	ScheduleMode          string `json:"schedule_mode"`
	TickMS                int    `json:"tick_ms,omitempty"`
	Concurrency           int    `json:"concurrency"`
	DurationS             int    `json:"duration_s"`
	PlannedRequests       int    `json:"planned_requests"`
	CompletedRequests     int    `json:"completed_requests"`
	DecisionEngineVersion string `json:"decision_engine_version"`
	DecisionTapeHash      string `json:"decision_tape_hash"`
}

type Stats struct {
	TotalRequests   int            `json:"total_requests"`
	SuccessRate     float64        `json:"success_rate"`
	ErrorRate       float64        `json:"error_rate"`
	P50MS           int            `json:"p50_ms"`
	P95MS           int            `json:"p95_ms"`
	P99MS           int            `json:"p99_ms"`
	StatusHistogram map[string]int `json:"status_histogram"`
}

type Outcome struct {
	Seq          int64        `json:"seq"`
	RequestID    string       `json:"request_id"`
	Method       string       `json:"method"`
	Path         string       `json:"path"`
	StatusCode   *int         `json:"status_code,omitempty"`
	ErrorKind    string       `json:"error_kind"`
	LatencyMS    int          `json:"latency_ms"`
	ResponseHash string       `json:"response_hash"`
	ChaosApplied ChaosApplied `json:"chaos_applied"`
}

type ChaosApplied struct {
	LatencyInjectedMS   int      `json:"latency_injected_ms"`
	ErrorInjected       bool     `json:"error_injected"`
	ErrorMode           string   `json:"error_mode"`
	PayloadDriftApplied bool     `json:"payload_drift_applied"`
	PayloadMutations    []string `json:"payload_mutations"`
}

type Sample struct {
	Seq      int64          `json:"seq"`
	Request  SampleRequest  `json:"request"`
	Response SampleResponse `json:"response"`
}

type SampleRequest struct {
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	Headers     map[string]string `json:"headers"`
	BodyPreview string            `json:"body_preview"`
	BodySHA256  string            `json:"body_sha256"`
	Redacted    bool              `json:"redacted"`
}

type SampleResponse struct {
	StatusCode  *int              `json:"status_code"`
	ErrorKind   string            `json:"error_kind"`
	Headers     map[string]string `json:"headers"`
	BodyPreview string            `json:"body_preview"`
	BodySHA256  string            `json:"body_sha256"`
	Redacted    bool              `json:"redacted"`
}

type Observability struct {
	MetricsSnapshot map[string]any `json:"metrics_snapshot,omitempty"`
	TraceRefs       []string       `json:"trace_refs,omitempty"`
	LogsExcerpt     []LogLine      `json:"logs_excerpt,omitempty"`
}

type LogLine struct {
	Timestamp string         `json:"timestamp"`
	Level     string         `json:"level"`
	Message   string         `json:"message"`
	Attrs     map[string]any `json:"attrs,omitempty"`
}

type Assertions struct {
	Overall       string       `json:"overall"`
	Rules         []RuleResult `json:"rules"`
	ViolatedRules []string     `json:"violated_rules"`
}

type RuleResult struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Passed   bool   `json:"passed"`
	Observed any    `json:"observed"`
	Expected any    `json:"expected"`
	Message  string `json:"message"`
}

type RedactionSummary struct {
	HeadersRedacted     []string `json:"headers_redacted"`
	JSONPathsRedacted   []string `json:"json_paths_redacted"`
	Mask                string   `json:"mask"`
	MaxBodyPreviewBytes int      `json:"max_body_preview_bytes"`
	MaxSamples          int      `json:"max_samples"`
}

type Repro struct {
	Command                          string `json:"command"`
	ScriptPath                       string `json:"script_path"`
	ExpectedDeterministicFingerprint string `json:"expected_deterministic_fingerprint"`
}

type Environment struct {
	GoVersion  string         `json:"go_version,omitempty"`
	OS         string         `json:"os,omitempty"`
	Arch       string         `json:"arch,omitempty"`
	HTTPClient map[string]any `json:"http_client,omitempty"`
}

type CollectInput struct {
	ScenarioPath          string
	ScenarioSHA256        string
	ScenarioSchemaVersion int
	ToollabVersion         string
	Mode                  string
	RunSeed               string
	ChaosSeed             string
	DBSeedReference       string
	ScheduleMode          string
	TickMS                int
	Concurrency           int
	DurationS             int
	PlannedRequests       int
	CompletedRequests     int
	DecisionEngineVersion string
	DecisionTapeHash      string
	Outcomes              []OutcomeInput
	Unknowns              []string
	Assertions            Assertions
	StartedAt             time.Time
	FinishedAt            time.Time
	ReproCommand          string
	ReproScriptPath       string
	Redaction             RedactionSummary
	Observability         *Observability
}

type OutcomeInput struct {
	Seq             int64
	RequestID       string
	Method          string
	Path            string
	StatusCode      *int
	ErrorKind       string
	LatencyMS       int
	ResponseHash    string
	ChaosApplied    ChaosApplied
	RequestURL      string
	RequestHeaders  map[string]string
	RequestBody     []byte
	ResponseHeaders map[string]string
	ResponseBody    []byte
}
