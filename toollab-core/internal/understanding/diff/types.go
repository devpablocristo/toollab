package diff

type Diff struct {
	SchemaVersion  int             `json:"schema_version"`
	RunA           RunRef          `json:"run_a"`
	RunB           RunRef          `json:"run_b"`
	ScenarioDelta  ScenarioDelta   `json:"scenario_delta"`
	StatsDelta     StatsDelta      `json:"stats_delta"`
	EndpointDelta  []EndpointDelta `json:"endpoint_delta"`
	InvariantDelta InvariantDelta  `json:"invariant_delta"`
	DiscoveryDelta DiscoveryDelta  `json:"discovery_delta"`
	Unknowns       []string        `json:"unknowns"`
	Anchors        []Anchor        `json:"anchors"`
	Determinism    Determinism     `json:"determinism"`
	GeneratedAtUTC string          `json:"generated_at_utc,omitempty"`
}

type RunRef struct {
	RunID        string `json:"run_id"`
	EvidencePath string `json:"evidence_path"`
}

type ScenarioDelta struct {
	Changed      bool   `json:"changed"`
	ScenarioSHAA string `json:"scenario_sha_a"`
	ScenarioSHAB string `json:"scenario_sha_b"`
}

type StatsDelta struct {
	P50MS     int     `json:"p50_ms"`
	P95MS     int     `json:"p95_ms"`
	P99MS     int     `json:"p99_ms"`
	ErrorRate float64 `json:"error_rate"`
}

type EndpointDelta struct {
	Key            string  `json:"key"`
	LatencyDeltaMS int     `json:"latency_delta_ms"`
	ErrorRateDelta float64 `json:"error_rate_delta"`
}

type InvariantDelta struct {
	Changed            bool     `json:"changed"`
	NewViolations      []string `json:"new_violations"`
	ResolvedViolations []string `json:"resolved_violations"`
}

type DiscoveryDelta struct {
	ProfileHashChanged bool `json:"profile_hash_changed,omitempty"`
	SchemaHashChanged  bool `json:"schema_hash_changed,omitempty"`
}

type Anchor struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type Determinism struct {
	CanonicalWriterVersion string `json:"canonical_writer_version"`
	DiffFingerprint        string `json:"diff_fingerprint"`
}
