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
	Description     json.RawMessage `json:"description,omitempty"`
	Unknowns        []string        `json:"unknowns,omitempty"`
}

// ServiceDescription mirrors the adapter-side description for typed access.
type ServiceDescription struct {
	Purpose              string                `json:"purpose"`
	Domain               string                `json:"domain"`
	Consumers            string                `json:"consumers,omitempty"`
	Models               []ModelDescription    `json:"models,omitempty"`
	EndpointDescriptions []EndpointDescription `json:"endpoints,omitempty"`
	Dependencies         []DependencyInfo      `json:"dependencies,omitempty"`
}

type ModelDescription struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Fields      []FieldDescription `json:"fields,omitempty"`
	Relations   []RelationInfo     `json:"relations,omitempty"`
}

type FieldDescription struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description,omitempty"`
	Example     string `json:"example,omitempty"`
}

type RelationInfo struct {
	Target      string `json:"target"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

type EndpointDescription struct {
	Method          string      `json:"method"`
	Path            string      `json:"path"`
	Summary         string      `json:"summary"`
	Description     string      `json:"description,omitempty"`
	Category        string      `json:"category,omitempty"`
	RequestExample  string      `json:"request_example,omitempty"`
	ResponseExample string      `json:"response_example,omitempty"`
	ErrorCodes      []ErrorCode `json:"error_codes,omitempty"`
	RequiresAuth    bool        `json:"requires_auth"`
}

type ErrorCode struct {
	Status      int    `json:"status"`
	Description string `json:"description"`
}

type DependencyInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
}
