package repoaudit

import "time"

const (
	SourceTypePath = "path"

	DocPolicyIgnoreExisting = "ignore_existing_docs"
	DocPolicyAllowExisting  = "allow_existing_docs"

	AuditStatusRunning   = "running"
	AuditStatusCompleted = "completed"
	AuditStatusFailed    = "failed"
)

type Repo struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	SourceType string    `json:"source_type"`
	SourcePath string    `json:"source_path"`
	DocPolicy  string    `json:"doc_policy"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type AuditRun struct {
	ID             string            `json:"id"`
	RepoID         string            `json:"repo_id"`
	Status         string            `json:"status"`
	Score          int               `json:"score"`
	ScoreBreakdown map[string]int    `json:"score_breakdown"`
	Summary        string            `json:"summary"`
	Stack          map[string]string `json:"stack"`
	CreatedAt      time.Time         `json:"created_at"`
	CompletedAt    *time.Time        `json:"completed_at,omitempty"`
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

type Evidence struct {
	ID        string    `json:"id"`
	AuditID   string    `json:"audit_id"`
	Kind      string    `json:"kind"`
	Ref       string    `json:"ref,omitempty"`
	Summary   string    `json:"summary"`
	Command   string    `json:"command,omitempty"`
	FilePath  string    `json:"file_path,omitempty"`
	Line      int       `json:"line,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Finding struct {
	ID           string         `json:"id"`
	AuditID      string         `json:"audit_id"`
	RuleID       string         `json:"rule_id,omitempty"`
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

type ScoreItem struct {
	ID             string     `json:"id"`
	AuditID        string     `json:"audit_id"`
	Category       string     `json:"category"`
	MaxPoints      int        `json:"max_points"`
	AwardedPoints  int        `json:"awarded_points"`
	DeductedPoints int        `json:"deducted_points"`
	Reason         string     `json:"reason"`
	EvidenceRefs   []Evidence `json:"evidence_refs"`
	CreatedAt      time.Time  `json:"created_at"`
}

type GeneratedDoc struct {
	ID           string    `json:"id"`
	AuditID      string    `json:"audit_id"`
	Title        string    `json:"title"`
	Content      string    `json:"content"`
	SourcePolicy string    `json:"source_policy"`
	CreatedAt    time.Time `json:"created_at"`
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

type Inventory struct {
	Files       []string          `json:"files"`
	Manifests   []string          `json:"manifests"`
	CI          []string          `json:"ci"`
	Migrations  []string          `json:"migrations"`
	TestFiles   []string          `json:"test_files"`
	Commands    []string          `json:"commands"`
	Stack       map[string]string `json:"stack"`
	DocsSkipped int               `json:"docs_skipped"`
}

type AuditResult struct {
	Run        AuditRun       `json:"run"`
	Findings   []Finding      `json:"findings"`
	Docs       []GeneratedDoc `json:"docs"`
	Tests      []TestResult   `json:"tests"`
	Evidence   []Evidence     `json:"evidence"`
	ScoreItems []ScoreItem    `json:"score_items"`
}
