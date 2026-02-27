package mapmodel

type SystemMap struct {
	SchemaVersion   int              `json:"schema_version"`
	ServiceIdentity ServiceIdentity  `json:"service_identity"`
	Resources       []Resource       `json:"resources"`
	Endpoints       []Endpoint       `json:"endpoints"`
	Flows           []Flow           `json:"flows"`
	Invariants      []map[string]any `json:"invariants"`
	Limits          map[string]any   `json:"limits"`
	Unknowns        []string         `json:"unknowns"`
	Anchors         []Anchor         `json:"anchors"`
	Partial         bool             `json:"partial"`
	Determinism     Determinism      `json:"determinism"`
	GeneratedAtUTC  string           `json:"generated_at_utc,omitempty"`
}

type ServiceIdentity struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Environment string `json:"environment,omitempty"`
}

type Resource struct {
	Name string   `json:"name"`
	Kind string   `json:"kind,omitempty"`
	Tags []string `json:"tags,omitempty"`
}

type Endpoint struct {
	Method       string   `json:"method"`
	Path         string   `json:"path"`
	ContentTypes []string `json:"content_types,omitempty"`
	Tags         []string `json:"tags,omitempty"`
}

type Flow struct {
	ID          string        `json:"id"`
	Description string        `json:"description,omitempty"`
	Requests    []FlowRequest `json:"requests"`
}

type FlowRequest struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

type Anchor struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type Determinism struct {
	CanonicalWriterVersion string `json:"canonical_writer_version"`
	SystemMapFingerprint   string `json:"system_map_fingerprint"`
}
