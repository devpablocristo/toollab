package projectaudit

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestEngineAuditsProjectAndPersistsResult(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	engine := NewEngine(store, t.TempDir())
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module sample\n\ngo 1.26\n")
	writeFile(t, root, "main.go", "package main\n\nimport \"net/http\"\n\nfunc main(){ http.ListenAndServe(\":8080\", nil) }\n")

	project, err := store.CreateProject("sample", root)
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.Run(context.Background(), project, AuditConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Run.Score < 0 || result.Run.Score > 100 {
		t.Fatalf("invalid score: %d", result.Run.Score)
	}
	if len(result.Findings) == 0 {
		t.Fatalf("expected findings")
	}
	if len(result.Evidence) == 0 {
		t.Fatalf("expected evidence")
	}
}

func TestStoreListsEmptyCollectionsAsEmptySlices(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	project, err := store.CreateProject("sample", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runs, err := store.ListAudits(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if runs == nil {
		t.Fatalf("expected non-nil audit slice")
	}
}

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	migrations, err := MigrationStatements()
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) == 0 {
		t.Fatal("expected migrations")
	}
	if _, err := db.Exec(migrations[0]); err != nil {
		t.Fatal(err)
	}
	return db
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
