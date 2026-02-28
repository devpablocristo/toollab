package usecases

import (
	"fmt"

	auditDomain "toollab-core/internal/audit/usecases/domain"
	discoveryDomain "toollab-core/internal/discovery/usecases/domain"
	evidenceDomain "toollab-core/internal/evidence/usecases/domain"
	"toollab-core/internal/interpretation/usecases/domain"
	"toollab-core/internal/shared"
)

type ValidationMode string

const (
	ModeStrict  ValidationMode = "strict"
	ModeLenient ValidationMode = "lenient"
)

type refIndex struct {
	evidenceIDs map[string]bool
	findingIDs  map[string]bool
	endpoints   []discoveryDomain.Endpoint
}

func buildRefIndex(pack *evidenceDomain.EvidencePack, report *auditDomain.AuditReport, model *discoveryDomain.ServiceModel) refIndex {
	idx := refIndex{
		evidenceIDs: make(map[string]bool),
		findingIDs:  make(map[string]bool),
	}
	if pack != nil {
		for _, item := range pack.Items {
			idx.evidenceIDs[item.EvidenceID] = true
		}
	}
	if report != nil {
		for _, f := range report.Findings {
			idx.findingIDs[f.FindingID] = true
		}
	}
	if model != nil {
		idx.endpoints = model.Endpoints
	}
	return idx
}

func (ri refIndex) validEvidenceRef(ref string) bool { return ri.evidenceIDs[ref] }
func (ri refIndex) validFindingRef(ref string) bool  { return ri.findingIDs[ref] }

func (ri refIndex) validModelRef(mr shared.ModelRef) bool {
	for _, ep := range ri.endpoints {
		if ep.Method == mr.Kind && ep.Path == mr.ID {
			return true
		}
		if ep.Ref != nil && mr.File != "" && ep.Ref.File == mr.File {
			return true
		}
		if mr.Kind == "endpoint" && ep.Method+" "+ep.Path == mr.ID {
			return true
		}
	}
	return false
}

func (ri refIndex) hasAnyValidRef(evidenceRefs, findingRefs []string, modelRefs []shared.ModelRef) bool {
	for _, r := range evidenceRefs {
		if ri.validEvidenceRef(r) {
			return true
		}
	}
	for _, r := range findingRefs {
		if ri.validFindingRef(r) {
			return true
		}
	}
	for _, r := range modelRefs {
		if ri.validModelRef(r) {
			return true
		}
	}
	// If there are no finding/audit IDs to validate against, accept items
	// with refs that simply couldn't be resolved (e.g. new pipeline without AuditReport).
	if len(ri.findingIDs) == 0 && len(findingRefs) > 0 && len(evidenceRefs) == 0 {
		return true
	}
	return false
}

type ValidateResult struct {
	Interp              domain.LLMInterpretation
	RejectedClaimsCount int
}

func Validate(
	interp domain.LLMInterpretation,
	mode ValidationMode,
	pack *evidenceDomain.EvidencePack,
	report *auditDomain.AuditReport,
	model *discoveryDomain.ServiceModel,
) (ValidateResult, error) {
	ri := buildRefIndex(pack, report, model)
	rejected := 0
	var degraded []domain.OpenQuestion

	// Validate facts
	var validFacts []domain.Fact
	for _, f := range interp.Facts {
		if ri.hasAnyValidRef(f.EvidenceRefs, f.FindingRefs, f.ModelRefs) {
			validFacts = append(validFacts, f)
		} else {
			if mode == ModeStrict {
				return ValidateResult{}, fmt.Errorf("strict validation failed: fact %q has no valid refs", f.ID)
			}
			rejected++
			degraded = append(degraded, domain.OpenQuestion{
				ID:         domain.DeterministicID("degraded:"+f.Text, nil, nil, nil),
				Question:   f.Text,
				WhyMissing: "missing_or_invalid_refs",
			})
		}
	}

	// Validate inferences
	var validInferences []domain.Inference
	for _, inf := range interp.Inferences {
		if ri.hasAnyValidRef(inf.EvidenceRefs, inf.FindingRefs, inf.ModelRefs) {
			validInferences = append(validInferences, inf)
		} else {
			if mode == ModeStrict {
				return ValidateResult{}, fmt.Errorf("strict validation failed: inference %q has no valid refs", inf.ID)
			}
			rejected++
			degraded = append(degraded, domain.OpenQuestion{
				ID:         domain.DeterministicID("degraded:"+inf.Text, nil, nil, nil),
				Question:   inf.Text,
				WhyMissing: "missing_or_invalid_refs",
			})
		}
	}

	// Validate guided steps
	var validSteps []domain.GuidedStep
	for _, gs := range interp.GuidedTour {
		if ri.hasAnyValidRef(gs.Refs.EvidenceRefs, gs.Refs.FindingRefs, gs.Refs.ModelRefs) {
			validSteps = append(validSteps, gs)
		} else {
			if mode == ModeStrict {
				return ValidateResult{}, fmt.Errorf("strict validation failed: guided step %q has no valid refs", gs.ID)
			}
			rejected++
			degraded = append(degraded, domain.OpenQuestion{
				ID:         domain.DeterministicID("degraded:"+gs.Title, nil, nil, nil),
				Question:   gs.Title + ": " + gs.Action,
				WhyMissing: "missing_or_invalid_refs",
			})
		}
	}

	// Validate scenario suggestions
	var validSuggestions []domain.ScenarioSuggestion
	for _, ss := range interp.ScenarioSuggestions {
		if ri.hasAnyValidRef(ss.Refs.EvidenceRefs, ss.Refs.FindingRefs, ss.Refs.ModelRefs) {
			validSuggestions = append(validSuggestions, ss)
		} else {
			if mode == ModeStrict {
				return ValidateResult{}, fmt.Errorf("strict validation failed: scenario suggestion %q has no valid refs", ss.ID)
			}
			rejected++
			degraded = append(degraded, domain.OpenQuestion{
				ID:         domain.DeterministicID("degraded:"+ss.Name, nil, nil, nil),
				Question:   ss.Name + ": " + ss.ExpectedObservation,
				WhyMissing: "missing_or_invalid_refs",
			})
		}
	}

	interp.Facts = validFacts
	interp.Inferences = validInferences
	interp.GuidedTour = validSteps
	interp.ScenarioSuggestions = validSuggestions
	interp.OpenQuestions = append(interp.OpenQuestions, degraded...)

	return ValidateResult{
		Interp:              interp,
		RejectedClaimsCount: rejected,
	}, nil
}
