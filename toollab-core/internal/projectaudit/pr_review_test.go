package projectaudit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTaskSpecStoreLifecycle(t *testing.T) {
	store := NewStore(newTestDB(t))
	project, err := store.CreateProject("sample", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	spec, err := store.CreateTaskSpec(project.ID, CreateTaskSpecRequest{
		Module:          "api",
		Title:           "Add users endpoint",
		TaskDescription: "Implement users list endpoint.",
		SpecMD:          "## Scope\napi users only",
	})
	if err != nil {
		t.Fatal(err)
	}
	if spec.SpecStatus != specStatusProvided {
		t.Fatalf("expected default provided status, got %q", spec.SpecStatus)
	}
	if spec.Slug != "api-add-users-endpoint" {
		t.Fatalf("unexpected slug: %q", spec.Slug)
	}

	specs, err := store.ListTaskSpecs(project.ID, "api")
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected one spec, got %d", len(specs))
	}

	updated, err := store.UpdateTaskSpec(spec.ID, CreateTaskSpecRequest{
		Module:          "api",
		Title:           "Update users endpoint",
		TaskDescription: "Narrowed scope.",
		SpecMD:          "## Scope\napi users and tests",
		SpecStatus:      "draft",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.SpecStatus != specStatusDraft || updated.Title != "Update users endpoint" {
		t.Fatalf("unexpected updated spec: %+v", updated)
	}
}

func TestPRReviewStoreAndHTTPDetail(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	engine := NewEngine(store, t.TempDir())
	project, err := store.CreateProject("sample", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	spec, err := store.CreateTaskSpec(project.ID, CreateTaskSpecRequest{
		Module: "api",
		Title:  "API task",
		SpecMD: "## Scope\napi/users.go\n## Acceptance Criteria\nresponse stays compatible",
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := engine.ReviewPR(context.Background(), project.ID, CreatePRReviewRequest{
		TaskSpecID:  spec.ID,
		Title:       "Touch API",
		Description: "Change users handler.",
		DiffText:    sampleAPIDiff(),
		TestOutput:  "PASS go test ./...",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Review.ID == "" || len(result.Files) == 0 || len(result.Findings) == 0 {
		t.Fatalf("expected persisted review with files/findings: %+v", result)
	}

	list, err := store.ListPRReviews(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected one review, got %d", len(list))
	}
	if list[0].DiffText != "" {
		t.Fatalf("list should not expose diff_text")
	}

	handler := NewHandler(store, engine)
	req := httptest.NewRequest(http.MethodGet, "/pr-reviews/"+result.Review.ID, nil)
	rec := httptest.NewRecorder()
	handler.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var detail PRReviewResult
	if err := json.NewDecoder(rec.Body).Decode(&detail); err != nil {
		t.Fatal(err)
	}
	if detail.Review.ID != result.Review.ID || len(detail.Files) == 0 || len(detail.Findings) == 0 {
		t.Fatalf("detail should return review + files + findings: %+v", detail)
	}
}

func TestReviewPRValidation(t *testing.T) {
	store := NewStore(newTestDB(t))
	engine := NewEngine(store, t.TempDir())
	project, err := store.CreateProject("sample", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := engine.ReviewPR(context.Background(), project.ID, CreatePRReviewRequest{Title: "No diff"}); !isValidation(err) {
		t.Fatalf("expected validation error for empty diff, got %v", err)
	}
	tooLargeDiff := "diff --git a/a.go b/a.go\n" + strings.Repeat("+x", (maxPRReviewDiffBytes/2)+1)
	if _, err := engine.ReviewPR(context.Background(), project.ID, CreatePRReviewRequest{Title: "Large diff", DiffText: tooLargeDiff}); !isValidation(err) {
		t.Fatalf("expected validation error for large diff, got %v", err)
	}
	if _, err := engine.ReviewPR(context.Background(), project.ID, CreatePRReviewRequest{Title: "Bad spec", DiffText: sampleAPIDiff(), TaskSpecID: "missing"}); !isNotFound(err) {
		t.Fatalf("expected not found for missing spec, got %v", err)
	}
}

func TestParseDiffDetectsFilesAndLineCounts(t *testing.T) {
	files := parseDiff(sampleMultiFileDiff())
	if len(files) != 2 {
		t.Fatalf("expected two files, got %d: %+v", len(files), files)
	}
	if files[0].Path != "api/users.go" || files[0].Additions != 2 || files[0].Deletions != 1 || files[0].ChangeType != "modified" {
		t.Fatalf("unexpected first file: %+v", files[0])
	}
	if files[1].Path != "db/migrations/002.sql" || files[1].Additions != 1 || files[1].ChangeType != "added" {
		t.Fatalf("unexpected second file: %+v", files[1])
	}
}

func TestClassifyRisk(t *testing.T) {
	cases := []struct {
		path      string
		wantArea  string
		wantLevel string
	}{
		{"internal/auth/session.go", "security/auth", prSeverityHigh},
		{"db/migrations/002.sql", "database", prSeverityHigh},
		{"api/handler.go", "api_contract", prSeverityHigh},
		{"package.json", "dependencies", prSeverityMedium},
		{"Dockerfile", "deploy_config", prSeverityMedium},
		{"service_test.go", "tests", prSeverityLow},
		{"docs/README.md", "docs", prSeverityLow},
		{"src/app.go", "code", prSeverityLow},
	}
	for _, tc := range cases {
		area, level := classifyRisk(tc.path, "")
		if area != tc.wantArea || level != tc.wantLevel {
			t.Fatalf("classifyRisk(%q) = %s/%s, want %s/%s", tc.path, area, level, tc.wantArea, tc.wantLevel)
		}
	}
}

func TestScorePRReviewCapsAndDecision(t *testing.T) {
	files := []PRReviewFile{{Path: "api/users.go", RiskArea: "api_contract", RiskLevel: prSeverityHigh}}
	findings := []PRReviewFinding{
		{Code: "pr.critical_zone_without_tests", Severity: prSeverityHigh},
		{Code: "pr.api_contract_touched", Severity: prSeverityHigh},
		{Code: "pr.db_migration_touched", Severity: prSeverityHigh},
	}
	score, decision, _ := scorePRReview(findings, files, prSpecProvided, sampleAPIDiff(), "")
	if decision != prDecisionBlockMerge || score > 65 {
		t.Fatalf("expected critical high policy block with cap <=65, got score=%d decision=%s", score, decision)
	}

	score, decision, _ = scorePRReview([]PRReviewFinding{{Code: "pr.secret_pattern", Severity: prSeverityCritical, Status: prStatusConfirmed}}, files, prSpecProvided, sampleAPIDiff(), "PASS")
	if decision != prDecisionBlockMerge || score > 35 {
		t.Fatalf("expected secret block and cap <=35, got score=%d decision=%s", score, decision)
	}

	score, decision, _ = scorePRReview([]PRReviewFinding{{Code: "pr.out_of_scope_possible", Severity: prSeverityMedium}}, files, prSpecProvided, sampleAPIDiff(), "PASS")
	if decision == prDecisionBlockMerge {
		t.Fatalf("out_of_scope_possible must not block by itself")
	}
	if score != 92 {
		t.Fatalf("expected only medium penalty, got %d", score)
	}

	score, _, _ = scorePRReview(nil, files, prSpecMissing, sampleAPIDiff(), "PASS")
	if score != 85 {
		t.Fatalf("expected no-spec cap at 85, got %d", score)
	}
	score, _, _ = scorePRReview(nil, files, prSpecProvided, sampleAPIDiff(), "FAIL go test")
	if score != 60 {
		t.Fatalf("expected failed test cap at 60, got %d", score)
	}
}

func TestBuildReviewPromptIncludesCoreContext(t *testing.T) {
	prompt := buildReviewPrompt(
		CreatePRReviewRequest{Title: "Review API", DiffText: sampleAPIDiff(), TestOutput: "PASS", ProjectRules: "Keep API stable."},
		"## Scope\nAPI only",
		[]PRReviewFile{{Path: "api/users.go", ChangeType: "modified", RiskArea: "api_contract", RiskLevel: prSeverityHigh, Additions: 1}},
		nil,
		90,
		prDecisionApprove,
		prConfidenceHigh,
		prSpecProvided,
	)
	for _, expected := range []string{"# ToolLab PR Guard Review", "## SDD Context", "## Diff", "PASS", "Machine-readable JSON", "Keep API stable."} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt missing %q:\n%s", expected, prompt)
		}
	}
}

func sampleAPIDiff() string {
	return strings.TrimSpace(`
diff --git a/api/users.go b/api/users.go
index 1111111..2222222 100644
--- a/api/users.go
+++ b/api/users.go
@@ -1,3 +1,4 @@
 package api
-func Users() {}
+func Users() { handleUsers() }
`)
}

func sampleMultiFileDiff() string {
	return strings.TrimSpace(`
diff --git a/api/users.go b/api/users.go
index 1111111..2222222 100644
--- a/api/users.go
+++ b/api/users.go
@@ -1,3 +1,4 @@
 package api
-func Users() {}
+func Users() { handleUsers() }
+// TODO: add pagination
diff --git a/db/migrations/002.sql b/db/migrations/002.sql
new file mode 100644
--- /dev/null
+++ b/db/migrations/002.sql
@@ -0,0 +1 @@
+CREATE TABLE users (id TEXT PRIMARY KEY);
`)
}
