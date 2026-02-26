package chaos

import (
	"encoding/json"
	"testing"

	"toollab-core/internal/determinism"
	"toollab-core/internal/scenario"
)

func TestChaosDeterministicBySeq(t *testing.T) {
	tapeA := determinism.NewTapeRecorder()
	tapeB := determinism.NewTapeRecorder()
	engA, _ := determinism.NewEngine("99", "chaos_seed", tapeA)
	engB, _ := determinism.NewEngine("99", "chaos_seed", tapeB)

	cfg := scenario.Chaos{
		Latency:     scenario.LatencyConfig{Mode: "uniform", MinMS: 10, MaxMS: 20},
		ErrorRate:   0.2,
		ErrorStatus: []int{503},
		ErrorMode:   "abort",
	}
	ca := NewEngine(cfg, engA)
	cb := NewEngine(cfg, engB)
	req := scenario.RequestSpec{ID: "x", JSONBody: json.RawMessage(`{"a":1}`)}
	body := []byte(`{"a":1}`)

	for i := int64(0); i < 20; i++ {
		da, _ := ca.Apply(req, i, body)
		db, _ := cb.Apply(req, i, body)
		if da.Abort != db.Abort || da.LatencyMS != db.LatencyMS {
			t.Fatalf("decision mismatch at seq=%d", i)
		}
	}

	hA, _ := tapeA.Hash()
	hB, _ := tapeB.Hash()
	if hA != hB {
		t.Fatalf("tape hash mismatch: %s vs %s", hA, hB)
	}
}

func TestChaosAbortModeWhenErrorRateOne(t *testing.T) {
	eng, _ := determinism.NewEngine("1", "chaos_seed", nil)
	cfg := scenario.Chaos{Latency: scenario.LatencyConfig{Mode: "none"}, ErrorRate: 1, ErrorMode: "abort"}
	c := NewEngine(cfg, eng)
	req := scenario.RequestSpec{ID: "x", JSONBody: json.RawMessage(`{"a":1}`)}

	d, _ := c.Apply(req, 0, []byte(`{"a":1}`))
	if !d.Abort || !d.ErrorInjected {
		t.Fatalf("expected abort with error injection")
	}
}

func TestChaosPayloadDriftDeterministic(t *testing.T) {
	eng, _ := determinism.NewEngine("10", "chaos_seed", nil)
	cfg := scenario.Chaos{
		Latency:   scenario.LatencyConfig{Mode: "none"},
		ErrorRate: 0,
		PayloadDrift: &scenario.PayloadDriftConfig{
			Enabled:          true,
			Rate:             1,
			AllowedMutations: []string{"json_set"},
		},
	}
	c := NewEngine(cfg, eng)
	req := scenario.RequestSpec{ID: "x", JSONBody: json.RawMessage(`{"a":1}`)}
	_, out := c.Apply(req, 4, []byte(`{"a":1}`))
	if string(out) == `{"a":1}` {
		t.Fatalf("expected drift mutation")
	}
}
