CREATE TABLE IF NOT EXISTS projects (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    source_path TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_projects_created ON projects(created_at DESC);

CREATE TABLE IF NOT EXISTS audit_runs (
    id              TEXT PRIMARY KEY,
    project_id      TEXT NOT NULL REFERENCES projects(id),
    status          TEXT NOT NULL,
    score           INTEGER NOT NULL DEFAULT 0,
    score_breakdown TEXT NOT NULL DEFAULT '{}',
    summary         TEXT NOT NULL DEFAULT '',
    stack_json      TEXT NOT NULL DEFAULT '{}',
    created_at      TEXT NOT NULL,
    completed_at    TEXT
);

CREATE INDEX IF NOT EXISTS idx_audit_runs_project_created ON audit_runs(project_id, created_at DESC);

CREATE TABLE IF NOT EXISTS evidence (
    id         TEXT PRIMARY KEY,
    audit_id   TEXT NOT NULL REFERENCES audit_runs(id),
    kind       TEXT NOT NULL,
    summary    TEXT NOT NULL,
    command    TEXT NOT NULL DEFAULT '',
    file_path  TEXT NOT NULL DEFAULT '',
    line       INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_evidence_audit ON evidence(audit_id);

CREATE TABLE IF NOT EXISTS findings (
    id            TEXT PRIMARY KEY,
    audit_id      TEXT NOT NULL REFERENCES audit_runs(id),
    rule_id       TEXT NOT NULL DEFAULT '',
    severity      TEXT NOT NULL,
    priority      TEXT NOT NULL,
    state         TEXT NOT NULL,
    category      TEXT NOT NULL,
    title         TEXT NOT NULL,
    description   TEXT NOT NULL,
    confidence    TEXT NOT NULL,
    file_path     TEXT NOT NULL DEFAULT '',
    line          INTEGER NOT NULL DEFAULT 0,
    evidence_json TEXT NOT NULL DEFAULT '[]',
    details_json  TEXT NOT NULL DEFAULT '{}',
    created_at    TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_findings_audit_severity ON findings(audit_id, severity);

CREATE TABLE IF NOT EXISTS generated_docs (
    id         TEXT PRIMARY KEY,
    audit_id   TEXT NOT NULL REFERENCES audit_runs(id),
    title      TEXT NOT NULL,
    content    TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_generated_docs_audit_created ON generated_docs(audit_id, created_at DESC);

CREATE TABLE IF NOT EXISTS test_results (
    id             TEXT PRIMARY KEY,
    audit_id       TEXT NOT NULL REFERENCES audit_runs(id),
    kind           TEXT NOT NULL,
    name           TEXT NOT NULL,
    command        TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL,
    output         TEXT NOT NULL DEFAULT '',
    generated_path TEXT NOT NULL DEFAULT '',
    created_at     TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_test_results_audit ON test_results(audit_id);

CREATE TABLE IF NOT EXISTS score_items (
    id              TEXT PRIMARY KEY,
    audit_id        TEXT NOT NULL REFERENCES audit_runs(id),
    category        TEXT NOT NULL,
    max_points      INTEGER NOT NULL,
    awarded_points  INTEGER NOT NULL,
    deducted_points INTEGER NOT NULL,
    reason          TEXT NOT NULL,
    created_at      TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_score_items_audit ON score_items(audit_id);
