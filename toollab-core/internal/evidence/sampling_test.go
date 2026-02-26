package evidence

import "testing"

func TestDeterministicSampling(t *testing.T) {
	outcomes := make([]OutcomeInput, 0, 20)
	for i := 0; i < 20; i++ {
		outcomes = append(outcomes, OutcomeInput{Seq: int64(i)})
	}
	a := SelectSampleIndexes(outcomes, 5, "123")
	b := SelectSampleIndexes(outcomes, 5, "123")
	if len(a) != len(b) {
		t.Fatalf("sample size mismatch")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("deterministic sampling mismatch")
		}
	}
}
