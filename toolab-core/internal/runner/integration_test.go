package runner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"toolab-core/internal/determinism"
	"toolab-core/internal/scenario"
)

func TestBaseExecutor_DeterministicOutcomesBySeq(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not_found"}`))
	}))
	defer srv.Close()

	s := &scenario.Scenario{
		Version: 1,
		Mode:    "black",
		Target: scenario.Target{
			BaseURL: srv.URL,
			Headers: map[string]string{},
			Auth:    scenario.Auth{Type: "none"},
		},
		Workload: scenario.Workload{
			Requests: []scenario.RequestSpec{
				{ID: "health", Method: "GET", Path: "/health", Body: ptr(""), TimeoutMS: 2000, Weight: 1, Query: map[string]string{}, Headers: map[string]string{}},
				{ID: "missing", Method: "GET", Path: "/missing", Body: ptr(""), TimeoutMS: 2000, Weight: 1, Query: map[string]string{}, Headers: map[string]string{}},
			},
			Concurrency:  3,
			DurationS:    2,
			ScheduleMode: "closed_loop",
		},
		Chaos:        scenario.Chaos{Latency: scenario.LatencyConfig{Mode: "none"}, ErrorRate: 0, ErrorStatus: []int{503}, ErrorMode: "abort"},
		Expectations: scenario.Expectations{MaxErrorRate: 1, MaxP95MS: 5000, Invariants: []scenario.InvariantConfig{}},
		Seeds:        scenario.Seeds{RunSeed: "42", ChaosSeed: "99"},
		Redaction:    scenario.RedactionConfig{Headers: []string{"authorization"}, JSONPaths: []string{}, Mask: "***", MaxBodyPreviewBytes: 1024, MaxSamples: 10},
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

	execA := NewBaseExecutor(s, planA, nil)
	execB := NewBaseExecutor(s, planB, nil)
	outA, err := execA.Execute(context.Background())
	if err != nil {
		t.Fatalf("execute A: %v", err)
	}
	outB, err := execB.Execute(context.Background())
	if err != nil {
		t.Fatalf("execute B: %v", err)
	}

	if len(outA) != len(outB) {
		t.Fatalf("outcome length mismatch")
	}

	for i := range outA {
		a := outA[i]
		b := outB[i]
		if a.Seq != b.Seq {
			t.Fatalf("seq mismatch at %d: %d != %d", i, a.Seq, b.Seq)
		}
		if a.RequestID != b.RequestID || a.ErrorKind != b.ErrorKind || a.ResponseHash != b.ResponseHash {
			t.Fatalf("deterministic mismatch at seq=%d\nA=%s/%s/%s\nB=%s/%s/%s", a.Seq, a.RequestID, a.ErrorKind, a.ResponseHash, b.RequestID, b.ErrorKind, b.ResponseHash)
		}
		if a.StatusCode != nil && b.StatusCode != nil && *a.StatusCode != *b.StatusCode {
			t.Fatalf("status mismatch at seq %d", a.Seq)
		}
	}

	for i, out := range outA {
		expectedSeq := int64(i)
		if out.Seq != expectedSeq {
			t.Fatalf("outcomes not sorted by seq: idx=%d seq=%d", i, out.Seq)
		}
	}

	if len(outA) == 0 {
		t.Fatal("expected outcomes")
	}
	_ = fmt.Sprintf("%d", outA[0].LatencyMS)
}

func ptr(s string) *string {
	return &s
}
