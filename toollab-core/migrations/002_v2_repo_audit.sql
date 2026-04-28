CREATE TABLE IF NOT EXISTS v2_repos (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    source_type TEXT NOT NULL CHECK(source_type = 'path'),
    source_path TEXT NOT NULL,
    doc_policy  TEXT NOT NULL DEFAULT 'ignore_existing_docs',
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_v2_repos_created ON v2_repos(created_at DESC);

CREATE TABLE IF NOT EXISTS v2_audit_runs (
    id              TEXT PRIMARY KEY,
    repo_id         TEXT NOT NULL REFERENCES v2_repos(id),
    status          TEXT NOT NULL,
    score           INTEGER NOT NULL DEFAULT 0,
    score_breakdown TEXT NOT NULL DEFAULT '{}',
    summary         TEXT NOT NULL DEFAULT '',
    stack_json      TEXT NOT NULL DEFAULT '{}',
    created_at      TEXT NOT NULL,
    completed_at    TEXT
);

CREATE INDEX IF NOT EXISTS idx_v2_audit_runs_repo_created ON v2_audit_runs(repo_id, created_at DESC);

CREATE TABLE IF NOT EXISTS v2_evidence (
    id         TEXT PRIMARY KEY,
    audit_id   TEXT NOT NULL REFERENCES v2_audit_runs(id),
    kind       TEXT NOT NULL,
    ref        TEXT NOT NULL DEFAULT '',
    summary    TEXT NOT NULL,
    command    TEXT NOT NULL DEFAULT '',
    file_path  TEXT NOT NULL DEFAULT '',
    line       INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_v2_evidence_audit ON v2_evidence(audit_id);

CREATE TABLE IF NOT EXISTS v2_findings (
    id          TEXT PRIMARY KEY,
    audit_id    TEXT NOT NULL REFERENCES v2_audit_runs(id),
    rule_id     TEXT NOT NULL DEFAULT '',
    severity    TEXT NOT NULL,
    priority    TEXT NOT NULL,
    state       TEXT NOT NULL,
    category    TEXT NOT NULL,
    title       TEXT NOT NULL,
    description TEXT NOT NULL,
    confidence  TEXT NOT NULL,
    file_path   TEXT NOT NULL DEFAULT '',
    line        INTEGER NOT NULL DEFAULT 0,
    evidence_json TEXT NOT NULL DEFAULT '[]',
    details_json  TEXT NOT NULL DEFAULT '{}',
    created_at  TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_v2_findings_audit_severity ON v2_findings(audit_id, severity);

CREATE TABLE IF NOT EXISTS v2_docs (
    id            TEXT PRIMARY KEY,
    audit_id      TEXT NOT NULL REFERENCES v2_audit_runs(id),
    title         TEXT NOT NULL,
    content       TEXT NOT NULL,
    source_policy TEXT NOT NULL,
    created_at    TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_v2_docs_audit_created ON v2_docs(audit_id, created_at DESC);

CREATE TABLE IF NOT EXISTS v2_tests (
    id             TEXT PRIMARY KEY,
    audit_id       TEXT NOT NULL REFERENCES v2_audit_runs(id),
    kind           TEXT NOT NULL,
    name           TEXT NOT NULL,
    command        TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL,
    output         TEXT NOT NULL DEFAULT '',
    generated_path TEXT NOT NULL DEFAULT '',
    created_at     TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_v2_tests_audit ON v2_tests(audit_id);

CREATE TABLE IF NOT EXISTS v2_score_items (
    id              TEXT PRIMARY KEY,
    audit_id        TEXT NOT NULL REFERENCES v2_audit_runs(id),
    category        TEXT NOT NULL,
    max_points      INTEGER NOT NULL,
    awarded_points  INTEGER NOT NULL,
    deducted_points INTEGER NOT NULL,
    reason          TEXT NOT NULL,
    evidence_json   TEXT NOT NULL DEFAULT '[]',
    created_at      TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_v2_score_items_audit ON v2_score_items(audit_id);
