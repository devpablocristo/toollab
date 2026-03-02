package domain

import (
	"strconv"
	"time"
)

// DossierV2Full is the complete run output (stored, used by UI).
type DossierV2Full struct {
	SchemaVersion string                 `json:"schema_version"`
	RunID         string                 `json:"run_id"`
	RunConfig     RunConfig              `json:"run_config"`
	CreatedAt     time.Time              `json:"created_at"`
	Status        RunStatus              `json:"status"`
	RunMode       RunMode                `json:"run_mode"`
	RunModeDetail *RunModeClassification `json:"run_mode_detail,omitempty"`
	TargetProfile TargetProfile          `json:"target_profile"`

	AST ASTSection `json:"ast"`

	Runtime RuntimeSection `json:"runtime"`

	Confirmations []Confirmation `json:"confirmations"`
	FindingsRaw   []FindingRaw   `json:"findings_raw"`
	ExportsIndex  ExportsIndex   `json:"exports_index"`

	Scoring    ScoringResult `json:"scoring"`
	RunSummary RunSummary    `json:"run_summary"`

	StepResults []StepResult `json:"step_results"`
}

type ASTSection struct {
	EndpointCatalog EndpointCatalog  `json:"endpoint_catalog"`
	RouterGraph     RouterGraph      `json:"router_graph"`
	ASTEntities     []ASTEntity      `json:"ast_entities,omitempty"`
	ASTCodePatterns []ASTCodePattern `json:"ast_code_patterns,omitempty"`
}

type RuntimeSection struct {
	EvidenceSamples     []EvidenceSample     `json:"evidence_samples"`
	ErrorSignatures     []ErrorSignature     `json:"error_signatures,omitempty"`
	SmokeResults        []SmokeResult        `json:"smoke_results,omitempty"`
	FuzzResults         []FuzzResult         `json:"fuzz_results,omitempty"`
	LogicResults        []LogicResult        `json:"logic_results,omitempty"`
	AbuseResults        []AbuseResult        `json:"abuse_results,omitempty"`
	AuthMatrix          *AuthMatrix          `json:"auth_matrix,omitempty"`
	InferredContracts   []InferredContract   `json:"inferred_contracts,omitempty"`
	SemanticAnnotations []SemanticAnnotation `json:"semantic_annotations,omitempty"`
	DerivedMetrics      DerivedMetrics       `json:"derived_metrics"`
	Discrepancies       Discrepancies        `json:"discrepancies"`
}

type DerivedMetrics struct {
	TotalRequests     int                       `json:"total_requests"`
	SuccessRate       float64                   `json:"success_rate"`
	ErrorRate         float64                   `json:"error_rate"`
	P50Ms             int64                     `json:"p50_ms"`
	P95Ms             int64                     `json:"p95_ms"`
	P99Ms             int64                     `json:"p99_ms"`
	CoveragePct       float64                   `json:"coverage_pct"`
	UsefulCoveragePct float64                   `json:"useful_coverage_pct,omitempty"`
	EndpointsTested   int                       `json:"endpoints_tested"`
	EndpointsUseful   int                       `json:"endpoints_useful,omitempty"`
	EndpointsTotal    int                       `json:"endpoints_total"`
	StatusHistogram   map[int]int               `json:"status_histogram,omitempty"`
	ResilienceMetrics *ResilienceMetrics        `json:"resilience_metrics,omitempty"`
	OpenAPIValidation *OpenAPIValidationSummary `json:"openapi_validation,omitempty"`
}

type OpenAPIValidationSummary struct {
	SpecDetected     bool    `json:"spec_detected"`
	SpecEvidenceRef  string  `json:"spec_evidence_ref,omitempty"`
	SpecEndpoints    int     `json:"spec_endpoints"`
	ASTEndpoints     int     `json:"ast_endpoints"`
	ASTMissingInSpec int     `json:"ast_missing_in_spec"`
	SpecMissingInAST int     `json:"spec_missing_in_ast"`
	MatchPct         float64 `json:"match_pct"`
}

type Discrepancies struct {
	ASTvsRuntime []AuthDiscrepancy `json:"ast_vs_runtime,omitempty"`
}

// ExportsIndex lists generated export files.
type ExportsIndex struct {
	PostmanCollection string `json:"postman_collection,omitempty"`
	CurlBook          string `json:"curl_book,omitempty"`
	OpenAPIInferred   string `json:"openapi_inferred,omitempty"`
	AuthMatrixCSV     string `json:"auth_matrix_csv,omitempty"`
	EndpointCSV       string `json:"endpoint_catalog_csv,omitempty"`
	ContractMatrixCSV string `json:"contract_matrix_csv,omitempty"`
	HotspotsCSV       string `json:"endpoint_hotspots_csv,omitempty"`
	EnvExample        string `json:"env_example,omitempty"`
}

// RunSummary is the lightweight UI-friendly summary.
type RunSummary struct {
	RunID                  string                     `json:"run_id"`
	Status                 RunStatus                  `json:"status"`
	RunMode                RunMode                    `json:"run_mode"`
	RunModeDetail          *RunModeClassification     `json:"run_mode_detail,omitempty"`
	DurationSeconds        int                        `json:"duration_seconds"`
	EndpointsDiscoveredAST int                        `json:"endpoints_discovered_ast"`
	EndpointsConfirmedRT   int                        `json:"endpoints_confirmed_runtime"`
	EndpointsRuntimeOnly   int                        `json:"endpoints_runtime_only"`
	EndpointsNotReachable  int                        `json:"endpoints_not_reachable"`
	EvidenceCountFull      int                        `json:"evidence_count_full"`
	EvidenceCountLLM       int                        `json:"evidence_count_llm"`
	CoveragePct            float64                    `json:"coverage_pct"`
	CoverageUsefulPct      float64                    `json:"coverage_useful_pct,omitempty"`
	EndpointsUseful        int                        `json:"endpoints_useful,omitempty"`
	AuthReady              bool                       `json:"auth_ready"`
	AuthReadinessReason    string                     `json:"auth_readiness_reason,omitempty"`
	OpenAPIValidation      *OpenAPIValidationSummary  `json:"openapi_validation,omitempty"`
	BaselineDelta          *BaselineDelta             `json:"baseline_delta,omitempty"`
	ScoresAvailable        bool                       `json:"scores_available"`
	Scores                 map[ScoreDimension]float64 `json:"scores,omitempty"`
	TopFindings            []FindingSummary           `json:"top_findings,omitempty"`
	BudgetUsage            BudgetUsage                `json:"budget_usage"`
}

type BaselineDelta struct {
	PreviousRunID          string                     `json:"previous_run_id"`
	CoveragePctDelta       float64                    `json:"coverage_pct_delta"`
	UsefulCoveragePctDelta float64                    `json:"useful_coverage_pct_delta,omitempty"`
	ScoreDeltas            map[ScoreDimension]float64 `json:"score_deltas,omitempty"`
	Regressions            []string                   `json:"regressions,omitempty"`
}

type FindingSummary struct {
	ID           string          `json:"id"`
	Severity     FindingSeverity `json:"severity"`
	Title        string          `json:"title"`
	EvidenceRefs []string        `json:"evidence_refs,omitempty"`
}

// DossierV2LLM is the compacted version sent to the LLM.
type DossierV2LLM struct {
	SchemaVersion string        `json:"schema_version"`
	RunID         string        `json:"run_id"`
	RunMode       RunMode       `json:"run_mode"`
	RunSummary    RunSummary    `json:"run_summary"`
	TargetProfile TargetProfile `json:"target_profile"`

	AST ASTSection `json:"ast"`

	Runtime RuntimeSectionLLM `json:"runtime"`

	Confirmations []Confirmation `json:"confirmations"`
	FindingsRaw   []FindingRaw   `json:"findings_raw"`
}

// RuntimeSectionLLM is the compacted runtime section for LLM.
type RuntimeSectionLLM struct {
	EvidenceSamples     []EvidenceSample     `json:"evidence_samples"`
	ErrorSignatures     []ErrorSignature     `json:"error_signatures,omitempty"`
	SmokeResults        []SmokeResult        `json:"smoke_results,omitempty"`
	AuthMatrix          *AuthMatrix          `json:"auth_matrix,omitempty"`
	InferredContracts   []InferredContract   `json:"inferred_contracts,omitempty"`
	SemanticAnnotations []SemanticAnnotation `json:"semantic_annotations,omitempty"`
	DerivedMetrics      DerivedMetrics       `json:"derived_metrics"`
	Discrepancies       Discrepancies        `json:"discrepancies"`
}

// CompactConfig controls how the dossier is compacted for LLM.
type CompactConfig struct {
	MaxEvidenceSamples int `json:"max_evidence_samples"`
	MaxBodySnippet     int `json:"max_body_snippet"`
}

func DefaultCompactConfig() CompactConfig {
	return CompactConfig{
		MaxEvidenceSamples: 200,
		MaxBodySnippet:     2048,
	}
}

// CompactForLLM produces a DossierV2LLM from a DossierV2Full.
func CompactForLLM(full *DossierV2Full, cfg CompactConfig) DossierV2LLM {
	llm := DossierV2LLM{
		SchemaVersion: full.SchemaVersion,
		RunID:         full.RunID,
		RunMode:       full.RunMode,
		RunSummary:    full.RunSummary,
		TargetProfile: full.TargetProfile,
		AST:           full.AST,
		Confirmations: full.Confirmations,
		FindingsRaw:   full.FindingsRaw,
	}

	samples := selectTopSamples(full, cfg.MaxEvidenceSamples)
	for i := range samples {
		samples[i] = truncateSample(samples[i], cfg.MaxBodySnippet)
	}

	llm.Runtime = RuntimeSectionLLM{
		EvidenceSamples:     samples,
		ErrorSignatures:     full.Runtime.ErrorSignatures,
		SmokeResults:        full.Runtime.SmokeResults,
		AuthMatrix:          full.Runtime.AuthMatrix,
		InferredContracts:   full.Runtime.InferredContracts,
		SemanticAnnotations: full.Runtime.SemanticAnnotations,
		DerivedMetrics:      full.Runtime.DerivedMetrics,
		Discrepancies:       full.Runtime.Discrepancies,
	}

	return llm
}

func selectTopSamples(full *DossierV2Full, max int) []EvidenceSample {
	if len(full.Runtime.EvidenceSamples) <= max {
		cp := make([]EvidenceSample, len(full.Runtime.EvidenceSamples))
		copy(cp, full.Runtime.EvidenceSamples)
		return cp
	}

	selected := make(map[string]bool)
	var out []EvidenceSample
	add := func(s EvidenceSample) {
		if !selected[s.EvidenceID] && len(out) < max {
			selected[s.EvidenceID] = true
			out = append(out, s)
		}
	}

	smokeRefs := make(map[string]bool)
	for _, sr := range full.Runtime.SmokeResults {
		for _, ref := range sr.EvidenceRefs {
			smokeRefs[ref] = true
		}
	}

	authRefs := make(map[string]bool)
	if full.Runtime.AuthMatrix != nil {
		for _, e := range full.Runtime.AuthMatrix.Entries {
			for _, ref := range e.EvidenceRefs {
				authRefs[ref] = true
			}
		}
	}

	confirmRefs := make(map[string]bool)
	for _, c := range full.Confirmations {
		confirmRefs[c.ReplayEvidence] = true
		confirmRefs[c.OriginalEvidence] = true
	}

	// Priority 1: smoke evidence
	for _, s := range full.Runtime.EvidenceSamples {
		if smokeRefs[s.EvidenceID] {
			add(s)
		}
	}
	// Priority 2: auth evidence
	for _, s := range full.Runtime.EvidenceSamples {
		if authRefs[s.EvidenceID] {
			add(s)
		}
	}
	// Priority 3: confirmation evidence
	for _, s := range full.Runtime.EvidenceSamples {
		if confirmRefs[s.EvidenceID] {
			add(s)
		}
	}
	// Priority 4: diverse status codes (one per status per endpoint)
	seen := make(map[string]bool)
	for _, s := range full.Runtime.EvidenceSamples {
		if s.Response == nil {
			continue
		}
		key := s.EndpointID + ":" + strconv.Itoa(s.Response.Status)
		if !seen[key] {
			seen[key] = true
			add(s)
		}
	}
	// Priority 5: fill remaining
	for _, s := range full.Runtime.EvidenceSamples {
		add(s)
	}

	return out
}

func truncateSample(s EvidenceSample, maxBody int) EvidenceSample {
	if len(s.Request.Body) > maxBody {
		s.Request.Body = s.Request.Body[:maxBody]
	}
	if s.Response != nil && len(s.Response.BodySnippet) > maxBody {
		s.Response.BodySnippet = s.Response.BodySnippet[:maxBody]
	}
	return s
}
