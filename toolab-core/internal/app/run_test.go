package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRunScenarioReproducibleFingerprintAndTapeHash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not_found"}`))
	}))
	defer srv.Close()

	scenarioYAML := fmt.Sprintf(`
version: 1
mode: black
target:
  base_url: %s
  headers: {}
  auth:
    type: none
workload:
  requests:
    - id: get_health
      method: GET
      path: /health
      body: ""
      weight: 1
  concurrency: 2
  duration_s: 2
  schedule_mode: open_loop
  tick_ms: 100
chaos:
  latency:
    mode: uniform
    min_ms: 0
    max_ms: 10
  error_rate: 0.1
  error_statuses: [503]
  error_mode: abort
expectations:
  max_error_rate: 0.5
  max_p95_ms: 2000
  invariants:
    - type: no_5xx_allowed
seeds:
  run_seed: "123"
  chaos_seed: "456"
`, srv.URL)

	tmp := t.TempDir()
	scenarioPath := filepath.Join(tmp, "scenario.yaml")
	if err := os.WriteFile(scenarioPath, []byte(scenarioYAML), 0o644); err != nil {
		t.Fatalf("write scenario: %v", err)
	}

	outBase := filepath.Join(tmp, "runs")
	resA, err := RunScenario(context.Background(), scenarioPath, outBase)
	if err != nil {
		t.Fatalf("run A failed: %v", err)
	}
	resB, err := RunScenario(context.Background(), scenarioPath, outBase)
	if err != nil {
		t.Fatalf("run B failed: %v", err)
	}

	if resA.Bundle.Execution.DecisionTapeHash != resB.Bundle.Execution.DecisionTapeHash {
		t.Fatalf("decision tape hash mismatch")
	}
	if resA.Bundle.DeterministicFingerprint != resB.Bundle.DeterministicFingerprint {
		t.Fatalf("deterministic fingerprint mismatch")
	}

	if _, err := os.Stat(filepath.Join(resA.RunDir, "evidence.json")); err != nil {
		t.Fatalf("missing evidence artifact: %v", err)
	}
	if _, err := os.Stat(filepath.Join(resA.RunDir, "decision_tape.jsonl")); err != nil {
		t.Fatalf("missing decision tape artifact: %v", err)
	}
}

func TestRunScenarioMissingSeedsHardError(t *testing.T) {
	tmp := t.TempDir()
	scenarioPath := filepath.Join(tmp, "scenario.yaml")
	invalid := `
version: 1
mode: black
target:
  base_url: http://localhost:8080
  headers: {}
  auth:
    type: none
workload:
  requests:
    - id: x
      method: GET
      path: /
      body: ""
  concurrency: 1
  duration_s: 1
  schedule_mode: closed_loop
chaos:
  latency: {mode: none}
  error_rate: 0
  error_statuses: [503]
  error_mode: abort
expectations:
  max_error_rate: 1
  max_p95_ms: 1000
  invariants: []
`
	if err := os.WriteFile(scenarioPath, []byte(invalid), 0o644); err != nil {
		t.Fatalf("write invalid scenario: %v", err)
	}
	_, err := RunScenario(context.Background(), scenarioPath, filepath.Join(tmp, "run"))
	if err == nil {
		t.Fatalf("expected hard error for missing seeds")
	}
}
