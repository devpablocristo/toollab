package usecases

import (
	"encoding/json"
	"fmt"
	"time"

	artifactUC "toollab-core/internal/artifact/usecases"
	evidenceDomain "toollab-core/internal/evidence/usecases/domain"
	"toollab-core/internal/run/usecases/domain"
	runnerDomain "toollab-core/internal/runner/usecases/domain"
	scenarioDomain "toollab-core/internal/scenario/usecases/domain"
	"toollab-core/internal/shared"
	targetDomain "toollab-core/internal/target/usecases/domain"
)

type ExecuteResponse struct {
	RunID                string        `json:"run_id"`
	Status               domain.Status `json:"status"`
	StartedAt            time.Time     `json:"started_at"`
	FinishedAt           time.Time     `json:"finished_at"`
	EvidencePackRevision int           `json:"evidence_pack_revision"`
	PackID               string        `json:"pack_id"`
}

type ExecuteOptions struct {
	SubsetCaseIDs []string `json:"subset_case_ids,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	TimeoutMs     int      `json:"timeout_ms,omitempty"`
	MaxBodyBytes  int64    `json:"max_body_bytes,omitempty"`
}

type Executor struct {
	runRepo     domain.Repository
	targetRepo  targetDomain.Repository
	artifactSvc *artifactUC.Service
	runner      runnerDomain.Runner
	ingestor    evidenceDomain.Ingestor
}

func NewExecutor(
	runRepo domain.Repository,
	targetRepo targetDomain.Repository,
	artifactSvc *artifactUC.Service,
	runner runnerDomain.Runner,
	ingestor evidenceDomain.Ingestor,
) *Executor {
	return &Executor{
		runRepo:     runRepo,
		targetRepo:  targetRepo,
		artifactSvc: artifactSvc,
		runner:      runner,
		ingestor:    ingestor,
	}
}

func (e *Executor) ExecuteRun(runID string, opts ExecuteOptions) (ExecuteResponse, error) {
	run, err := e.runRepo.GetByID(runID)
	if err != nil {
		return ExecuteResponse{}, err
	}
	if run.Status == domain.StatusRunning {
		return ExecuteResponse{}, fmt.Errorf("%w: run is already running", shared.ErrConflict)
	}

	target, err := e.targetRepo.GetByID(run.TargetID)
	if err != nil {
		return ExecuteResponse{}, fmt.Errorf("loading target: %w", err)
	}
	baseURL := target.RuntimeHint.BaseURL
	if baseURL == "" {
		return ExecuteResponse{}, fmt.Errorf("%w: target has no base_url configured", shared.ErrInvalidInput)
	}

	planData, _, err := e.artifactSvc.GetLatest(runID, shared.ArtifactScenarioPlan)
	if err != nil {
		return ExecuteResponse{}, fmt.Errorf("%w: no scenario_plan found for this run", shared.ErrInvalidInput)
	}
	var plan scenarioDomain.ScenarioPlan
	if err := json.Unmarshal(planData, &plan); err != nil {
		return ExecuteResponse{}, fmt.Errorf("parsing scenario_plan: %w", err)
	}

	if err := e.runRepo.UpdateStatus(runID, domain.StatusRunning); err != nil {
		return ExecuteResponse{}, fmt.Errorf("setting status to running: %w", err)
	}

	runnerOpts := runnerDomain.Options{
		TimeoutMs:    opts.TimeoutMs,
		MaxBodyBytes: opts.MaxBodyBytes,
		SubsetIDs:    opts.SubsetCaseIDs,
		Tags:         opts.Tags,
	}
	execResult := e.runner.Run(baseURL, plan, runnerOpts)
	execResult.RunID = runID

	pack, rev, err := e.ingestor.Ingest(runID, execResult)

	finalStatus := domain.StatusCompleted
	now := shared.Now()
	if err != nil {
		finalStatus = domain.StatusFailed
		_ = e.runRepo.UpdateStatusCompleted(runID, finalStatus, now)
		return ExecuteResponse{}, fmt.Errorf("ingesting evidence: %w", err)
	}

	_ = e.runRepo.UpdateStatusCompleted(runID, finalStatus, now)

	return ExecuteResponse{
		RunID:                runID,
		Status:               finalStatus,
		StartedAt:            execResult.StartedAt,
		FinishedAt:           execResult.FinishedAt,
		EvidencePackRevision: rev,
		PackID:               pack.PackID,
	}, nil
}
