package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	artifactUC "toollab-core/internal/artifact/usecases"
	"toollab-core/internal/interpretation/usecases/domain"
	"toollab-core/internal/shared"
)

type Service struct {
	dossierBuilder *DossierBuilder
	provider       Provider
	artifactSvc    *artifactUC.Service
}

func NewService(dossierBuilder *DossierBuilder, provider Provider, artifactSvc *artifactUC.Service) *Service {
	return &Service{
		dossierBuilder: dossierBuilder,
		provider:       provider,
		artifactSvc:    artifactSvc,
	}
}

type InterpretOptions struct {
	Kind            InterpretKind `json:"kind,omitempty"`
	Mode            string        `json:"mode,omitempty"`
	TopEndpoints    int           `json:"top_endpoints,omitempty"`
	TopFindings     int           `json:"top_findings,omitempty"`
	MaxSnippetBytes int           `json:"max_snippet_bytes,omitempty"`
	ProviderName    string        `json:"provider,omitempty"`
}

type InterpretResult struct {
	RunID                    string `json:"run_id"`
	LLMInterpretationRev     int    `json:"llm_interpretation_revision"`
	FactsCount               int    `json:"facts_count"`
	InferencesCount          int    `json:"inferences_count"`
	QuestionsCount           int    `json:"questions_count"`
	RejectedClaimsCount      int    `json:"rejected_claims_count"`
	ProviderName             string `json:"provider_name"`
	ValidationMode           string `json:"validation_mode"`
}

func (s *Service) Interpret(ctx context.Context, runID string, opts InterpretOptions) (InterpretResult, error) {
	kind := opts.Kind
	if kind == "" {
		kind = KindAnalysis
	}

	mode := ModeLenient
	if opts.Mode == "strict" {
		mode = ModeStrict
	}

	log.Printf("interpret [%s]: building dossier for run %s", kind, runID)

	dossierOpts := DossierOptions{
		TopEndpoints:    opts.TopEndpoints,
		TopFindings:     opts.TopFindings,
		MaxSnippetBytes: opts.MaxSnippetBytes,
	}
	dossier, arts, err := s.dossierBuilder.Build(runID, dossierOpts)
	if err != nil {
		return InterpretResult{}, fmt.Errorf("dossier build: %w", err)
	}

	log.Printf("interpret [%s]: dossier built — %d samples, %d highlights, sending to %s",
		kind, len(dossier.EvidenceSamples), len(dossier.AuditHighlights), s.provider.Name())

	rawResponse, err := s.provider.Interpret(ctx, dossier, kind)
	if err != nil {
		return InterpretResult{}, fmt.Errorf("provider error: %w", err)
	}

	log.Printf("interpret [%s]: provider returned %d bytes, parsing...", kind, len(rawResponse))

	var interp domain.LLMInterpretation
	if err := json.Unmarshal(rawResponse, &interp); err != nil {
		return InterpretResult{}, fmt.Errorf("parsing provider response: %w", err)
	}

	interp.RunID = runID
	if interp.SchemaVersion == "" {
		interp.SchemaVersion = "v1"
	}
	if interp.CreatedAt.IsZero() {
		interp.CreatedAt = shared.Now()
	}

	vr, err := Validate(interp, mode, arts.pack, arts.audit, arts.model)
	if err != nil {
		log.Printf("interpret [%s]: validation failed (mode=%s): %v — saving raw anyway", kind, mode, err)
		vr = ValidateResult{Interp: interp, RejectedClaimsCount: 0}
	}

	interp = vr.Interp
	interp.Stats = domain.Stats{
		FactsCount:          len(interp.Facts),
		InferencesCount:     len(interp.Inferences),
		QuestionsCount:      len(interp.OpenQuestions),
		RejectedClaimsCount: vr.RejectedClaimsCount,
		ProviderName:        s.provider.Name(),
		ValidationMode:      string(mode),
	}

	interpJSON, err := json.Marshal(interp)
	if err != nil {
		return InterpretResult{}, fmt.Errorf("marshaling interpretation: %w", err)
	}

	artifactType := shared.ArtifactLLMInterpretation
	if kind == KindDocumentation {
		artifactType = shared.ArtifactLLMDocumentation
	}

	putResult, err := s.artifactSvc.Put(runID, artifactType, interpJSON)
	if err != nil {
		return InterpretResult{}, fmt.Errorf("saving %s: %w", kind, err)
	}

	log.Printf("interpret [%s]: saved (rev %d) — %d facts, %d inferences, %d rejected",
		kind, putResult.Revision, interp.Stats.FactsCount, interp.Stats.InferencesCount, vr.RejectedClaimsCount)

	return InterpretResult{
		RunID:                runID,
		LLMInterpretationRev: putResult.Revision,
		FactsCount:           interp.Stats.FactsCount,
		InferencesCount:      interp.Stats.InferencesCount,
		QuestionsCount:       interp.Stats.QuestionsCount,
		RejectedClaimsCount:  interp.Stats.RejectedClaimsCount,
		ProviderName:         interp.Stats.ProviderName,
		ValidationMode:       interp.Stats.ValidationMode,
	}, nil
}

// WithProvider returns a new Service using the given Provider.
func (s *Service) WithProvider(p Provider) *Service {
	return &Service{
		dossierBuilder: s.dossierBuilder,
		provider:       p,
		artifactSvc:    s.artifactSvc,
	}
}
