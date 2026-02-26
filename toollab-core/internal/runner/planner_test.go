package runner

import (
	"path/filepath"
	"testing"

	"toollab-core/internal/determinism"
	"toollab-core/internal/scenario"
)

func TestBuildPlanOpenLoopDeterministic(t *testing.T) {
	s, _, err := scenario.Load(filepath.Join("..", "..", "..", "testdata", "scenario", "valid", "minimal_open_loop.yaml"))
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}

	deciderA, err := determinism.NewEngine(s.Seeds.RunSeed, "run_seed", nil)
	if err != nil {
		t.Fatalf("new decider A: %v", err)
	}
	deciderB, err := determinism.NewEngine(s.Seeds.RunSeed, "run_seed", nil)
	if err != nil {
		t.Fatalf("new decider B: %v", err)
	}

	planA, err := BuildPlan(s, deciderA)
	if err != nil {
		t.Fatalf("build plan A: %v", err)
	}
	planB, err := BuildPlan(s, deciderB)
	if err != nil {
		t.Fatalf("build plan B: %v", err)
	}

	if len(planA.PlannedRequests) != 100 {
		t.Fatalf("expected 100 planned requests, got %d", len(planA.PlannedRequests))
	}
	if len(planA.PlannedRequests) != len(planB.PlannedRequests) {
		t.Fatalf("plan lengths differ")
	}
	for i := range planA.PlannedRequests {
		if planA.PlannedRequests[i] != planB.PlannedRequests[i] {
			t.Fatalf("non deterministic plan at idx %d", i)
		}
	}
}
