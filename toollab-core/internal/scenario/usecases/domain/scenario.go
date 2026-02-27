package domain

import "encoding/json"

type ScenarioPlan struct {
	PlanID        string         `json:"plan_id"`
	RunID         string         `json:"run_id"`
	SchemaVersion string         `json:"schema_version"`
	Cases         []ScenarioCase `json:"cases"`
}

type ScenarioCase struct {
	CaseID  string   `json:"case_id"`
	Name    string   `json:"name"`
	Enabled bool     `json:"enabled"`
	Tags    []string `json:"tags,omitempty"`
	Request CaseRequest `json:"request"`
	Auth    *CaseAuth   `json:"auth,omitempty"`
	Expect  *CaseExpect `json:"expect,omitempty"`
}

type CaseRequest struct {
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	PathParams map[string]string `json:"path_params,omitempty"`
	Query      map[string]string `json:"query,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	BodyJSON   json.RawMessage   `json:"body_json,omitempty"`
}

type CaseAuth struct {
	Mode       string `json:"mode"` // none, bearer_token, api_key
	Token      string `json:"token,omitempty"`
	HeaderName string `json:"header_name,omitempty"`
	Value      string `json:"value,omitempty"`
}

type CaseExpect struct {
	StatusIn []int `json:"status_in,omitempty"`
}
