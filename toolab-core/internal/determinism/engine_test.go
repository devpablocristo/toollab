package determinism

import "testing"

func TestEngineDeterminism(t *testing.T) {
	tapeA := NewTapeRecorder()
	tapeB := NewTapeRecorder()

	a, err := NewEngine("123", "run_seed", tapeA)
	if err != nil {
		t.Fatalf("new engine A: %v", err)
	}
	b, err := NewEngine("123", "run_seed", tapeB)
	if err != nil {
		t.Fatalf("new engine B: %v", err)
	}

	for i := 0; i < 10; i++ {
		va := a.Uint64("workload_pick", int64(i), "request_pick", 0, "a")
		vb := b.Uint64("workload_pick", int64(i), "request_pick", 0, "a")
		if va != vb {
			t.Fatalf("deterministic mismatch at %d", i)
		}
	}

	hA, err := tapeA.Hash()
	if err != nil {
		t.Fatalf("hash A: %v", err)
	}
	hB, err := tapeB.Hash()
	if err != nil {
		t.Fatalf("hash B: %v", err)
	}
	if hA != hB {
		t.Fatalf("tape hash mismatch: %s != %s", hA, hB)
	}
}

func TestEngineSequenceSensitivity(t *testing.T) {
	eng, err := NewEngine("1", "run_seed", nil)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	v1 := eng.Uint64("chaos", 1, "latency", 0)
	v2 := eng.Uint64("chaos", 2, "latency", 0)
	if v1 == v2 {
		t.Fatalf("different seq should produce different values")
	}
}
