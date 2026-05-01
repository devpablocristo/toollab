// Package projectaudit implements ToolLab's deterministic AI project audit MVP.
package projectaudit

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

//go:embed audit/migrations/*.sql
var migrationsFS embed.FS

const (
	statusRunning   = "running"
	statusCompleted = "completed"
)

type Project struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	SourcePath string    `json:"source_path"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type AuditConfig struct {
	GenerateTests          bool `json:"generate_tests"`
	RunExistingTests       bool `json:"run_existing_tests"`
	AllowDocsRead          bool `json:"allow_docs_read"`
	AllowDependencyInstall bool `json:"allow_dependency_install"`
}

func DefaultAuditConfig() AuditConfig {
	return AuditConfig{
		GenerateTests:          true,
		RunExistingTests:       true,
		AllowDocsRead:          false,
		AllowDependencyInstall: false,
	}
}

type AuditRun struct {
	ID             string            `json:"id"`
	ProjectID      string            `json:"project_id"`
	Status         string            `json:"status"`
	Score          int               `json:"score"`
	ScoreBreakdown map[string]int    `json:"score_breakdown"`
	Summary        string            `json:"summary"`
	Stack          map[string]string `json:"stack"`
	CreatedAt      time.Time         `json:"created_at"`
	CompletedAt    *time.Time        `json:"completed_at,omitempty"`
}

type Evidence struct {
	ID        string    `json:"id"`
	AuditID   string    `json:"audit_id"`
	Kind      string    `json:"kind"`
	Summary   string    `json:"summary"`
	Command   string    `json:"command,omitempty"`
	FilePath  string    `json:"file_path,omitempty"`
	Line      int       `json:"line,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Finding struct {
	ID           string         `json:"id"`
	AuditID      string         `json:"audit_id"`
	RuleID       string         `json:"rule_id"`
	Severity     string         `json:"severity"`
	Priority     string         `json:"priority"`
	State        string         `json:"state"`
	Category     string         `json:"category"`
	Title        string         `json:"title"`
	Description  string         `json:"description"`
	Confidence   string         `json:"confidence"`
	FilePath     string         `json:"file_path,omitempty"`
	Line         int            `json:"line,omitempty"`
	EvidenceRefs []Evidence     `json:"evidence_refs"`
	Details      FindingDetails `json:"details"`
	CreatedAt    time.Time      `json:"created_at"`
}

type FindingDetails struct {
	WhyProblem            string `json:"why_problem,omitempty"`
	Impact                string `json:"impact,omitempty"`
	RiskOfChange          string `json:"risk_of_change,omitempty"`
	MinimumRecommendation string `json:"minimum_recommendation,omitempty"`
	Avoid                 string `json:"avoid,omitempty"`
	Validation            string `json:"validation,omitempty"`
}

type GeneratedDoc struct {
	ID        string    `json:"id"`
	AuditID   string    `json:"audit_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type TestResult struct {
	ID            string    `json:"id"`
	AuditID       string    `json:"audit_id"`
	Kind          string    `json:"kind"`
	Name          string    `json:"name"`
	Command       string    `json:"command,omitempty"`
	Status        string    `json:"status"`
	Output        string    `json:"output,omitempty"`
	GeneratedPath string    `json:"generated_path,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type ScoreItem struct {
	ID             string    `json:"id"`
	AuditID        string    `json:"audit_id"`
	Category       string    `json:"category"`
	MaxPoints      int       `json:"max_points"`
	AwardedPoints  int       `json:"awarded_points"`
	DeductedPoints int       `json:"deducted_points"`
	Reason         string    `json:"reason"`
	CreatedAt      time.Time `json:"created_at"`
}

type AuditResult struct {
	Run        AuditRun       `json:"run"`
	Findings   []Finding      `json:"findings"`
	Evidence   []Evidence     `json:"evidence"`
	Docs       []GeneratedDoc `json:"docs"`
	Tests      []TestResult   `json:"tests"`
	ScoreItems []ScoreItem    `json:"score_items"`
}

type inventory struct {
	Files       []string
	Manifests   []string
	CI          []string
	Migrations  []string
	TestFiles   []string
	Commands    []testCommand
	Stack       map[string]string
	DocsSkipped int
}

type testCommand struct {
	Name string
	Dir  string
	Bin  string
	Args []string
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// MigrationStatements returns the SQL migrations owned by the audit service.
func MigrationStatements() ([]string, error) {
	entries, err := migrationsFS.ReadDir("audit/migrations")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	statements := make([]string, 0, len(names))
	for _, name := range names {
		data, err := migrationsFS.ReadFile("audit/migrations/" + name)
		if err != nil {
			return nil, err
		}
		statements = append(statements, string(data))
	}
	return statements, nil
}

func (s *Store) CreateProject(name, sourcePath string) (Project, error) {
	if strings.TrimSpace(name) == "" {
		return Project{}, validationError("name is required")
	}
	if strings.TrimSpace(sourcePath) == "" {
		return Project{}, validationError("source_path is required")
	}
	now := time.Now().UTC()
	project := Project{ID: newID(), Name: name, SourcePath: sourcePath, CreatedAt: now, UpdatedAt: now}
	_, err := s.db.Exec(
		`INSERT INTO projects (id,name,source_path,created_at,updated_at) VALUES (?,?,?,?,?)`,
		project.ID, project.Name, project.SourcePath, fmtTime(project.CreatedAt), fmtTime(project.UpdatedAt),
	)
	return project, err
}

func (s *Store) ListProjects() ([]Project, error) {
	rows, err := s.db.Query(`SELECT id,name,source_path,created_at,updated_at FROM projects ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, project)
	}
	return out, rows.Err()
}

func (s *Store) GetProject(id string) (Project, error) {
	row := s.db.QueryRow(`SELECT id,name,source_path,created_at,updated_at FROM projects WHERE id=?`, id)
	var p Project
	var created, updated string
	err := row.Scan(&p.ID, &p.Name, &p.SourcePath, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, notFoundError("project not found")
	}
	if err != nil {
		return Project{}, err
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339, created)
	p.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
	return p, nil
}

func (s *Store) CreateAudit(projectID string) (AuditRun, error) {
	now := time.Now().UTC()
	run := AuditRun{
		ID:             newID(),
		ProjectID:      projectID,
		Status:         statusRunning,
		ScoreBreakdown: map[string]int{},
		Stack:          map[string]string{},
		CreatedAt:      now,
	}
	_, err := s.db.Exec(`INSERT INTO audit_runs (id,project_id,status,created_at) VALUES (?,?,?,?)`, run.ID, run.ProjectID, run.Status, fmtTime(run.CreatedAt))
	return run, err
}

func (s *Store) CompleteAudit(run AuditRun) error {
	stackJSON, _ := json.Marshal(run.Stack)
	breakdownJSON, _ := json.Marshal(run.ScoreBreakdown)
	now := time.Now().UTC()
	_, err := s.db.Exec(
		`UPDATE audit_runs SET status=?, score=?, score_breakdown=?, summary=?, stack_json=?, completed_at=? WHERE id=?`,
		run.Status, run.Score, string(breakdownJSON), run.Summary, string(stackJSON), fmtTime(now), run.ID,
	)
	return err
}

func (s *Store) GetAudit(id string) (AuditRun, error) {
	row := s.db.QueryRow(`SELECT id,project_id,status,score,score_breakdown,summary,stack_json,created_at,completed_at FROM audit_runs WHERE id=?`, id)
	return scanAudit(row)
}

func (s *Store) ListAudits(projectID string) ([]AuditRun, error) {
	rows, err := s.db.Query(
		`SELECT id,project_id,status,score,score_breakdown,summary,stack_json,created_at,completed_at FROM audit_runs WHERE project_id=? ORDER BY created_at DESC`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditRun
	for rows.Next() {
		run, err := scanAuditRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	if out == nil {
		out = []AuditRun{}
	}
	return out, rows.Err()
}

func (s *Store) SaveEvidence(e Evidence) (Evidence, error) {
	if e.ID == "" {
		e.ID = newID()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(
		`INSERT INTO evidence (id,audit_id,kind,summary,command,file_path,line,created_at) VALUES (?,?,?,?,?,?,?,?)`,
		e.ID, e.AuditID, e.Kind, e.Summary, e.Command, e.FilePath, e.Line, fmtTime(e.CreatedAt),
	)
	return e, err
}

func (s *Store) SaveFinding(f Finding) (Finding, error) {
	if f.ID == "" {
		f.ID = newID()
	}
	if f.CreatedAt.IsZero() {
		f.CreatedAt = time.Now().UTC()
	}
	refsJSON, _ := json.Marshal(f.EvidenceRefs)
	detailsJSON, _ := json.Marshal(f.Details)
	_, err := s.db.Exec(
		`INSERT INTO findings (id,audit_id,rule_id,severity,priority,state,category,title,description,confidence,file_path,line,evidence_json,details_json,created_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		f.ID, f.AuditID, f.RuleID, f.Severity, f.Priority, f.State, f.Category, f.Title, f.Description, f.Confidence, f.FilePath, f.Line, string(refsJSON), string(detailsJSON), fmtTime(f.CreatedAt),
	)
	return f, err
}

func (s *Store) SaveDoc(doc GeneratedDoc) (GeneratedDoc, error) {
	if doc.ID == "" {
		doc.ID = newID()
	}
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(`INSERT INTO generated_docs (id,audit_id,title,content,created_at) VALUES (?,?,?,?,?)`, doc.ID, doc.AuditID, doc.Title, doc.Content, fmtTime(doc.CreatedAt))
	return doc, err
}

func (s *Store) SaveTestResult(t TestResult) (TestResult, error) {
	if t.ID == "" {
		t.ID = newID()
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(
		`INSERT INTO test_results (id,audit_id,kind,name,command,status,output,generated_path,created_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		t.ID, t.AuditID, t.Kind, t.Name, t.Command, t.Status, t.Output, t.GeneratedPath, fmtTime(t.CreatedAt),
	)
	return t, err
}

func (s *Store) SaveScoreItem(item ScoreItem) (ScoreItem, error) {
	if item.ID == "" {
		item.ID = newID()
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(
		`INSERT INTO score_items (id,audit_id,category,max_points,awarded_points,deducted_points,reason,created_at) VALUES (?,?,?,?,?,?,?,?)`,
		item.ID, item.AuditID, item.Category, item.MaxPoints, item.AwardedPoints, item.DeductedPoints, item.Reason, fmtTime(item.CreatedAt),
	)
	return item, err
}

func (s *Store) ListEvidence(auditID string) ([]Evidence, error) {
	rows, err := s.db.Query(`SELECT id,audit_id,kind,summary,command,file_path,line,created_at FROM evidence WHERE audit_id=? ORDER BY created_at ASC`, auditID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Evidence
	for rows.Next() {
		var e Evidence
		var created string
		if err := rows.Scan(&e.ID, &e.AuditID, &e.Kind, &e.Summary, &e.Command, &e.FilePath, &e.Line, &created); err != nil {
			return nil, err
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339, created)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) ListFindings(auditID string) ([]Finding, error) {
	rows, err := s.db.Query(
		`SELECT id,audit_id,rule_id,severity,priority,state,category,title,description,confidence,file_path,line,evidence_json,details_json,created_at FROM findings WHERE audit_id=?`,
		auditID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Finding
	for rows.Next() {
		var f Finding
		var refsJSON, detailsJSON, created string
		if err := rows.Scan(&f.ID, &f.AuditID, &f.RuleID, &f.Severity, &f.Priority, &f.State, &f.Category, &f.Title, &f.Description, &f.Confidence, &f.FilePath, &f.Line, &refsJSON, &detailsJSON, &created); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(refsJSON), &f.EvidenceRefs)
		_ = json.Unmarshal([]byte(detailsJSON), &f.Details)
		f.CreatedAt, _ = time.Parse(time.RFC3339, created)
		out = append(out, f)
	}
	sortFindings(out)
	return out, rows.Err()
}

func (s *Store) ListDocs(auditID string) ([]GeneratedDoc, error) {
	rows, err := s.db.Query(`SELECT id,audit_id,title,content,created_at FROM generated_docs WHERE audit_id=? ORDER BY created_at DESC`, auditID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GeneratedDoc
	for rows.Next() {
		var doc GeneratedDoc
		var created string
		if err := rows.Scan(&doc.ID, &doc.AuditID, &doc.Title, &doc.Content, &created); err != nil {
			return nil, err
		}
		doc.CreatedAt, _ = time.Parse(time.RFC3339, created)
		out = append(out, doc)
	}
	return out, rows.Err()
}

func (s *Store) ListTests(auditID string) ([]TestResult, error) {
	rows, err := s.db.Query(`SELECT id,audit_id,kind,name,command,status,output,generated_path,created_at FROM test_results WHERE audit_id=? ORDER BY created_at ASC`, auditID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TestResult
	for rows.Next() {
		var t TestResult
		var created string
		if err := rows.Scan(&t.ID, &t.AuditID, &t.Kind, &t.Name, &t.Command, &t.Status, &t.Output, &t.GeneratedPath, &created); err != nil {
			return nil, err
		}
		t.CreatedAt, _ = time.Parse(time.RFC3339, created)
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) ListScoreItems(auditID string) ([]ScoreItem, error) {
	rows, err := s.db.Query(`SELECT id,audit_id,category,max_points,awarded_points,deducted_points,reason,created_at FROM score_items WHERE audit_id=? ORDER BY category ASC`, auditID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScoreItem
	for rows.Next() {
		var item ScoreItem
		var created string
		if err := rows.Scan(&item.ID, &item.AuditID, &item.Category, &item.MaxPoints, &item.AwardedPoints, &item.DeductedPoints, &item.Reason, &created); err != nil {
			return nil, err
		}
		item.CreatedAt, _ = time.Parse(time.RFC3339, created)
		out = append(out, item)
	}
	return out, rows.Err()
}

type Engine struct {
	store   *Store
	dataDir string
}

func NewEngine(store *Store, dataDir string) *Engine {
	return &Engine{store: store, dataDir: dataDir}
}

func (e *Engine) Run(ctx context.Context, project Project, cfg AuditConfig) (AuditResult, error) {
	if info, err := os.Stat(project.SourcePath); err != nil || !info.IsDir() {
		return AuditResult{}, validationError("source_path is not an accessible directory")
	}
	run, err := e.store.CreateAudit(project.ID)
	if err != nil {
		return AuditResult{}, err
	}

	inv, inventoryEvidence := scanInventory(project.SourcePath, cfg.AllowDocsRead)
	inventoryEvidence.AuditID = run.ID
	inventoryEvidence, _ = e.store.SaveEvidence(inventoryEvidence)

	findings := buildFindings(run.ID, project.SourcePath, inv)
	tests := []TestResult{}
	if cfg.RunExistingTests {
		tests = append(tests, runExistingTests(ctx, inv, cfg.AllowDependencyInstall)...)
	}
	if cfg.GenerateTests {
		tests = append(tests, generatedTestSignals(inv)...)
	}
	findings = append(findings, findingsFromTests(run.ID, tests)...)
	doc := generateDoc(run.ID, project, inv, findings, tests)

	result := AuditResult{Evidence: []Evidence{inventoryEvidence}}
	for _, finding := range findings {
		finding.AuditID = run.ID
		for i := range finding.EvidenceRefs {
			finding.EvidenceRefs[i].AuditID = run.ID
			saved, err := e.store.SaveEvidence(finding.EvidenceRefs[i])
			if err == nil {
				finding.EvidenceRefs[i] = saved
				result.Evidence = append(result.Evidence, saved)
			}
		}
		saved, err := e.store.SaveFinding(finding)
		if err == nil {
			result.Findings = append(result.Findings, saved)
		}
	}
	for _, test := range tests {
		test.AuditID = run.ID
		saved, err := e.store.SaveTestResult(test)
		if err == nil {
			result.Tests = append(result.Tests, saved)
		}
	}
	if _, err := e.store.SaveDoc(doc); err != nil {
		return AuditResult{}, err
	}

	breakdown, scoreItems := scoreBreakdown(run.ID, result.Findings, result.Tests, inv)
	for _, item := range scoreItems {
		saved, err := e.store.SaveScoreItem(item)
		if err == nil {
			result.ScoreItems = append(result.ScoreItems, saved)
		}
	}
	run.Stack = inv.Stack
	run.ScoreBreakdown = breakdown
	run.Score = totalScore(breakdown)
	run.Status = statusCompleted
	run.Summary = fmt.Sprintf("%d findings, %d tests recorded, %d evidence items.", len(result.Findings), len(result.Tests), len(result.Evidence))
	if err := e.store.CompleteAudit(run); err != nil {
		return AuditResult{}, err
	}
	return e.loadResult(run.ID)
}

func (e *Engine) loadResult(auditID string) (AuditResult, error) {
	run, err := e.store.GetAudit(auditID)
	if err != nil {
		return AuditResult{}, err
	}
	findings, err := e.store.ListFindings(auditID)
	if err != nil {
		return AuditResult{}, err
	}
	evidence, err := e.store.ListEvidence(auditID)
	if err != nil {
		return AuditResult{}, err
	}
	docs, err := e.store.ListDocs(auditID)
	if err != nil {
		return AuditResult{}, err
	}
	tests, err := e.store.ListTests(auditID)
	if err != nil {
		return AuditResult{}, err
	}
	scoreItems, err := e.store.ListScoreItems(auditID)
	if err != nil {
		return AuditResult{}, err
	}
	return AuditResult{Run: run, Findings: emptyFindings(findings), Evidence: emptyEvidence(evidence), Docs: emptyDocs(docs), Tests: emptyTests(tests), ScoreItems: emptyScoreItems(scoreItems)}, nil
}

type Handler struct {
	store  *Store
	engine *Engine
}

func NewHandler(store *Store, engine *Engine) *Handler {
	return &Handler{store: store, engine: engine}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/projects", h.listProjects)
	r.Post("/projects", h.createProject)
	r.Get("/projects/{project_id}/audits", h.listProjectAudits)
	r.Post("/projects/{project_id}/audits", h.createAudit)
	r.Get("/audits/{audit_id}", h.getAudit)
	r.Get("/audits/{audit_id}/findings", h.listFindings)
	r.Get("/audits/{audit_id}/evidence", h.listEvidence)
	r.Get("/audits/{audit_id}/docs", h.listDocs)
	r.Get("/audits/{audit_id}/tests", h.listTests)
	r.Get("/audits/{audit_id}/score", h.listScoreItems)
	return r
}

func (h *Handler) createProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       string `json:"name"`
		SourcePath string `json:"source_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, validationError("invalid JSON body"))
		return
	}
	project, err := h.store.CreateProject(req.Name, req.SourcePath)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, project)
}

func (h *Handler) listProjects(w http.ResponseWriter, _ *http.Request) {
	projects, err := h.store.ListProjects()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": emptyProjects(projects)})
}

func (h *Handler) createAudit(w http.ResponseWriter, r *http.Request) {
	project, err := h.store.GetProject(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	cfg := DefaultAuditConfig()
	if r.Body != nil {
		var req AuditConfig
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			cfg = req
		}
	}
	result, err := h.engine.Run(r.Context(), project, cfg)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) listProjectAudits(w http.ResponseWriter, r *http.Request) {
	runs, err := h.store.ListAudits(chi.URLParam(r, "project_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": emptyAudits(runs)})
}

func (h *Handler) getAudit(w http.ResponseWriter, r *http.Request) {
	run, err := h.store.GetAudit(chi.URLParam(r, "audit_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (h *Handler) listFindings(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListFindings(chi.URLParam(r, "audit_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": emptyFindings(items)})
}

func (h *Handler) listEvidence(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListEvidence(chi.URLParam(r, "audit_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": emptyEvidence(items)})
}

func (h *Handler) listDocs(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListDocs(chi.URLParam(r, "audit_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": emptyDocs(items)})
}

func (h *Handler) listTests(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListTests(chi.URLParam(r, "audit_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": emptyTests(items)})
}

func (h *Handler) listScoreItems(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListScoreItems(chi.URLParam(r, "audit_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": emptyScoreItems(items)})
}

func scanInventory(root string, allowDocs bool) (inventory, Evidence) {
	inv := inventory{Stack: map[string]string{}}
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if shouldSkipDir(rel) {
				return filepath.SkipDir
			}
			if !allowDocs && isDocPath(rel, true) {
				inv.DocsSkipped++
				return filepath.SkipDir
			}
			return nil
		}
		if !allowDocs && isDocPath(rel, false) {
			inv.DocsSkipped++
			return nil
		}
		inv.Files = append(inv.Files, rel)
		classifyFile(&inv, root, rel)
		return nil
	})
	sort.Strings(inv.Files)
	sort.Strings(inv.Manifests)
	sort.Strings(inv.CI)
	sort.Strings(inv.Migrations)
	sort.Strings(inv.TestFiles)
	detectStack(&inv)
	detectCommands(&inv, root)
	return inv, Evidence{Kind: "inventory", Summary: fmt.Sprintf("Inventoried %d files, %d manifests, %d CI files, %d test files; skipped %d docs by policy.", len(inv.Files), len(inv.Manifests), len(inv.CI), len(inv.TestFiles), inv.DocsSkipped)}
}

func classifyFile(inv *inventory, root, rel string) {
	base := filepath.Base(rel)
	switch base {
	case "go.mod", "package.json", "package-lock.json", "yarn.lock", "pnpm-lock.yaml", "pyproject.toml", "requirements.txt", "Pipfile", "Makefile", "Dockerfile":
		inv.Manifests = append(inv.Manifests, rel)
	}
	if strings.HasPrefix(rel, ".github/workflows/") || base == ".gitlab-ci.yml" || base == "Jenkinsfile" {
		inv.CI = append(inv.CI, rel)
	}
	if strings.HasSuffix(strings.ToLower(rel), ".sql") || strings.Contains(strings.ToLower(rel), "migration") {
		inv.Migrations = append(inv.Migrations, rel)
	}
	if isTestFile(rel) {
		inv.TestFiles = append(inv.TestFiles, rel)
	}
	_ = root
}

func detectStack(inv *inventory) {
	if hasFile(inv.Files, "go.mod") {
		inv.Stack["go"] = "detected"
	}
	if hasFile(inv.Files, "package.json") {
		inv.Stack["node"] = "detected"
		inv.Stack["react"] = "probable"
	}
	if hasFile(inv.Files, "pyproject.toml") || hasFile(inv.Files, "requirements.txt") || hasFile(inv.Files, "Pipfile") {
		inv.Stack["python"] = "detected"
	}
	if len(inv.Migrations) > 0 {
		inv.Stack["database"] = "detected"
	}
	if len(inv.CI) > 0 {
		inv.Stack["ci"] = "detected"
	}
}

func detectCommands(inv *inventory, root string) {
	for _, file := range inv.Files {
		switch filepath.Base(file) {
		case "go.mod":
			inv.Commands = append(inv.Commands, testCommand{Name: "go test", Dir: filepath.Join(root, filepath.Dir(file)), Bin: "go", Args: []string{"test", "./...", "-count=1"}})
		case "package.json":
			dir := filepath.Join(root, filepath.Dir(file))
			if scripts := readPackageScripts(filepath.Join(root, file)); scripts["typecheck"] {
				inv.Commands = append(inv.Commands, testCommand{Name: "npm run typecheck", Dir: dir, Bin: "npm", Args: []string{"run", "typecheck"}})
			}
			if scripts := readPackageScripts(filepath.Join(root, file)); scripts["build"] {
				inv.Commands = append(inv.Commands, testCommand{Name: "npm run build", Dir: dir, Bin: "npm", Args: []string{"run", "build"}})
			}
			if scripts := readPackageScripts(filepath.Join(root, file)); scripts["test"] {
				inv.Commands = append(inv.Commands, testCommand{Name: "npm test", Dir: dir, Bin: "npm", Args: []string{"test"}})
			}
		}
	}
}

func buildFindings(auditID, root string, inv inventory) []Finding {
	var findings []Finding
	if len(inv.CI) == 0 {
		findings = append(findings, finding(auditID, "repo.no_ci", "Medium", "P2", "Confirmed", "ci", "No CI workflow detected", "No GitHub Actions, GitLab CI, or Jenkins workflow was found.", "High", "", 0, Evidence{Kind: "inventory", Summary: "No CI files found."}))
	}
	if len(inv.TestFiles) == 0 {
		findings = append(findings, finding(auditID, "repo.no_tests", "Medium", "P2", "Confirmed", "tests", "No automated test files detected", "No Go, JS/TS, or Python test files were found.", "High", "", 0, Evidence{Kind: "inventory", Summary: "No test files found by naming convention."}))
	}
	for _, rel := range inv.Files {
		path := filepath.Join(root, rel)
		lines, ok := readLines(path)
		if !ok {
			continue
		}
		findings = append(findings, sourceFindings(auditID, rel, lines)...)
	}
	return findings
}

func sourceFindings(auditID, rel string, lines []string) []Finding {
	var findings []Finding
	content := strings.Join(lines, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.Contains(trimmed, "http.ListenAndServe(") && !strings.Contains(content, "ReadHeaderTimeout"):
			findings = append(findings, finding(auditID, "go.http_server_without_timeouts", "Medium", "P2", "Probable", "go", "HTTP server without explicit timeouts", "A Go HTTP server was detected without visible timeout configuration.", "Medium", rel, i+1, Evidence{Kind: "source", FilePath: rel, Line: i + 1, Summary: trimmed}))
		case strings.Contains(trimmed, "shell=True"):
			findings = append(findings, finding(auditID, "python.subprocess_shell_true", "High", "P1", "Confirmed", "security", "subprocess uses shell=True", "Shell command execution can become command injection if input reaches it.", "High", rel, i+1, Evidence{Kind: "source", FilePath: rel, Line: i + 1, Summary: trimmed}))
		case strings.Contains(trimmed, "requests.") && strings.Contains(trimmed, "(") && !strings.Contains(trimmed, "timeout="):
			findings = append(findings, finding(auditID, "python.requests_without_timeout", "Medium", "P2", "Probable", "python", "HTTP request without timeout", "A Python requests call was detected without timeout= on the same line.", "Medium", rel, i+1, Evidence{Kind: "source", FilePath: rel, Line: i + 1, Summary: trimmed}))
		case strings.Contains(trimmed, "fetch(") && !strings.Contains(rel, "api."):
			findings = append(findings, finding(auditID, "frontend.direct_fetch", "Low", "P3", "Hypothesis", "frontend", "Direct fetch call outside API module", "A direct fetch call can fragment error/loading handling.", "Low", rel, i+1, Evidence{Kind: "source", FilePath: rel, Line: i + 1, Summary: trimmed}))
		case strings.Contains(strings.ToUpper(trimmed), "CREATE TABLE") && !statementHasPrimaryKey(lines, i):
			findings = append(findings, finding(auditID, "sql.table_without_primary_key", "Medium", "P2", "Confirmed", "database", "Table without primary key", "A CREATE TABLE statement did not show a PRIMARY KEY before the statement ended.", "High", rel, i+1, Evidence{Kind: "sql", FilePath: rel, Line: i + 1, Summary: trimmed}))
		}
	}
	return findings
}

func runExistingTests(ctx context.Context, inv inventory, allowInstall bool) []TestResult {
	var out []TestResult
	for _, command := range inv.Commands {
		if command.Bin == "npm" && !allowInstall {
			if _, err := os.Stat(filepath.Join(command.Dir, "node_modules")); err != nil {
				out = append(out, TestResult{Kind: "existing", Name: command.Name, Command: command.String(), Status: "blocked", Output: "node_modules is absent and dependency installation is disabled"})
				continue
			}
		}
		out = append(out, runCommand(ctx, command))
	}
	return out
}

func generatedTestSignals(inv inventory) []TestResult {
	if len(inv.Stack) == 0 {
		return nil
	}
	return []TestResult{{Kind: "generated", Name: "generated smoke test", Status: "blocked", Output: "MVP records generated-test intent but does not write generated tests into the source project."}}
}

func runCommand(ctx context.Context, command testCommand) TestResult {
	runCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(runCtx, command.Bin, command.Args...)
	cmd.Dir = command.Dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	status := "passed"
	if err != nil {
		status = "failed"
		if runCtx.Err() != nil {
			status = "timeout"
		}
	}
	return TestResult{Kind: "existing", Name: command.Name, Command: command.String(), Status: status, Output: truncate(buf.String(), 8000)}
}

func findingsFromTests(auditID string, tests []TestResult) []Finding {
	var findings []Finding
	for _, test := range tests {
		if test.Status == "passed" {
			continue
		}
		priority := "P2"
		severity := "Medium"
		if test.Status == "blocked" {
			priority = "P3"
			severity = "Low"
		}
		findings = append(findings, finding(auditID, "tests.not_passing", severity, priority, "Confirmed", "tests", "Validation did not pass: "+test.Name, "A validation command failed, timed out, or was blocked.", "High", test.GeneratedPath, 0, Evidence{Kind: "test", Command: test.Command, Summary: test.Name + " status: " + test.Status}))
	}
	return findings
}

func generateDoc(auditID string, project Project, inv inventory, findings []Finding, tests []TestResult) GeneratedDoc {
	var b strings.Builder
	b.WriteString("# " + project.Name + "\n\n")
	b.WriteString("Generated by ToolLab from project inventory, deterministic rules, evidence, and test results.\n\n")
	b.WriteString("## Stack\n\n")
	for _, key := range sortedKeys(inv.Stack) {
		b.WriteString("- " + key + ": " + inv.Stack[key] + "\n")
	}
	b.WriteString("\n## Findings\n\n")
	if len(findings) == 0 {
		b.WriteString("- No findings detected by MVP rules.\n")
	}
	for _, f := range findings {
		b.WriteString(fmt.Sprintf("- [%s/%s] %s\n", f.Severity, f.Priority, f.Title))
	}
	b.WriteString("\n## Validation\n\n")
	if len(tests) == 0 {
		b.WriteString("- No validation commands were recorded.\n")
	}
	for _, t := range tests {
		b.WriteString(fmt.Sprintf("- %s: %s\n", t.Name, t.Status))
	}
	return GeneratedDoc{AuditID: auditID, Title: project.Name + " audit documentation", Content: b.String(), CreatedAt: time.Now().UTC()}
}

func scoreBreakdown(auditID string, findings []Finding, tests []TestResult, inv inventory) (map[string]int, []ScoreItem) {
	breakdown := map[string]int{
		"build_tests":       15,
		"findings":          35,
		"test_coverage":     5,
		"docs_traceability": 10,
		"ci_config":         0,
	}
	if len(inv.CI) > 0 {
		breakdown["ci_config"] = 10
	}
	if len(inv.TestFiles) > 0 {
		breakdown["test_coverage"] = 15
	}
	passed, failed := 0, 0
	for _, test := range tests {
		if test.Status == "passed" {
			passed++
		}
		if test.Status == "failed" || test.Status == "timeout" {
			failed++
		}
	}
	if passed > 0 && failed == 0 {
		breakdown["build_tests"] = 30
	}
	if failed > 0 {
		breakdown["build_tests"] = 5
	}
	penalty := 0
	for _, f := range findings {
		switch f.Severity {
		case "High":
			penalty += 12
		case "Medium":
			penalty += 6
		case "Low":
			penalty += 2
		}
	}
	if penalty > 35 {
		penalty = 35
	}
	breakdown["findings"] = 35 - penalty
	items := []ScoreItem{
		scoreItem(auditID, "build_tests", 30, breakdown["build_tests"], "Validation command results."),
		scoreItem(auditID, "findings", 35, breakdown["findings"], "Finding severity penalties."),
		scoreItem(auditID, "test_coverage", 15, breakdown["test_coverage"], "Detected test files and generated-test signal."),
		scoreItem(auditID, "docs_traceability", 10, breakdown["docs_traceability"], "Generated documentation from evidence."),
		scoreItem(auditID, "ci_config", 10, breakdown["ci_config"], "Detected CI configuration."),
	}
	return breakdown, items
}

func scoreItem(auditID, category string, max, awarded int, reason string) ScoreItem {
	return ScoreItem{AuditID: auditID, Category: category, MaxPoints: max, AwardedPoints: awarded, DeductedPoints: max - awarded, Reason: reason, CreatedAt: time.Now().UTC()}
}

func totalScore(breakdown map[string]int) int {
	total := 0
	for _, v := range breakdown {
		total += v
	}
	if total < 0 {
		return 0
	}
	if total > 100 {
		return 100
	}
	return total
}

func finding(auditID, ruleID, severity, priority, state, category, title, description, confidence, filePath string, line int, evidence Evidence) Finding {
	details := FindingDetails{
		WhyProblem:            description,
		Impact:                "May reduce trust in AI-generated work.",
		RiskOfChange:          "Low if fixed with a focused change.",
		MinimumRecommendation: "Apply the smallest change that addresses the evidence.",
		Avoid:                 "Avoid broad rewrites without stronger evidence.",
		Validation:            "Re-run ToolLab and the project validation commands.",
	}
	return Finding{
		AuditID:      auditID,
		RuleID:       ruleID,
		Severity:     severity,
		Priority:     priority,
		State:        state,
		Category:     category,
		Title:        title,
		Description:  description,
		Confidence:   confidence,
		FilePath:     filePath,
		Line:         line,
		EvidenceRefs: []Evidence{evidence},
		Details:      details,
		CreatedAt:    time.Now().UTC(),
	}
}

func scanProject(rows *sql.Rows) (Project, error) {
	var p Project
	var created, updated string
	if err := rows.Scan(&p.ID, &p.Name, &p.SourcePath, &created, &updated); err != nil {
		return Project{}, err
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339, created)
	p.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
	return p, nil
}

func scanAudit(row *sql.Row) (AuditRun, error) {
	var run AuditRun
	var breakdownJSON, stackJSON, created string
	var completed *string
	err := row.Scan(&run.ID, &run.ProjectID, &run.Status, &run.Score, &breakdownJSON, &run.Summary, &stackJSON, &created, &completed)
	if errors.Is(err, sql.ErrNoRows) {
		return AuditRun{}, notFoundError("audit not found")
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
	if err := rows.Scan(&run.ID, &run.ProjectID, &run.Status, &run.Score, &breakdownJSON, &run.Summary, &stackJSON, &created, &completed); err != nil {
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

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "INTERNAL"
	if isValidation(err) {
		status = http.StatusBadRequest
		code = "VALIDATION_ERROR"
	}
	if isNotFound(err) {
		status = http.StatusNotFound
		code = "NOT_FOUND"
	}
	writeJSON(w, status, map[string]string{"code": code, "error": err.Error()})
}

type appError struct {
	kind string
	msg  string
}

func (e appError) Error() string { return e.msg }

func validationError(msg string) error { return appError{kind: "validation", msg: msg} }
func notFoundError(msg string) error   { return appError{kind: "not_found", msg: msg} }
func isValidation(err error) bool {
	var e appError
	return errors.As(err, &e) && e.kind == "validation"
}
func isNotFound(err error) bool {
	var e appError
	return errors.As(err, &e) && e.kind == "not_found"
}

func fmtTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func shouldSkipDir(rel string) bool {
	switch filepath.Base(rel) {
	case ".git", "node_modules", "vendor", "dist", "build", "coverage", "__pycache__", ".cache", ".pytest_cache", "bin", "data":
		return true
	default:
		return false
	}
}

func isDocPath(rel string, isDir bool) bool {
	base := strings.ToLower(filepath.Base(rel))
	if strings.HasPrefix(base, "readme") || strings.HasPrefix(base, "changelog") {
		return true
	}
	if isDir {
		return base == "docs" || base == "wiki"
	}
	return strings.HasSuffix(base, ".md") || strings.HasSuffix(base, ".mdx") || strings.Contains(rel, "/docs/")
}

func isTestFile(rel string) bool {
	base := strings.ToLower(filepath.Base(rel))
	return strings.HasSuffix(base, "_test.go") ||
		strings.Contains(base, ".test.") ||
		strings.Contains(base, ".spec.") ||
		(strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py")) ||
		strings.HasSuffix(base, "_test.py")
}

func hasFile(files []string, name string) bool {
	for _, file := range files {
		if file == name || strings.HasSuffix(file, "/"+name) {
			return true
		}
	}
	return false
}

func readPackageScripts(path string) map[string]bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]bool{}
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	_ = json.Unmarshal(data, &pkg)
	out := map[string]bool{}
	for name := range pkg.Scripts {
		out[name] = true
	}
	return out
}

func readLines(path string) ([]string, bool) {
	data, err := os.ReadFile(path)
	if err != nil || len(data) > 512_000 {
		return nil, false
	}
	return strings.Split(string(data), "\n"), true
}

func statementHasPrimaryKey(lines []string, start int) bool {
	for i := start; i < len(lines) && i < start+80; i++ {
		if strings.Contains(strings.ToUpper(lines[i]), "PRIMARY KEY") {
			return true
		}
		if strings.Contains(lines[i], ";") {
			return false
		}
	}
	return false
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortFindings(findings []Finding) {
	order := map[string]int{"High": 0, "Medium": 1, "Low": 2, "Informative": 3}
	sort.SliceStable(findings, func(i, j int) bool {
		return order[findings[i].Severity] < order[findings[j].Severity]
	})
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n...[truncated]"
}

func emptyProjects(items []Project) []Project {
	if items == nil {
		return []Project{}
	}
	return items
}
func emptyAudits(items []AuditRun) []AuditRun {
	if items == nil {
		return []AuditRun{}
	}
	return items
}
func emptyFindings(items []Finding) []Finding {
	if items == nil {
		return []Finding{}
	}
	return items
}
func emptyEvidence(items []Evidence) []Evidence {
	if items == nil {
		return []Evidence{}
	}
	return items
}
func emptyDocs(items []GeneratedDoc) []GeneratedDoc {
	if items == nil {
		return []GeneratedDoc{}
	}
	return items
}
func emptyTests(items []TestResult) []TestResult {
	if items == nil {
		return []TestResult{}
	}
	return items
}
func emptyScoreItems(items []ScoreItem) []ScoreItem {
	if items == nil {
		return []ScoreItem{}
	}
	return items
}

func (c testCommand) String() string {
	return strings.TrimSpace(c.Bin + " " + strings.Join(c.Args, " "))
}

var _ = regexp.MustCompile
