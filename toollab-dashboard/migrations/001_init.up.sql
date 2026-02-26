CREATE TABLE IF NOT EXISTS targets (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    base_url    TEXT NOT NULL,
    description TEXT DEFAULT '',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS scenarios (
    id              TEXT PRIMARY KEY,
    target_id       TEXT NOT NULL REFERENCES targets(id),
    name            TEXT NOT NULL,
    sha256          TEXT NOT NULL,
    yaml_content    TEXT NOT NULL,
    source          TEXT DEFAULT 'manual',
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS runs (
    id                      TEXT PRIMARY KEY,
    scenario_id             TEXT REFERENCES scenarios(id),
    target_id               TEXT NOT NULL REFERENCES targets(id),
    run_seed                TEXT NOT NULL,
    chaos_seed              TEXT NOT NULL,
    verdict                 TEXT NOT NULL DEFAULT 'unknown',
    total_requests          INTEGER DEFAULT 0,
    completed_requests      INTEGER DEFAULT 0,
    success_rate            REAL DEFAULT 0,
    error_rate              REAL DEFAULT 0,
    p50_ms                  INTEGER DEFAULT 0,
    p95_ms                  INTEGER DEFAULT 0,
    p99_ms                  INTEGER DEFAULT 0,
    duration_s              INTEGER DEFAULT 0,
    concurrency             INTEGER DEFAULT 1,
    status_histogram        TEXT DEFAULT '{}',
    evidence_json           TEXT,
    deterministic_fingerprint TEXT,
    golden_run_dir          TEXT,
    started_at              DATETIME,
    finished_at             DATETIME,
    created_at              DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS interpretations (
    id          TEXT PRIMARY KEY,
    run_id      TEXT NOT NULL REFERENCES runs(id),
    model       TEXT NOT NULL,
    narrative   TEXT NOT NULL,
    input_sha256 TEXT,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS assertion_results (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id      TEXT NOT NULL REFERENCES runs(id),
    rule_id     TEXT NOT NULL,
    rule_type   TEXT NOT NULL,
    passed      BOOLEAN NOT NULL,
    observed    TEXT,
    expected    TEXT,
    message     TEXT
);

CREATE INDEX idx_runs_target ON runs(target_id);
CREATE INDEX idx_runs_verdict ON runs(verdict);
CREATE INDEX idx_runs_created ON runs(created_at DESC);
CREATE INDEX idx_scenarios_target ON scenarios(target_id);
CREATE INDEX idx_interpretations_run ON interpretations(run_id);
CREATE INDEX idx_assertions_run ON assertion_results(run_id);
