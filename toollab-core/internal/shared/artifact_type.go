package shared

import "fmt"

type ArtifactType string

const (
	ArtifactServiceModel      ArtifactType = "service_model"
	ArtifactModelReport       ArtifactType = "model_report"
	ArtifactScenarioPlan      ArtifactType = "scenario_plan"
	ArtifactEvidencePack      ArtifactType = "evidence_pack"
	ArtifactAuditReport       ArtifactType = "audit_report"
	ArtifactLLMInterpretation ArtifactType = "llm_interpretation"
	ArtifactEvidenceMetrics  ArtifactType = "evidence_metrics"
	ArtifactAnalysis         ArtifactType = "analysis"
)

var validTypes = map[ArtifactType]bool{
	ArtifactServiceModel:      true,
	ArtifactModelReport:       true,
	ArtifactScenarioPlan:      true,
	ArtifactEvidencePack:      true,
	ArtifactAuditReport:       true,
	ArtifactLLMInterpretation: true,
	ArtifactEvidenceMetrics:   true,
	ArtifactAnalysis:          true,
}

func (t ArtifactType) Valid() bool { return validTypes[t] }

func ParseArtifactType(s string) (ArtifactType, error) {
	t := ArtifactType(s)
	if !t.Valid() {
		return "", fmt.Errorf("invalid artifact type: %q", s)
	}
	return t, nil
}
