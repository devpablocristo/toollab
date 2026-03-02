package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	artifactUC "toollab-core/internal/artifact"
	d "toollab-core/internal/pipeline/usecases/domain"
	"toollab-core/internal/exports"
	runDomain "toollab-core/internal/run/usecases/domain"
	"toollab-core/internal/shared"
	targetDomain "toollab-core/internal/target/usecases/domain"
)

// ProgressEvent is emitted during pipeline execution.
type ProgressEvent struct {
	Step    d.PipelineStep `json:"step"`
	Phase   string         `json:"phase"`
	Message string         `json:"message"`
	Current int            `json:"current,omitempty"`
	Total   int            `json:"total,omitempty"`
}

type ProgressEmitter func(ProgressEvent)

func noopEmitter(ProgressEvent) {}

// StepRunner is the interface each pipeline step implements.
type StepRunner interface {
	Name() d.PipelineStep
	Run(ctx context.Context, state *PipelineState) d.StepResult
}

// PipelineState is the shared mutable state flowing through the pipeline.
type PipelineState struct {
	RunID         string
	Target        targetDomain.Target
	Config        d.RunConfig
	Budget        *d.BudgetTracker
	Evidence      *d.EvidenceStore
	ErrSigBuilder *d.ErrorSignatureBuilder

	TargetProfile       *d.TargetProfile
	Catalog             *d.EndpointCatalog
	RouterGraph         *d.RouterGraph
	ASTEntities         []d.ASTEntity
	ASTCodePatterns     []d.ASTCodePattern
	Contracts           []d.InferredContract
	SchemaRegistry      *d.SchemaRegistry
	SemanticAnnotations []d.SemanticAnnotation
	SmokeResults        []d.SmokeResult
	AuthMatrix          *d.AuthMatrix
	FuzzResults         []d.FuzzResult
	LogicResults        []d.LogicResult
	AbuseResults        []d.AbuseResult
	ResilienceMetrics   *d.ResilienceMetrics
	OpenAPIValidation   *d.OpenAPIValidationSummary
	FindingsRaw         []d.FindingRaw
	Confirmations       []d.Confirmation

	StepResults []d.StepResult
	Emit        ProgressEmitter
}

func (s *PipelineState) AddFindings(findings ...d.FindingRaw) {
	s.FindingsRaw = append(s.FindingsRaw, findings...)
}

// Orchestrator runs the analysis pipeline.
type Orchestrator struct {
	targetRepo  targetDomain.Repository
	runRepo     runDomain.Repository
	artifactSvc *artifactUC.Service
	steps       []StepRunner
	llmRunner   LLMRunner
	activeMu    sync.Mutex
	activeRuns  map[string]activeRun // key: targetID
}

type activeRun struct {
	runID  string
	cancel context.CancelFunc
}

// LLMRunner generates LLM reports asynchronously.
type LLMRunner interface {
	RunAsync(ctx context.Context, runID string, docsMiniJSON, auditLLMJSON []byte, lang string)
}

func NewOrchestrator(
	targetRepo targetDomain.Repository,
	runRepo runDomain.Repository,
	artifactSvc *artifactUC.Service,
	steps []StepRunner,
	llmRunner LLMRunner,
) *Orchestrator {
	return &Orchestrator{
		targetRepo:  targetRepo,
		runRepo:     runRepo,
		artifactSvc: artifactSvc,
		steps:       steps,
		llmRunner:   llmRunner,
		activeRuns:  map[string]activeRun{},
	}
}

// AnalyzeResult is the output of an analysis run.
type AnalyzeResult struct {
	TargetID    string         `json:"target_id"`
	RunID       string         `json:"run_id"`
	RunSummary  d.RunSummary   `json:"run_summary"`
}

// Analyze runs the full analysis pipeline. lang is "en" or "es" for LLM output language.
func (o *Orchestrator) Analyze(ctx context.Context, targetID string, lang string, emit ProgressEmitter) (AnalyzeResult, error) {
	if emit == nil {
		emit = noopEmitter
	}

	emit(ProgressEvent{Step: d.StepPreflight, Phase: "init", Message: "Loading target..."})
	target, err := o.targetRepo.GetByID(targetID)
	if err != nil {
		return AnalyzeResult{}, fmt.Errorf("loading target: %w", err)
	}

	seed := fmt.Sprintf("%d", time.Now().UnixNano())
	config := d.DefaultRunConfig(seed)

	run := runDomain.Run{
		ID:        shared.NewID(),
		TargetID:  targetID,
		Status:    runDomain.StatusRunning,
		Seed:      seed,
		CreatedAt: shared.Now(),
	}
	if err := o.runRepo.Insert(run); err != nil {
		return AnalyzeResult{}, fmt.Errorf("creating run: %w", err)
	}

	runCtx, runCancel := context.WithCancel(ctx)
	defer runCancel()
	prevRunID := o.registerActiveRun(targetID, run.ID, runCancel)
	if prevRunID != "" {
		emit(ProgressEvent{
			Step:    d.StepPreflight,
			Phase:   "init",
			Message: fmt.Sprintf("Previous run %s cancelled by re-analyze request", shortRunID(prevRunID)),
		})
		o.markRunKilled(prevRunID)
	}
	defer o.unregisterActiveRun(targetID, run.ID)

	emit(ProgressEvent{Step: d.StepPreflight, Phase: "init", Message: fmt.Sprintf("Run %s created", run.ID[:8])})

	state := &PipelineState{
		RunID:         run.ID,
		Target:        target,
		Config:        config,
		Budget:        d.NewBudgetTracker(config.Budget),
		Evidence:      d.NewEvidenceStore(seed),
		ErrSigBuilder: d.NewErrorSignatureBuilder(),
		Emit:          emit,
	}

	startTime := time.Now()
	finalStatus := d.RunCompleted

	for _, step := range o.steps {
		if runCtx.Err() != nil {
			finalStatus = d.RunFailed
			break
		}
		if state.Budget.Exhausted() {
			emit(ProgressEvent{
				Step:    step.Name(),
				Phase:   "budget",
				Message: fmt.Sprintf("Budget exhausted, skipping %s and remaining steps", step.Name()),
			})
			finalStatus = d.RunPartial
			break
		}

		emit(ProgressEvent{Step: step.Name(), Phase: "start", Message: fmt.Sprintf("Starting %s...", step.Name())})
		result := step.Run(runCtx, state)
		state.StepResults = append(state.StepResults, result)

		if result.Status == "failed" {
			emit(ProgressEvent{
				Step:    step.Name(),
				Phase:   "error",
				Message: fmt.Sprintf("Step %s failed: %s", step.Name(), result.Error),
			})
			if step.Name() == d.StepPreflight || step.Name() == d.StepDiscovery {
				finalStatus = d.RunFailed
				break
			}
		} else {
			emit(ProgressEvent{
				Step:    step.Name(),
				Phase:   "done",
				Message: fmt.Sprintf("Step %s completed (%dms, %d requests)", step.Name(), result.DurationMs, result.BudgetUsed),
			})
		}
	}

	duration := int(time.Since(startTime).Seconds())

	runModeClass := d.ClassifyRunMode(state.Evidence.Samples(), state.SmokeResults, state.Confirmations)
	state.OpenAPIValidation = o.computeOpenAPIValidation(state)
	o.addOpenAPIContractFinding(state)
	runModeClass = o.adjustRunModeForAuth(state, runModeClass)
	emit(ProgressEvent{Step: d.StepReport, Phase: "progress",
		Message: fmt.Sprintf("Run mode: %s (%s)", runModeClass.Mode, runModeClass.Reason)})

	summary := o.buildRunSummary(state, finalStatus, duration, runModeClass)

	dossierFull := o.buildDossierFull(state, finalStatus, summary, runModeClass)

	o.saveArtifacts(state, &dossierFull, &summary)

	emit(ProgressEvent{Step: d.StepReport, Phase: "progress", Message: "Generating endpoint intelligence & exports..."})
	exportsIndex := exports.GenerateExportsPost(o.artifactSvc, run.ID, &dossierFull)
	dossierFull.ExportsIndex = exportsIndex

	runStatus := runDomain.StatusCompleted
	if finalStatus == d.RunFailed {
		runStatus = runDomain.StatusFailed
	}
	_ = o.runRepo.UpdateStatusCompleted(run.ID, runStatus, shared.Now())

	emit(ProgressEvent{Step: d.StepReport, Phase: "done", Message: "Pipeline complete. LLM reports generating in background..."})

	if o.llmRunner != nil && finalStatus != d.RunFailed {
		docsMini := d.CompactForDocsMini(&dossierFull, d.TargetMeta{
			Name:        target.Name,
			Description: target.Description,
			SourcePath:  target.Source.Value,
		})
		docsMiniJSON, _ := json.Marshal(docsMini)
		o.artifactSvc.Put(run.ID, shared.ArtifactDossierDocsMini, docsMiniJSON)

		auditLLM := d.CompactForLLM(&dossierFull, d.DefaultCompactConfig())
		auditLLMJSON, _ := json.Marshal(auditLLM)
		o.artifactSvc.Put(run.ID, shared.ArtifactDossierLLM, auditLLMJSON)

		go o.llmRunner.RunAsync(context.Background(), run.ID, docsMiniJSON, auditLLMJSON, lang)
	}

	return AnalyzeResult{
		TargetID:   targetID,
		RunID:      run.ID,
		RunSummary: summary,
	}, nil
}

func (o *Orchestrator) registerActiveRun(targetID, runID string, cancel context.CancelFunc) string {
	o.activeMu.Lock()
	defer o.activeMu.Unlock()
	prev, ok := o.activeRuns[targetID]
	if ok && prev.cancel != nil {
		prev.cancel()
	}
	o.activeRuns[targetID] = activeRun{runID: runID, cancel: cancel}
	if ok {
		return prev.runID
	}
	return ""
}

func (o *Orchestrator) unregisterActiveRun(targetID, runID string) {
	o.activeMu.Lock()
	defer o.activeMu.Unlock()
	current, ok := o.activeRuns[targetID]
	if !ok || current.runID != runID {
		return
	}
	delete(o.activeRuns, targetID)
}

func (o *Orchestrator) markRunKilled(runID string) {
	if runID == "" {
		return
	}
	if err := o.runRepo.UpdateStatusCompleted(runID, runDomain.StatusFailed, shared.Now()); err != nil {
		log.Printf("mark previous run failed (%s): %v", runID, err)
	}
}

func shortRunID(runID string) string {
	if len(runID) >= 8 {
		return runID[:8]
	}
	return runID
}

func (o *Orchestrator) buildRunSummary(state *PipelineState, status d.RunStatus, durationSec int, rmc d.RunModeClassification) d.RunSummary {
	summary := d.RunSummary{
		RunID:           state.RunID,
		Status:          status,
		RunMode:         rmc.Mode,
		RunModeDetail:   &rmc,
		DurationSeconds: durationSec,
		EvidenceCountFull: state.Evidence.Count(),
		BudgetUsage:     state.Budget.Usage(),
	}

	authReady, authReason := o.evaluateAuthReadiness(state)
	summary.AuthReady = authReady
	summary.AuthReadinessReason = authReason

	if state.Catalog != nil {
		summary.EndpointsDiscoveredAST = state.Catalog.TotalCount
		tested := make(map[string]bool)
		for _, s := range state.Evidence.Samples() {
			if s.EndpointID != "" {
				tested[s.EndpointID] = true
			}
		}
		summary.EndpointsConfirmedRT = len(tested)
		if state.Catalog.TotalCount > 0 {
			summary.CoveragePct = float64(len(tested)) / float64(state.Catalog.TotalCount) * 100
		}
	}
	endpointsUseful, usefulCoverage := o.computeUsefulCoverage(state)
	summary.EndpointsUseful = endpointsUseful
	summary.CoverageUsefulPct = usefulCoverage
	summary.OpenAPIValidation = state.OpenAPIValidation

	scores := d.ComputeScores(d.ScoringInput{
		RunMode:       rmc.Mode,
		Catalog:       state.Catalog,
		AuthMatrix:    state.AuthMatrix,
		Contracts:     state.Contracts,
		Evidence:      state.Evidence,
		Findings:      state.FindingsRaw,
		Confirmations: state.Confirmations,
		Signatures:    state.ErrSigBuilder.Build(),
	})
	summary.ScoresAvailable = scores.Auditable
	if scores.Auditable {
		summary.Scores = make(map[d.ScoreDimension]float64)
		for dim, ds := range scores.Scores {
			summary.Scores[dim] = ds.Score
		}
	}

	max := 5
	if len(state.FindingsRaw) < max {
		max = len(state.FindingsRaw)
	}
	for _, f := range state.FindingsRaw[:max] {
		summary.TopFindings = append(summary.TopFindings, d.FindingSummary{
			ID:           f.FindingID,
			Severity:     f.Severity,
			Title:        f.Title,
			EvidenceRefs: f.EvidenceRefs,
		})
	}

	if prev, ok := o.loadPreviousRunSummary(state.Target.ID, state.RunID); ok {
		summary.BaselineDelta = computeBaselineDelta(prev, summary)
	}

	return summary
}

func (o *Orchestrator) buildDossierFull(state *PipelineState, status d.RunStatus, summary d.RunSummary, rmc d.RunModeClassification) d.DossierV2Full {
	dossier := d.DossierV2Full{
		SchemaVersion: "v2",
		RunID:         state.RunID,
		RunConfig:     state.Config,
		CreatedAt:     time.Now().UTC(),
		Status:        status,
		RunMode:       rmc.Mode,
		RunModeDetail: &rmc,
		Confirmations: state.Confirmations,
		FindingsRaw:   state.FindingsRaw,
		RunSummary:    summary,
		StepResults:   state.StepResults,
	}

	if state.TargetProfile != nil {
		dossier.TargetProfile = *state.TargetProfile
	}

	if state.Catalog != nil {
		dossier.AST.EndpointCatalog = *state.Catalog
	}
	if state.RouterGraph != nil {
		dossier.AST.RouterGraph = *state.RouterGraph
	}
	dossier.AST.ASTEntities = state.ASTEntities
	dossier.AST.ASTCodePatterns = state.ASTCodePatterns

	dossier.Runtime = d.RuntimeSection{
		EvidenceSamples:     state.Evidence.Samples(),
		ErrorSignatures:     state.ErrSigBuilder.Build(),
		SmokeResults:        state.SmokeResults,
		FuzzResults:         state.FuzzResults,
		LogicResults:        state.LogicResults,
		AbuseResults:        state.AbuseResults,
		InferredContracts:   state.Contracts,
		SemanticAnnotations: state.SemanticAnnotations,
	}

	if state.AuthMatrix != nil {
		dossier.Runtime.AuthMatrix = state.AuthMatrix
		dossier.Runtime.Discrepancies.ASTvsRuntime = state.AuthMatrix.Discrepancies
	}

	dossier.Runtime.DerivedMetrics = o.buildDerivedMetrics(state)
	if state.ResilienceMetrics != nil {
		dossier.Runtime.DerivedMetrics.ResilienceMetrics = state.ResilienceMetrics
	}

	scoring := d.ComputeScores(d.ScoringInput{
		RunMode:       rmc.Mode,
		Catalog:       state.Catalog,
		AuthMatrix:    state.AuthMatrix,
		Contracts:     state.Contracts,
		Evidence:      state.Evidence,
		Findings:      state.FindingsRaw,
		Confirmations: state.Confirmations,
		Signatures:    state.ErrSigBuilder.Build(),
	})
	dossier.Scoring = scoring

	return dossier
}

func (o *Orchestrator) buildDerivedMetrics(state *PipelineState) d.DerivedMetrics {
	samples := state.Evidence.Samples()
	total := len(samples)
	if total == 0 {
		return d.DerivedMetrics{}
	}

	var successes int
	var latencies []int64
	statusHist := make(map[int]int)
	tested := make(map[string]bool)

	for _, s := range samples {
		if s.Response != nil {
			statusHist[s.Response.Status]++
			if s.Response.Status < 400 {
				successes++
			}
		}
		latencies = append(latencies, s.Timing.LatencyMs)
		if s.EndpointID != "" {
			tested[s.EndpointID] = true
		}
	}

	sortInt64s(latencies)

	endpointsTotal := 0
	if state.Catalog != nil {
		endpointsTotal = state.Catalog.TotalCount
	}
	covPct := 0.0
	if endpointsTotal > 0 {
		covPct = float64(len(tested)) / float64(endpointsTotal) * 100
	}
	endpointsUseful, usefulCoverage := o.computeUsefulCoverage(state)

	return d.DerivedMetrics{
		TotalRequests:   total,
		SuccessRate:     float64(successes) / float64(total),
		ErrorRate:       float64(total-successes) / float64(total),
		P50Ms:           percentile(latencies, 50),
		P95Ms:           percentile(latencies, 95),
		P99Ms:           percentile(latencies, 99),
		CoveragePct:     covPct,
		UsefulCoveragePct: usefulCoverage,
		EndpointsTested: len(tested),
		EndpointsUseful: endpointsUseful,
		EndpointsTotal:  endpointsTotal,
		StatusHistogram: statusHist,
		OpenAPIValidation: state.OpenAPIValidation,
	}
}

func (o *Orchestrator) saveArtifacts(state *PipelineState, dossier *d.DossierV2Full, summary *d.RunSummary) {
	save := func(artType shared.ArtifactType, v any) {
		data, err := json.Marshal(v)
		if err != nil {
			log.Printf("marshal %s: %v", artType, err)
			return
		}
		if _, err := o.artifactSvc.Put(state.RunID, artType, data); err != nil {
			log.Printf("save %s: %v", artType, err)
		}
	}

	if state.TargetProfile != nil {
		save(shared.ArtifactTargetProfile, state.TargetProfile)
	}
	if state.Catalog != nil {
		save(shared.ArtifactEndpointCatalog, state.Catalog)
	}
	if state.RouterGraph != nil {
		save(shared.ArtifactRouterGraph, state.RouterGraph)
	}
	if len(state.ASTEntities) > 0 {
		save(shared.ArtifactASTEntities, state.ASTEntities)
	}
	if len(state.ASTCodePatterns) > 0 {
		save(shared.ArtifactASTCodePatterns, state.ASTCodePatterns)
	}
	if len(state.Contracts) > 0 {
		save(shared.ArtifactInferredContracts, state.Contracts)
	}
	if state.SchemaRegistry != nil {
		save(shared.ArtifactSchemaRegistry, state.SchemaRegistry)
	}
	if len(state.SemanticAnnotations) > 0 {
		save(shared.ArtifactSemanticAnnot, state.SemanticAnnotations)
	}
	if len(state.SmokeResults) > 0 {
		save(shared.ArtifactSmokeResults, state.SmokeResults)
	}
	if state.AuthMatrix != nil {
		save(shared.ArtifactAuthMatrix, state.AuthMatrix)
	}
	if len(state.FuzzResults) > 0 {
		save(shared.ArtifactFuzzResults, state.FuzzResults)
	}
	if len(state.LogicResults) > 0 {
		save(shared.ArtifactLogicResults, state.LogicResults)
	}
	if len(state.AbuseResults) > 0 {
		save(shared.ArtifactAbuseResults, state.AbuseResults)
	}
	if len(state.Confirmations) > 0 {
		save(shared.ArtifactConfirmations, state.Confirmations)
	}
	if len(state.FindingsRaw) > 0 {
		save(shared.ArtifactFindingsRaw, state.FindingsRaw)
	}

	sigs := state.ErrSigBuilder.Build()
	if len(sigs) > 0 {
		save(shared.ArtifactErrorSignatures, sigs)
	}

	evidence := d.RawEvidence{
		SchemaVersion:   "v2",
		RunID:           state.RunID,
		CreatedAt:       time.Now().UTC(),
		Samples:         state.Evidence.Samples(),
		ErrorSignatures: sigs,
		TotalCount:      state.Evidence.Count(),
	}
	save(shared.ArtifactRawEvidence, evidence)

	save(shared.ArtifactScoring, dossier.Scoring)
	save(shared.ArtifactRunSummary, summary)
	save(shared.ArtifactDossierFull, dossier)
}

func sortInt64s(s []int64) {
	for i := 1; i < len(s); i++ {
		key := s[i]
		j := i - 1
		for j >= 0 && s[j] > key {
			s[j+1] = s[j]
			j--
		}
		s[j+1] = key
	}
}

func percentile(sorted []int64, p int) int64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := (p * len(sorted)) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func (o *Orchestrator) adjustRunModeForAuth(state *PipelineState, c d.RunModeClassification) d.RunModeClassification {
	if c.Mode == d.RunModeOffline {
		return c
	}
	authReady, reason := o.evaluateAuthReadiness(state)
	if authReady {
		return c
	}
	if c.Mode == d.RunModeOnlineStrong || c.Mode == d.RunModeOnlineGood {
		c.Mode = d.RunModeOnlinePartial
	}
	if reason != "" {
		c.Reason = c.Reason + " Auth readiness: " + reason
	}
	return c
}

func (o *Orchestrator) evaluateAuthReadiness(state *PipelineState) (bool, string) {
	if state.AuthMatrix == nil || len(state.AuthMatrix.Entries) == 0 {
		return true, "auth matrix unavailable"
	}
	protected := 0
	for _, e := range state.AuthMatrix.Entries {
		if e.NoAuth == d.AuthDenied {
			protected++
		}
	}
	if protected == 0 {
		return true, "no protected endpoints detected"
	}
	if len(state.Target.RuntimeHint.AuthHeaders) == 0 {
		return false, "protected endpoints detected but target has no auth_headers configured"
	}

	protected2xxWithAuth := 0
	protectedSet := make(map[string]bool)
	for _, e := range state.AuthMatrix.Entries {
		if e.NoAuth == d.AuthDenied {
			protectedSet[e.EndpointID] = true
		}
	}
	seen := make(map[string]bool)
	for _, s := range state.Evidence.Samples() {
		if s.Response == nil || s.Response.Status < 200 || s.Response.Status >= 300 {
			continue
		}
		if !protectedSet[s.EndpointID] {
			continue
		}
		if hasAuthHeaders(s.Request.Headers) {
			seen[s.EndpointID] = true
		}
	}
	protected2xxWithAuth = len(seen)
	if protected2xxWithAuth == 0 {
		return false, "auth_headers configured but no protected endpoint returned 2xx with auth"
	}
	return true, fmt.Sprintf("%d/%d protected endpoints validated with authenticated 2xx evidence", protected2xxWithAuth, protected)
}

func (o *Orchestrator) computeUsefulCoverage(state *PipelineState) (int, float64) {
	if state.Catalog == nil || state.Catalog.TotalCount == 0 {
		return 0, 0
	}
	protected := make(map[string]bool)
	if state.AuthMatrix != nil {
		for _, e := range state.AuthMatrix.Entries {
			if e.NoAuth == d.AuthDenied {
				protected[e.EndpointID] = true
			}
		}
	}
	useful := make(map[string]bool)
	for _, s := range state.Evidence.Samples() {
		if s.Response == nil || s.Response.Status < 200 || s.Response.Status >= 300 {
			continue
		}
		if len(protected) == 0 {
			useful[s.EndpointID] = true
			continue
		}
		if protected[s.EndpointID] {
			if hasAuthHeaders(s.Request.Headers) {
				useful[s.EndpointID] = true
			}
			continue
		}
		useful[s.EndpointID] = true
	}
	count := len(useful)
	return count, float64(count) / float64(state.Catalog.TotalCount) * 100
}

func hasAuthHeaders(headers map[string]string) bool {
	for k := range headers {
		kl := strings.ToLower(k)
		if kl == "authorization" || kl == "x-api-key" || kl == "x-nexus-core-key" || kl == "cookie" {
			return true
		}
	}
	return false
}

func (o *Orchestrator) computeOpenAPIValidation(state *PipelineState) *d.OpenAPIValidationSummary {
	if state.Catalog == nil || len(state.Catalog.Endpoints) == 0 {
		return nil
	}
	specMethods, specRef := extractSpecMethods(state.Evidence.Samples())
	if len(specMethods) == 0 {
		return &d.OpenAPIValidationSummary{
			SpecDetected: false,
			ASTEndpoints: len(state.Catalog.Endpoints),
		}
	}
	astMethods := make(map[string]bool, len(state.Catalog.Endpoints))
	for _, ep := range state.Catalog.Endpoints {
		key := strings.ToUpper(ep.Method) + " " + normalizePathForCompare(ep.Path)
		astMethods[key] = true
	}
	astMissingInSpec := 0
	for k := range astMethods {
		if !specMethods[k] {
			astMissingInSpec++
		}
	}
	specMissingInAST := 0
	matches := 0
	for k := range specMethods {
		if astMethods[k] {
			matches++
		} else {
			specMissingInAST++
		}
	}
	matchPct := 0.0
	if len(astMethods) > 0 {
		matchPct = float64(matches) / float64(len(astMethods)) * 100
	}
	return &d.OpenAPIValidationSummary{
		SpecDetected:     true,
		SpecEvidenceRef:  specRef,
		SpecEndpoints:    len(specMethods),
		ASTEndpoints:     len(astMethods),
		ASTMissingInSpec: astMissingInSpec,
		SpecMissingInAST: specMissingInAST,
		MatchPct:         matchPct,
	}
}

func extractSpecMethods(samples []d.EvidenceSample) (map[string]bool, string) {
	for _, s := range samples {
		if s.Response == nil || s.Response.Status != 200 {
			continue
		}
		if s.Request.Path != "/openapi.yaml" {
			continue
		}
		if !strings.Contains(strings.ToLower(s.Response.BodySnippet), "openapi:") {
			continue
		}
		return parseOpenAPIMethods(s.Response.BodySnippet), s.EvidenceID
	}
	return map[string]bool{}, ""
}

func parseOpenAPIMethods(yamlText string) map[string]bool {
	out := map[string]bool{}
	lines := strings.Split(yamlText, "\n")
	inPaths := false
	currentPath := ""
	pathRe := regexp.MustCompile(`^\s{2}(/[^:]+):\s*$`)
	methodRe := regexp.MustCompile(`^\s{4}(get|post|put|patch|delete|head|options|trace):\s*$`)
	for _, line := range lines {
		if strings.HasPrefix(line, "paths:") {
			inPaths = true
			continue
		}
		if inPaths && len(strings.TrimSpace(line)) > 0 && !strings.HasPrefix(line, " ") {
			break
		}
		if !inPaths {
			continue
		}
		if m := pathRe.FindStringSubmatch(line); len(m) == 2 {
			currentPath = m[1]
			continue
		}
		if currentPath == "" {
			continue
		}
		if m := methodRe.FindStringSubmatch(line); len(m) == 2 {
			key := strings.ToUpper(m[1]) + " " + normalizePathForCompare(currentPath)
			out[key] = true
		}
	}
	return out
}

func normalizePathForCompare(p string) string {
	if p == "" {
		return "/"
	}
	paramColon := regexp.MustCompile(`:[^/]+`)
	paramBraces := regexp.MustCompile(`\{[^}]+\}`)
	p = paramColon.ReplaceAllString(p, "{param}")
	p = paramBraces.ReplaceAllString(p, "{param}")
	p = strings.ReplaceAll(p, "//", "/")
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

func (o *Orchestrator) addOpenAPIContractFinding(state *PipelineState) {
	v := state.OpenAPIValidation
	if v == nil || !v.SpecDetected {
		return
	}
	if v.ASTMissingInSpec == 0 && v.SpecMissingInAST == 0 {
		return
	}
	severity := d.SeverityLow
	if v.MatchPct < 80 || v.ASTMissingInSpec > 5 {
		severity = d.SeverityMedium
	}
	var refs []string
	if v.SpecEvidenceRef != "" {
		refs = []string{v.SpecEvidenceRef}
	}
	state.AddFindings(d.FindingRaw{
		FindingID:      d.FindingID(string(d.TaxConSchemaViolation), "_openapi", refs),
		TaxonomyID:     d.TaxConSchemaViolation,
		Severity:       severity,
		Category:       d.FindCatContract,
		Title:          "OpenAPI coverage mismatch against discovered endpoints",
		Description:    fmt.Sprintf("Spec endpoints=%d, AST endpoints=%d, AST missing in spec=%d, spec missing in AST=%d, match=%.1f%%.", v.SpecEndpoints, v.ASTEndpoints, v.ASTMissingInSpec, v.SpecMissingInAST, v.MatchPct),
		EvidenceRefs:   refs,
		Confidence:     0.8,
		Classification: d.ClassCandidate,
	})
}

func (o *Orchestrator) loadPreviousRunSummary(targetID, currentRunID string) (d.RunSummary, bool) {
	runs, err := o.runRepo.ListByTarget(targetID)
	if err != nil {
		return d.RunSummary{}, false
	}
	var prev *runDomain.Run
	for i := range runs {
		r := runs[i]
		if r.ID == currentRunID || r.Status != runDomain.StatusCompleted {
			continue
		}
		if prev == nil || r.CreatedAt.After(prev.CreatedAt) {
			tmp := r
			prev = &tmp
		}
	}
	if prev == nil {
		return d.RunSummary{}, false
	}
	data, _, err := o.artifactSvc.GetLatest(prev.ID, shared.ArtifactRunSummary)
	if err != nil {
		return d.RunSummary{}, false
	}
	var out d.RunSummary
	if err := json.Unmarshal(data, &out); err != nil {
		return d.RunSummary{}, false
	}
	return out, true
}

func computeBaselineDelta(prev, curr d.RunSummary) *d.BaselineDelta {
	delta := &d.BaselineDelta{
		PreviousRunID:          prev.RunID,
		CoveragePctDelta:       curr.CoveragePct - prev.CoveragePct,
		UsefulCoveragePctDelta: curr.CoverageUsefulPct - prev.CoverageUsefulPct,
		ScoreDeltas:            map[d.ScoreDimension]float64{},
	}
	for dim, currVal := range curr.Scores {
		if prevVal, ok := prev.Scores[dim]; ok {
			delta.ScoreDeltas[dim] = currVal - prevVal
		}
	}
	if delta.CoveragePctDelta < -5 {
		delta.Regressions = append(delta.Regressions, fmt.Sprintf("coverage_pct dropped %.1f points", -delta.CoveragePctDelta))
	}
	if delta.UsefulCoveragePctDelta < -5 {
		delta.Regressions = append(delta.Regressions, fmt.Sprintf("useful_coverage_pct dropped %.1f points", -delta.UsefulCoveragePctDelta))
	}
	for dim, dlt := range delta.ScoreDeltas {
		if dlt <= -0.5 {
			delta.Regressions = append(delta.Regressions, fmt.Sprintf("%s score dropped %.2f", dim, -dlt))
		}
	}
	return delta
}
