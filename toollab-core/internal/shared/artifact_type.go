package shared

import "fmt"

type ArtifactType string

const (
	ArtifactTargetProfile     ArtifactType = "target_profile"
	ArtifactEndpointCatalog   ArtifactType = "endpoint_catalog"
	ArtifactRouterGraph       ArtifactType = "router_graph"
	ArtifactASTEntities       ArtifactType = "ast_entities"
	ArtifactASTCodePatterns   ArtifactType = "ast_code_patterns"
	ArtifactInferredContracts ArtifactType = "inferred_contracts"
	ArtifactSchemaRegistry    ArtifactType = "schema_registry"
	ArtifactSemanticAnnot     ArtifactType = "semantic_annotations"
	ArtifactSmokeResults      ArtifactType = "smoke_results"
	ArtifactAuthMatrix        ArtifactType = "auth_matrix"
	ArtifactFuzzResults       ArtifactType = "fuzz_results"
	ArtifactErrorSignatures   ArtifactType = "error_signatures"
	ArtifactLogicResults      ArtifactType = "logic_results"
	ArtifactAbuseResults      ArtifactType = "abuse_results"
	ArtifactConfirmations     ArtifactType = "confirmations"
	ArtifactFindingsRaw       ArtifactType = "findings_raw"
	ArtifactDossierFull       ArtifactType = "dossier_full"
	ArtifactDossierLLM        ArtifactType = "dossier_llm"
	ArtifactRunSummary        ArtifactType = "run_summary"
	ArtifactScoring           ArtifactType = "scoring"
	ArtifactRawEvidence       ArtifactType = "raw_evidence"
	ArtifactLLMDocs           ArtifactType = "llm_docs"
	ArtifactLLMAudit          ArtifactType = "llm_audit"
	ArtifactPostmanCollection    ArtifactType = "postman_collection"
	ArtifactCurlBook             ArtifactType = "curl_book"
	ArtifactOpenAPIInferred      ArtifactType = "openapi_inferred"
	ArtifactOpenAPIAST           ArtifactType = "openapi_ast"
	ArtifactEndpointIntelligence ArtifactType = "endpoint_intelligence"
	ArtifactEndpointIntelIndex   ArtifactType = "endpoint_intelligence_index"
	ArtifactEndpointQueries      ArtifactType = "endpoint_queries"
	ArtifactEnvExample           ArtifactType = "env_example"
	ArtifactRunSummaryExport     ArtifactType = "run_summary_export"
	ArtifactDossierDocsMini      ArtifactType = "dossier_docs_mini"
)

var validTypes = map[ArtifactType]bool{
	ArtifactTargetProfile:     true,
	ArtifactEndpointCatalog:   true,
	ArtifactRouterGraph:       true,
	ArtifactASTEntities:       true,
	ArtifactASTCodePatterns:   true,
	ArtifactInferredContracts: true,
	ArtifactSchemaRegistry:    true,
	ArtifactSemanticAnnot:     true,
	ArtifactSmokeResults:      true,
	ArtifactAuthMatrix:        true,
	ArtifactFuzzResults:       true,
	ArtifactErrorSignatures:   true,
	ArtifactLogicResults:      true,
	ArtifactAbuseResults:      true,
	ArtifactConfirmations:     true,
	ArtifactFindingsRaw:       true,
	ArtifactDossierFull:       true,
	ArtifactDossierLLM:        true,
	ArtifactRunSummary:        true,
	ArtifactScoring:           true,
	ArtifactRawEvidence:       true,
	ArtifactLLMDocs:           true,
	ArtifactLLMAudit:          true,
	ArtifactPostmanCollection:    true,
	ArtifactCurlBook:             true,
	ArtifactOpenAPIInferred:      true,
	ArtifactOpenAPIAST:           true,
	ArtifactEndpointIntelligence: true,
	ArtifactEndpointIntelIndex:   true,
	ArtifactEndpointQueries:      true,
	ArtifactEnvExample:           true,
	ArtifactRunSummaryExport:     true,
	ArtifactDossierDocsMini:      true,
}

func (t ArtifactType) Valid() bool { return validTypes[t] }

func ParseArtifactType(s string) (ArtifactType, error) {
	t := ArtifactType(s)
	if !t.Valid() {
		return "", fmt.Errorf("invalid artifact type: %q", s)
	}
	return t, nil
}
