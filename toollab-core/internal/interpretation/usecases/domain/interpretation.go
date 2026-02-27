package domain

import "toollab-core/internal/shared"

type Interpreter interface {
	Interpret(dossier Dossier) (Interpretation, error)
}

type Dossier struct {
	RunID        string `json:"run_id"`
	ModelSummary string `json:"model_summary"`
	TopFindings  string `json:"top_findings"`
	Gaps         string `json:"gaps"`
}

type Interpretation struct {
	Facts               []Claim  `json:"facts"`
	Inferences          []Claim  `json:"inferences"`
	OpenQuestions       []string `json:"open_questions"`
	GuidedTour          []string `json:"guided_tour"`
	ScenarioSuggestions []string `json:"scenario_suggestions"`
}

type Claim struct {
	Text string            `json:"text"`
	Refs []shared.ModelRef `json:"refs"`
}
