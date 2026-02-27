package scenario

import "encoding/json"

type Scenario struct {
	Version       int             `json:"version"`
	Mode          string          `json:"mode"`
	Target        Target          `json:"target"`
	Workload      Workload        `json:"workload"`
	Chaos         Chaos           `json:"chaos"`
	Expectations  Expectations    `json:"expectations"`
	Seeds         Seeds           `json:"seeds"`
	Observability *Observability  `json:"observability,omitempty"`
	Redaction     RedactionConfig `json:"redaction"`
}

type Target struct {
	BaseURL string            `json:"base_url"`
	Headers map[string]string `json:"headers"`
	Auth    Auth              `json:"auth"`
}

type Auth struct {
	Type           string `json:"type"`
	BearerTokenEnv string `json:"bearer_token_env,omitempty"`
	UsernameEnv    string `json:"username_env,omitempty"`
	PasswordEnv    string `json:"password_env,omitempty"`
	APIKeyEnv      string `json:"api_key_env,omitempty"`
	In             string `json:"in,omitempty"`
	Name           string `json:"name,omitempty"`
}

type Workload struct {
	Requests     []RequestSpec `json:"requests"`
	Concurrency  int           `json:"concurrency"`
	DurationS    int           `json:"duration_s"`
	ScheduleMode string        `json:"schedule_mode"`
	TickMS       int           `json:"tick_ms,omitempty"`
}

type RequestSpec struct {
	ID             string            `json:"id"`
	Method         string            `json:"method"`
	Path           string            `json:"path"`
	Query          map[string]string `json:"query"`
	Headers        map[string]string `json:"headers"`
	Body           *string           `json:"body,omitempty"`
	JSONBody       json.RawMessage   `json:"json_body,omitempty"`
	TimeoutMS      int               `json:"timeout_ms"`
	Weight         int               `json:"weight"`
	IdempotencyKey string            `json:"idempotency_key,omitempty"`
}

type Chaos struct {
	Latency      LatencyConfig       `json:"latency"`
	ErrorRate    float64             `json:"error_rate"`
	ErrorStatus  []int               `json:"error_statuses"`
	ErrorMode    string              `json:"error_mode"`
	Flapping     *FlappingConfig     `json:"flapping,omitempty"`
	PayloadDrift *PayloadDriftConfig `json:"payload_drift,omitempty"`
}

type LatencyConfig struct {
	Mode  string `json:"mode"`
	MS    int    `json:"ms,omitempty"`
	MinMS int    `json:"min_ms,omitempty"`
	MaxMS int    `json:"max_ms,omitempty"`
}

type FlappingConfig struct {
	Enabled       bool    `json:"enabled"`
	PeriodRequest int     `json:"period_requests,omitempty"`
	DownRatio     float64 `json:"down_ratio,omitempty"`
	Behavior      string  `json:"behavior,omitempty"`
}

type PayloadDriftConfig struct {
	Enabled          bool     `json:"enabled"`
	Rate             float64  `json:"rate,omitempty"`
	AllowedMutations []string `json:"allowed_mutations,omitempty"`
}

type Expectations struct {
	MaxErrorRate float64           `json:"max_error_rate"`
	MaxP95MS     int               `json:"max_p95_ms"`
	Invariants   []InvariantConfig `json:"invariants"`
}

type InvariantConfig struct {
	Type      string  `json:"type"`
	Max       float64 `json:"max,omitempty"`
	Status    int     `json:"status,omitempty"`
	RequestID string  `json:"request_id,omitempty"`
}

type Seeds struct {
	RunSeed         string `json:"run_seed"`
	ChaosSeed       string `json:"chaos_seed"`
	DBSeedReference string `json:"db_seed_reference,omitempty"`
}

type Observability struct {
	Metrics *MetricsConfig `json:"metrics,omitempty"`
	Traces  *TracesConfig  `json:"traces,omitempty"`
	Logs    *LogsConfig    `json:"logs,omitempty"`
}

type MetricsConfig struct {
	Endpoint string `json:"endpoint"`
	ScrapeAt string `json:"scrape_at"`
	Timeout  int    `json:"timeout_ms,omitempty"`
}

type TracesConfig struct {
	Enabled  bool   `json:"enabled"`
	Endpoint string `json:"endpoint,omitempty"`
	Query    string `json:"query,omitempty"`
	Timeout  int    `json:"timeout_ms,omitempty"`
}

type LogsConfig struct {
	Enabled  bool   `json:"enabled"`
	Source   string `json:"source,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	FilePath string `json:"file_path,omitempty"`
	Format   string `json:"format,omitempty"`
	MaxLines int    `json:"max_lines,omitempty"`
}

type RedactionConfig struct {
	Headers             []string `json:"headers"`
	JSONPaths           []string `json:"json_paths"`
	Mask                string   `json:"mask"`
	MaxBodyPreviewBytes int      `json:"max_body_preview_bytes"`
	MaxSamples          int      `json:"max_samples"`
}

type Fingerprint struct {
	ScenarioPath string `json:"scenario_path"`
	ScenarioSHA  string `json:"scenario_sha256"`
}

type rawScenario struct {
	Version       int             `yaml:"version" json:"version"`
	Mode          string          `yaml:"mode" json:"mode"`
	Target        Target          `yaml:"target" json:"target"`
	Workload      Workload        `yaml:"workload" json:"workload"`
	Chaos         Chaos           `yaml:"chaos" json:"chaos"`
	Expectations  Expectations    `yaml:"expectations" json:"expectations"`
	Seeds         rawSeeds        `yaml:"seeds" json:"seeds"`
	Observability *Observability  `yaml:"observability" json:"observability"`
	Redaction     RedactionConfig `yaml:"redaction" json:"redaction"`
}

type rawSeeds struct {
	RunSeed         any    `yaml:"run_seed" json:"run_seed"`
	ChaosSeed       any    `yaml:"chaos_seed" json:"chaos_seed"`
	DBSeedReference string `yaml:"db_seed_reference" json:"db_seed_reference"`
}
