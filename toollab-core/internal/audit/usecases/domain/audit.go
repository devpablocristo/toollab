package domain

type Auditor interface {
	Audit(runID string) (AuditReport, error)
}

type AuditReport struct {
	Findings []Finding `json:"findings"`
}

type Finding struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Description string `json:"description"`
	EvidenceRef string `json:"evidence_ref,omitempty"`
}
