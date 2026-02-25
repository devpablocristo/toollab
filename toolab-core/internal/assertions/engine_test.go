package assertions

import (
	"testing"

	"toolab-core/internal/evidence"
	"toolab-core/internal/scenario"
)

func TestEvaluateThresholdsAndInvariants(t *testing.T) {
	status200 := 200
	status500 := 500
	bundle := &evidence.Bundle{
		Stats: evidence.Stats{ErrorRate: 0.2, P95MS: 350},
		Outcomes: []evidence.Outcome{
			{RequestID: "idemp", StatusCode: &status200, ResponseHash: "aaa"},
			{RequestID: "idemp", StatusCode: &status200, ResponseHash: "bbb"},
			{RequestID: "x", StatusCode: &status500, ResponseHash: "ccc"},
		},
	}

	expect := scenario.Expectations{
		MaxErrorRate: 0.1,
		MaxP95MS:     300,
		Invariants: []scenario.InvariantConfig{
			{Type: "no_5xx_allowed"},
			{Type: "max_4xx_rate", Max: 0.5},
			{Type: "status_code_rate", Status: 500, Max: 0.2},
			{Type: "idempotent_key_identical_response", RequestID: "idemp"},
		},
	}

	result := Evaluate(expect, bundle)
	if result.Overall != "FAIL" {
		t.Fatalf("expected FAIL overall")
	}
	if len(result.ViolatedRules) == 0 {
		t.Fatalf("expected violated rules")
	}
}

func TestEvaluatePass(t *testing.T) {
	status200 := 200
	bundle := &evidence.Bundle{
		Stats: evidence.Stats{ErrorRate: 0.0, P95MS: 10},
		Outcomes: []evidence.Outcome{
			{RequestID: "idemp", StatusCode: &status200, ResponseHash: "aaa"},
			{RequestID: "idemp", StatusCode: &status200, ResponseHash: "aaa"},
		},
	}
	expect := scenario.Expectations{
		MaxErrorRate: 0.1,
		MaxP95MS:     50,
		Invariants: []scenario.InvariantConfig{
			{Type: "no_5xx_allowed"},
			{Type: "idempotent_key_identical_response", RequestID: "idemp"},
		},
	}
	result := Evaluate(expect, bundle)
	if result.Overall != "PASS" {
		t.Fatalf("expected PASS overall")
	}
}
