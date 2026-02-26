package discovery

import (
	"encoding/json"
)

type AuthConfig struct {
	Kind       string
	EnvVar     string
	Location   string
	Name       string
	HeaderName string
}

type HTTPConfig struct {
	TimeoutMS int
	MaxBytes  int64
}

type FetchInfo struct {
	Source string `json:"source"`
	URL    string `json:"url,omitempty"`
	File   string `json:"file,omitempty"`
	Hash   string `json:"sha256"`
}

type Manifest struct {
	AdapterVersion  string            `json:"adapter_version"`
	StandardVersion string            `json:"standard_version,omitempty"`
	AppName         string            `json:"app_name"`
	AppVersion      string            `json:"app_version"`
	Capabilities    []string          `json:"capabilities"`
	Links           map[string]string `json:"links,omitempty"`
}

type Profile struct {
	StandardVersion string          `json:"standard_version"`
	ProfileVersion  string          `json:"profile_version"`
	Manifest        *Manifest       `json:"manifest,omitempty"`
	Schema          json.RawMessage `json:"schema,omitempty"`
	SuggestedFlows  json.RawMessage `json:"suggested_flows,omitempty"`
	Invariants      json.RawMessage `json:"invariants,omitempty"`
	Limits          json.RawMessage `json:"limits,omitempty"`
	Environment     json.RawMessage `json:"environment,omitempty"`
	OpenAPI         json.RawMessage `json:"openapi,omitempty"`
	Unknowns        []string        `json:"unknowns,omitempty"`
}
