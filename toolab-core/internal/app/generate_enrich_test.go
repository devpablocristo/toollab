package app

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateScenario_DeterministicAndNoSecretLeak(t *testing.T) {
	specPath := filepath.Join("..", "..", "..", "testdata", "openapi", "petstore.yaml")
	specBytes, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read openapi fixture: %v", err)
	}

	const token = "top-secret-token"
	if err := os.Setenv("OPENAPI_BEARER_TEST", token); err != nil {
		t.Fatalf("set env: %v", err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("OPENAPI_BEARER_TEST") })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write(specBytes)
	}))
	defer srv.Close()

	tmp := t.TempDir()
	outA := filepath.Join(tmp, "scenario.a.yaml")
	outB := filepath.Join(tmp, "scenario.b.yaml")

	resA, err := GenerateScenario(context.Background(), GenerateConfig{
		From:            "openapi",
		OpenAPIURL:      srv.URL,
		OpenAPIAuthFlag: "bearer:OPENAPI_BEARER_TEST",
		OutPath:         outA,
		Mode:            "smoke",
	})
	if err != nil {
		t.Fatalf("generate A: %v", err)
	}
	resB, err := GenerateScenario(context.Background(), GenerateConfig{
		From:            "openapi",
		OpenAPIURL:      srv.URL,
		OpenAPIAuthFlag: "bearer:OPENAPI_BEARER_TEST",
		OutPath:         outB,
		Mode:            "smoke",
	})
	if err != nil {
		t.Fatalf("generate B: %v", err)
	}
	if !bytes.Equal(resA.ScenarioYAML, resB.ScenarioYAML) {
		t.Fatalf("expected deterministic generated scenario bytes")
	}
	if resA.ScenarioSHA != resB.ScenarioSHA {
		t.Fatalf("expected deterministic scenario hash")
	}
	if strings.Contains(string(resA.ScenarioYAML), token) || strings.Contains(string(resA.MetaJSON), token) {
		t.Fatalf("secret token leaked in generated outputs")
	}
}

func TestEnrichScenario_Deterministic(t *testing.T) {
	specPath := filepath.Join("..", "..", "..", "testdata", "openapi", "petstore.yaml")
	baseScenario := `
version: 1
mode: black
target:
  base_url: http://localhost:8080
  headers: {}
  auth:
    type: none
workload:
  requests:
    - id: get_health
      method: GET
      path: /health
      body: ""
      timeout_ms: 5000
      weight: 1
  concurrency: 1
  duration_s: 1
  schedule_mode: closed_loop
chaos:
  latency: {mode: none}
  error_rate: 0
  error_statuses: [503]
  error_mode: abort
expectations:
  max_error_rate: 0.5
  max_p95_ms: 2000
  invariants:
    - type: no_5xx_allowed
seeds:
  run_seed: "11"
  chaos_seed: "22"
redaction:
  headers: ["authorization", "cookie", "set-cookie", "x-api-key"]
`

	tmp := t.TempDir()
	basePath := filepath.Join(tmp, "base.yaml")
	if err := os.WriteFile(basePath, []byte(baseScenario), 0o644); err != nil {
		t.Fatalf("write base scenario: %v", err)
	}

	outA := filepath.Join(tmp, "enriched.a.yaml")
	outB := filepath.Join(tmp, "enriched.b.yaml")
	resA, err := EnrichScenario(context.Background(), EnrichConfig{
		BaseScenarioPath: basePath,
		UseOpenAPI:       true,
		OpenAPIFile:      specPath,
		OutPath:          outA,
		MergeStrategy:    "conservative",
	})
	if err != nil {
		t.Fatalf("enrich A: %v", err)
	}
	resB, err := EnrichScenario(context.Background(), EnrichConfig{
		BaseScenarioPath: basePath,
		UseOpenAPI:       true,
		OpenAPIFile:      specPath,
		OutPath:          outB,
		MergeStrategy:    "conservative",
	})
	if err != nil {
		t.Fatalf("enrich B: %v", err)
	}
	if !bytes.Equal(resA.ScenarioYAML, resB.ScenarioYAML) {
		t.Fatalf("expected deterministic enriched scenario bytes")
	}
	if resA.ScenarioSHA != resB.ScenarioSHA {
		t.Fatalf("expected deterministic enriched scenario hash")
	}
}
