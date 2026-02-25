package report

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"toolab-core/internal/evidence"
)

type ArtifactIndex struct {
	EvidencePath     string
	ReportJSONPath   string
	ReportMDPath     string
	JUnitPath        string
	ReproScriptPath  string
	DecisionTapePath string
}

func Generate(runDir string, bundle *evidence.Bundle, decisionTape string) (*ArtifactIndex, error) {
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return nil, fmt.Errorf("create run dir: %w", err)
	}

	index := &ArtifactIndex{
		EvidencePath:     filepath.Join(runDir, "evidence.json"),
		ReportJSONPath:   filepath.Join(runDir, "report.json"),
		ReportMDPath:     filepath.Join(runDir, "report.md"),
		JUnitPath:        filepath.Join(runDir, "junit.xml"),
		ReproScriptPath:  filepath.Join(runDir, "repro.sh"),
		DecisionTapePath: filepath.Join(runDir, "decision_tape.jsonl"),
	}

	if err := writeJSON(index.EvidencePath, bundle); err != nil {
		return nil, err
	}
	if err := writeReportJSON(index.ReportJSONPath, bundle); err != nil {
		return nil, err
	}
	if err := writeReportMD(index.ReportMDPath, bundle); err != nil {
		return nil, err
	}
	if err := writeJUnit(index.JUnitPath, bundle); err != nil {
		return nil, err
	}
	if err := writeReproScript(index.ReproScriptPath, bundle); err != nil {
		return nil, err
	}
	if err := os.WriteFile(index.DecisionTapePath, []byte(decisionTape), 0o644); err != nil {
		return nil, fmt.Errorf("write decision tape: %w", err)
	}

	bundle.Repro.ScriptPath = index.ReproScriptPath
	bundle.Repro.ExpectedDeterministicFingerprint = bundle.DeterministicFingerprint
	if err := writeJSON(index.EvidencePath, bundle); err != nil {
		return nil, err
	}

	return index, nil
}

func writeJSON(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json %s: %w", path, err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write json %s: %w", path, err)
	}
	return nil
}

func writeReportJSON(path string, bundle *evidence.Bundle) error {
	report := map[string]any{
		"run_id":                    bundle.Metadata.RunID,
		"overall":                   bundle.Assertions.Overall,
		"violated_rules":            bundle.Assertions.ViolatedRules,
		"stats":                     bundle.Stats,
		"decision_tape_hash":        bundle.Execution.DecisionTapeHash,
		"deterministic_fingerprint": bundle.DeterministicFingerprint,
		"scenario_sha256":           bundle.ScenarioFingerprint.ScenarioSHA256,
	}
	return writeJSON(path, report)
}

func writeReportMD(path string, bundle *evidence.Bundle) error {
	b := &strings.Builder{}
	fmt.Fprintln(b, "# Toolab Report")
	fmt.Fprintln(b)
	fmt.Fprintln(b, "## 1. Executive summary (30s)")
	fmt.Fprintf(b, "- overall: **%s**\n", bundle.Assertions.Overall)
	fmt.Fprintf(b, "- total_requests: %d\n", bundle.Stats.TotalRequests)
	fmt.Fprintf(b, "- error_rate: %.6f\n", bundle.Stats.ErrorRate)
	fmt.Fprintf(b, "- p95_ms: %d\n", bundle.Stats.P95MS)
	fmt.Fprintf(b, "- deterministic_fingerprint: `%s`\n", bundle.DeterministicFingerprint)
	fmt.Fprintln(b)

	fmt.Fprintln(b, "## 2. Qué pasó")
	fmt.Fprintf(b, "- success_rate: %.6f\n", bundle.Stats.SuccessRate)
	fmt.Fprintf(b, "- p50/p95/p99: %d/%d/%d ms\n", bundle.Stats.P50MS, bundle.Stats.P95MS, bundle.Stats.P99MS)
	fmt.Fprintf(b, "- status_histogram: `%v`\n", bundle.Stats.StatusHistogram)
	fmt.Fprintln(b)

	fmt.Fprintln(b, "## 3. Qué se rompió")
	if len(bundle.Assertions.ViolatedRules) == 0 {
		fmt.Fprintln(b, "- no violated rules")
	} else {
		for _, v := range bundle.Assertions.ViolatedRules {
			fmt.Fprintf(b, "- %s\n", v)
		}
	}
	fmt.Fprintln(b)

	fmt.Fprintln(b, "## 4. Qué está probado")
	for _, r := range bundle.Assertions.Rules {
		if r.Passed {
			fmt.Fprintf(b, "- %s\n", r.ID)
		}
	}
	fmt.Fprintln(b)

	fmt.Fprintln(b, "## 5. Qué es unknown")
	if len(bundle.Unknowns) == 0 {
		fmt.Fprintln(b, "- none")
	} else {
		for _, u := range bundle.Unknowns {
			fmt.Fprintf(b, "- %s\n", u)
		}
	}
	fmt.Fprintln(b)

	fmt.Fprintln(b, "## 6. Cómo reproducir")
	fmt.Fprintf(b, "- command: `%s`\n", bundle.Repro.Command)
	fmt.Fprintf(b, "- script: `%s`\n", bundle.Repro.ScriptPath)
	fmt.Fprintf(b, "- expected fingerprint: `%s`\n", bundle.DeterministicFingerprint)
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

type junitTestSuites struct {
	XMLName xml.Name         `xml:"testsuites"`
	Suites  []junitTestSuite `xml:"testsuite"`
}

type junitTestSuite struct {
	Name     string          `xml:"name,attr"`
	Tests    int             `xml:"tests,attr"`
	Failures int             `xml:"failures,attr"`
	Cases    []junitTestCase `xml:"testcase"`
}

type junitTestCase struct {
	Name    string        `xml:"name,attr"`
	Failure *junitFailure `xml:"failure,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Text    string `xml:",chardata"`
}

func writeJUnit(path string, bundle *evidence.Bundle) error {
	cases := make([]junitTestCase, 0, len(bundle.Assertions.Rules))
	failures := 0
	for _, r := range bundle.Assertions.Rules {
		c := junitTestCase{Name: r.ID}
		if !r.Passed {
			failures++
			c.Failure = &junitFailure{Message: r.Message, Text: fmt.Sprintf("observed=%v expected=%v", r.Observed, r.Expected)}
		}
		cases = append(cases, c)
	}
	doc := junitTestSuites{Suites: []junitTestSuite{{
		Name:     "toolab.assertions",
		Tests:    len(cases),
		Failures: failures,
		Cases:    cases,
	}}}
	raw, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal junit: %w", err)
	}
	raw = append([]byte(xml.Header), raw...)
	return os.WriteFile(path, raw, 0o644)
}

func writeReproScript(path string, bundle *evidence.Bundle) error {
	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

SCENARIO_PATH="${1:-%s}"
OUT_BASE="${2:-./golden_runs}"
EXPECTED="%s"

toolab run "$SCENARIO_PATH" --out "$OUT_BASE"
LATEST_DIR="$(ls -1dt "$OUT_BASE"/* | head -n 1)"
ACTUAL="$(python3 - <<'PY' "$LATEST_DIR/evidence.json"
import json,sys
print(json.load(open(sys.argv[1]))['deterministic_fingerprint'])
PY
)"

echo "expected: $EXPECTED"
echo "actual:   $ACTUAL"

if [[ "$ACTUAL" != "$EXPECTED" ]]; then
  echo "fingerprint mismatch"
  exit 1
fi

echo "repro ok"
`, bundle.ScenarioFingerprint.ScenarioPath, bundle.DeterministicFingerprint)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		return fmt.Errorf("write repro script: %w", err)
	}
	return nil
}
