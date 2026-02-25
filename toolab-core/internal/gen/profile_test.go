package gen

import "testing"

func TestChaosProfileNone(t *testing.T) {
	chaos, exp, err := ChaosProfile("none")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chaos.Latency.Mode != "none" {
		t.Errorf("expected latency mode none, got %q", chaos.Latency.Mode)
	}
	if chaos.ErrorRate != 0 {
		t.Errorf("expected error rate 0, got %f", chaos.ErrorRate)
	}
	if chaos.Flapping != nil {
		t.Errorf("expected no flapping")
	}
	if exp.MaxP95MS != 200 {
		t.Errorf("expected max p95 200, got %d", exp.MaxP95MS)
	}
}

func TestChaosProfileLight(t *testing.T) {
	chaos, _, err := ChaosProfile("light")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chaos.Latency.Mode != "uniform" {
		t.Errorf("expected uniform latency, got %q", chaos.Latency.Mode)
	}
	if chaos.Latency.MinMS != 5 || chaos.Latency.MaxMS != 30 {
		t.Errorf("unexpected latency range: %d-%d", chaos.Latency.MinMS, chaos.Latency.MaxMS)
	}
	if chaos.ErrorRate != 0.02 {
		t.Errorf("expected error rate 0.02, got %f", chaos.ErrorRate)
	}
}

func TestChaosProfileModerate(t *testing.T) {
	chaos, _, err := ChaosProfile("moderate")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chaos.Flapping == nil {
		t.Fatal("expected flapping to be set")
	}
	if !chaos.Flapping.Enabled {
		t.Error("expected flapping enabled")
	}
}

func TestChaosProfileAggressive(t *testing.T) {
	chaos, _, err := ChaosProfile("aggressive")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chaos.PayloadDrift == nil {
		t.Fatal("expected payload drift to be set")
	}
	if !chaos.PayloadDrift.Enabled {
		t.Error("expected drift enabled")
	}
	if len(chaos.PayloadDrift.AllowedMutations) != 3 {
		t.Errorf("expected 3 mutations, got %d", len(chaos.PayloadDrift.AllowedMutations))
	}
}

func TestChaosProfileUnknown(t *testing.T) {
	_, _, err := ChaosProfile("extreme")
	if err == nil {
		t.Fatal("expected error for unknown profile")
	}
}
