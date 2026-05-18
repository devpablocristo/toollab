CREATE TABLE IF NOT EXISTS task_specs (
    id               TEXT PRIMARY KEY,
    project_id       TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    module           TEXT NOT NULL DEFAULT '',
    title            TEXT NOT NULL,
    slug             TEXT NOT NULL,
    task_description TEXT NOT NULL DEFAULT '',
    spec_md          TEXT NOT NULL,
    spec_status      TEXT NOT NULL DEFAULT 'provided',
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_task_specs_project ON task_specs(project_id);
CREATE INDEX IF NOT EXISTS idx_task_specs_module ON task_specs(module);
CREATE INDEX IF NOT EXISTS idx_task_specs_created ON task_specs(created_at DESC);

CREATE TABLE IF NOT EXISTS pr_reviews (
    id             TEXT PRIMARY KEY,
    project_id     TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    task_spec_id   TEXT REFERENCES task_specs(id) ON DELETE SET NULL,
    title          TEXT NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    diff_text      TEXT NOT NULL,
    project_rules  TEXT NOT NULL DEFAULT '',
    test_output    TEXT NOT NULL DEFAULT '',
    review_prompt  TEXT NOT NULL DEFAULT '',
    summary        TEXT NOT NULL DEFAULT '',
    score          INTEGER NOT NULL DEFAULT 0,
    decision       TEXT NOT NULL DEFAULT 'REVIEW_REQUIRED',
    confidence     TEXT NOT NULL DEFAULT 'MEDIUM',
    spec_status    TEXT NOT NULL DEFAULT 'SPEC_MISSING',
    sdd_context_md TEXT NOT NULL DEFAULT '',
    stack_json     TEXT NOT NULL DEFAULT '[]',
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_pr_reviews_project ON pr_reviews(project_id);
CREATE INDEX IF NOT EXISTS idx_pr_reviews_task_spec ON pr_reviews(task_spec_id);
CREATE INDEX IF NOT EXISTS idx_pr_reviews_created ON pr_reviews(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_pr_reviews_decision ON pr_reviews(decision);
CREATE INDEX IF NOT EXISTS idx_pr_reviews_score ON pr_reviews(score);

CREATE TABLE IF NOT EXISTS pr_review_files (
    id          TEXT PRIMARY KEY,
    review_id   TEXT NOT NULL REFERENCES pr_reviews(id) ON DELETE CASCADE,
    path        TEXT NOT NULL,
    change_type TEXT NOT NULL DEFAULT 'modified',
    risk_area   TEXT NOT NULL DEFAULT 'code',
    risk_level  TEXT NOT NULL DEFAULT 'LOW',
    additions   INTEGER NOT NULL DEFAULT 0,
    deletions   INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_pr_review_files_review ON pr_review_files(review_id);

CREATE TABLE IF NOT EXISTS pr_review_findings (
    id                   TEXT PRIMARY KEY,
    review_id             TEXT NOT NULL REFERENCES pr_reviews(id) ON DELETE CASCADE,
    code                  TEXT NOT NULL,
    severity              TEXT NOT NULL,
    status                TEXT NOT NULL,
    title                 TEXT NOT NULL,
    files_json            TEXT NOT NULL DEFAULT '[]',
    spec_rule_affected    TEXT NOT NULL DEFAULT '',
    problem               TEXT NOT NULL DEFAULT '',
    evidence              TEXT NOT NULL DEFAULT '',
    impact                TEXT NOT NULL DEFAULT '',
    suggested_fix         TEXT NOT NULL DEFAULT '',
    ai_correction_prompt  TEXT NOT NULL DEFAULT '',
    sort_order            INTEGER NOT NULL DEFAULT 0,
    created_at            TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_pr_review_findings_review ON pr_review_findings(review_id);
CREATE INDEX IF NOT EXISTS idx_pr_review_findings_severity ON pr_review_findings(severity);
