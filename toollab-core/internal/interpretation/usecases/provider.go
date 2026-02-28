package usecases

import (
	"context"
	"encoding/json"
	"strconv"

	"toollab-core/internal/interpretation/usecases/domain"
	"toollab-core/internal/shared"
)

type Provider interface {
	Name() string
	Interpret(ctx context.Context, dossier domain.Dossier) ([]byte, error)
}

// MockProvider produces a deterministic, well-anchored LLMInterpretation from the dossier.
type MockProvider struct{}

func NewMockProvider() *MockProvider { return &MockProvider{} }

func (m *MockProvider) Name() string { return "mock" }

func (m *MockProvider) Interpret(_ context.Context, dossier domain.Dossier) ([]byte, error) {
	var firstEvidenceID, secondEvidenceID string
	if len(dossier.EvidenceSamples) > 0 {
		firstEvidenceID = dossier.EvidenceSamples[0].EvidenceID
	}
	if len(dossier.EvidenceSamples) > 1 {
		secondEvidenceID = dossier.EvidenceSamples[1].EvidenceID
	}

	var firstFindingID string
	if len(dossier.AuditHighlights) > 0 {
		firstFindingID = dossier.AuditHighlights[0].FindingID
	}

	var firstModelRef *shared.ModelRef
	if len(dossier.EndpointsTop) > 0 && dossier.EndpointsTop[0].ModelRef != nil {
		firstModelRef = dossier.EndpointsTop[0].ModelRef
	}

	var facts []domain.Fact
	if firstFindingID != "" && firstEvidenceID != "" {
		facts = append(facts, domain.Fact{
			Text:         "Multiple endpoints are accessible without authentication headers and return 2xx status codes.",
			EvidenceRefs: []string{firstEvidenceID},
			FindingRefs:  []string{firstFindingID},
			Confidence:   0.9,
		})
	}
	if firstModelRef != nil && secondEvidenceID != "" {
		facts = append(facts, domain.Fact{
			Text:         "The service exposes " + itoa(dossier.ServiceOverview.EndpointsCount) + " HTTP endpoints discovered via AST analysis.",
			EvidenceRefs: []string{secondEvidenceID},
			ModelRefs:    []shared.ModelRef{*firstModelRef},
			Confidence:   0.95,
		})
	}

	for i := range facts {
		facts[i].ID = domain.DeterministicID(facts[i].Text, facts[i].EvidenceRefs, facts[i].FindingRefs, facts[i].ModelRefs)
	}

	var inferences []domain.Inference
	if firstFindingID != "" && firstEvidenceID != "" {
		inf := domain.Inference{
			Text:            "The observed 2xx responses without authentication suggest a possible missing auth middleware.",
			RuleOfInference: "observed_2xx_without_auth_implies_possible_auth_gap",
			EvidenceRefs:    []string{firstEvidenceID},
			FindingRefs:     []string{firstFindingID},
			Confidence:      0.75,
		}
		inf.ID = domain.DeterministicID(inf.Text, inf.EvidenceRefs, inf.FindingRefs, inf.ModelRefs)
		inferences = append(inferences, inf)
	}

	var targetEndpointKey string
	if len(dossier.EndpointsTop) > 0 {
		targetEndpointKey = dossier.EndpointsTop[0].EndpointKey
	}

	oq := domain.OpenQuestion{
		Question:   "What happens when an invalid or expired bearer token is sent to protected endpoints?",
		WhyMissing: "insufficient_evidence",
		SuggestedProbe: &domain.ScenarioSuggestion{
			Name:              "Test invalid bearer token",
			TargetEndpointKey: targetEndpointKey,
			RequestPatch: domain.RequestPatch{
				Headers: map[string]string{"Authorization": "Bearer invalid-token-xxx"},
			},
			ExpectedObservation: "Expect 401 Unauthorized",
			Refs:                buildSuggestionRefs(firstEvidenceID, firstFindingID),
		},
	}
	oq.ID = domain.DeterministicID(oq.Question, nil, nil, nil)
	if oq.SuggestedProbe != nil {
		oq.SuggestedProbe.ID = domain.DeterministicID(oq.SuggestedProbe.Name, oq.SuggestedProbe.Refs.EvidenceRefs, oq.SuggestedProbe.Refs.FindingRefs, oq.SuggestedProbe.Refs.ModelRefs)
	}

	var guidedSteps []domain.GuidedStep
	if firstFindingID != "" {
		gs1 := domain.GuidedStep{
			Title:  "Review the top auth finding",
			Action: "ReviewFinding",
			Target: domain.StepTarget{FindingID: firstFindingID},
			Refs:   domain.StepRefs{FindingRefs: []string{firstFindingID}},
		}
		gs1.ID = domain.DeterministicID(gs1.Title, gs1.Refs.EvidenceRefs, gs1.Refs.FindingRefs, gs1.Refs.ModelRefs)
		guidedSteps = append(guidedSteps, gs1)
	}
	if targetEndpointKey != "" && firstEvidenceID != "" {
		gs2 := domain.GuidedStep{
			Title:  "Inspect the top endpoint",
			Action: "InspectEndpoint",
			Target: domain.StepTarget{EndpointKey: targetEndpointKey},
			Refs:   domain.StepRefs{EvidenceRefs: []string{firstEvidenceID}},
		}
		gs2.ID = domain.DeterministicID(gs2.Title, gs2.Refs.EvidenceRefs, gs2.Refs.FindingRefs, gs2.Refs.ModelRefs)
		guidedSteps = append(guidedSteps, gs2)
	}

	var suggestions []domain.ScenarioSuggestion
	if targetEndpointKey != "" {
		ss := domain.ScenarioSuggestion{
			Name:              "Test endpoint with custom header",
			TargetEndpointKey: targetEndpointKey,
			RequestPatch: domain.RequestPatch{
				Headers: map[string]string{"X-Custom-Test": "value"},
			},
			ExpectedObservation: "Observe if the endpoint behavior changes with custom headers",
			Refs:                buildSuggestionRefs(firstEvidenceID, firstFindingID),
		}
		ss.ID = domain.DeterministicID(ss.Name, ss.Refs.EvidenceRefs, ss.Refs.FindingRefs, ss.Refs.ModelRefs)
		suggestions = append(suggestions, ss)
	}

	interp := domain.LLMInterpretation{
		SchemaVersion:       "v1",
		RunID:               dossier.RunID,
		CreatedAt:           shared.Now(),
		Facts:               facts,
		Inferences:          inferences,
		OpenQuestions:        []domain.OpenQuestion{oq},
		GuidedTour:          guidedSteps,
		ScenarioSuggestions: suggestions,
	}

	return json.Marshal(interp)
}

func buildSuggestionRefs(evidenceID, findingID string) domain.StepRefs {
	refs := domain.StepRefs{}
	if evidenceID != "" {
		refs.EvidenceRefs = []string{evidenceID}
	}
	if findingID != "" {
		refs.FindingRefs = []string{findingID}
	}
	return refs
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
