package shared

type ModelRef struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	File string `json:"file,omitempty"`
	Line int    `json:"line,omitempty"`
}

type EvidenceRef struct {
	EvidenceID string `json:"evidence_id"`
}
