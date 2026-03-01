package domain

// FindingTaxonomy defines stable IDs for all finding types.
type FindingTaxonomy string

const (
	// Security
	TaxSecAuthBypass     FindingTaxonomy = "SEC-AUTH-BYPASS"
	TaxSecIDOR           FindingTaxonomy = "SEC-IDOR"
	TaxSecSQLi           FindingTaxonomy = "SEC-SQLI"
	TaxSecXSSReflected   FindingTaxonomy = "SEC-XSS-REFLECTED"
	TaxSecPathTraversal  FindingTaxonomy = "SEC-PATH-TRAVERSAL"
	TaxSecCMDi           FindingTaxonomy = "SEC-CMDI"
	TaxSecDataExposure   FindingTaxonomy = "SEC-DATA-EXPOSURE"
	TaxSecInfoLeak       FindingTaxonomy = "SEC-INFO-LEAK"
	TaxSecHiddenCritical FindingTaxonomy = "SEC-HIDDEN-CRITICAL"
	TaxSecHeadersMissing FindingTaxonomy = "SEC-HEADERS-MISSING"

	// Robustness
	TaxRobMalformedCrash    FindingTaxonomy = "ROB-MALFORMED-CRASH"
	TaxRobLargePayloadCrash FindingTaxonomy = "ROB-LARGE-PAYLOAD-CRASH"
	TaxRobBoundaryCrash     FindingTaxonomy = "ROB-BOUNDARY-CRASH"
	TaxRobTimeoutCrash      FindingTaxonomy = "ROB-TIMEOUT-CRASH"

	// Contract
	TaxConStatusSemantics       FindingTaxonomy = "CON-STATUS-SEMANTICS"
	TaxConContentType           FindingTaxonomy = "CON-CONTENT-TYPE"
	TaxConErrorFormatInconsist  FindingTaxonomy = "CON-ERROR-FORMAT-INCONSISTENT"
	TaxConSchemaViolation       FindingTaxonomy = "CON-SCHEMA-VIOLATION"

	// Performance
	TaxPerfP95High    FindingTaxonomy = "PERF-P95-HIGH"
	TaxPerfP99High    FindingTaxonomy = "PERF-P99-HIGH"
	TaxPerfDegradation FindingTaxonomy = "PERF-DEGRADATION"

	// Abuse
	TaxAbuseNoRateLimit FindingTaxonomy = "ABUSE-NO-RATE-LIMIT"
	TaxAbuseDOS         FindingTaxonomy = "ABUSE-DOS"

	// Logic
	TaxLogicIDOR          FindingTaxonomy = "LOGIC-IDOR"
	TaxLogicDuplicate     FindingTaxonomy = "LOGIC-DUPLICATE"
	TaxLogicStateInvalid  FindingTaxonomy = "LOGIC-STATE-INVALID"
	TaxLogicIdempotency   FindingTaxonomy = "LOGIC-IDEMPOTENCY"
	TaxLogicRaceCondition FindingTaxonomy = "LOGIC-RACE-CONDITION"

	// Auth
	TaxAuthASTDiscrepancy FindingTaxonomy = "AUTH-AST-DISCREPANCY"
	TaxAuthWeakMechanism  FindingTaxonomy = "AUTH-WEAK-MECHANISM"

	// Observability
	TaxObsNoCorrelationID FindingTaxonomy = "OBS-NO-CORRELATION-ID"
	TaxObsNoStructuredErr FindingTaxonomy = "OBS-NO-STRUCTURED-ERROR"
	TaxObsNoRequestID     FindingTaxonomy = "OBS-NO-REQUEST-ID"
)

// FindingSeverity is the impact level.
type FindingSeverity string

const (
	SeverityCritical FindingSeverity = "critical"
	SeverityHigh     FindingSeverity = "high"
	SeverityMedium   FindingSeverity = "medium"
	SeverityLow      FindingSeverity = "low"
	SeverityInfo     FindingSeverity = "info"
)

// FindingClassification is the confirmation status.
type FindingClassification string

const (
	ClassCandidate    FindingClassification = "candidate"
	ClassConfirmed    FindingClassification = "confirmed"
	ClassAnomaly      FindingClassification = "anomaly"
	ClassInconclusive FindingClassification = "inconclusive"
)

// FindingCategory groups findings by domain.
type FindingCategory string

const (
	FindCatAuth      FindingCategory = "auth"
	FindCatIDOR      FindingCategory = "idor"
	FindCatInjection FindingCategory = "injection"
	FindCatInfoLeak  FindingCategory = "info_leak"
	FindCatHeaders   FindingCategory = "headers"
	FindCatLogic     FindingCategory = "logic"
	FindCatRateLimit FindingCategory = "rate_limit"
	FindCatDOS       FindingCategory = "dos"
	FindCatContract  FindingCategory = "contract"
	FindCatRobust    FindingCategory = "robustness"
	FindCatPerf      FindingCategory = "performance"
	FindCatObs       FindingCategory = "observability"
	FindCatOther     FindingCategory = "other"
)

// FindingRaw is a candidate finding before LLM interpretation.
type FindingRaw struct {
	FindingID      string                `json:"finding_id"`
	TaxonomyID     FindingTaxonomy       `json:"taxonomy_id"`
	Severity       FindingSeverity       `json:"severity"`
	Category       FindingCategory       `json:"category"`
	EndpointID     string                `json:"endpoint_id,omitempty"`
	Title          string                `json:"title"`
	Description    string                `json:"description"`
	EvidenceRefs   []string              `json:"evidence_refs"`
	ASTRefs        []ASTRef              `json:"ast_refs,omitempty"`
	Confidence     float64               `json:"confidence"`
	Classification FindingClassification `json:"classification"`
}

// Confirmation is the result of replaying a finding.
type Confirmation struct {
	FindingID         string                `json:"finding_id"`
	OriginalEvidence  string                `json:"original_evidence_ref"`
	ReplayEvidence    string                `json:"replay_evidence_ref"`
	VariationEvidence string                `json:"variation_evidence_ref,omitempty"`
	Classification    FindingClassification `json:"classification"`
	Notes             string                `json:"notes,omitempty"`
}
