package analyze

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	analysisDomain "toollab-core/internal/analyze/domain"
	artifactUC "toollab-core/internal/artifact/usecases"
	discoveryDomain "toollab-core/internal/discovery/usecases/domain"
	discoveryUC "toollab-core/internal/discovery/usecases"
	evidenceDomain "toollab-core/internal/evidence/usecases/domain"
	interpretUC "toollab-core/internal/interpretation/usecases"
	runDomain "toollab-core/internal/run/usecases/domain"
	runnerDomain "toollab-core/internal/runner/usecases/domain"
	runnerUC "toollab-core/internal/runner/usecases"
	scenarioDomain "toollab-core/internal/scenario/usecases/domain"
	"toollab-core/internal/shared"
	targetDomain "toollab-core/internal/target/usecases/domain"
)

type Orchestrator struct {
	targetRepo   targetDomain.Repository
	runRepo      runDomain.Repository
	artifactSvc  *artifactUC.Service
	discoverySvc *discoveryUC.Service
	runner       *runnerUC.HTTPRunner
	ingestor     evidenceDomain.Ingestor
	interpSvc    *interpretUC.Service
}

func NewOrchestrator(
	targetRepo targetDomain.Repository,
	runRepo runDomain.Repository,
	artifactSvc *artifactUC.Service,
	discoverySvc *discoveryUC.Service,
	runner *runnerUC.HTTPRunner,
	ingestor evidenceDomain.Ingestor,
	interpSvc *interpretUC.Service,
) *Orchestrator {
	return &Orchestrator{
		targetRepo:   targetRepo,
		runRepo:      runRepo,
		artifactSvc:  artifactSvc,
		discoverySvc: discoverySvc,
		runner:       runner,
		ingestor:     ingestor,
		interpSvc:    interpSvc,
	}
}

type AnalyzeResult struct {
	TargetID string                   `json:"target_id"`
	RunID    string                   `json:"run_id"`
	Analysis analysisDomain.Analysis  `json:"analysis"`
}

func (o *Orchestrator) Analyze(ctx context.Context, targetID string, emit ProgressEmitter) (AnalyzeResult, error) {
	if emit == nil {
		emit = noopEmitter
	}

	emit.phase("init", "Loading target...")
	target, err := o.targetRepo.GetByID(targetID)
	if err != nil {
		return AnalyzeResult{}, fmt.Errorf("loading target: %w", err)
	}

	emit.step("init", "Target loaded: %s", target.Name)

	// 1. Create run
	run := runDomain.Run{
		ID:        shared.NewID(),
		TargetID:  targetID,
		Status:    runDomain.StatusRunning,
		CreatedAt: shared.Now(),
	}
	if err := o.runRepo.Insert(run); err != nil {
		return AnalyzeResult{}, fmt.Errorf("creating run: %w", err)
	}
	emit.step("init", "Run created: %s", run.ID[:8])

	// 2. Discover endpoints
	emit.phase("discovery", "Analyzing source code (AST)...")
	discoverResult, err := o.discoverySvc.Discover(run.ID, targetID, discoveryUC.DiscoverOptions{
		GenerateScenarioPlan: true,
	})
	if err != nil {
		o.failRun(run.ID)
		return AnalyzeResult{}, fmt.Errorf("discovery: %w", err)
	}
	emit.step("discovery", "Found %d endpoints (%.0f%% confidence)",
		discoverResult.EndpointsCount, discoverResult.Confidence*100)

	// 3. Load service model + scenario plan
	modelData, _, err := o.artifactSvc.GetLatest(run.ID, shared.ArtifactServiceModel)
	if err != nil {
		o.failRun(run.ID)
		return AnalyzeResult{}, fmt.Errorf("loading service model: %w", err)
	}
	var model discoveryDomain.ServiceModel
	json.Unmarshal(modelData, &model)

	planData, _, err := o.artifactSvc.GetLatest(run.ID, shared.ArtifactScenarioPlan)
	if err != nil {
		o.failRun(run.ID)
		return AnalyzeResult{}, fmt.Errorf("loading scenario plan: %w", err)
	}
	var plan scenarioDomain.ScenarioPlan
	json.Unmarshal(planData, &plan)

	baseCases := len(plan.Cases)
	emit.step("probes", "Base scenarios: %d cases", baseCases)

	// 4. Generate all probes
	emit.phase("probes", "Generating security probes...")
	plan = generateAllProbes(plan, target)
	probeCount := len(plan.Cases) - baseCases
	emit.step("probes", "Generated %d probes (%d total cases)", probeCount, len(plan.Cases))

	summary := countPlanProbes(plan)
	if summary.InjectionProbes > 0 {
		emit.step("probes", "  Injection probes: %d (SQLi, XSS, path traversal, command injection)", summary.InjectionProbes)
	}
	if summary.MalformedProbes > 0 {
		emit.step("probes", "  Malformed input probes: %d", summary.MalformedProbes)
	}
	if summary.BoundaryProbes > 0 {
		emit.step("probes", "  Boundary value probes: %d", summary.BoundaryProbes)
	}
	if summary.MethodTamperProbes > 0 {
		emit.step("probes", "  Method tamper probes: %d", summary.MethodTamperProbes)
	}
	if summary.HiddenEndpointProbes > 0 {
		emit.step("probes", "  Hidden endpoint probes: %d", summary.HiddenEndpointProbes)
	}
	if summary.LargePayloadProbes > 0 {
		emit.step("probes", "  Large payload probes: %d", summary.LargePayloadProbes)
	}
	if summary.ContentTypeProbes > 0 {
		emit.step("probes", "  Content-type mismatch probes: %d", summary.ContentTypeProbes)
	}
	if summary.NoAuthProbes > 0 {
		emit.step("probes", "  No-auth probes: %d", summary.NoAuthProbes)
	}

	planJSON, _ := json.Marshal(plan)
	o.artifactSvc.Put(run.ID, shared.ArtifactScenarioPlan, planJSON)

	// 5. Execute all requests
	baseURL := target.RuntimeHint.BaseURL
	if baseURL == "" {
		o.failRun(run.ID)
		return AnalyzeResult{}, fmt.Errorf("%w: target has no base_url", shared.ErrInvalidInput)
	}
	if rw := shared.HostRewrite(); rw != nil {
		baseURL = rw(baseURL)
	}

	emit.phase("execute", fmt.Sprintf("Executing %d requests against %s...", len(plan.Cases), baseURL))

	runnerOpts := runnerDomain.Options{
		TimeoutMs:    10000,
		MaxBodyBytes: 1024 * 1024,
		AuthHeaders:  target.RuntimeHint.AuthHeaders,
	}

	total := len(plan.Cases)
	caseProgress := func(idx int, c scenarioDomain.ScenarioCase, cr evidenceDomain.CaseResult) {
		status := "error"
		if cr.Response != nil {
			status = fmt.Sprintf("%d", cr.Response.Status)
		} else if cr.Error != "" {
			status = "ERR"
		}
		emit(ProgressEvent{
			Phase:   "execute",
			Message: fmt.Sprintf("[%s] %s %s → %s (%dms)", tagsLabel(c.Tags), c.Request.Method, c.Request.Path, status, cr.TimingMs),
			Current: idx + 1,
			Total:   total,
		})
	}

	execResult := o.runner.RunWithProgress(baseURL, plan, runnerOpts, caseProgress)
	execResult.RunID = run.ID

	successes := 0
	errors := 0
	for _, cr := range execResult.Cases {
		if cr.Response != nil && cr.Response.Status < 500 {
			successes++
		} else {
			errors++
		}
	}
	emit.step("execute", "Execution complete: %d successes, %d errors", successes, errors)

	// 5b. Ingest evidence
	emit.phase("ingest", "Ingesting evidence...")
	pack, _, err := o.ingestor.Ingest(run.ID, execResult)
	if err != nil {
		o.failRun(run.ID)
		return AnalyzeResult{}, fmt.Errorf("ingesting evidence: %w", err)
	}
	emit.step("ingest", "Ingested %d evidence items", len(pack.Items))

	// 6. Run all evaluations
	emit.phase("evaluate", "Running security audit...")
	security := runFullSecurityAudit(&pack)
	emit.step("evaluate", "Security: %d findings (score %d/100, grade %s)", security.Summary.Total, security.Score, security.Grade)

	emit.phase("evaluate", "Validating API contract...")
	contract := runContractValidation(&pack)
	emit.step("evaluate", "Contract: %.0f%% compliance (%d violations)", contract.ComplianceRate*100, contract.TotalViolations)

	emit.phase("evaluate", "Computing performance metrics...")
	perf := buildPerformanceMetrics(&pack)
	emit.step("evaluate", "Performance: P50=%dms P95=%dms P99=%dms", perf.P50Ms, perf.P95Ms, perf.P99Ms)

	emit.phase("evaluate", "Analyzing coverage...")
	coverage := buildCoverageReport(&model, &pack)
	emit.step("evaluate", "Coverage: %d/%d endpoints (%.0f%%)", coverage.TestedEndpoints, coverage.TotalEndpoints, coverage.CoverageRate*100)

	emit.phase("evaluate", "Analyzing API behavior...")
	behavior := buildBehaviorAnalysis(&pack)
	emit.step("evaluate", "Behavior: input_validation=%s, auth=%s, errors=%s",
		behavior.InvalidInput.Quality, behavior.MissingAuth.Quality, behavior.ErrorConsistency.Quality)

	if len(behavior.InferredModels) > 0 {
		emit.step("evaluate", "Inferred %d data models from responses", len(behavior.InferredModels))
	}

	probesSummary := countProbes(&pack)
	overallScore := calcOverallScore(security, contract, coverage, perf)

	analysis := analysisDomain.Analysis{
		TargetID:   targetID,
		TargetName: target.Name,
		CreatedAt:  shared.Now(),
		RunID:      run.ID,
		Discovery: analysisDomain.DiscoverySummary{
			Framework:      model.Framework,
			EndpointsCount: discoverResult.EndpointsCount,
			Confidence:     discoverResult.Confidence,
			Gaps:           discoverResult.Gaps,
		},
		Performance:   perf,
		Security:      security,
		Contract:      contract,
		Coverage:      coverage,
		Behavior:      behavior,
		ProbesSummary: probesSummary,
		Score:         overallScore,
		Grade:         scoreToGrade(overallScore),
	}

	emit.step("evaluate", "Overall score: %d/100 (grade %s)", overallScore, scoreToGrade(overallScore))

	// 7. Save analysis artifact
	analysisJSON, _ := json.Marshal(analysis)
	o.artifactSvc.Put(run.ID, shared.ArtifactAnalysis, analysisJSON)

	// 8. Mark run completed (before LLM so results are available immediately)
	_ = o.runRepo.UpdateStatusCompleted(run.ID, runDomain.StatusCompleted, shared.Now())
	emit.phase("done", "Analysis complete! AI documentation generating in background...")

	// 9. LLM Documentation + Analysis (async, in parallel — doesn't block the response)
	for _, kind := range []interpretUC.InterpretKind{interpretUC.KindDocumentation, interpretUC.KindAnalysis} {
		go func(k interpretUC.InterpretKind) {
			bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			opts := interpretUC.InterpretOptions{Kind: k}
			if _, err := o.interpSvc.Interpret(bgCtx, run.ID, opts); err != nil {
				log.Printf("LLM %s failed (run %s): %v", k, run.ID, err)
				errJSON, _ := json.Marshal(map[string]string{
					"error":  err.Error(),
					"status": "failed",
				})
				artifactType := shared.ArtifactLLMInterpretation
				if k == interpretUC.KindDocumentation {
					artifactType = shared.ArtifactLLMDocumentation
				}
				o.artifactSvc.Put(run.ID, artifactType, errJSON)
			} else {
				log.Printf("LLM %s completed (run %s)", k, run.ID)
			}
		}(kind)
	}

	return AnalyzeResult{
		TargetID: targetID,
		RunID:    run.ID,
		Analysis: analysis,
	}, nil
}

func (o *Orchestrator) failRun(runID string) {
	_ = o.runRepo.UpdateStatusCompleted(runID, runDomain.StatusFailed, shared.Now())
}

func countProbes(pack *evidenceDomain.EvidencePack) analysisDomain.ProbesSummary {
	s := analysisDomain.ProbesSummary{}
	for _, item := range pack.Items {
		isProbe := false
		for _, t := range item.Tags {
			switch t {
			case "probe":
				isProbe = true
			case "sqli", "xss", "cmdi", "path-traversal":
				s.InjectionProbes++
			case "malformed":
				s.MalformedProbes++
			case "boundary":
				s.BoundaryProbes++
			case "method-tamper":
				s.MethodTamperProbes++
			case "hidden-endpoint":
				s.HiddenEndpointProbes++
			case "large-payload":
				s.LargePayloadProbes++
			case "content-type-mismatch":
				s.ContentTypeProbes++
			case "no-auth":
				s.NoAuthProbes++
			}
		}
		if isProbe {
			s.TotalProbes++
		}
	}
	return s
}

func countPlanProbes(plan scenarioDomain.ScenarioPlan) analysisDomain.ProbesSummary {
	s := analysisDomain.ProbesSummary{}
	for _, c := range plan.Cases {
		isProbe := false
		for _, t := range c.Tags {
			switch t {
			case "probe":
				isProbe = true
			case "sqli", "xss", "cmdi", "path-traversal":
				s.InjectionProbes++
			case "malformed":
				s.MalformedProbes++
			case "boundary":
				s.BoundaryProbes++
			case "method-tamper":
				s.MethodTamperProbes++
			case "hidden-endpoint":
				s.HiddenEndpointProbes++
			case "large-payload":
				s.LargePayloadProbes++
			case "content-type-mismatch":
				s.ContentTypeProbes++
			case "no-auth":
				s.NoAuthProbes++
			}
		}
		if isProbe {
			s.TotalProbes++
		}
	}
	return s
}

func tagsLabel(tags []string) string {
	for _, t := range tags {
		switch t {
		case "sqli":
			return "SQLi"
		case "xss":
			return "XSS"
		case "cmdi":
			return "CMDi"
		case "path-traversal":
			return "PathTrav"
		case "malformed":
			return "Malformed"
		case "boundary":
			return "Boundary"
		case "method-tamper":
			return "MethodTamper"
		case "hidden-endpoint":
			return "HiddenEP"
		case "large-payload":
			return "LargePayload"
		case "content-type-mismatch":
			return "CTMismatch"
		case "no-auth":
			return "NoAuth"
		}
	}
	return "Base"
}
