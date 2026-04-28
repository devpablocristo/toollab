ALTER TABLE v2_findings ADD COLUMN rule_id TEXT NOT NULL DEFAULT '';
ALTER TABLE v2_findings ADD COLUMN details_json TEXT NOT NULL DEFAULT '{}';

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
