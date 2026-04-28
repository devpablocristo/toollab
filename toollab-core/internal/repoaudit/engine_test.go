package repoaudit

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestScanInventoryExcludesDocsByDefault(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module sample\n\ngo 1.26\n")
	writeTestFile(t, root, "main.go", "package main\nfunc main() {}\n")
	writeTestFile(t, root, "README.md", "contaminating docs\n")
	writeTestFile(t, root, "docs/ARCH.md", "contaminating docs\n")

	inv, evidence := scanInventory(root, false)

	if len(evidence) != 1 {
		t.Fatalf("expected inventory evidence, got %d", len(evidence))
	}
	if inv.DocsSkipped != 2 {
		t.Fatalf("expected 2 skipped docs, got %d", inv.DocsSkipped)
	}
	for _, f := range inv.Files {
		if f == "README.md" || f == "docs/ARCH.md" {
			t.Fatalf("doc file %q should not be inventoried by default", f)
		}
	}
	if inv.Stack["go"] != "detected" {
		t.Fatalf("expected Go stack detection, got %#v", inv.Stack)
	}
}

func TestScanInventoryDetectsReactPythonDatabaseAndCI(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "package.json", `{"dependencies":{"react":"18.3.1"},"scripts":{"build":"vite build"}}`)
	writeTestFile(t, root, "pyproject.toml", "[project]\nname='sample'\n")
	writeTestFile(t, root, "migrations/001.sql", "CREATE TABLE things(id text);\n")
	writeTestFile(t, root, ".github/workflows/ci.yml", "name: ci\n")

	inv, _ := scanInventory(root, false)

	for _, key := range []string{"node", "react", "python", "database", "ci"} {
		if inv.Stack[key] != "detected" {
			t.Fatalf("expected %s detected, stack=%#v", key, inv.Stack)
		}
	}
	if len(inv.CI) != 1 {
		t.Fatalf("expected one CI file, got %d", len(inv.CI))
	}
}

func TestSandboxGenerationDoesNotMutateOriginalSource(t *testing.T) {
	source := t.TempDir()
	sandbox := filepath.Join(t.TempDir(), "sandbox")
	writeTestFile(t, source, "go.mod", "module sample\n\ngo 1.26\n")
	writeTestFile(t, source, "main.go", "package main\nfunc main() {}\n")

	if err := prepareSandbox(context.Background(), source, sandbox); err != nil {
		t.Fatalf("prepare sandbox: %v", err)
	}
	inv, _ := scanInventory(source, false)
	results := generateAndRunTests(context.Background(), sandbox, inv)
	if len(results) == 0 {
		t.Fatalf("expected generated test result")
	}
	if _, err := os.Stat(filepath.Join(source, "zz_tollab_generated_test.go")); !os.IsNotExist(err) {
		t.Fatalf("generated test leaked into original source")
	}
	if _, err := os.Stat(filepath.Join(sandbox, "zz_tollab_generated_test.go")); err != nil {
		t.Fatalf("generated test was not written to sandbox: %v", err)
	}
}

func TestStorePersistsV2RepoAndAuditOutputs(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	migration, err := os.ReadFile(filepath.Join("..", "..", "cmd", "toollab-dashboard", "migrations", "002_v2_repo_audit.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(migration)); err != nil {
		t.Fatal(err)
	}
	store := NewStore(db)

	repo, err := store.CreateRepo("sample", SourceTypePath, "/tmp/sample", "")
	if err != nil {
		t.Fatalf("create repo: %v", err)
	}
	run, err := store.CreateAudit(repo.ID)
	if err != nil {
		t.Fatalf("create audit: %v", err)
	}
	finding := Finding{
		AuditID:     run.ID,
		Severity:    "Medium",
		Priority:    "P2",
		State:       "Confirmed",
		Category:    "tests",
		Title:       "No tests",
		Description: "No test files detected.",
		Confidence:  "Alta",
	}
	if _, err := store.SaveFinding(finding); err != nil {
		t.Fatalf("save finding: %v", err)
	}
	if _, err := store.SaveEvidence(Evidence{AuditID: run.ID, Kind: "inventory", Summary: "inventory evidence"}); err != nil {
		t.Fatalf("save evidence: %v", err)
	}
	if err := store.SaveDoc(GeneratedDoc{AuditID: run.ID, Title: "Doc", Content: "content", SourcePolicy: DocPolicyIgnoreExisting}); err != nil {
		t.Fatalf("save doc: %v", err)
	}
	if err := store.SaveTestResult(TestResult{AuditID: run.ID, Kind: "generated", Name: "smoke", Status: "passed"}); err != nil {
		t.Fatalf("save test: %v", err)
	}

	findings, err := store.ListFindings(run.ID)
	if err != nil || len(findings) != 1 {
		t.Fatalf("list findings: len=%d err=%v", len(findings), err)
	}
	docs, err := store.ListDocs(run.ID)
	if err != nil || len(docs) != 1 {
		t.Fatalf("list docs: len=%d err=%v", len(docs), err)
	}
	tests, err := store.ListTests(run.ID)
	if err != nil || len(tests) != 1 {
		t.Fatalf("list tests: len=%d err=%v", len(tests), err)
	}
	evidence, err := store.ListEvidence(run.ID)
	if err != nil || len(evidence) != 1 {
		t.Fatalf("list evidence: len=%d err=%v", len(evidence), err)
	}
}

func TestEngineReturnsPersistedAuditResultAndRespectsDisabledTests(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	migration, err := os.ReadFile(filepath.Join("..", "..", "cmd", "toollab-dashboard", "migrations", "002_v2_repo_audit.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(migration)); err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	writeTestFile(t, source, "go.mod", "module sample\n\ngo 1.26\n")
	writeTestFile(t, source, "main.go", "package main\nfunc main() {}\n")

	store := NewStore(db)
	repo, err := store.CreateRepo("sample", SourceTypePath, source, "")
	if err != nil {
		t.Fatalf("create repo: %v", err)
	}
	engine := NewEngine(store, t.TempDir())
	result, err := engine.Run(context.Background(), repo, AuditConfig{
		GenerateTests:    false,
		RunExistingTests: false,
	})
	if err != nil {
		t.Fatalf("run audit: %v", err)
	}
	if result.Run.CompletedAt == nil {
		t.Fatalf("expected persisted completed_at in returned run")
	}
	if len(result.Tests) != 0 {
		t.Fatalf("expected disabled tests to be respected, got %d tests", len(result.Tests))
	}
	if len(result.Findings) == 0 || result.Findings[0].ID == "" {
		t.Fatalf("expected returned findings to include persisted ids")
	}
	if len(result.Docs) == 0 || result.Docs[0].ID == "" {
		t.Fatalf("expected returned docs to include persisted ids")
	}
	if len(result.Evidence) == 0 || result.Evidence[0].ID == "" {
		t.Fatalf("expected returned evidence to include persisted ids")
	}
}

func TestRulePacksProduceDeterministicFindings(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module sample\n\ngo 1.26\n")
	writeTestFile(t, root, "main.go", `package main

import (
	"database/sql"
	"net/http"
)

func main() {
	http.ListenAndServe(":8080", nil)
}

func write(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	return tx.Commit()
}
`)
	writeTestFile(t, root, "package.json", `{"dependencies":{"react":"18.3.1"},"scripts":{"dev":"vite"}}`)
	writeTestFile(t, root, "src/App.tsx", `import { useEffect } from 'react'
export function App() {
	useEffect(() => {
		fetch('/api/users')
	}, [])
	return null
}`)
	writeTestFile(t, root, "pyproject.toml", "[project]\nname='sample'\n")
	writeTestFile(t, root, "scripts/job.py", `import os
import requests
import subprocess

TOKEN = os.environ["TOKEN"]
requests.get("https://example.com")
subprocess.run("echo hi", shell=True)
`)
	writeTestFile(t, root, "migrations/001.sql", `CREATE TABLE orders (
  id INTEGER,
  user_id TEXT
);
SELECT * FROM orders LIMIT 10;`)

	inv, _ := scanInventory(root, false)
	findings := buildFindings("audit-1", root, inv)

	assertRuleIDs(t, findings,
		"go.http_server_without_timeouts",
		"go.transaction_missing_commit_or_rollback",
		"react.direct_api_call_outside_client",
		"react.no_build_script",
		"react.no_typecheck_script",
		"python.requests_without_timeout",
		"python.subprocess_shell_true",
		"python.direct_environ_index",
		"python.import_side_effect",
		"sql.table_without_primary_key",
		"sql.id_column_without_fk",
		"sql.limit_without_order",
		"node.no_lockfile",
	)
}

func TestFullAuditPersistsRuleIDsEvidenceRefsAndScoreItems(t *testing.T) {
	store, engine := newTestStoreAndEngine(t)
	source := t.TempDir()
	writeTestFile(t, source, "go.mod", "module sample\n\ngo 1.26\n")
	writeTestFile(t, source, "main.go", `package main

import "net/http"

func main() {
	http.ListenAndServe(":8080", nil)
}
`)
	repo, err := store.CreateRepo("sample", SourceTypePath, source, "")
	if err != nil {
		t.Fatalf("create repo: %v", err)
	}
	result, err := engine.Run(context.Background(), repo, AuditConfig{GenerateTests: false, RunExistingTests: false})
	if err != nil {
		t.Fatalf("run audit: %v", err)
	}
	if len(result.ScoreItems) != 5 {
		t.Fatalf("expected 5 score items, got %d", len(result.ScoreItems))
	}
	foundRule := false
	for _, finding := range result.Findings {
		if finding.RuleID == "go.http_server_without_timeouts" {
			foundRule = true
			if len(finding.EvidenceRefs) == 0 || finding.EvidenceRefs[0].ID == "" {
				t.Fatalf("expected persisted evidence ref for %s", finding.RuleID)
			}
			if finding.Details.MinimumRecommendation == "" {
				t.Fatalf("expected details for %s", finding.RuleID)
			}
		}
	}
	if !foundRule {
		t.Fatalf("expected go timeout rule in findings: %#v", result.Findings)
	}
	for _, item := range result.ScoreItems {
		if item.ID == "" {
			t.Fatalf("expected persisted score item id")
		}
		if item.MaxPoints <= 0 {
			t.Fatalf("score item has invalid max points: %#v", item)
		}
	}
}

func TestHandlerListsEvidenceAndScoreItems(t *testing.T) {
	store, engine := newTestStoreAndEngine(t)
	source := t.TempDir()
	writeTestFile(t, source, "go.mod", "module sample\n\ngo 1.26\n")
	writeTestFile(t, source, "main.go", "package main\nfunc main() {}\n")
	repo, err := store.CreateRepo("sample", SourceTypePath, source, "")
	if err != nil {
		t.Fatalf("create repo: %v", err)
	}
	result, err := engine.Run(context.Background(), repo, AuditConfig{GenerateTests: false, RunExistingTests: false})
	if err != nil {
		t.Fatalf("run audit: %v", err)
	}
	handler := NewHandler(store, engine).Routes()

	evidenceResp := httptest.NewRecorder()
	handler.ServeHTTP(evidenceResp, httptest.NewRequest(http.MethodGet, "/audits/"+result.Run.ID+"/evidence", nil))
	if evidenceResp.Code != http.StatusOK {
		t.Fatalf("evidence status=%d body=%s", evidenceResp.Code, evidenceResp.Body.String())
	}
	var evidenceBody struct {
		Items []Evidence `json:"items"`
	}
	if err := json.Unmarshal(evidenceResp.Body.Bytes(), &evidenceBody); err != nil {
		t.Fatal(err)
	}
	if len(evidenceBody.Items) == 0 {
		t.Fatalf("expected evidence items")
	}

	scoreResp := httptest.NewRecorder()
	handler.ServeHTTP(scoreResp, httptest.NewRequest(http.MethodGet, "/audits/"+result.Run.ID+"/score", nil))
	if scoreResp.Code != http.StatusOK {
		t.Fatalf("score status=%d body=%s", scoreResp.Code, scoreResp.Body.String())
	}
	var scoreBody struct {
		Items []ScoreItem `json:"items"`
	}
	if err := json.Unmarshal(scoreResp.Body.Bytes(), &scoreBody); err != nil {
		t.Fatal(err)
	}
	if len(scoreBody.Items) != 5 {
		t.Fatalf("expected 5 score items, got %d", len(scoreBody.Items))
	}
}

func newTestStoreAndEngine(t *testing.T) (*Store, *Engine) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	migration, err := os.ReadFile(filepath.Join("..", "..", "cmd", "toollab-dashboard", "migrations", "002_v2_repo_audit.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(migration)); err != nil {
		t.Fatal(err)
	}
	store := NewStore(db)
	return store, NewEngine(store, t.TempDir())
}

func assertRuleIDs(t *testing.T, findings []Finding, expected ...string) {
	t.Helper()
	seen := map[string]bool{}
	for _, finding := range findings {
		seen[finding.RuleID] = true
	}
	for _, ruleID := range expected {
		if !seen[ruleID] {
			t.Fatalf("missing rule %s; seen=%v", ruleID, seen)
		}
	}
}

func writeTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
