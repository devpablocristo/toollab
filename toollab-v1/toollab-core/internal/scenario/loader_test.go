package scenario

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadScenario_NormalizesDefaultsAndHashesStable(t *testing.T) {
	scenarioPath := filepath.Join("..", "..", "..", "testdata", "scenario", "valid", "minimal_open_loop.yaml")

	s1, fp1, err := Load(scenarioPath)
	if err != nil {
		t.Fatalf("first load failed: %v", err)
	}
	s2, fp2, err := Load(scenarioPath)
	if err != nil {
		t.Fatalf("second load failed: %v", err)
	}

	if fp1.ScenarioSHA != fp2.ScenarioSHA {
		t.Fatalf("scenario hash should be stable: %s != %s", fp1.ScenarioSHA, fp2.ScenarioSHA)
	}

	req := s1.Workload.Requests[0]
	if req.TimeoutMS != 5000 {
		t.Fatalf("expected timeout default 5000, got %d", req.TimeoutMS)
	}
	if req.Weight != 1 {
		t.Fatalf("expected weight default 1, got %d", req.Weight)
	}
	if s1.Seeds.RunSeed != "123" || s1.Seeds.ChaosSeed != "456" {
		t.Fatalf("unexpected normalized seeds: %+v", s1.Seeds)
	}

	if s2.Redaction.Mask == "" || s2.Redaction.MaxSamples == 0 {
		t.Fatalf("redaction defaults should be set")
	}
}

func TestLoadScenario_CanonicalHashIndependentOfYAMLKeyOrder(t *testing.T) {
	contentA := `
version: 1
mode: black
target:
  base_url: http://localhost:8080
  headers: {}
  auth: {type: none}
workload:
  schedule_mode: open_loop
  tick_ms: 100
  duration_s: 1
  concurrency: 1
  requests:
    - id: req
      method: GET
      path: /health
      body: ""
chaos:
  latency: {mode: none}
  error_rate: 0
  error_statuses: [503]
  error_mode: abort
expectations:
  max_error_rate: 0.1
  max_p95_ms: 100
  invariants: []
seeds:
  run_seed: "11"
  chaos_seed: "22"
`
	contentB := `
mode: black
version: 1
seeds:
  chaos_seed: "22"
  run_seed: "11"
expectations:
  invariants: []
  max_p95_ms: 100
  max_error_rate: 0.1
chaos:
  error_mode: abort
  error_statuses: [503]
  error_rate: 0
  latency: {mode: none}
workload:
  requests:
    - path: /health
      method: GET
      id: req
      body: ""
  concurrency: 1
  duration_s: 1
  tick_ms: 100
  schedule_mode: open_loop
target:
  auth: {type: none}
  headers: {}
  base_url: http://localhost:8080
`

	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.yaml")
	fileB := filepath.Join(dir, "b.yaml")
	if err := os.WriteFile(fileA, []byte(contentA), 0o644); err != nil {
		t.Fatalf("write file a: %v", err)
	}
	if err := os.WriteFile(fileB, []byte(contentB), 0o644); err != nil {
		t.Fatalf("write file b: %v", err)
	}

	_, fpA, err := Load(fileA)
	if err != nil {
		t.Fatalf("load file a: %v", err)
	}
	_, fpB, err := Load(fileB)
	if err != nil {
		t.Fatalf("load file b: %v", err)
	}

	if fpA.ScenarioSHA != fpB.ScenarioSHA {
		t.Fatalf("expected canonical hash equality, got %s and %s", fpA.ScenarioSHA, fpB.ScenarioSHA)
	}
}
