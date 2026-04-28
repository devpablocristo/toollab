package repoaudit

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/devpablocristo/core/errors/go/domainerr"
	"github.com/google/uuid"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func (s *Store) CreateRepo(name, sourceType, sourcePath, docPolicy string) (Repo, error) {
	if name == "" {
		return Repo{}, domainerr.Validation("name is required")
	}
	if sourceType != SourceTypePath {
		return Repo{}, domainerr.Validation("source_type must be path")
	}
	if sourcePath == "" {
		return Repo{}, domainerr.Validation("source_path is required")
	}
	if docPolicy == "" {
		docPolicy = DocPolicyIgnoreExisting
	}
	if docPolicy != DocPolicyIgnoreExisting && docPolicy != DocPolicyAllowExisting {
		return Repo{}, domainerr.Validation("invalid doc_policy")
	}
	now := time.Now().UTC()
	repo := Repo{
		ID:         uuid.New().String(),
		Name:       name,
		SourceType: sourceType,
		SourcePath: sourcePath,
		DocPolicy:  docPolicy,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	_, err := s.db.Exec(
		`INSERT INTO v2_repos (id,name,source_type,source_path,doc_policy,created_at,updated_at) VALUES (?,?,?,?,?,?,?)`,
		repo.ID, repo.Name, repo.SourceType, repo.SourcePath, repo.DocPolicy, fmtTime(repo.CreatedAt), fmtTime(repo.UpdatedAt),
	)
	return repo, err
}

func (s *Store) ListRepos() ([]Repo, error) {
	rows, err := s.db.Query(`SELECT id,name,source_type,source_path,doc_policy,created_at,updated_at FROM v2_repos ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var repos []Repo
	for rows.Next() {
		repo, err := scanRepo(rows)
		if err != nil {
			return nil, err
		}
		repos = append(repos, repo)
	}
	return repos, rows.Err()
}

func (s *Store) GetRepo(id string) (Repo, error) {
	row := s.db.QueryRow(`SELECT id,name,source_type,source_path,doc_policy,created_at,updated_at FROM v2_repos WHERE id=?`, id)
	var repo Repo
	var created, updated string
	err := row.Scan(&repo.ID, &repo.Name, &repo.SourceType, &repo.SourcePath, &repo.DocPolicy, &created, &updated)
	if err == sql.ErrNoRows {
		return Repo{}, domainerr.NotFound("repo not found")
	}
	if err != nil {
		return Repo{}, err
	}
	repo.CreatedAt, _ = time.Parse(time.RFC3339, created)
	repo.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
	return repo, nil
}

func (s *Store) CreateAudit(repoID string) (AuditRun, error) {
	now := time.Now().UTC()
	run := AuditRun{
		ID:             uuid.New().String(),
		RepoID:         repoID,
		Status:         AuditStatusRunning,
		ScoreBreakdown: map[string]int{},
		Stack:          map[string]string{},
		CreatedAt:      now,
	}
	_, err := s.db.Exec(
		`INSERT INTO v2_audit_runs (id,repo_id,status,created_at) VALUES (?,?,?,?)`,
		run.ID, run.RepoID, run.Status, fmtTime(run.CreatedAt),
	)
	return run, err
}

func (s *Store) CompleteAudit(run AuditRun) error {
	stackJSON, _ := json.Marshal(run.Stack)
	breakdownJSON, _ := json.Marshal(run.ScoreBreakdown)
	completed := time.Now().UTC()
	run.CompletedAt = &completed
	_, err := s.db.Exec(
		`UPDATE v2_audit_runs SET status=?, score=?, score_breakdown=?, summary=?, stack_json=?, completed_at=? WHERE id=?`,
		run.Status, run.Score, string(breakdownJSON), run.Summary, string(stackJSON), fmtTime(completed), run.ID,
	)
	return err
}

func (s *Store) GetAudit(id string) (AuditRun, error) {
	row := s.db.QueryRow(`SELECT id,repo_id,status,score,score_breakdown,summary,stack_json,created_at,completed_at FROM v2_audit_runs WHERE id=?`, id)
	return scanAudit(row)
}

func (s *Store) ListAudits(repoID string) ([]AuditRun, error) {
	rows, err := s.db.Query(
		`SELECT id,repo_id,status,score,score_breakdown,summary,stack_json,created_at,completed_at FROM v2_audit_runs WHERE repo_id=? ORDER BY created_at DESC`,
		repoID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var runs []AuditRun
	for rows.Next() {
		run, err := scanAuditRows(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (s *Store) SaveEvidence(e Evidence) (Evidence, error) {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(
		`INSERT INTO v2_evidence (id,audit_id,kind,ref,summary,command,file_path,line,created_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		e.ID, e.AuditID, e.Kind, e.Ref, e.Summary, e.Command, e.FilePath, e.Line, fmtTime(e.CreatedAt),
	)
	return e, err
}

func (s *Store) SaveFinding(f Finding) (Finding, error) {
	if f.ID == "" {
		f.ID = uuid.New().String()
	}
	if f.CreatedAt.IsZero() {
		f.CreatedAt = time.Now().UTC()
	}
	refsJSON, _ := json.Marshal(f.EvidenceRefs)
	detailsJSON, _ := json.Marshal(f.Details)
	_, err := s.db.Exec(
		`INSERT INTO v2_findings (id,audit_id,rule_id,severity,priority,state,category,title,description,confidence,file_path,line,evidence_json,details_json,created_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		f.ID, f.AuditID, f.RuleID, f.Severity, f.Priority, f.State, f.Category, f.Title, f.Description, f.Confidence, f.FilePath, f.Line, string(refsJSON), string(detailsJSON), fmtTime(f.CreatedAt),
	)
	return f, err
}

func (s *Store) SaveDoc(d GeneratedDoc) error {
	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(
		`INSERT INTO v2_docs (id,audit_id,title,content,source_policy,created_at) VALUES (?,?,?,?,?,?)`,
		d.ID, d.AuditID, d.Title, d.Content, d.SourcePolicy, fmtTime(d.CreatedAt),
	)
	return err
}

func (s *Store) SaveTestResult(t TestResult) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(
		`INSERT INTO v2_tests (id,audit_id,kind,name,command,status,output,generated_path,created_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		t.ID, t.AuditID, t.Kind, t.Name, t.Command, t.Status, t.Output, t.GeneratedPath, fmtTime(t.CreatedAt),
	)
	return err
}

func (s *Store) SaveScoreItem(item ScoreItem) error {
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now().UTC()
	}
	refsJSON, _ := json.Marshal(item.EvidenceRefs)
	_, err := s.db.Exec(
		`INSERT INTO v2_score_items (id,audit_id,category,max_points,awarded_points,deducted_points,reason,evidence_json,created_at)
		 VALUES (?,?,?,?,?,?,?,?,?)`,
		item.ID, item.AuditID, item.Category, item.MaxPoints, item.AwardedPoints, item.DeductedPoints, item.Reason, string(refsJSON), fmtTime(item.CreatedAt),
	)
	return err
}

func (s *Store) ListFindings(auditID string) ([]Finding, error) {
	rows, err := s.db.Query(
		`SELECT id,audit_id,rule_id,severity,priority,state,category,title,description,confidence,file_path,line,evidence_json,details_json,created_at FROM v2_findings WHERE audit_id=?`,
		auditID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var findings []Finding
	for rows.Next() {
		f, err := scanFinding(rows)
		if err != nil {
			return nil, err
		}
		findings = append(findings, f)
	}
	sortFindings(findings)
	return findings, rows.Err()
}

func (s *Store) ListScoreItems(auditID string) ([]ScoreItem, error) {
	rows, err := s.db.Query(
		`SELECT id,audit_id,category,max_points,awarded_points,deducted_points,reason,evidence_json,created_at FROM v2_score_items WHERE audit_id=? ORDER BY category ASC`,
		auditID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []ScoreItem
	for rows.Next() {
		var item ScoreItem
		var refsJSON, created string
		if err := rows.Scan(&item.ID, &item.AuditID, &item.Category, &item.MaxPoints, &item.AwardedPoints, &item.DeductedPoints, &item.Reason, &refsJSON, &created); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(refsJSON), &item.EvidenceRefs)
		item.CreatedAt, _ = time.Parse(time.RFC3339, created)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListEvidence(auditID string) ([]Evidence, error) {
	rows, err := s.db.Query(
		`SELECT id,audit_id,kind,ref,summary,command,file_path,line,created_at FROM v2_evidence WHERE audit_id=? ORDER BY created_at ASC`,
		auditID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var evidence []Evidence
	for rows.Next() {
		var e Evidence
		var created string
		if err := rows.Scan(&e.ID, &e.AuditID, &e.Kind, &e.Ref, &e.Summary, &e.Command, &e.FilePath, &e.Line, &created); err != nil {
			return nil, err
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339, created)
		evidence = append(evidence, e)
	}
	return evidence, rows.Err()
}

func (s *Store) ListDocs(auditID string) ([]GeneratedDoc, error) {
	rows, err := s.db.Query(`SELECT id,audit_id,title,content,source_policy,created_at FROM v2_docs WHERE audit_id=? ORDER BY created_at DESC`, auditID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var docs []GeneratedDoc
	for rows.Next() {
		var d GeneratedDoc
		var created string
		if err := rows.Scan(&d.ID, &d.AuditID, &d.Title, &d.Content, &d.SourcePolicy, &created); err != nil {
			return nil, err
		}
		d.CreatedAt, _ = time.Parse(time.RFC3339, created)
		docs = append(docs, d)
	}
	return docs, rows.Err()
}

func (s *Store) ListTests(auditID string) ([]TestResult, error) {
	rows, err := s.db.Query(`SELECT id,audit_id,kind,name,command,status,output,generated_path,created_at FROM v2_tests WHERE audit_id=? ORDER BY created_at ASC`, auditID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tests []TestResult
	for rows.Next() {
		var t TestResult
		var created string
		if err := rows.Scan(&t.ID, &t.AuditID, &t.Kind, &t.Name, &t.Command, &t.Status, &t.Output, &t.GeneratedPath, &created); err != nil {
			return nil, err
		}
		t.CreatedAt, _ = time.Parse(time.RFC3339, created)
		tests = append(tests, t)
	}
	return tests, rows.Err()
}

func scanRepo(rows *sql.Rows) (Repo, error) {
	var repo Repo
	var created, updated string
	if err := rows.Scan(&repo.ID, &repo.Name, &repo.SourceType, &repo.SourcePath, &repo.DocPolicy, &created, &updated); err != nil {
		return Repo{}, err
	}
	repo.CreatedAt, _ = time.Parse(time.RFC3339, created)
	repo.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
	return repo, nil
}

func scanAudit(row *sql.Row) (AuditRun, error) {
	var run AuditRun
	var breakdownJSON, stackJSON, created string
	var completed *string
	err := row.Scan(&run.ID, &run.RepoID, &run.Status, &run.Score, &breakdownJSON, &run.Summary, &stackJSON, &created, &completed)
	if err == sql.ErrNoRows {
		return AuditRun{}, domainerr.NotFound("audit not found")
	}
	if err != nil {
		return AuditRun{}, err
	}
	parseAuditJSON(&run, breakdownJSON, stackJSON, created, completed)
	return run, nil
}

func scanAuditRows(rows *sql.Rows) (AuditRun, error) {
	var run AuditRun
	var breakdownJSON, stackJSON, created string
	var completed *string
	if err := rows.Scan(&run.ID, &run.RepoID, &run.Status, &run.Score, &breakdownJSON, &run.Summary, &stackJSON, &created, &completed); err != nil {
		return AuditRun{}, err
	}
	parseAuditJSON(&run, breakdownJSON, stackJSON, created, completed)
	return run, nil
}

func parseAuditJSON(run *AuditRun, breakdownJSON, stackJSON, created string, completed *string) {
	run.ScoreBreakdown = map[string]int{}
	run.Stack = map[string]string{}
	_ = json.Unmarshal([]byte(breakdownJSON), &run.ScoreBreakdown)
	_ = json.Unmarshal([]byte(stackJSON), &run.Stack)
	run.CreatedAt, _ = time.Parse(time.RFC3339, created)
	if completed != nil {
		t, _ := time.Parse(time.RFC3339, *completed)
		run.CompletedAt = &t
	}
}

func scanFinding(rows *sql.Rows) (Finding, error) {
	var f Finding
	var refsJSON, detailsJSON, created string
	if err := rows.Scan(&f.ID, &f.AuditID, &f.RuleID, &f.Severity, &f.Priority, &f.State, &f.Category, &f.Title, &f.Description, &f.Confidence, &f.FilePath, &f.Line, &refsJSON, &detailsJSON, &created); err != nil {
		return Finding{}, err
	}
	_ = json.Unmarshal([]byte(refsJSON), &f.EvidenceRefs)
	_ = json.Unmarshal([]byte(detailsJSON), &f.Details)
	f.CreatedAt, _ = time.Parse(time.RFC3339, created)
	return f, nil
}

func fmtTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }
