package app

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildSystemMap_DeterministicFromOpenAPI(t *testing.T) {
	specPath := filepath.Join("..", "..", "..", "testdata", "openapi", "petstore.yaml")
	resA, err := BuildSystemMap(context.Background(), MapConfig{
		From:         "openapi",
		OpenAPIInput: specPath,
		Print:        true,
		Seed:         "123",
	})
	if err != nil {
		t.Fatalf("map A: %v", err)
	}
	resB, err := BuildSystemMap(context.Background(), MapConfig{
		From:         "openapi",
		OpenAPIInput: specPath,
		Print:        true,
		Seed:         "123",
	})
	if err != nil {
		t.Fatalf("map B: %v", err)
	}
	if !bytes.Equal(resA.SystemMapJSON, resB.SystemMapJSON) {
		t.Fatalf("system_map.json must be deterministic byte-for-byte")
	}
	if resA.MapFP != resB.MapFP {
		t.Fatalf("system map fingerprint must be stable")
	}
}

func TestExplainRun_DeterministicAndUnknowns(t *testing.T) {
	tmp := t.TempDir()
	runDir := filepath.Join(tmp, "run")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	evidenceFixture := filepath.Join("..", "..", "..", "testdata", "evidence", "valid", "minimal.json")
	raw, err := os.ReadFile(evidenceFixture)
	if err != nil {
		t.Fatalf("read evidence fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "evidence.json"), raw, 0o644); err != nil {
		t.Fatalf("write evidence run file: %v", err)
	}

	resA, err := ExplainRun(context.Background(), ExplainConfig{
		RunDir: runDir,
		Print:  true,
	})
	if err != nil {
		t.Fatalf("explain A: %v", err)
	}
	resB, err := ExplainRun(context.Background(), ExplainConfig{
		RunDir: runDir,
		Print:  true,
	})
	if err != nil {
		t.Fatalf("explain B: %v", err)
	}
	if !bytes.Equal(resA.UnderstandingJSON, resB.UnderstandingJSON) {
		t.Fatalf("understanding.json must be deterministic byte-for-byte")
	}

	var doc map[string]any
	if err := json.Unmarshal(resA.UnderstandingJSON, &doc); err != nil {
		t.Fatalf("decode understanding: %v", err)
	}
	unknowns := doc["unknowns"].([]any)
	if len(unknowns) == 0 {
		t.Fatalf("expected unknowns when discovery is missing")
	}
}

func TestDiffRuns_Deterministic(t *testing.T) {
	tmp := t.TempDir()
	runA := filepath.Join(tmp, "run-a")
	runB := filepath.Join(tmp, "run-b")
	if err := os.MkdirAll(runA, 0o755); err != nil {
		t.Fatalf("mkdir runA: %v", err)
	}
	if err := os.MkdirAll(runB, 0o755); err != nil {
		t.Fatalf("mkdir runB: %v", err)
	}

	fixturePath := filepath.Join("..", "..", "..", "testdata", "evidence", "valid", "minimal.json")
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var evidenceA map[string]any
	if err := json.Unmarshal(raw, &evidenceA); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	evidenceB := cloneAnyMap(evidenceA)
	statsB := evidenceB["stats"].(map[string]any)
	statsB["p95_ms"] = float64(25)
	statsB["error_rate"] = float64(0.2)

	rawA, _ := json.MarshalIndent(evidenceA, "", "  ")
	rawB, _ := json.MarshalIndent(evidenceB, "", "  ")
	rawA = append(rawA, '\n')
	rawB = append(rawB, '\n')
	if err := os.WriteFile(filepath.Join(runA, "evidence.json"), rawA, 0o644); err != nil {
		t.Fatalf("write evidence A: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runB, "evidence.json"), rawB, 0o644); err != nil {
		t.Fatalf("write evidence B: %v", err)
	}

	resA, err := DiffRuns(context.Background(), DiffConfig{
		RunADir: runA,
		RunBDir: runB,
		Print:   true,
	})
	if err != nil {
		t.Fatalf("diff A: %v", err)
	}
	resB, err := DiffRuns(context.Background(), DiffConfig{
		RunADir: runA,
		RunBDir: runB,
		Print:   true,
	})
	if err != nil {
		t.Fatalf("diff B: %v", err)
	}
	if !bytes.Equal(resA.DiffJSON, resB.DiffJSON) {
		t.Fatalf("diff.json must be deterministic byte-for-byte")
	}
}

func cloneAnyMap(in map[string]any) map[string]any {
	raw, _ := json.Marshal(in)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return out
}
