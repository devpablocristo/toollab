package domain

import "strconv"

// ScoreDimension names the 6 scoring dimensions.
type ScoreDimension string

const (
	DimSecurity      ScoreDimension = "security"
	DimAuth          ScoreDimension = "auth"
	DimContract      ScoreDimension = "contract"
	DimRobustness    ScoreDimension = "robustness"
	DimPerformance   ScoreDimension = "performance"
	DimObservability ScoreDimension = "observability"
)

// DimensionScore is a single 0–5 score with evidence.
type DimensionScore struct {
	Score        float64  `json:"score_0_to_5"`
	Rationale    string   `json:"rationale"`
	EvidenceRefs []string `json:"evidence_refs"`
}

// Grade maps a 0–5 score to a letter grade.
func Grade(score float64) string {
	switch {
	case score >= 4.5:
		return "A"
	case score >= 3.5:
		return "B"
	case score >= 2.5:
		return "C"
	case score >= 1.5:
		return "D"
	default:
		return "F"
	}
}

// EndpointHotspot is a high-risk endpoint.
type EndpointHotspot struct {
	EndpointID   string   `json:"endpoint_id"`
	Method       string   `json:"method"`
	Path         string   `json:"path"`
	RiskNotes    string   `json:"risk_notes"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

// ScoringInput is the data needed to compute scores.
type ScoringInput struct {
	RunMode       RunMode
	Catalog       *EndpointCatalog
	AuthMatrix    *AuthMatrix
	Contracts     []InferredContract
	Evidence      *EvidenceStore
	Findings      []FindingRaw
	Confirmations []Confirmation
	Signatures    []ErrorSignature
	SmokePassed   int
	SmokeFailed   int
}

// ScoringResult contains all dimension scores and hotspots.
// Auditable indicates if scores are meaningful given the run mode.
type ScoringResult struct {
	Auditable  bool                              `json:"auditable"`
	RunMode    RunMode                           `json:"run_mode"`
	Confidence string                            `json:"confidence"` // none, low, medium, high
	Scores     map[ScoreDimension]DimensionScore `json:"scores"`
	Hotspots   []EndpointHotspot                 `json:"endpoint_risk_hotspots"`
}

// ComputeScores applies the dimensional scoring rules respecting run mode.
func ComputeScores(in ScoringInput) ScoringResult {
	result := ScoringResult{
		RunMode: in.RunMode,
		Scores:  make(map[ScoreDimension]DimensionScore),
	}

	if in.RunMode == RunModeOffline {
		result.Auditable = false
		result.Confidence = "none"
		notAuditable := DimensionScore{
			Score:     -1,
			Rationale: "API not auditable: service was offline during this run. Scores cannot be computed without runtime evidence.",
		}
		result.Scores[DimSecurity] = notAuditable
		result.Scores[DimAuth] = notAuditable
		result.Scores[DimContract] = notAuditable
		result.Scores[DimRobustness] = notAuditable
		result.Scores[DimPerformance] = notAuditable
		result.Scores[DimObservability] = notAuditable
		return result
	}

	result.Auditable = true
	if in.RunMode == RunModeOnlinePartial {
		result.Confidence = "low"
	} else if in.RunMode == RunModeOnlineGood {
		result.Confidence = "medium"
	} else {
		result.Confidence = "high"
	}

	result.Scores[DimSecurity] = scoreSecurityDim(in)
	result.Scores[DimAuth] = scoreAuthDim(in)
	result.Scores[DimContract] = scoreContractDim(in)
	result.Scores[DimRobustness] = scoreRobustnessDim(in)
	result.Scores[DimPerformance] = scorePerformanceDim(in)
	result.Scores[DimObservability] = scoreObservabilityDim(in)
	result.Hotspots = computeHotspots(in)

	return result
}

func scoreSecurityDim(in ScoringInput) DimensionScore {
	score := 5.0
	var refs []string
	var reasons []string

	for _, f := range in.Findings {
		if f.Classification == ClassConfirmed || f.Classification == ClassCandidate {
			switch f.Category {
			case FindCatInjection:
				if f.Severity == SeverityCritical {
					score -= 2.5
				} else if f.Severity == SeverityHigh {
					score -= 1.5
				} else {
					score -= 0.5
				}
				reasons = append(reasons, f.Title)
				refs = appendRefs(refs, f.EvidenceRefs)
			case FindCatInfoLeak:
				score -= 1.0
				reasons = append(reasons, f.Title)
				refs = appendRefs(refs, f.EvidenceRefs)
			case FindCatHeaders:
				score -= 0.3
				refs = appendRefs(refs, f.EvidenceRefs)
			}
		}
	}

	for _, f := range in.Findings {
		if f.TaxonomyID == TaxSecHiddenCritical && f.Classification == ClassConfirmed {
			score -= 2.0
			reasons = append(reasons, "hidden critical path exposed")
			refs = appendRefs(refs, f.EvidenceRefs)
		}
	}

	if score < 0 {
		score = 0
	}
	rationale := "Evaluación de seguridad basada en findings confirmados."
	if len(reasons) > 0 {
		rationale = "Penalizaciones: " + joinMax(reasons, 5, "; ") + "."
	}
	if len(in.Findings) == 0 {
		rationale = "Sin findings de seguridad detectados en evidencia disponible."
	}

	return DimensionScore{Score: score, Rationale: rationale, EvidenceRefs: uniqueRefs(refs)}
}

func scoreAuthDim(in ScoringInput) DimensionScore {
	score := 5.0
	var refs []string
	var reasons []string

	if in.AuthMatrix != nil {
		for _, entry := range in.AuthMatrix.Entries {
			if entry.NoAuth == AuthAllowed && isWriteMethod(entry.Method) {
				score -= 1.5
				reasons = append(reasons, entry.Method+" "+entry.Path+" allows write without auth")
				refs = append(refs, entry.EvidenceRefs...)
			}
		}
		for _, d := range in.AuthMatrix.Discrepancies {
			score -= 1.0
			reasons = append(reasons, "AST↔runtime discrepancy: "+d.Description)
			refs = append(refs, d.EvidenceRefs...)
		}
	}

	for _, f := range in.Findings {
		if f.TaxonomyID == TaxSecAuthBypass && f.Classification == ClassConfirmed {
			score -= 2.0
			reasons = append(reasons, f.Title)
			refs = appendRefs(refs, f.EvidenceRefs)
		}
	}

	if score < 0 {
		score = 0
	}
	rationale := "Autenticación evaluada contra auth_matrix y findings."
	if len(reasons) > 0 {
		rationale = "Penalizaciones: " + joinMax(reasons, 5, "; ") + "."
	}

	return DimensionScore{Score: score, Rationale: rationale, EvidenceRefs: uniqueRefs(refs)}
}

func scoreContractDim(in ScoringInput) DimensionScore {
	score := 5.0
	var refs []string

	violations := 0
	for _, f := range in.Findings {
		if f.Category == FindCatContract {
			violations++
			switch f.Severity {
			case SeverityHigh:
				score -= 1.0
			case SeverityMedium:
				score -= 0.5
			default:
				score -= 0.2
			}
			refs = appendRefs(refs, f.EvidenceRefs)
		}
	}
	if score < 0 {
		score = 0
	}
	rationale := "Contratos API evaluados por status codes, content-types y consistencia de errores."
	if violations > 0 {
		rationale += " " + strconv.Itoa(violations) + " violaciones detectadas."
	}

	return DimensionScore{Score: score, Rationale: rationale, EvidenceRefs: uniqueRefs(refs)}
}

func scoreRobustnessDim(in ScoringInput) DimensionScore {
	score := 5.0
	var refs []string

	crashes := 0
	for _, f := range in.Findings {
		if f.Category == FindCatRobust {
			crashes++
			switch f.Severity {
			case SeverityCritical:
				score -= 2.0
			case SeverityHigh:
				score -= 1.0
			default:
				score -= 0.5
			}
			refs = appendRefs(refs, f.EvidenceRefs)
		}
	}

	for _, sig := range in.Signatures {
		if sig.Status >= 500 && sig.Count > 5 {
			score -= 0.5
			refs = append(refs, sig.SampleEvidenceRefs...)
		}
	}

	if score < 0 {
		score = 0
	}
	return DimensionScore{
		Score:        score,
		Rationale:    "Robustez evaluada por crashes, 5xx bajo malformed/boundary/content-type mismatch.",
		EvidenceRefs: uniqueRefs(refs),
	}
}

func scorePerformanceDim(in ScoringInput) DimensionScore {
	score := 5.0
	var refs []string

	for _, f := range in.Findings {
		if f.Category == FindCatPerf {
			score -= 1.0
			refs = appendRefs(refs, f.EvidenceRefs)
		}
	}
	if score < 0 {
		score = 0
	}
	return DimensionScore{
		Score:        score,
		Rationale:    "Performance evaluada por p95/p99 y degradación bajo carga.",
		EvidenceRefs: uniqueRefs(refs),
	}
}

func scoreObservabilityDim(in ScoringInput) DimensionScore {
	score := 5.0
	var refs []string

	for _, f := range in.Findings {
		if f.Category == FindCatObs {
			score -= 0.5
			refs = appendRefs(refs, f.EvidenceRefs)
		}
	}
	if score < 0 {
		score = 0
	}
	return DimensionScore{
		Score:        score,
		Rationale:    "Observabilidad evaluada por correlation IDs, request IDs y estructura de errores.",
		EvidenceRefs: uniqueRefs(refs),
	}
}

func computeHotspots(in ScoringInput) []EndpointHotspot {
	risk := make(map[string]float64)
	notes := make(map[string][]string)
	refs := make(map[string][]string)
	epInfo := make(map[string][2]string) // method, path

	for _, f := range in.Findings {
		if f.EndpointID == "" {
			continue
		}
		var w float64
		switch f.Severity {
		case SeverityCritical:
			w = 4
		case SeverityHigh:
			w = 2
		case SeverityMedium:
			w = 1
		default:
			w = 0.5
		}
		risk[f.EndpointID] += w
		notes[f.EndpointID] = append(notes[f.EndpointID], f.Title)
		refs[f.EndpointID] = appendRefs(refs[f.EndpointID], f.EvidenceRefs)
	}

	if in.Catalog != nil {
		for _, ep := range in.Catalog.Endpoints {
			epInfo[ep.EndpointID] = [2]string{ep.Method, ep.Path}
		}
	}

	type kv struct {
		id   string
		risk float64
	}
	var sorted []kv
	for id, r := range risk {
		sorted = append(sorted, kv{id, r})
	}
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].risk > sorted[i].risk {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	max := 10
	if len(sorted) < max {
		max = len(sorted)
	}

	out := make([]EndpointHotspot, 0, max)
	for _, kv := range sorted[:max] {
		info := epInfo[kv.id]
		out = append(out, EndpointHotspot{
			EndpointID:   kv.id,
			Method:       info[0],
			Path:         info[1],
			RiskNotes:    joinMax(notes[kv.id], 3, "; "),
			EvidenceRefs: uniqueRefs(refs[kv.id]),
		})
	}
	return out
}

func isWriteMethod(m string) bool {
	return m == "POST" || m == "PUT" || m == "PATCH" || m == "DELETE"
}

func appendRefs(existing, new []string) []string {
	return append(existing, new...)
}

func uniqueRefs(refs []string) []string {
	seen := make(map[string]bool, len(refs))
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
}

func joinMax(ss []string, max int, sep string) string {
	if len(ss) > max {
		ss = ss[:max]
	}
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
