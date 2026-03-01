package domain

import (
	"fmt"
	"time"
)

// RunConfig holds budget and execution configuration for a run.
type RunConfig struct {
	RunSeed        string         `json:"run_seed"`
	ChaosSeed      string         `json:"chaos_seed,omitempty"`
	Budget         Budget         `json:"budget"`
	PriorityOrder  []string       `json:"priority_order"`
	StopConditions StopConditions `json:"stop_conditions"`
}

type Budget struct {
	MaxRequestsTotal       int                `json:"max_requests_total"`
	MaxRequestsPerEndpoint int                `json:"max_requests_per_endpoint"`
	MaxRequestsPerCategory map[string]int     `json:"max_requests_per_category"`
	MaxDurationSeconds     int                `json:"max_duration_seconds"`
	MaxConcurrentRequests  int                `json:"max_concurrent_requests"`
}

type StopConditions struct {
	StopIfNoBaseURL          bool `json:"stop_if_no_base_url"`
	StopIfAllRequestsTimeout bool `json:"stop_if_all_requests_timeout"`
}

// DefaultRunConfig returns a sensible default configuration.
func DefaultRunConfig(seed string) RunConfig {
	return RunConfig{
		RunSeed: seed,
		Budget: Budget{
			MaxRequestsTotal:       5000,
			MaxRequestsPerEndpoint: 120,
			MaxRequestsPerCategory: map[string]int{
				"preflight":   30,
				"smoke":       120,
				"auth":        200,
				"guided_fuzz": 2000,
				"logic":       1200,
				"abuse":       600,
				"confirm":     850,
			},
			MaxDurationSeconds:    900,
			MaxConcurrentRequests: 10,
		},
		PriorityOrder: []string{
			"preflight", "discovery", "schema", "smoke",
			"auth", "guided_fuzz", "logic", "abuse", "confirm", "report",
		},
		StopConditions: StopConditions{
			StopIfNoBaseURL:          true,
			StopIfAllRequestsTimeout: true,
		},
	}
}

// BudgetTracker monitors resource consumption during a run.
type BudgetTracker struct {
	Config     Budget
	startTime  time.Time
	requests   int
	byEndpoint map[string]int
	byCategory map[string]int
}

func NewBudgetTracker(cfg Budget) *BudgetTracker {
	return &BudgetTracker{
		Config:     cfg,
		startTime:  time.Now(),
		byEndpoint: make(map[string]int),
		byCategory: make(map[string]int),
	}
}

func (b *BudgetTracker) CanRequest(endpointID, category string) bool {
	if b.requests >= b.Config.MaxRequestsTotal {
		return false
	}
	if b.byEndpoint[endpointID] >= b.Config.MaxRequestsPerEndpoint {
		return false
	}
	if max, ok := b.Config.MaxRequestsPerCategory[category]; ok && b.byCategory[category] >= max {
		return false
	}
	if b.Config.MaxDurationSeconds > 0 && int(time.Since(b.startTime).Seconds()) >= b.Config.MaxDurationSeconds {
		return false
	}
	return true
}

func (b *BudgetTracker) Record(endpointID, category string) {
	b.requests++
	b.byEndpoint[endpointID]++
	b.byCategory[category]++
}

func (b *BudgetTracker) Usage() BudgetUsage {
	return BudgetUsage{
		RequestsTotal:   b.requests,
		DurationSeconds: int(time.Since(b.startTime).Seconds()),
		ByCategory:      copyMap(b.byCategory),
	}
}

func (b *BudgetTracker) Exhausted() bool {
	if b.requests >= b.Config.MaxRequestsTotal {
		return true
	}
	if b.Config.MaxDurationSeconds > 0 && int(time.Since(b.startTime).Seconds()) >= b.Config.MaxDurationSeconds {
		return true
	}
	return false
}

type BudgetUsage struct {
	RequestsTotal   int            `json:"requests_total"`
	DurationSeconds int            `json:"duration_seconds"`
	ByCategory      map[string]int `json:"by_category,omitempty"`
}

func copyMap(m map[string]int) map[string]int {
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// TargetProfile is the output of Step 0 (Preflight).
type TargetProfile struct {
	SchemaVersion        string            `json:"schema_version"`
	BaseURL              string            `json:"base_url"`
	BasePaths            []string          `json:"base_paths,omitempty"`
	VersioningHint       string            `json:"versioning_hint,omitempty"`
	ObservedContentTypes ContentTypes      `json:"observed_content_types"`
	FrameworkGuess       string            `json:"framework_guess,omitempty"`
	AuthHints            AuthHints         `json:"auth_hints,omitempty"`
	TimeoutDefaults      TimeoutDefaults   `json:"timeout_defaults"`
	HealthEndpoints      []string          `json:"health_endpoints,omitempty"`
	Redirects            []string          `json:"redirects,omitempty"`
	SupportsGzip         bool              `json:"supports_gzip"`
	SupportsCookies      bool              `json:"supports_cookies"`
}

type ContentTypes struct {
	Consumes []string `json:"consumes,omitempty"`
	Produces []string `json:"produces,omitempty"`
}

type AuthHints struct {
	HeadersSeen    []string `json:"headers_seen,omitempty"`
	MiddlewareNames []string `json:"middleware_names,omitempty"`
	LoginRoutes    []string `json:"login_routes,omitempty"`
	Mechanisms     []string `json:"mechanisms,omitempty"`
}

type TimeoutDefaults struct {
	ConnectMs int `json:"connect_ms"`
	ReadMs    int `json:"read_ms"`
}

// RunStatus tracks the overall state of a run.
type RunStatus string

const (
	RunCompleted RunStatus = "completed"
	RunPartial   RunStatus = "partial"
	RunFailed    RunStatus = "failed"
)

// RunMode classifies the quality/availability of runtime evidence.
type RunMode string

const (
	RunModeOffline       RunMode = "offline"
	RunModeOnlinePartial RunMode = "online_partial"
	RunModeOnlineGood    RunMode = "online_good"
	RunModeOnlineStrong  RunMode = "online_strong"
)

// RunModeClassification holds the classified mode plus diagnostics.
type RunModeClassification struct {
	Mode                RunMode  `json:"mode"`
	TotalSamples        int      `json:"total_samples"`
	HTTPResponses       int      `json:"http_responses"`
	ConnectionErrors    int      `json:"connection_errors"`
	ConnectionErrorPct  float64  `json:"connection_error_pct"`
	HappyPathEndpoints  int      `json:"happy_path_endpoints"`
	RealErrors          int      `json:"real_errors"`
	ConfirmedFindings   int      `json:"confirmed_findings"`
	Reason              string   `json:"reason"`
}

// ClassifyRunMode implements the quality gates.
func ClassifyRunMode(samples []EvidenceSample, smoke []SmokeResult, confirmations []Confirmation) RunModeClassification {
	c := RunModeClassification{TotalSamples: len(samples)}

	for _, s := range samples {
		if s.Response != nil {
			c.HTTPResponses++
			if s.Response.Status >= 400 && s.Response.BodySnippet != "" {
				c.RealErrors++
			}
		}
		if s.Error != "" {
			c.ConnectionErrors++
		}
	}

	if c.TotalSamples > 0 {
		c.ConnectionErrorPct = float64(c.ConnectionErrors) / float64(c.TotalSamples) * 100
	}

	happyEndpoints := make(map[string]bool)
	for _, s := range smoke {
		if s.Passed && s.StatusCode >= 200 && s.StatusCode < 300 {
			happyEndpoints[s.EndpointID] = true
		}
	}
	c.HappyPathEndpoints = len(happyEndpoints)

	for _, conf := range confirmations {
		if conf.Classification == ClassConfirmed {
			c.ConfirmedFindings++
		}
	}

	// Gate: OFFLINE
	if c.HTTPResponses == 0 || c.ConnectionErrorPct >= 80 {
		c.Mode = RunModeOffline
		if c.HTTPResponses == 0 {
			c.Reason = "No valid HTTP responses received. Service appears to be down."
		} else {
			c.Reason = fmt.Sprintf("%.0f%% connection errors (%d/%d). Service mostly unreachable.",
				c.ConnectionErrorPct, c.ConnectionErrors, c.TotalSamples)
		}
		return c
	}

	// Gate: ONLINE_GOOD requires:
	//   >= 10 HTTP responses, >= 3 happy-path endpoints, >= 1 real error
	if c.HTTPResponses >= 10 && c.HappyPathEndpoints >= 3 && c.RealErrors >= 1 {
		if c.ConfirmedFindings > 0 {
			c.Mode = RunModeOnlineStrong
			c.Reason = fmt.Sprintf("Strong evidence: %d HTTP responses, %d happy endpoints, %d confirmed findings.",
				c.HTTPResponses, c.HappyPathEndpoints, c.ConfirmedFindings)
		} else {
			c.Mode = RunModeOnlineGood
			c.Reason = fmt.Sprintf("Good evidence: %d HTTP responses, %d happy endpoints, %d errors captured.",
				c.HTTPResponses, c.HappyPathEndpoints, c.RealErrors)
		}
		return c
	}

	c.Mode = RunModeOnlinePartial
	var missing []string
	if c.HTTPResponses < 10 {
		missing = append(missing, fmt.Sprintf("need >=10 HTTP responses, have %d", c.HTTPResponses))
	}
	if c.HappyPathEndpoints < 3 {
		missing = append(missing, fmt.Sprintf("need >=3 happy-path endpoints, have %d", c.HappyPathEndpoints))
	}
	if c.RealErrors < 1 {
		missing = append(missing, "need >=1 real error captured")
	}
	c.Reason = "Partial evidence: " + joinStrings(missing, "; ") + "."
	return c
}

func joinStrings(ss []string, sep string) string {
	r := ""
	for i, s := range ss {
		if i > 0 {
			r += sep
		}
		r += s
	}
	return r
}

// PipelineStep identifies a step in the analysis pipeline.
type PipelineStep string

const (
	StepPreflight  PipelineStep = "preflight"
	StepDiscovery  PipelineStep = "discovery"
	StepSchema     PipelineStep = "schema"
	StepAnnotation PipelineStep = "annotation"
	StepSmoke      PipelineStep = "smoke"
	StepAuth       PipelineStep = "auth"
	StepFuzz       PipelineStep = "fuzz"
	StepLogic      PipelineStep = "logic"
	StepAbuse      PipelineStep = "abuse"
	StepConfirm    PipelineStep = "confirm"
	StepReport     PipelineStep = "report"
)

// StepResult is the output of any pipeline step.
type StepResult struct {
	Step          PipelineStep `json:"step"`
	Status        string       `json:"status"` // ok, partial, skipped, failed
	DurationMs    int64        `json:"duration_ms"`
	BudgetUsed    int          `json:"budget_used"`
	ArtifactsKeys []string     `json:"artifacts_keys,omitempty"`
	Error         string       `json:"error,omitempty"`
}
