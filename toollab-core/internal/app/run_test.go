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

func TestRunScenarioReproducibleTapeHash(t *testing.T) {
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
  concurrency: 1
  duration_s: 1
  schedule_mode: closed_loop
chaos:
  latency:
    mode: none
  error_rate: 0
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
redaction:
  headers: ["authorization", "cookie", "set-cookie", "x-api-key", "date"]
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
	if len(resA.Bundle.DeterministicFingerprint) != 64 || len(resB.Bundle.DeterministicFingerprint) != 64 {
		t.Fatalf("invalid deterministic fingerprint length")
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

func TestPipelineRunExplainDiff(t *testing.T) {
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

	scenarioA := fmt.Sprintf(`
version: 1
mode: black
target:
  base_url: %s
  headers: {}
  auth: { type: none }
workload:
  requests:
    - id: get_health
      method: GET
      path: /health
      body: ""
      weight: 1
  concurrency: 1
  duration_s: 1
  schedule_mode: closed_loop
chaos:
  latency: { mode: none }
  error_rate: 0
  error_statuses: [503]
  error_mode: abort
expectations:
  max_error_rate: 1
  max_p95_ms: 2000
  invariants:
    - type: no_5xx_allowed
seeds:
  run_seed: "1"
  chaos_seed: "2"
redaction:
  headers: ["authorization", "cookie", "set-cookie", "x-api-key"]
`, srv.URL)

	scenarioB := fmt.Sprintf(`
version: 1
mode: black
target:
  base_url: %s
  headers: {}
  auth: { type: none }
workload:
  requests:
    - id: get_health
      method: GET
      path: /health
      body: ""
      weight: 1
  concurrency: 1
  duration_s: 2
  schedule_mode: closed_loop
chaos:
  latency: { mode: none }
  error_rate: 0
  error_statuses: [503]
  error_mode: abort
expectations:
  max_error_rate: 1
  max_p95_ms: 2000
  invariants:
    - type: no_5xx_allowed
seeds:
  run_seed: "1"
  chaos_seed: "2"
redaction:
  headers: ["authorization", "cookie", "set-cookie", "x-api-key"]
`, srv.URL)

	tmp := t.TempDir()
	scenarioAPath := filepath.Join(tmp, "a.yaml")
	scenarioBPath := filepath.Join(tmp, "b.yaml")
	if err := os.WriteFile(scenarioAPath, []byte(scenarioA), 0o644); err != nil {
		t.Fatalf("write scenario A: %v", err)
	}
	if err := os.WriteFile(scenarioBPath, []byte(scenarioB), 0o644); err != nil {
		t.Fatalf("write scenario B: %v", err)
	}

	outBase := filepath.Join(tmp, "runs")
	runA, err := RunScenario(context.Background(), scenarioAPath, outBase)
	if err != nil {
		t.Fatalf("run scenario A: %v", err)
	}
	runB, err := RunScenario(context.Background(), scenarioBPath, outBase)
	if err != nil {
		t.Fatalf("run scenario B: %v", err)
	}

	explainOut := filepath.Join(runA.RunDir, "explain_out")
	explainRes, err := ExplainRun(context.Background(), ExplainConfig{
		RunDir: runA.RunDir,
		OutDir: explainOut,
	})
	if err != nil {
		t.Fatalf("explain run A: %v", err)
	}
	if explainRes.Fingerprint == "" {
		t.Fatalf("expected understanding fingerprint")
	}

	diffOut := filepath.Join(tmp, "diff")
	diffRes, err := DiffRuns(context.Background(), DiffConfig{
		RunADir: runA.RunDir,
		RunBDir: runB.RunDir,
		OutDir:  diffOut,
	})
	if err != nil {
		t.Fatalf("diff runs: %v", err)
	}
	if diffRes.Fingerprint == "" {
		t.Fatalf("expected diff fingerprint")
	}
}
