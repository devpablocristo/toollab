CREATE TABLE IF NOT EXISTS targets (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    source_type   TEXT NOT NULL,
    source_value  TEXT NOT NULL,
    runtime_hint  TEXT NOT NULL DEFAULT '{}',
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS runs (
    id            TEXT PRIMARY KEY,
    target_id     TEXT NOT NULL REFERENCES targets(id),
    status        TEXT NOT NULL DEFAULT 'created',
    seed          TEXT NOT NULL DEFAULT '',
    notes         TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL,
    completed_at  TEXT
);

CREATE INDEX IF NOT EXISTS idx_runs_target_created ON runs(target_id, created_at DESC);

CREATE TABLE IF NOT EXISTS artifacts (
    id              TEXT PRIMARY KEY,
    run_id          TEXT NOT NULL REFERENCES runs(id),
    type            TEXT NOT NULL,
    schema_version  TEXT NOT NULL DEFAULT 'v1',
    revision        INTEGER NOT NULL,
    content_hash    TEXT NOT NULL,
    size_bytes      INTEGER NOT NULL,
    storage_path    TEXT NOT NULL,
    created_at      TEXT NOT NULL,
    UNIQUE(run_id, type, revision)
);

CREATE INDEX IF NOT EXISTS idx_artifacts_run_type_rev ON artifacts(run_id, type, revision DESC);
