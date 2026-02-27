package runs

import "time"

type Run struct {
	ID                       string    `json:"id"`
	ScenarioID               *string   `json:"scenario_id,omitempty"`
	TargetID                 string    `json:"target_id"`
	RunSeed                  string    `json:"run_seed"`
	ChaosSeed                string    `json:"chaos_seed"`
	Verdict                  string    `json:"verdict"`
	TotalRequests            int       `json:"total_requests"`
	CompletedRequests        int       `json:"completed_requests"`
	SuccessRate              float64   `json:"success_rate"`
	ErrorRate                float64   `json:"error_rate"`
	P50MS                    int       `json:"p50_ms"`
	P95MS                    int       `json:"p95_ms"`
	P99MS                    int       `json:"p99_ms"`
	DurationS                int       `json:"duration_s"`
	Concurrency              int       `json:"concurrency"`
	StatusHistogram          string    `json:"status_histogram"`
	DeterministicFingerprint string    `json:"deterministic_fingerprint"`
	GoldenRunDir             string    `json:"golden_run_dir"`
	StartedAt                time.Time `json:"started_at"`
	FinishedAt               time.Time `json:"finished_at"`
	CreatedAt                time.Time `json:"created_at"`
}

type AssertionResult struct {
	ID       int    `json:"id"`
	RunID    string `json:"run_id"`
	RuleID   string `json:"rule_id"`
	RuleType string `json:"rule_type"`
	Passed   bool   `json:"passed"`
	Observed string `json:"observed"`
	Expected string `json:"expected"`
	Message  string `json:"message"`
}

type RunDetail struct {
	Run          Run               `json:"run"`
	Assertions   []AssertionResult `json:"assertions"`
	Interpretation *string         `json:"interpretation,omitempty"`
}
