package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"toollab-core/internal/evidence"
)

func TestGenerateArtifacts(t *testing.T) {
	status := 200
	bundle := &evidence.Bundle{
		SchemaVersion:            1,
		Metadata:                 evidence.Metadata{ToollabVersion: "0.1.0", Mode: "black", RunID: "run", RunSeed: "1", ChaosSeed: "2"},
		ScenarioFingerprint:      evidence.ScenarioFingerprint{ScenarioPath: "scenario.yaml", ScenarioSHA256: "abc", ScenarioSchemaVersion: 1},
		Execution:                evidence.Execution{DecisionEngineVersion: "v1", DecisionTapeHash: "hash"},
		Stats:                    evidence.Stats{TotalRequests: 1, ErrorRate: 0, SuccessRate: 1, P50MS: 1, P95MS: 1, P99MS: 1, StatusHistogram: map[string]int{"200": 1}},
		Outcomes:                 []evidence.Outcome{{Seq: 0, RequestID: "r", Method: "GET", Path: "/", StatusCode: &status, ErrorKind: "none", LatencyMS: 1, ResponseHash: "h"}},
		Samples:                  []evidence.Sample{},
		Assertions:               evidence.Assertions{Overall: "PASS", Rules: []evidence.RuleResult{{ID: "rule1", Passed: true}}, ViolatedRules: []string{}},
		Unknowns:                 []string{},
		RedactionSummary:         evidence.RedactionSummary{Mask: "***", MaxBodyPreviewBytes: 1024, MaxSamples: 10},
		Repro:                    evidence.Repro{Command: "toollab run scenario.yaml"},
		DeterministicFingerprint: "fp",
	}

	dir := t.TempDir()
	artifacts, err := Generate(dir, bundle, "{\"x\":1}\n")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	for _, p := range []string{artifacts.EvidencePath, artifacts.ReportJSONPath, artifacts.ReportMDPath, artifacts.JUnitPath, artifacts.ReproScriptPath, artifacts.DecisionTapePath} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("missing artifact %s: %v", p, err)
		}
	}

	rawMD, err := os.ReadFile(filepath.Clean(artifacts.ReportMDPath))
	if err != nil {
		t.Fatalf("read report.md: %v", err)
	}
	text := string(rawMD)
	for _, section := range []string{"Executive summary", "Qué pasó", "Qué se rompió", "Qué está probado", "Qué es unknown", "Cómo reproducir"} {
		if !strings.Contains(text, section) {
			t.Fatalf("missing section %q", section)
		}
	}
}
