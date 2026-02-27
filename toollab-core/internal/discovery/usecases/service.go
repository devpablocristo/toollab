package usecases

import (
	"encoding/json"
	"fmt"

	artifactUC "toollab-core/internal/artifact/usecases"
	"toollab-core/internal/discovery/usecases/domain"
	"toollab-core/internal/shared"
	targetDomain "toollab-core/internal/target/usecases/domain"
)

type Service struct {
	analyzer    domain.Analyzer
	artifactSvc *artifactUC.Service
	targetRepo  targetDomain.Repository
}

func NewService(analyzer domain.Analyzer, artifactSvc *artifactUC.Service, targetRepo targetDomain.Repository) *Service {
	return &Service{analyzer: analyzer, artifactSvc: artifactSvc, targetRepo: targetRepo}
}

type DiscoverOptions struct {
	FrameworkHint        string `json:"framework_hint,omitempty"`
	GenerateScenarioPlan bool   `json:"generate_scenario_plan"`
}

type DiscoverResult struct {
	RunID                 string   `json:"run_id"`
	ServiceModelRevision  int      `json:"service_model_revision"`
	ModelReportRevision   int      `json:"model_report_revision"`
	ScenarioPlanRevision  int      `json:"scenario_plan_revision,omitempty"`
	EndpointsCount        int      `json:"endpoints_count"`
	Confidence            float64  `json:"confidence"`
	Gaps                  []string `json:"gaps"`
}

func (s *Service) Discover(runID, targetID string, opts DiscoverOptions) (DiscoverResult, error) {
	target, err := s.targetRepo.GetByID(targetID)
	if err != nil {
		return DiscoverResult{}, fmt.Errorf("loading target: %w", err)
	}

	localPath := target.Source.Value
	if target.Source.Type != targetDomain.SourcePath {
		return DiscoverResult{}, fmt.Errorf("%w: only path sources are supported for discovery in v1", shared.ErrInvalidInput)
	}

	hint := domain.HintAuto
	if opts.FrameworkHint == "chi" {
		hint = domain.HintChi
	}

	model, report, err := s.analyzer.Analyze(localPath, hint)
	if err != nil {
		return DiscoverResult{}, fmt.Errorf("analyzing source: %w", err)
	}

	modelJSON, err := json.Marshal(model)
	if err != nil {
		return DiscoverResult{}, fmt.Errorf("marshaling service model: %w", err)
	}
	modelResult, err := s.artifactSvc.Put(runID, shared.ArtifactServiceModel, modelJSON)
	if err != nil {
		return DiscoverResult{}, fmt.Errorf("saving service model: %w", err)
	}

	reportJSON, err := json.Marshal(report)
	if err != nil {
		return DiscoverResult{}, fmt.Errorf("marshaling model report: %w", err)
	}
	reportResult, err := s.artifactSvc.Put(runID, shared.ArtifactModelReport, reportJSON)
	if err != nil {
		return DiscoverResult{}, fmt.Errorf("saving model report: %w", err)
	}

	result := DiscoverResult{
		RunID:                runID,
		ServiceModelRevision: modelResult.Revision,
		ModelReportRevision:  reportResult.Revision,
		EndpointsCount:       report.EndpointsCount,
		Confidence:           report.Confidence,
		Gaps:                 report.Gaps,
	}

	if opts.GenerateScenarioPlan && len(model.Endpoints) > 0 {
		plan := GenerateScenarioPlan(runID, model)
		planJSON, err := json.Marshal(plan)
		if err != nil {
			return DiscoverResult{}, fmt.Errorf("marshaling scenario plan: %w", err)
		}
		planResult, err := s.artifactSvc.Put(runID, shared.ArtifactScenarioPlan, planJSON)
		if err != nil {
			return DiscoverResult{}, fmt.Errorf("saving scenario plan: %w", err)
		}
		result.ScenarioPlanRevision = planResult.Revision
	}

	if result.Gaps == nil {
		result.Gaps = []string{}
	}

	return result, nil
}
