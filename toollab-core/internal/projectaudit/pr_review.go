package projectaudit

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	maxPRReviewDiffBytes = 1 << 20

	prDecisionApprove        = "APPROVE"
	prDecisionReviewRequired = "REVIEW_REQUIRED"
	prDecisionBlockMerge     = "BLOCK_MERGE"

	prConfidenceHigh   = "HIGH"
	prConfidenceMedium = "MEDIUM"
	prConfidenceLow    = "LOW"

	prSpecProvided = "SPEC_PROVIDED"
	prSpecMissing  = "SPEC_MISSING"

	prSeverityCritical = "CRITICAL"
	prSeverityHigh     = "HIGH"
	prSeverityMedium   = "MEDIUM"
	prSeverityLow      = "LOW"

	prStatusConfirmed      = "CONFIRMED"
	prStatusLikely         = "LIKELY"
	prStatusPossible       = "POSSIBLE"
	prStatusMissingContext = "MISSING_CONTEXT"
)

type PRReview struct {
	ID           string `json:"id"`
	ProjectID    string `json:"project_id"`
	TaskSpecID   string `json:"task_spec_id,omitempty"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	DiffText     string `json:"diff_text,omitempty"`
	ProjectRules string `json:"project_rules"`
	TestOutput   string `json:"test_output"`
	ReviewPrompt string `json:"review_prompt"`
	Summary      string `json:"summary"`
	Score        int    `json:"score"`
	Decision     string `json:"decision"`
	Confidence   string `json:"confidence"`
	SpecStatus   string `json:"spec_status"`
	SDDContextMD string `json:"sdd_context_md"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type CreatePRReviewRequest struct {
	TaskSpecID   string `json:"task_spec_id"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	DiffText     string `json:"diff_text"`
	ProjectRules string `json:"project_rules"`
	TestOutput   string `json:"test_output"`
}

type PRReviewFile struct {
	ID         string `json:"id"`
	ReviewID   string `json:"review_id"`
	Path       string `json:"path"`
	ChangeType string `json:"change_type"`
	RiskArea   string `json:"risk_area"`
	RiskLevel  string `json:"risk_level"`
	Additions  int    `json:"additions"`
	Deletions  int    `json:"deletions"`
}

type PRReviewFinding struct {
	ID                 string   `json:"id"`
	ReviewID           string   `json:"review_id"`
	Code               string   `json:"code"`
	Severity           string   `json:"severity"`
	Status             string   `json:"status"`
	Title              string   `json:"title"`
	Files              []string `json:"files"`
	SpecRuleAffected   string   `json:"spec_rule_affected"`
	Problem            string   `json:"problem"`
	Evidence           string   `json:"evidence"`
	Impact             string   `json:"impact"`
	SuggestedFix       string   `json:"suggested_fix"`
	AICorrectionPrompt string   `json:"ai_correction_prompt"`
	SortOrder          int      `json:"sort_order"`
	CreatedAt          string   `json:"created_at"`
}

type PRReviewResult struct {
	Review   PRReview          `json:"review"`
	Files    []PRReviewFile    `json:"files"`
	Findings []PRReviewFinding `json:"findings"`
}

type computedReview struct {
	Files        []PRReviewFile
	Findings     []PRReviewFinding
	ReviewPrompt string
	Summary      string
	Score        int
	Decision     string
	Confidence   string
	SpecStatus   string
	SDDContextMD string
	Stack        []string
}

func (e *Engine) ReviewPR(ctx context.Context, projectID string, req CreatePRReviewRequest) (PRReviewResult, error) {
	_ = ctx
	if _, err := e.store.GetProject(projectID); err != nil {
		return PRReviewResult{}, err
	}
	if strings.TrimSpace(req.Title) == "" {
		return PRReviewResult{}, validationError("title is required")
	}
	if strings.TrimSpace(req.DiffText) == "" {
		return PRReviewResult{}, validationError("diff_text is required")
	}
	if len([]byte(req.DiffText)) > maxPRReviewDiffBytes {
		return PRReviewResult{}, validationError("diff_text exceeds 1 MiB")
	}

	var spec *TaskSpec
	if strings.TrimSpace(req.TaskSpecID) != "" {
		loaded, err := e.store.GetTaskSpec(strings.TrimSpace(req.TaskSpecID))
		if err != nil {
			return PRReviewResult{}, err
		}
		if loaded.ProjectID != projectID {
			return PRReviewResult{}, notFoundError("task spec not found")
		}
		spec = &loaded
	}

	parsedFiles := parseDiff(req.DiffText)
	files := reviewFilesFromParsed(parsedFiles)
	specStatus := prSpecMissing
	sddContext := inferredSDDContext(req, files)
	if spec != nil {
		specStatus = prSpecProvided
		sddContext = spec.SpecMD
	}

	findings := buildPRFindings(req, spec, specStatus, parsedFiles, files)
	score, decision, confidence := scorePRReview(findings, files, specStatus, req.DiffText, req.TestOutput)
	summary := summarizePRReview(files, findings, score, decision)
	for i := range findings {
		findings[i].SortOrder = i + 1
		findings[i].AICorrectionPrompt = buildAICorrectionPrompt(req, sddContext, findings[i])
	}
	reviewPrompt := buildReviewPrompt(req, sddContext, files, findings, score, decision, confidence, specStatus)

	return e.store.CreatePRReview(projectID, req, computedReview{
		Files:        files,
		Findings:     findings,
		ReviewPrompt: reviewPrompt,
		Summary:      summary,
		Score:        score,
		Decision:     decision,
		Confidence:   confidence,
		SpecStatus:   specStatus,
		SDDContextMD: sddContext,
		Stack:        stackFromFiles(files),
	})
}

func (s *Store) CreatePRReview(projectID string, req CreatePRReviewRequest, result computedReview) (PRReviewResult, error) {
	now := fmtTime(time.Now().UTC())
	reviewID := newID()
	stackJSON, _ := json.Marshal(result.Stack)
	taskSpecID := sql.NullString{String: strings.TrimSpace(req.TaskSpecID), Valid: strings.TrimSpace(req.TaskSpecID) != ""}

	tx, err := s.db.Begin()
	if err != nil {
		return PRReviewResult{}, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`INSERT INTO pr_reviews
		 (id,project_id,task_spec_id,title,description,diff_text,project_rules,test_output,review_prompt,summary,score,decision,confidence,spec_status,sdd_context_md,stack_json,created_at,updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		reviewID, projectID, taskSpecID, strings.TrimSpace(req.Title), strings.TrimSpace(req.Description), req.DiffText, strings.TrimSpace(req.ProjectRules), strings.TrimSpace(req.TestOutput), result.ReviewPrompt, result.Summary, result.Score, result.Decision, result.Confidence, result.SpecStatus, result.SDDContextMD, string(stackJSON), now, now,
	)
	if err != nil {
		return PRReviewResult{}, err
	}

	files := make([]PRReviewFile, 0, len(result.Files))
	for _, file := range result.Files {
		file.ID = newID()
		file.ReviewID = reviewID
		_, err = tx.Exec(
			`INSERT INTO pr_review_files (id,review_id,path,change_type,risk_area,risk_level,additions,deletions)
			 VALUES (?,?,?,?,?,?,?,?)`,
			file.ID, file.ReviewID, file.Path, file.ChangeType, file.RiskArea, file.RiskLevel, file.Additions, file.Deletions,
		)
		if err != nil {
			return PRReviewResult{}, err
		}
		files = append(files, file)
	}

	findings := make([]PRReviewFinding, 0, len(result.Findings))
	for _, finding := range result.Findings {
		finding.ID = newID()
		finding.ReviewID = reviewID
		if finding.CreatedAt == "" {
			finding.CreatedAt = now
		}
		filesJSON, _ := json.Marshal(finding.Files)
		_, err = tx.Exec(
			`INSERT INTO pr_review_findings
			 (id,review_id,code,severity,status,title,files_json,spec_rule_affected,problem,evidence,impact,suggested_fix,ai_correction_prompt,sort_order,created_at)
			 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			finding.ID, finding.ReviewID, finding.Code, finding.Severity, finding.Status, finding.Title, string(filesJSON), finding.SpecRuleAffected, finding.Problem, finding.Evidence, finding.Impact, finding.SuggestedFix, finding.AICorrectionPrompt, finding.SortOrder, finding.CreatedAt,
		)
		if err != nil {
			return PRReviewResult{}, err
		}
		findings = append(findings, finding)
	}

	if err := tx.Commit(); err != nil {
		return PRReviewResult{}, err
	}
	return PRReviewResult{
		Review: PRReview{
			ID:           reviewID,
			ProjectID:    projectID,
			TaskSpecID:   taskSpecID.String,
			Title:        strings.TrimSpace(req.Title),
			Description:  strings.TrimSpace(req.Description),
			DiffText:     req.DiffText,
			ProjectRules: strings.TrimSpace(req.ProjectRules),
			TestOutput:   strings.TrimSpace(req.TestOutput),
			ReviewPrompt: result.ReviewPrompt,
			Summary:      result.Summary,
			Score:        result.Score,
			Decision:     result.Decision,
			Confidence:   result.Confidence,
			SpecStatus:   result.SpecStatus,
			SDDContextMD: result.SDDContextMD,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		Files:    emptyPRReviewFiles(files),
		Findings: emptyPRReviewFindings(findings),
	}, nil
}

func (s *Store) ListPRReviews(projectID string) ([]PRReview, error) {
	rows, err := s.db.Query(
		`SELECT id,project_id,task_spec_id,title,description,project_rules,test_output,summary,score,decision,confidence,spec_status,sdd_context_md,created_at,updated_at
		 FROM pr_reviews WHERE project_id=? ORDER BY created_at DESC`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PRReview
	for rows.Next() {
		review, err := scanPRReviewListRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, review)
	}
	if out == nil {
		out = []PRReview{}
	}
	return out, rows.Err()
}

func (s *Store) GetPRReview(id string) (PRReviewResult, error) {
	row := s.db.QueryRow(
		`SELECT id,project_id,task_spec_id,title,description,diff_text,project_rules,test_output,review_prompt,summary,score,decision,confidence,spec_status,sdd_context_md,created_at,updated_at
		 FROM pr_reviews WHERE id=?`,
		id,
	)
	review, err := scanPRReviewDetailRow(row)
	if err != nil {
		return PRReviewResult{}, err
	}
	files, err := s.ListPRReviewFiles(id)
	if err != nil {
		return PRReviewResult{}, err
	}
	findings, err := s.ListPRReviewFindings(id)
	if err != nil {
		return PRReviewResult{}, err
	}
	return PRReviewResult{Review: review, Files: files, Findings: findings}, nil
}

func (s *Store) ListPRReviewFindings(reviewID string) ([]PRReviewFinding, error) {
	rows, err := s.db.Query(
		`SELECT id,review_id,code,severity,status,title,files_json,spec_rule_affected,problem,evidence,impact,suggested_fix,ai_correction_prompt,sort_order,created_at
		 FROM pr_review_findings WHERE review_id=? ORDER BY sort_order ASC, created_at ASC`,
		reviewID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PRReviewFinding
	for rows.Next() {
		var finding PRReviewFinding
		var filesJSON string
		if err := rows.Scan(&finding.ID, &finding.ReviewID, &finding.Code, &finding.Severity, &finding.Status, &finding.Title, &filesJSON, &finding.SpecRuleAffected, &finding.Problem, &finding.Evidence, &finding.Impact, &finding.SuggestedFix, &finding.AICorrectionPrompt, &finding.SortOrder, &finding.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(filesJSON), &finding.Files)
		out = append(out, finding)
	}
	return emptyPRReviewFindings(out), rows.Err()
}

func (s *Store) ListPRReviewFiles(reviewID string) ([]PRReviewFile, error) {
	rows, err := s.db.Query(
		`SELECT id,review_id,path,change_type,risk_area,risk_level,additions,deletions
		 FROM pr_review_files WHERE review_id=? ORDER BY path ASC`,
		reviewID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PRReviewFile
	for rows.Next() {
		var file PRReviewFile
		if err := rows.Scan(&file.ID, &file.ReviewID, &file.Path, &file.ChangeType, &file.RiskArea, &file.RiskLevel, &file.Additions, &file.Deletions); err != nil {
			return nil, err
		}
		out = append(out, file)
	}
	return emptyPRReviewFiles(out), rows.Err()
}

type prReviewListScanner interface {
	Scan(dest ...any) error
}

func scanPRReviewListRow(row prReviewListScanner) (PRReview, error) {
	var review PRReview
	var taskSpec sql.NullString
	err := row.Scan(&review.ID, &review.ProjectID, &taskSpec, &review.Title, &review.Description, &review.ProjectRules, &review.TestOutput, &review.Summary, &review.Score, &review.Decision, &review.Confidence, &review.SpecStatus, &review.SDDContextMD, &review.CreatedAt, &review.UpdatedAt)
	if err != nil {
		return PRReview{}, err
	}
	review.TaskSpecID = taskSpec.String
	return review, nil
}

func scanPRReviewDetailRow(row prReviewListScanner) (PRReview, error) {
	var review PRReview
	var taskSpec sql.NullString
	err := row.Scan(&review.ID, &review.ProjectID, &taskSpec, &review.Title, &review.Description, &review.DiffText, &review.ProjectRules, &review.TestOutput, &review.ReviewPrompt, &review.Summary, &review.Score, &review.Decision, &review.Confidence, &review.SpecStatus, &review.SDDContextMD, &review.CreatedAt, &review.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return PRReview{}, notFoundError("pr review not found")
	}
	if err != nil {
		return PRReview{}, err
	}
	review.TaskSpecID = taskSpec.String
	return review, nil
}

func reviewFilesFromParsed(parsed []parsedDiffFile) []PRReviewFile {
	files := make([]PRReviewFile, 0, len(parsed))
	for _, parsedFile := range parsed {
		riskArea, riskLevel := classifyRisk(parsedFile.Path, parsedFile.Chunk)
		files = append(files, PRReviewFile{
			Path:       parsedFile.Path,
			ChangeType: parsedFile.ChangeType,
			RiskArea:   riskArea,
			RiskLevel:  riskLevel,
			Additions:  parsedFile.Additions,
			Deletions:  parsedFile.Deletions,
		})
	}
	return files
}

func buildPRFindings(req CreatePRReviewRequest, spec *TaskSpec, specStatus string, parsed []parsedDiffFile, files []PRReviewFile) []PRReviewFinding {
	var findings []PRReviewFinding
	hasTests := hasTestChanges(files) || testOutputPasses(req.TestOutput)
	docsOnly := len(files) > 0
	for _, file := range files {
		if file.RiskArea != "docs" {
			docsOnly = false
			break
		}
	}
	if specStatus == prSpecMissing {
		findings = append(findings, prFinding("pr.no_spec", prSeverityMedium, prStatusMissingContext, "No SDD spec provided", nil, "", "No task spec was linked to this PR review.", "PR title/description were used as inferred context.", "The review cannot fully verify implementation intent.", "Store or attach a mini-spec for this task before relying on approval."))
	}
	if touchesCriticalArea(files) && !hasTests {
		findings = append(findings, prFinding("pr.critical_zone_without_tests", prSeverityHigh, prStatusLikely, "Critical area changed without tests", criticalFiles(files), "Tests", "The diff touches API, database, auth/security, or payments code without test files or passing test output.", "Changed high-risk files: "+strings.Join(criticalFiles(files), ", "), "Regression risk is higher because critical behavior changed without validation evidence.", "Add focused tests or paste passing test output for the changed critical behavior."))
	}
	if areaFiles := filesByRiskArea(files, "api_contract"); len(areaFiles) > 0 {
		findings = append(findings, prFinding("pr.api_contract_touched", prSeverityHigh, prStatusPossible, "API contract touched", areaFiles, "API contracts", "The diff changes files that look like routes, handlers, DTOs, or API contracts.", "Changed API files: "+strings.Join(areaFiles, ", "), "Clients or frontend code may rely on this contract.", "Verify request/response shape and add or run contract coverage."))
	}
	if areaFiles := filesByRiskArea(files, "database"); len(areaFiles) > 0 {
		findings = append(findings, prFinding("pr.db_migration_touched", prSeverityHigh, prStatusPossible, "Database schema or migration touched", areaFiles, "Database schema", "The diff changes SQL, migrations, or schema-related files.", "Changed database files: "+strings.Join(areaFiles, ", "), "Schema changes can affect data integrity or deployment safety.", "Review migration reversibility, constraints, and compatibility with existing data."))
	}
	if areaFiles := filesByRiskArea(files, "security/auth"); len(areaFiles) > 0 {
		findings = append(findings, prFinding("pr.auth_or_security_touched", prSeverityHigh, prStatusPossible, "Auth or security-sensitive code touched", areaFiles, "Security/auth", "The diff changes auth, permission, role, session, or token-related files.", "Changed security files: "+strings.Join(areaFiles, ", "), "Mistakes here can expose access-control or session risks.", "Review authorization paths and add focused tests for allowed and denied cases."))
	}
	if secretEvidence := secretPatternEvidence(parsed); secretEvidence != "" {
		findings = append(findings, prFinding("pr.secret_pattern", prSeverityCritical, prStatusConfirmed, "Possible secret committed in diff", allFilePaths(files), "Security", "The diff contains text matching a secret-like pattern.", secretEvidence, "Secrets in source control can compromise systems and credentials.", "Remove the secret, rotate it if real, and use environment variables or secret storage."))
	}
	if todoEvidence := todoEvidence(parsed); todoEvidence != "" {
		findings = append(findings, prFinding("pr.todo_or_incomplete", prSeverityMedium, prStatusConfirmed, "Incomplete implementation marker added", allFilePaths(files), "Implementation completeness", "Added lines contain TODO/FIXME/not implemented markers.", todoEvidence, "Incomplete code can ship accidental placeholders or disabled behavior.", "Replace placeholders with implemented behavior or remove them before merge."))
	}
	if !hasTests && !docsOnly {
		findings = append(findings, prFinding("pr.tests_missing", prSeverityMedium, prStatusLikely, "Tests missing for non-doc change", allFilePaths(files), "Tests", "No test files were changed and no passing test output was provided.", "Changed files are not documentation-only.", "The review has less confidence that behavior remains correct.", "Add focused tests or provide passing test output."))
	}
	if spec != nil {
		outOfScopeFiles := outOfScopeFiles(spec, files)
		if len(outOfScopeFiles) > 0 {
			findings = append(findings, prFinding("pr.out_of_scope_possible", prSeverityMedium, prStatusPossible, "Possible out-of-scope change", outOfScopeFiles, "SDD scope", "Some changed areas are not clearly mentioned in the linked SDD spec.", "Potentially out-of-scope files: "+strings.Join(outOfScopeFiles, ", "), "The change may include work beyond the stated task intent.", "Confirm the scope with a human reviewer or update the task spec."))
		}
	}
	sortPRFindings(findings)
	return findings
}

func prFinding(code, severity, status, title string, files []string, specRule, problem, evidence, impact, suggestedFix string) PRReviewFinding {
	return PRReviewFinding{
		Code:             code,
		Severity:         severity,
		Status:           status,
		Title:            title,
		Files:            files,
		SpecRuleAffected: specRule,
		Problem:          problem,
		Evidence:         evidence,
		Impact:           impact,
		SuggestedFix:     suggestedFix,
		CreatedAt:        fmtTime(time.Now().UTC()),
	}
}

func summarizePRReview(files []PRReviewFile, findings []PRReviewFinding, score int, decision string) string {
	return fmt.Sprintf("%d files changed, %d findings, score %d/100, decision %s.", len(files), len(findings), score, decision)
}

func inferredSDDContext(req CreatePRReviewRequest, files []PRReviewFile) string {
	var risks []string
	for _, file := range files {
		if file.RiskLevel == prSeverityHigh || file.RiskLevel == prSeverityMedium {
			risks = append(risks, fmt.Sprintf("- %s: %s/%s", file.Path, file.RiskArea, file.RiskLevel))
		}
	}
	if len(risks) == 0 {
		risks = append(risks, "- MISSING_CONTEXT")
	}
	return strings.Join([]string{
		"# Inferred SDD Context",
		"",
		"## Goal",
		strings.TrimSpace(req.Title + "\n\n" + req.Description),
		"",
		"## Scope",
		"MISSING_CONTEXT",
		"",
		"## Rules",
		"MISSING_CONTEXT",
		"",
		"## Contracts",
		"MISSING_CONTEXT",
		"",
		"## Acceptance Criteria",
		"MISSING_CONTEXT",
		"",
		"## Risks",
		strings.Join(risks, "\n"),
	}, "\n")
}

func stackFromFiles(files []PRReviewFile) []string {
	seen := map[string]bool{}
	for _, file := range files {
		if file.RiskArea != "" {
			seen[file.RiskArea] = true
		}
	}
	out := make([]string, 0, len(seen))
	for key := range seen {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func hasTestChanges(files []PRReviewFile) bool {
	for _, file := range files {
		if file.RiskArea == "tests" {
			return true
		}
	}
	return false
}

func testOutputPasses(output string) bool {
	upper := strings.ToUpper(output)
	return strings.Contains(upper, "PASS") && !strings.Contains(upper, "FAIL")
}

func testOutputFails(output string) bool {
	return strings.Contains(strings.ToUpper(output), "FAIL")
}

func touchesCriticalArea(files []PRReviewFile) bool {
	return len(criticalFiles(files)) > 0
}

func criticalFiles(files []PRReviewFile) []string {
	var out []string
	for _, file := range files {
		if isCriticalRiskArea(file.RiskArea) {
			out = append(out, file.Path)
		}
	}
	return out
}

func filesByRiskArea(files []PRReviewFile, area string) []string {
	var out []string
	for _, file := range files {
		if file.RiskArea == area {
			out = append(out, file.Path)
		}
	}
	return out
}

func allFilePaths(files []PRReviewFile) []string {
	out := make([]string, 0, len(files))
	for _, file := range files {
		out = append(out, file.Path)
	}
	return out
}

func outOfScopeFiles(spec *TaskSpec, files []PRReviewFile) []string {
	context := strings.ToLower(spec.Module + "\n" + spec.Title + "\n" + spec.TaskDescription + "\n" + spec.SpecMD)
	var out []string
	for _, file := range files {
		if file.RiskArea == "docs" || file.RiskArea == "tests" {
			continue
		}
		tokens := scopeTokens(file)
		mentioned := false
		for _, token := range tokens {
			if token != "" && strings.Contains(context, token) {
				mentioned = true
				break
			}
		}
		if !mentioned {
			out = append(out, file.Path)
		}
	}
	return out
}

func scopeTokens(file PRReviewFile) []string {
	path := strings.ToLower(file.Path)
	parts := strings.FieldsFunc(path, func(r rune) bool {
		return r == '/' || r == '_' || r == '-' || r == '.' || r == '\\'
	})
	return append(parts, strings.ToLower(file.RiskArea))
}

func secretPatternEvidence(files []parsedDiffFile) string {
	for _, file := range files {
		for _, line := range file.AddedLines {
			lower := strings.ToLower(line)
			if strings.Contains(lower, "api_key=") || strings.Contains(lower, "secret=") || strings.Contains(lower, "password=") || strings.Contains(lower, "token=") || strings.Contains(line, "PRIVATE KEY") {
				return fmt.Sprintf("%s: %s", file.Path, strings.TrimSpace(line))
			}
		}
	}
	return ""
}

func todoEvidence(files []parsedDiffFile) string {
	for _, file := range files {
		for _, line := range file.AddedLines {
			lower := strings.ToLower(line)
			if strings.Contains(line, "TODO") || strings.Contains(line, "FIXME") || strings.Contains(lower, `panic("todo`) || strings.Contains(lower, `throw new error("todo`) || strings.Contains(lower, "not implemented") {
				return fmt.Sprintf("%s: %s", file.Path, strings.TrimSpace(line))
			}
		}
	}
	return ""
}

func sortPRFindings(findings []PRReviewFinding) {
	order := map[string]int{prSeverityCritical: 0, prSeverityHigh: 1, prSeverityMedium: 2, prSeverityLow: 3}
	sort.SliceStable(findings, func(i, j int) bool {
		if order[findings[i].Severity] == order[findings[j].Severity] {
			return findings[i].Code < findings[j].Code
		}
		return order[findings[i].Severity] < order[findings[j].Severity]
	})
	for i := range findings {
		findings[i].SortOrder = i + 1
	}
}

func emptyPRReviewFiles(items []PRReviewFile) []PRReviewFile {
	if items == nil {
		return []PRReviewFile{}
	}
	return items
}

func emptyPRReviewFindings(items []PRReviewFinding) []PRReviewFinding {
	if items == nil {
		return []PRReviewFinding{}
	}
	return items
}
