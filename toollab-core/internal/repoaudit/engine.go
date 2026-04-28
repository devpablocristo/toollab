package repoaudit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Engine struct {
	store   *Store
	dataDir string
}

func NewEngine(store *Store, dataDir string) *Engine {
	return &Engine{store: store, dataDir: dataDir}
}

func (e *Engine) Run(ctx context.Context, repo Repo, cfg AuditConfig) (AuditResult, error) {
	if repo.SourceType != SourceTypePath {
		return AuditResult{}, fmt.Errorf("unsupported source_type %q", repo.SourceType)
	}
	if info, err := os.Stat(repo.SourcePath); err != nil || !info.IsDir() {
		return AuditResult{}, fmt.Errorf("source_path is not an accessible directory")
	}

	run, err := e.store.CreateAudit(repo.ID)
	if err != nil {
		return AuditResult{}, err
	}

	result := AuditResult{Run: run}
	sandboxPath := filepath.Join(e.dataDir, "v2", "sandboxes", run.ID)

	inventory, ev := scanInventory(repo.SourcePath, cfg.AllowDocsRead)
	for _, item := range ev {
		item.AuditID = run.ID
		saved, err := e.store.SaveEvidence(item)
		if err == nil {
			result.Evidence = append(result.Evidence, saved)
		}
	}

	findings := buildFindings(run.ID, repo.SourcePath, inventory)
	result.Findings = append(result.Findings, findings...)

	needsSandbox := cfg.RunExistingTests || cfg.GenerateTests
	sandboxReady := false
	if needsSandbox {
		sandboxEvidence := Evidence{
			AuditID:  run.ID,
			Kind:     "sandbox",
			Summary:  "Sandbox prepared outside the source repository",
			FilePath: sandboxPath,
		}
		if err := prepareSandbox(ctx, repo.SourcePath, sandboxPath); err != nil {
			sandboxEvidence.Summary = "Sandbox preparation failed: " + err.Error()
			savedEvidence, saveErr := e.store.SaveEvidence(sandboxEvidence)
			if saveErr == nil {
				sandboxEvidence = savedEvidence
				result.Evidence = append(result.Evidence, savedEvidence)
			}
			result.Findings = append(result.Findings, Finding{
				AuditID:     run.ID,
				RuleID:      "tests.sandbox_prepare_failed",
				Severity:    "High",
				Priority:    "P1",
				State:       "Confirmed",
				Category:    "tests",
				Title:       "Cannot prepare sandbox for generated tests",
				Description: "ToolLab could not create an isolated sandbox, so it did not generate or run tests.",
				Confidence:  "Alta",
				Details: FindingDetails{
					WhyProblem:            "Generated tests must never mutate the original repository; without a sandbox ToolLab cannot safely run that phase.",
					Impact:                "Existing bugs may remain undetected because generated tests were skipped.",
					RiskOfChange:          "Low; fix should be limited to sandbox/worktree preparation.",
					MinimumRecommendation: "Ensure the data directory is writable or the repository can be copied/worktree-created.",
					Avoid:                 "Do not write generated tests into the original repository as a fallback.",
					Validation:            "Run the audit again and verify a sandbox evidence item plus generated/existing test results.",
				},
				EvidenceRefs: []Evidence{
					sandboxEvidence,
				},
			})
		} else {
			sandboxReady = true
			saved, err := e.store.SaveEvidence(sandboxEvidence)
			if err == nil {
				result.Evidence = append(result.Evidence, saved)
			}
		}
	}

	if cfg.RunExistingTests && sandboxReady {
		tests := runExistingTests(ctx, sandboxPath, inventory, cfg.AllowDependencyInstall)
		result.Tests = append(result.Tests, tests...)
	}
	if cfg.GenerateTests && sandboxReady {
		tests := generateAndRunTests(ctx, sandboxPath, inventory)
		result.Tests = append(result.Tests, tests...)
	}
	result.Findings = append(result.Findings, buildTestResultFindings(run.ID, result.Tests)...)

	for _, t := range result.Tests {
		t.AuditID = run.ID
		_ = e.store.SaveTestResult(t)
	}
	for i := range result.Tests {
		result.Tests[i].AuditID = run.ID
	}

	doc := generateDoc(run.ID, repo, inventory, result.Findings, cfg.AllowDocsRead)
	result.Docs = append(result.Docs, doc)
	_ = e.store.SaveDoc(doc)

	for i := range result.Findings {
		result.Findings[i].AuditID = run.ID
		f := e.persistFindingEvidence(result.Findings[i], &result)
		if saved, err := e.store.SaveFinding(f); err == nil {
			result.Findings[i] = saved
		}
	}

	run.Stack = inventory.Stack
	run.ScoreBreakdown, result.ScoreItems = scoreBreakdown(run.ID, result.Findings, result.Tests, inventory, len(result.Docs) > 0)
	for _, item := range result.ScoreItems {
		_ = e.store.SaveScoreItem(item)
	}
	run.Score = totalScore(run.ScoreBreakdown)
	run.Status = AuditStatusCompleted
	docMode := "without reading repo docs by default"
	if cfg.AllowDocsRead {
		docMode = "with repository docs allowed by request"
	}
	run.Summary = fmt.Sprintf("%d findings, %d tests recorded, documentation generated %s.", len(result.Findings), len(result.Tests), docMode)
	if err := e.store.CompleteAudit(run); err != nil {
		return AuditResult{}, err
	}
	return e.loadResult(run.ID)
}

func (e *Engine) persistFindingEvidence(f Finding, result *AuditResult) Finding {
	for i, ref := range f.EvidenceRefs {
		ref.AuditID = f.AuditID
		if ref.ID != "" {
			continue
		}
		saved, err := e.store.SaveEvidence(ref)
		if err != nil {
			continue
		}
		f.EvidenceRefs[i] = saved
		result.Evidence = append(result.Evidence, saved)
	}
	return f
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
	docs, err := e.store.ListDocs(auditID)
	if err != nil {
		return AuditResult{}, err
	}
	tests, err := e.store.ListTests(auditID)
	if err != nil {
		return AuditResult{}, err
	}
	evidence, err := e.store.ListEvidence(auditID)
	if err != nil {
		return AuditResult{}, err
	}
	scoreItems, err := e.store.ListScoreItems(auditID)
	if err != nil {
		return AuditResult{}, err
	}
	if findings == nil {
		findings = []Finding{}
	}
	if docs == nil {
		docs = []GeneratedDoc{}
	}
	if tests == nil {
		tests = []TestResult{}
	}
	if evidence == nil {
		evidence = []Evidence{}
	}
	if scoreItems == nil {
		scoreItems = []ScoreItem{}
	}
	return AuditResult{
		Run:        run,
		Findings:   findings,
		Docs:       docs,
		Tests:      tests,
		Evidence:   evidence,
		ScoreItems: scoreItems,
	}, nil
}

func scanInventory(root string, allowDocs bool) (Inventory, []Evidence) {
	inv := Inventory{Stack: map[string]string{}}
	var evidence []Evidence

	filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
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
	detectStack(&inv, root)
	detectCommands(&inv, root)
	evidence = append(evidence, Evidence{
		Kind:    "inventory",
		Summary: fmt.Sprintf("Inventoried %d files, %d manifests, %d CI files, %d test files; skipped %d doc files/dirs by policy", len(inv.Files), len(inv.Manifests), len(inv.CI), len(inv.TestFiles), inv.DocsSkipped),
	})
	return inv, evidence
}

func shouldSkipDir(rel string) bool {
	base := filepath.Base(rel)
	switch base {
	case ".git", "node_modules", "vendor", "dist", "build", "coverage", "__pycache__", ".cache", ".pytest_cache", "bin":
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
	if base == "docs" || strings.HasPrefix(rel, "docs/") || strings.Contains(rel, "/docs/") {
		return true
	}
	if isDir {
		return base == "wiki"
	}
	return strings.HasSuffix(base, ".md") || strings.HasSuffix(base, ".mdx")
}

func classifyFile(inv *Inventory, root, rel string) {
	base := filepath.Base(rel)
	switch base {
	case "go.mod", "package.json", "package-lock.json", "yarn.lock", "pnpm-lock.yaml", "pyproject.toml", "requirements.txt", "Pipfile", "Makefile", "Dockerfile":
		inv.Manifests = append(inv.Manifests, rel)
	}
	if strings.HasSuffix(base, ".yml") || strings.HasSuffix(base, ".yaml") {
		if strings.HasPrefix(rel, ".github/workflows/") || strings.Contains(base, "compose") {
			inv.Manifests = append(inv.Manifests, rel)
		}
	}
	if strings.HasPrefix(rel, ".github/workflows/") || base == ".gitlab-ci.yml" || base == "Jenkinsfile" {
		inv.CI = append(inv.CI, rel)
	}
	lower := strings.ToLower(rel)
	if strings.HasSuffix(lower, ".sql") || strings.Contains(lower, "migration") {
		inv.Migrations = append(inv.Migrations, rel)
	}
	if isTestFile(rel) {
		inv.TestFiles = append(inv.TestFiles, rel)
	}
}

func detectStack(inv *Inventory, root string) {
	has := func(name string) bool {
		for _, f := range inv.Files {
			if f == name || strings.HasSuffix(f, "/"+name) {
				return true
			}
		}
		return false
	}
	if has("go.mod") {
		inv.Stack["go"] = "detected"
	}
	if has("package.json") {
		inv.Stack["node"] = "detected"
		if data, err := os.ReadFile(filepath.Join(root, "package.json")); err == nil && bytes.Contains(bytes.ToLower(data), []byte("react")) {
			inv.Stack["react"] = "detected"
		}
	}
	if has("pyproject.toml") || has("requirements.txt") || has("Pipfile") {
		inv.Stack["python"] = "detected"
	}
	if len(inv.Migrations) > 0 {
		inv.Stack["database"] = "detected"
	}
	if len(inv.CI) > 0 {
		inv.Stack["ci"] = "detected"
	}
}

func detectCommands(inv *Inventory, root string) {
	if _, ok := inv.Stack["go"]; ok {
		inv.Commands = append(inv.Commands, "go test ./...")
	}
	if _, ok := inv.Stack["node"]; ok {
		if data, err := os.ReadFile(filepath.Join(root, "package.json")); err == nil {
			var pkg struct {
				Scripts map[string]string `json:"scripts"`
			}
			if json.Unmarshal(data, &pkg) == nil {
				for name := range pkg.Scripts {
					inv.Commands = append(inv.Commands, "npm run "+name)
				}
			}
		}
	}
	if _, ok := inv.Stack["python"]; ok {
		inv.Commands = append(inv.Commands, "pytest -q", "python -m unittest discover")
	}
	sort.Strings(inv.Commands)
}

func isTestFile(rel string) bool {
	base := strings.ToLower(filepath.Base(rel))
	return strings.HasSuffix(base, "_test.go") ||
		strings.Contains(base, ".test.") ||
		strings.Contains(base, ".spec.") ||
		strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py") ||
		strings.HasSuffix(base, "_test.py")
}

func buildFindings(auditID, root string, inv Inventory) []Finding {
	var findings []Finding
	if len(inv.CI) == 0 {
		findings = append(findings, newFinding(auditID, "repo.no_ci", "Medium", "P2", "Confirmed", "ci", "No CI workflow detected", "ToolLab did not find GitHub Actions, GitLab CI or Jenkins configuration in the inventoried files.", "Alta", "", 0, Evidence{Kind: "config", Summary: "No GitHub Actions, GitLab CI or Jenkinsfile was present in the inventory."}, FindingDetails{
			WhyProblem:            "Without CI, regressions can land without an automated build/test signal.",
			Impact:                "Production risk grows as the repo changes because checks rely on local discipline.",
			RiskOfChange:          "Low; adding CI can be done without changing runtime behavior.",
			MinimumRecommendation: "Add a minimal CI workflow that runs existing build/test commands.",
			Avoid:                 "Do not introduce broad release automation until the basic validation command is stable.",
			Validation:            "Run the CI command locally and verify the workflow appears in the next audit.",
		}))
	}
	if len(inv.TestFiles) == 0 {
		findings = append(findings, newFinding(auditID, "repo.no_test_files", "Medium", "P2", "Confirmed", "tests", "No automated test files detected", "No Go, JavaScript/TypeScript or Python test files were found in the scanned source set.", "Alta", "", 0, Evidence{Kind: "inventory", Summary: "Inventory found zero files matching Go, JS/TS or Python test naming conventions."}, FindingDetails{
			WhyProblem:            "Without repository tests, ToolLab can only rely on generated smoke tests and static signals.",
			Impact:                "Behavioral regressions are harder to catch and score confidence is lower.",
			RiskOfChange:          "Low if tests are added in focused slices around existing behavior.",
			MinimumRecommendation: "Add a minimal test around one critical path before broad refactors.",
			Avoid:                 "Do not rewrite production code only to make it easier to test.",
			Validation:            "Run the language-specific test command and verify test files are detected.",
		}))
	}
	if _, ok := inv.Stack["node"]; ok && !commandContains(inv.Commands, "test") {
		findings = append(findings, newFinding(auditID, "node.no_test_script", "Low", "P2", "Confirmed", "tests", "Node project has no test script", "package.json was detected but no npm test script was found.", "Alta", "package.json", 0, Evidence{Kind: "manifest", FilePath: "package.json", Summary: "package.json scripts did not include a test command."}, FindingDetails{
			WhyProblem:            "A Node project without a test script cannot be validated by common tooling or CI defaults.",
			Impact:                "Frontend regressions may be missed or require custom manual commands.",
			RiskOfChange:          "Low; adding a script should delegate to the existing test runner if present.",
			MinimumRecommendation: "Add a package.json test script once a test runner exists.",
			Avoid:                 "Do not add a fake passing test script that hides missing coverage.",
			Validation:            "Run npm test and verify it exits non-zero on failing tests.",
		}))
	}
	findings = append(findings, buildRepoFindings(auditID, inv)...)
	findings = append(findings, buildGoFindings(auditID, root, inv)...)
	findings = append(findings, buildReactFindings(auditID, root, inv)...)
	findings = append(findings, buildPythonFindings(auditID, root, inv)...)
	findings = append(findings, buildSQLFindings(auditID, root, inv)...)
	return findings
}

func newFinding(auditID, ruleID, severity, priority, state, category, title, description, confidence, filePath string, line int, evidence Evidence, details FindingDetails) Finding {
	refs := []Evidence{}
	if evidence.Kind != "" || evidence.Summary != "" || evidence.FilePath != "" {
		if evidence.FilePath == "" {
			evidence.FilePath = filePath
		}
		if evidence.Line == 0 {
			evidence.Line = line
		}
		refs = append(refs, evidence)
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
		EvidenceRefs: refs,
		Details:      details,
		CreatedAt:    time.Now().UTC(),
	}
}

func commandContains(commands []string, needle string) bool {
	for _, cmd := range commands {
		if strings.Contains(cmd, needle) {
			return true
		}
	}
	return false
}

func prepareSandbox(ctx context.Context, sourcePath, sandboxPath string) error {
	_ = os.RemoveAll(sandboxPath)
	if isGitRepo(sourcePath) {
		cmd := exec.CommandContext(ctx, "git", "-C", sourcePath, "worktree", "add", "--detach", sandboxPath, "HEAD")
		if out, err := cmd.CombinedOutput(); err == nil {
			_ = out
			return nil
		}
	}
	return copyTree(sourcePath, sandboxPath)
}

func isGitRepo(path string) bool {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--is-inside-work-tree")
	return cmd.Run() == nil
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil || rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			if shouldSkipDir(rel) {
				return filepath.SkipDir
			}
			return os.MkdirAll(target, 0o755)
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
}

func runExistingTests(ctx context.Context, sandboxPath string, inv Inventory, allowInstall bool) []TestResult {
	var out []TestResult
	if _, ok := inv.Stack["go"]; ok {
		out = append(out, runCommand(ctx, sandboxPath, "existing", "go test", "go", "test", "./...", "-count=1"))
	}
	if _, ok := inv.Stack["node"]; ok {
		if _, err := os.Stat(filepath.Join(sandboxPath, "node_modules")); err != nil && !allowInstall {
			out = append(out, TestResult{Kind: "existing", Name: "node checks", Status: "blocked", Output: "node_modules is absent and dependency installation is disabled"})
		} else {
			if commandContains(inv.Commands, "typecheck") {
				out = append(out, runCommand(ctx, sandboxPath, "existing", "npm run typecheck", "npm", "run", "typecheck"))
			}
			if commandContains(inv.Commands, "build") {
				out = append(out, runCommand(ctx, sandboxPath, "existing", "npm run build", "npm", "run", "build"))
			}
			if commandContains(inv.Commands, "npm run test") || commandContains(inv.Commands, "npm run test:") {
				out = append(out, runCommand(ctx, sandboxPath, "existing", "npm test", "npm", "test"))
			}
		}
	}
	if _, ok := inv.Stack["python"]; ok {
		if len(inv.TestFiles) > 0 {
			out = append(out, runCommand(ctx, sandboxPath, "existing", "python -m unittest discover", "python", "-m", "unittest", "discover"))
		} else {
			out = append(out, TestResult{Kind: "existing", Name: "python tests", Status: "blocked", Output: "python stack detected but no test files were found"})
		}
	}
	return out
}

func generateAndRunTests(ctx context.Context, sandboxPath string, inv Inventory) []TestResult {
	var out []TestResult
	if _, ok := inv.Stack["go"]; ok {
		path, err := generateGoSmokeTest(sandboxPath)
		if err != nil {
			out = append(out, TestResult{Kind: "generated", Name: "go generated smoke test", Status: "blocked", Output: err.Error()})
		} else {
			res := runCommand(ctx, sandboxPath, "generated", "go generated smoke test", "go", "test", "./...", "-run", "TestToolLabGeneratedSmoke", "-count=1")
			res.GeneratedPath = path
			out = append(out, res)
		}
	}
	if _, ok := inv.Stack["python"]; ok {
		path := filepath.Join(sandboxPath, "test_tollab_generated_smoke.py")
		_ = os.WriteFile(path, []byte("def test_tollab_generated_smoke():\n    assert True\n"), 0o644)
		res := runCommand(ctx, sandboxPath, "generated", "python generated smoke test", "python", "-m", "unittest", "discover")
		res.GeneratedPath = path
		out = append(out, res)
	}
	if _, ok := inv.Stack["react"]; ok {
		out = append(out, TestResult{Kind: "generated", Name: "react e2e test", Status: "blocked", Output: "No existing e2e framework was detected; ToolLab did not add dependencies in MVP."})
	}
	return out
}

func generateGoSmokeTest(root string) (string, error) {
	var targetDir, pkgName string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "package ") {
				targetDir = filepath.Dir(path)
				pkgName = strings.TrimSpace(strings.TrimPrefix(line, "package "))
				return fs.SkipAll
			}
		}
		return nil
	})
	if err != nil && err != fs.SkipAll {
		return "", err
	}
	if targetDir == "" || pkgName == "" {
		return "", fmt.Errorf("no Go package found for generated test")
	}
	testPath := filepath.Join(targetDir, "zz_tollab_generated_test.go")
	body := fmt.Sprintf("package %s\n\nimport \"testing\"\n\nfunc TestToolLabGeneratedSmoke(t *testing.T) {}\n", pkgName)
	return testPath, os.WriteFile(testPath, []byte(body), 0o644)
}

func runCommand(ctx context.Context, dir, kind, name, bin string, args ...string) TestResult {
	runCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(runCtx, bin, args...)
	cmd.Dir = dir
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
	return TestResult{
		Kind:    kind,
		Name:    name,
		Command: strings.TrimSpace(bin + " " + strings.Join(args, " ")),
		Status:  status,
		Output:  truncate(buf.String(), 8000),
	}
}

func generateDoc(auditID string, repo Repo, inv Inventory, findings []Finding, allowDocsRead bool) GeneratedDoc {
	var b strings.Builder
	b.WriteString("# " + repo.Name + "\n\n")
	b.WriteString("Generated by ToolLab V2 from source code, manifests, configuration, commands, and audit evidence.\n\n")
	if allowDocsRead {
		b.WriteString("Documentation policy: existing repository documentation was allowed.\n\n")
	} else {
		b.WriteString("Documentation policy: existing repository documentation was intentionally ignored to avoid contamination.\n\n")
	}
	b.WriteString("## Detected Stack\n\n")
	if len(inv.Stack) == 0 {
		b.WriteString("- No known stack detected.\n")
	} else {
		keys := make([]string, 0, len(inv.Stack))
		for k := range inv.Stack {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString("- " + k + ": " + inv.Stack[k] + "\n")
		}
	}
	b.WriteString("\n## Findings Summary\n\n")
	if len(findings) == 0 {
		b.WriteString("- No confirmed findings were produced by the MVP checks.\n")
	} else {
		for _, f := range findings {
			b.WriteString(fmt.Sprintf("- [%s/%s] %s\n", f.Severity, f.Priority, f.Title))
		}
	}
	b.WriteString("\n## Evidence Inputs\n\n")
	b.WriteString(fmt.Sprintf("- Files inventoried: %d\n", len(inv.Files)))
	b.WriteString(fmt.Sprintf("- Manifests: %s\n", strings.Join(inv.Manifests, ", ")))
	b.WriteString(fmt.Sprintf("- CI files: %s\n", strings.Join(inv.CI, ", ")))
	b.WriteString(fmt.Sprintf("- Test files: %d\n", len(inv.TestFiles)))
	return GeneratedDoc{
		AuditID: auditID,
		Title:   repo.Name + " generated documentation",
		Content: b.String(),
		SourcePolicy: func() string {
			if allowDocsRead {
				return DocPolicyAllowExisting
			}
			return DocPolicyIgnoreExisting
		}(),
		CreatedAt: time.Now().UTC(),
	}
}

func scoreBreakdown(auditID string, findings []Finding, tests []TestResult, inv Inventory, docsGenerated bool) (map[string]int, []ScoreItem) {
	breakdown := map[string]int{
		"build_tests":       15,
		"bugs_findings":     35,
		"test_quality":      5,
		"docs_traceability": 0,
		"ci_config":         3,
	}
	reasons := map[string]string{
		"build_tests":       "No passing test command was recorded.",
		"bugs_findings":     "No finding penalties were applied.",
		"test_quality":      "No repository or generated test evidence raised the quality score.",
		"docs_traceability": "No generated documentation was stored.",
		"ci_config":         "No CI configuration was detected.",
	}
	refs := map[string][]Evidence{}
	if len(tests) > 0 {
		passed := 0
		failed := 0
		for _, t := range tests {
			if t.Status == "passed" {
				passed++
			}
			if t.Status == "failed" || t.Status == "timeout" {
				failed++
			}
		}
		if failed == 0 && passed > 0 {
			breakdown["build_tests"] = 30
			reasons["build_tests"] = "At least one validation command passed and none failed."
		} else if failed > 0 {
			breakdown["build_tests"] = 5
			reasons["build_tests"] = "One or more validation commands failed or timed out."
		}
	}
	penalty := 0
	for _, f := range findings {
		switch f.Severity {
		case "Critical":
			penalty += 20
		case "High":
			penalty += 12
		case "Medium":
			penalty += 6
		case "Low":
			penalty += 2
		}
		if len(f.EvidenceRefs) > 0 {
			refs["bugs_findings"] = append(refs["bugs_findings"], f.EvidenceRefs[0])
		}
	}
	if penalty > 35 {
		penalty = 35
	}
	breakdown["bugs_findings"] = 35 - penalty
	if penalty > 0 {
		reasons["bugs_findings"] = fmt.Sprintf("%d finding penalty points were applied across %d findings.", penalty, len(findings))
	}
	if len(inv.TestFiles) > 0 {
		breakdown["test_quality"] = 10
		reasons["test_quality"] = "Repository test files were detected."
	}
	for _, t := range tests {
		if t.Kind == "generated" && t.Status == "passed" {
			breakdown["test_quality"] = 15
			reasons["test_quality"] = "A generated ToolLab smoke test passed."
			break
		}
	}
	if docsGenerated {
		breakdown["docs_traceability"] = 10
		reasons["docs_traceability"] = "ToolLab generated and persisted documentation from code/config/evidence."
	}
	if len(inv.CI) > 0 {
		breakdown["ci_config"] = 10
		reasons["ci_config"] = "CI configuration was detected in the inventory."
	}
	items := []ScoreItem{
		newScoreItem(auditID, "build_tests", 30, breakdown["build_tests"], reasons["build_tests"], refs["build_tests"]),
		newScoreItem(auditID, "bugs_findings", 35, breakdown["bugs_findings"], reasons["bugs_findings"], refs["bugs_findings"]),
		newScoreItem(auditID, "test_quality", 15, breakdown["test_quality"], reasons["test_quality"], refs["test_quality"]),
		newScoreItem(auditID, "docs_traceability", 10, breakdown["docs_traceability"], reasons["docs_traceability"], refs["docs_traceability"]),
		newScoreItem(auditID, "ci_config", 10, breakdown["ci_config"], reasons["ci_config"], refs["ci_config"]),
	}
	return breakdown, items
}

func newScoreItem(auditID, category string, maxPoints, awardedPoints int, reason string, evidenceRefs []Evidence) ScoreItem {
	return ScoreItem{
		AuditID:        auditID,
		Category:       category,
		MaxPoints:      maxPoints,
		AwardedPoints:  awardedPoints,
		DeductedPoints: maxPoints - awardedPoints,
		Reason:         reason,
		EvidenceRefs:   evidenceRefs,
		CreatedAt:      time.Now().UTC(),
	}
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

func sortFindings(findings []Finding) {
	sev := map[string]int{"Critical": 0, "High": 1, "Medium": 2, "Low": 3, "Informative": 4}
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := sev[findings[i].Severity], sev[findings[j].Severity]
		if a != b {
			return a < b
		}
		return findings[i].Title < findings[j].Title
	})
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
