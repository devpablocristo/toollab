package explain

type Understanding struct {
	SchemaVersion  int         `json:"schema_version"`
	RunRef         RunRef      `json:"run_ref"`
	Sections       Sections    `json:"sections"`
	Claims         []Claim     `json:"claims"`
	Unknowns       []string    `json:"unknowns"`
	Anchors        []Anchor    `json:"anchors"`
	Determinism    Determinism `json:"determinism"`
	GeneratedAtUTC string      `json:"generated_at_utc,omitempty"`
}

type RunRef struct {
	RunID        string `json:"run_id"`
	EvidencePath string `json:"evidence_path"`
}

type Sections struct {
	WhatIs         Section `json:"what_is"`
	HowToUse       Section `json:"how_to_use"`
	WhatWasTested  Section `json:"what_was_tested"`
	WhatHappened   Section `json:"what_happened"`
	WhatFailed     Section `json:"what_failed"`
	WhatIsProven   Section `json:"what_is_proven"`
	WhatIsUnknown  Section `json:"what_is_unknown"`
	HowToReproduce Section `json:"how_to_reproduce"`
}

type Section struct {
	Summary string   `json:"summary"`
	Anchors []Anchor `json:"anchors"`
}

type Claim struct {
	Statement       string   `json:"statement"`
	Status          string   `json:"status"`
	Anchors         []Anchor `json:"anchors"`
	MissingEvidence []string `json:"missing_evidence"`
}

type Anchor struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type Determinism struct {
	CanonicalWriterVersion   string `json:"canonical_writer_version"`
	UnderstandingFingerprint string `json:"understanding_fingerprint"`
	NarrativeOnly            bool   `json:"narrative_only,omitempty"`
	LLMInputSHA256           string `json:"llm_input_sha256,omitempty"`
}
