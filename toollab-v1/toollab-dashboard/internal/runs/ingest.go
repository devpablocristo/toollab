package runs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type evidenceBundle struct {
	Metadata struct {
		RunID     string `json:"run_id"`
		RunSeed   string `json:"run_seed"`
		ChaosSeed string `json:"chaos_seed"`
		StartedUTC  string `json:"started_utc"`
		FinishedUTC string `json:"finished_utc"`
	} `json:"metadata"`
	Execution struct {
		Concurrency       int `json:"concurrency"`
		DurationS         int `json:"duration_s"`
		PlannedRequests   int `json:"planned_requests"`
		CompletedRequests int `json:"completed_requests"`
	} `json:"execution"`
	Stats struct {
		TotalRequests   int            `json:"total_requests"`
		SuccessRate     float64        `json:"success_rate"`
		ErrorRate       float64        `json:"error_rate"`
		P50MS           int            `json:"p50_ms"`
		P95MS           int            `json:"p95_ms"`
		P99MS           int            `json:"p99_ms"`
		StatusHistogram map[string]int `json:"status_histogram"`
	} `json:"stats"`
	Assertions struct {
		Overall string `json:"overall"`
		Rules   []struct {
			ID       string `json:"id"`
			Type     string `json:"type"`
			Passed   bool   `json:"passed"`
			Observed any    `json:"observed"`
			Expected any    `json:"expected"`
			Message  string `json:"message"`
		} `json:"rules"`
	} `json:"assertions"`
	DeterministicFingerprint string `json:"deterministic_fingerprint"`
}

func IngestFromDir(store *Store, runDir string, targetID string) (*Run, error) {
	evidencePath := filepath.Join(runDir, "evidence.json")
	raw, err := os.ReadFile(evidencePath)
	if err != nil {
		return nil, fmt.Errorf("read evidence: %w", err)
	}

	var bundle evidenceBundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		return nil, fmt.Errorf("parse evidence: %w", err)
	}

	statusHist, _ := json.Marshal(bundle.Stats.StatusHistogram)
	startedAt, _ := time.Parse(time.RFC3339, bundle.Metadata.StartedUTC)
	finishedAt, _ := time.Parse(time.RFC3339, bundle.Metadata.FinishedUTC)

	absDir, _ := filepath.Abs(runDir)

	r := &Run{
		ID:                       bundle.Metadata.RunID,
		TargetID:                 targetID,
		RunSeed:                  bundle.Metadata.RunSeed,
		ChaosSeed:                bundle.Metadata.ChaosSeed,
		Verdict:                  bundle.Assertions.Overall,
		TotalRequests:            bundle.Stats.TotalRequests,
		CompletedRequests:        bundle.Execution.CompletedRequests,
		SuccessRate:              bundle.Stats.SuccessRate,
		ErrorRate:                bundle.Stats.ErrorRate,
		P50MS:                    bundle.Stats.P50MS,
		P95MS:                    bundle.Stats.P95MS,
		P99MS:                    bundle.Stats.P99MS,
		DurationS:                bundle.Execution.DurationS,
		Concurrency:              bundle.Execution.Concurrency,
		StatusHistogram:          string(statusHist),
		DeterministicFingerprint: bundle.DeterministicFingerprint,
		GoldenRunDir:             absDir,
		StartedAt:                startedAt,
		FinishedAt:               finishedAt,
		CreatedAt:                time.Now().UTC(),
	}

	if err := store.Insert(r); err != nil {
		return nil, fmt.Errorf("insert run: %w", err)
	}

	for _, rule := range bundle.Assertions.Rules {
		obs, _ := json.Marshal(rule.Observed)
		exp, _ := json.Marshal(rule.Expected)
		a := &AssertionResult{
			RunID:    r.ID,
			RuleID:   rule.ID,
			RuleType: rule.Type,
			Passed:   rule.Passed,
			Observed: string(obs),
			Expected: string(exp),
			Message:  rule.Message,
		}
		if err := store.InsertAssertion(a); err != nil {
			return nil, fmt.Errorf("insert assertion %s: %w", rule.ID, err)
		}
	}

	return r, nil
}
