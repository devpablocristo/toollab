package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateUpgradesExistingV2FindingSchemaIdempotently(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE v2_findings (
		id TEXT PRIMARY KEY,
		audit_id TEXT NOT NULL,
		severity TEXT NOT NULL,
		priority TEXT NOT NULL,
		state TEXT NOT NULL,
		category TEXT NOT NULL,
		title TEXT NOT NULL,
		description TEXT NOT NULL,
		confidence TEXT NOT NULL,
		file_path TEXT NOT NULL DEFAULT '',
		line INTEGER NOT NULL DEFAULT 0,
		evidence_json TEXT NOT NULL DEFAULT '[]',
		created_at TEXT NOT NULL
	)`)
	if err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile(filepath.Join("migrations", "003_v2_findings_score.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate(db, []string{string(migration), string(migration)}); err != nil {
		t.Fatalf("migration should be idempotent for duplicate columns: %v", err)
	}
	assertColumnExists(t, db, "v2_findings", "rule_id")
	assertColumnExists(t, db, "v2_findings", "details_json")
	if _, err := db.Exec(`INSERT INTO v2_score_items (id,audit_id,category,max_points,awarded_points,deducted_points,reason,created_at) VALUES ('1','a','ci',10,3,7,'reason','now')`); err != nil {
		t.Fatalf("score table was not created: %v", err)
	}
}

func assertColumnExists(t *testing.T, db *sql.DB, table, column string) {
	t.Helper()
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			t.Fatal(err)
		}
		if name == column {
			return
		}
	}
	t.Fatalf("column %s.%s not found", table, column)
}
