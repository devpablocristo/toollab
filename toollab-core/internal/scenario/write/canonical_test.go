package write

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"

	"toollab-core/internal/scenario"
)

func TestWriteCanonicalScenario_IsStable(t *testing.T) {
	jsonBody := json.RawMessage(`{"name":"neo"}`)
	s := &scenario.Scenario{
		Version: 1,
		Mode:    "black",
		Target: scenario.Target{
			BaseURL: "http://example.test",
			Headers: map[string]string{},
			Auth:    scenario.Auth{Type: "none"},
		},
		Workload: scenario.Workload{
			Requests: []scenario.RequestSpec{
				{
					ID:        "z_post_pets",
					Method:    "POST",
					Path:      "/pets",
					Query:     map[string]string{},
					Headers:   map[string]string{"Content-Type": "application/json"},
					JSONBody:  jsonBody,
					TimeoutMS: 5000,
					Weight:    1,
				},
				{
					ID:        "a_get_health",
					Method:    "GET",
					Path:      "/health",
					Query:     map[string]string{},
					Headers:   map[string]string{},
					Body:      strPtr(""),
					TimeoutMS: 5000,
					Weight:    1,
				},
			},
			Concurrency:  1,
			DurationS:    1,
			ScheduleMode: "closed_loop",
		},
		Chaos: scenario.Chaos{
			Latency:     scenario.LatencyConfig{Mode: "none"},
			ErrorRate:   0,
			ErrorStatus: []int{503},
			ErrorMode:   "abort",
		},
		Expectations: scenario.Expectations{
			MaxErrorRate: 1,
			MaxP95MS:     1000,
			Invariants:   []scenario.InvariantConfig{{Type: "no_5xx_allowed"}},
		},
		Seeds: scenario.Seeds{
			RunSeed:   "1",
			ChaosSeed: "2",
		},
		Redaction: scenario.RedactionConfig{
			Headers:             []string{"cookie", "authorization"},
			JSONPaths:           []string{"$.token", "$.secret"},
			Mask:                "***REDACTED***",
			MaxBodyPreviewBytes: 4096,
			MaxSamples:          50,
		},
	}

	yamlA, shaA, err := WriteCanonicalScenario(s)
	if err != nil {
		t.Fatalf("first canonical write: %v", err)
	}
	yamlB, shaB, err := WriteCanonicalScenario(s)
	if err != nil {
		t.Fatalf("second canonical write: %v", err)
	}
	if string(yamlA) != string(yamlB) {
		t.Fatalf("canonical yaml mismatch across runs")
	}
	if shaA != shaB {
		t.Fatalf("scenario sha mismatch across runs")
	}

	var decoded map[string]any
	if err := yaml.Unmarshal(yamlA, &decoded); err != nil {
		t.Fatalf("yaml decode: %v", err)
	}
	workload := decoded["workload"].(map[string]any)
	reqs := workload["requests"].([]any)
	first := reqs[0].(map[string]any)["id"].(string)
	if first != "a_get_health" {
		t.Fatalf("expected deterministic request ordering by fingerprint, got first id=%s", first)
	}
}

func strPtr(v string) *string { return &v }
