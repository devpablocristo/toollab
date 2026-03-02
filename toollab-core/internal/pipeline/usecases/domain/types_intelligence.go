package domain

import "time"

// EndpointIntelligence is the top-level artifact for the Endpoint Intelligence feature.
type EndpointIntelligence struct {
	SchemaVersion string              `json:"schema_version"`
	RunID         string              `json:"run_id"`
	BaseURL       string              `json:"base_url"`
	GeneratedAt   time.Time           `json:"generated_at"`
	Domains       []IntelDomain       `json:"domains"`
	OpenQuestions []IntelOpenQuestion `json:"open_questions,omitempty"`
}

type IntelDomain struct {
	DomainName        string          `json:"domain_name"`
	DomainBasis       string          `json:"domain_basis"`
	DomainDescription string          `json:"domain_description"`
	Endpoints         []IntelEndpoint `json:"endpoints"`
}

type IntelEndpoint struct {
	EndpointID        string             `json:"endpoint_id"`
	Method            string             `json:"method"`
	PathTemplate      string             `json:"path_template"`
	OperationID       string             `json:"operation_id"`
	Tags              []string           `json:"tags,omitempty"`
	Auth              IntelAuth          `json:"auth"`
	WhatItDoes        IntelWhatItDoes    `json:"what_it_does"`
	Inputs            IntelInputs        `json:"inputs"`
	Outputs           IntelOutputs       `json:"outputs"`
	HowToQuery        IntelHowToQuery    `json:"how_to_query"`
	TestsYouShouldRun []IntelTest        `json:"tests_you_should_run,omitempty"`
	SecurityNotes     IntelSecurityNotes `json:"security_notes"`
	ASTRefs           []ASTRef           `json:"ast_refs,omitempty"`
	EvidenceRefs      []string           `json:"evidence_refs,omitempty"`
}

type IntelAuth struct {
	Required  string `json:"required"`
	From      string `json:"from"`
	Mechanism string `json:"mechanism"`
	Notes     string `json:"notes,omitempty"`
}

type IntelWhatItDoes struct {
	Summary    string           `json:"summary"`
	Detailed   string           `json:"detailed"`
	Confidence float64          `json:"confidence"`
	Facts      []IntelFact      `json:"facts,omitempty"`
	Inferences []IntelInference `json:"inferences,omitempty"`
}

type IntelFact struct {
	Text         string   `json:"text"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

type IntelInference struct {
	Text            string   `json:"text"`
	RuleOfInference string   `json:"rule_of_inference"`
	Confidence      float64  `json:"confidence"`
	ASTRefs         []ASTRef `json:"ast_refs,omitempty"`
	EvidenceRefs    []string `json:"evidence_refs,omitempty"`
}

type IntelInputs struct {
	PathParams  []IntelParam      `json:"path_params,omitempty"`
	QueryParams []IntelQueryParam `json:"query_params,omitempty"`
	Headers     []IntelHeader     `json:"headers,omitempty"`
	Body        *IntelBody        `json:"body,omitempty"`
}

type IntelParam struct {
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	Meaning    string  `json:"meaning"`
	Source     string  `json:"source"`
	Confidence float64 `json:"confidence"`
}

type IntelQueryParam struct {
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	Meaning        string   `json:"meaning,omitempty"`
	ObservedValues []string `json:"observed_values,omitempty"`
	Source         string   `json:"source"`
	Confidence     float64  `json:"confidence"`
}

type IntelHeader struct {
	Name           string   `json:"name"`
	Required       string   `json:"required"`
	ObservedValues []string `json:"observed_values,omitempty"`
	Source         string   `json:"source"`
	Confidence     float64  `json:"confidence"`
}

type IntelBody struct {
	ContentType        string           `json:"content_type"`
	SchemaRef          string           `json:"schema_ref,omitempty"`
	RequiredFields     []IntelBodyField `json:"required_fields,omitempty"`
	ExampleEvidenceRef string           `json:"example_from_evidence_ref,omitempty"`
	Notes              string           `json:"notes,omitempty"`
}

type IntelBodyField struct {
	FieldPath  string  `json:"field_path"`
	Type       string  `json:"type"`
	Meaning    string  `json:"meaning,omitempty"`
	Source     string  `json:"source"`
	Confidence float64 `json:"confidence"`
}

type IntelOutputs struct {
	Responses    []IntelResponse    `json:"responses,omitempty"`
	CommonErrors []IntelCommonError `json:"common_errors,omitempty"`
}

type IntelResponse struct {
	Status      int    `json:"status"`
	ContentType string `json:"content_type"`
	SchemaRef   string `json:"schema_ref,omitempty"`
	WhatYouGet  string `json:"what_you_get"`
	ExampleRef  string `json:"example_ref,omitempty"`
}

type IntelCommonError struct {
	Status     int    `json:"status"`
	Meaning    string `json:"meaning"`
	ExampleRef string `json:"example_ref,omitempty"`
}

type IntelHowToQuery struct {
	Goal          string              `json:"goal"`
	ReadyCommands []IntelReadyCommand `json:"ready_commands"`
	QueryVariants []IntelQueryVariant `json:"query_variants,omitempty"`
	Warnings      []string            `json:"warnings,omitempty"`
}

type IntelReadyCommand struct {
	Name         string             `json:"name"`
	Kind         string             `json:"kind"`
	Command      string             `json:"command"`
	Placeholders []IntelPlaceholder `json:"placeholders,omitempty"`
	BasedOn      string             `json:"based_on"`
	EvidenceRefs []string           `json:"evidence_refs,omitempty"`
	Notes        string             `json:"notes,omitempty"`
}

type IntelPlaceholder struct {
	Name    string `json:"name"`
	Example string `json:"example"`
}

type IntelQueryVariant struct {
	VariantName  string   `json:"variant_name"`
	Description  string   `json:"description"`
	Command      string   `json:"command"`
	Source       string   `json:"source"`
	Confidence   float64  `json:"confidence"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
	Notes        string   `json:"notes,omitempty"`
}

type IntelTest struct {
	Name         string   `json:"name"`
	Why          string   `json:"why"`
	CommandRef   string   `json:"command_ref,omitempty"`
	Importance   string   `json:"importance"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

type IntelSecurityNotes struct {
	Exposures          []IntelExposure   `json:"exposures,omitempty"`
	ASTCodePatternsRel []IntelASTPattern `json:"ast_code_patterns_related,omitempty"`
}

type IntelExposure struct {
	Text         string   `json:"text"`
	Severity     string   `json:"severity"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

type IntelASTPattern struct {
	Pattern                string   `json:"pattern"`
	ASTRef                 ASTRef   `json:"ast_ref"`
	OnlyIfCorrelatedWithRT bool     `json:"only_if_correlated_with_runtime"`
	EvidenceRefs           []string `json:"evidence_refs,omitempty"`
}

type IntelOpenQuestion struct {
	Question   string `json:"question"`
	WhyMissing string `json:"why_missing"`
	Priority   string `json:"priority"`
}

// EndpointQueryScripts holds all generated shell scripts as a map.
type EndpointQueryScripts struct {
	SchemaVersion string                       `json:"schema_version"`
	RunID         string                       `json:"run_id"`
	BaseURL       string                       `json:"base_url"`
	Scripts       map[string]EndpointScriptSet `json:"scripts"`
}

type EndpointScriptSet struct {
	HappyPath    string `json:"happy_path,omitempty"`
	NoAuth       string `json:"no_auth,omitempty"`
	InvalidAuth  string `json:"invalid_auth,omitempty"`
	CommonErrors string `json:"common_errors,omitempty"`
	Variants     string `json:"variants,omitempty"`
	HTTPFile     string `json:"http_file,omitempty"`
}

// IntelIndex is the lightweight summary for the frontend table.
type IntelIndex struct {
	SchemaVersion  string               `json:"schema_version"`
	RunID          string               `json:"run_id"`
	BaseURL        string               `json:"base_url"`
	TotalEndpoints int                  `json:"total_endpoints"`
	Domains        []IntelDomainSummary `json:"domains"`
	Endpoints      []IntelEndpointIndex `json:"endpoints"`
}

type IntelDomainSummary struct {
	DomainName    string `json:"domain_name"`
	EndpointCount int    `json:"endpoint_count"`
}

type IntelEndpointIndex struct {
	EndpointID   string  `json:"endpoint_id"`
	Method       string  `json:"method"`
	Path         string  `json:"path"`
	OperationID  string  `json:"operation_id"`
	Domain       string  `json:"domain"`
	AuthRequired string  `json:"auth_required"`
	Summary      string  `json:"summary"`
	Confidence   float64 `json:"confidence"`
	CommandCount int     `json:"command_count"`
	HasEvidence  bool    `json:"has_evidence"`
}
