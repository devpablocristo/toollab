package enrich

import (
	"encoding/json"
	"strings"
	"testing"

	"toollab-core/internal/scenario"
)

func TestEnrich_ConservativePreservesManual(t *testing.T) {
	base := baseScenarioWithTimeout(5000)
	toollabSrc := baseScenarioWithTimeout(1500)
	openapiSrc := baseScenarioWithTimeout(1000)

	result, err := Enrich(Inputs{
		Base:        base,
		FromToollab:  toollabSrc,
		FromOpenAPI: openapiSrc,
		Strategy:    Conservative,
	})
	if err != nil {
		t.Fatalf("enrich conservative: %v", err)
	}
	if got := result.Scenario.Workload.Requests[0].TimeoutMS; got != 5000 {
		t.Fatalf("manual timeout must win in conservative mode, got %d", got)
	}
}

func TestEnrich_AggressiveToollabOnlyOverride(t *testing.T) {
	base := baseScenarioWithTimeout(5000)
	toollabSrc := baseScenarioWithTimeout(1500)
	openapiSrc := baseScenarioWithTimeout(900)

	result, err := Enrich(Inputs{
		Base:        base,
		FromToollab:  toollabSrc,
		FromOpenAPI: openapiSrc,
		Strategy:    Aggressive,
	})
	if err != nil {
		t.Fatalf("enrich aggressive: %v", err)
	}
	// Toollab can override defaults in aggressive mode.
	if got := result.Scenario.Workload.Requests[0].TimeoutMS; got != 1500 {
		t.Fatalf("expected toollab override to 1500, got %d", got)
	}
	// OpenAPI must not override toollab/manual values.
	if got := result.Scenario.Workload.Requests[0].TimeoutMS; got == 900 {
		t.Fatalf("openapi must not override existing timeout in aggressive mode")
	}

	for _, ch := range result.Changes {
		if !strings.HasPrefix(ch.Path, "/") {
			t.Fatalf("change path must be JSON Pointer, got %s", ch.Path)
		}
	}
}

func baseScenarioWithTimeout(timeout int) *scenario.Scenario {
	jsonBody := json.RawMessage(`{"n":1}`)
	return &scenario.Scenario{
		Version: 1,
		Mode:    "black",
		Target: scenario.Target{
			BaseURL: "http://svc",
			Headers: map[string]string{},
			Auth:    scenario.Auth{Type: "none"},
		},
		Workload: scenario.Workload{
			Requests: []scenario.RequestSpec{
				{
					ID:        "get_health",
					Method:    "GET",
					Path:      "/health",
					Query:     map[string]string{},
					Headers:   map[string]string{"Content-Type": "application/json"},
					JSONBody:  jsonBody,
					TimeoutMS: timeout,
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
		Seeds: scenario.Seeds{RunSeed: "1", ChaosSeed: "2"},
		Redaction: scenario.RedactionConfig{
			Headers:             []string{"authorization"},
			JSONPaths:           []string{},
			Mask:                "***REDACTED***",
			MaxBodyPreviewBytes: 1024,
			MaxSamples:          5,
		},
	}
}
