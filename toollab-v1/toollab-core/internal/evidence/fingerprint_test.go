package evidence

import "testing"

func TestDeterministicFingerprintIgnoresInformationalFields(t *testing.T) {
	bundle := &Bundle{
		SchemaVersion: 1,
		Metadata: Metadata{
			ToollabVersion: "0.1.0",
			Mode:          "black",
			RunID:         "run1",
			RunSeed:       "1",
			ChaosSeed:     "2",
			StartedUTC:    "2026-01-01T00:00:00Z",
			FinishedUTC:   "2026-01-01T00:00:01Z",
		},
		ScenarioFingerprint: ScenarioFingerprint{ScenarioPath: "a", ScenarioSHA256: "b", ScenarioSchemaVersion: 1},
		Execution:           Execution{DecisionEngineVersion: "v1", DecisionTapeHash: "tape"},
		Stats:               Stats{TotalRequests: 1, StatusHistogram: map[string]int{"200": 1}},
		Outcomes:            []Outcome{{Seq: 0, RequestID: "r", Method: "GET", Path: "/", ErrorKind: "none", LatencyMS: 1, ResponseHash: "h"}},
		Samples:             []Sample{},
		Assertions:          Assertions{Overall: "PASS", Rules: []RuleResult{}, ViolatedRules: []string{}},
		Unknowns:            []string{},
		RedactionSummary:    RedactionSummary{Mask: "***"},
		Repro:               Repro{Command: "toollab run scenario.yaml"},
		Environment:         &Environment{GoVersion: "go1.22"},
	}

	f1, err := ComputeDeterministicFingerprint(bundle)
	if err != nil {
		t.Fatalf("fingerprint1: %v", err)
	}
	bundle.Metadata.StartedUTC = "2030-01-01T00:00:00Z"
	bundle.Environment.GoVersion = "go1.30"
	f2, err := ComputeDeterministicFingerprint(bundle)
	if err != nil {
		t.Fatalf("fingerprint2: %v", err)
	}
	if f1 != f2 {
		t.Fatalf("fingerprint should ignore informational fields")
	}
}
