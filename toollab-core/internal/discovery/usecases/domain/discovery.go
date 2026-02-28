package domain

import (
	"time"

	"toollab-core/internal/shared"
)

type Analyzer interface {
	Analyze(localPath string, hint FrameworkHint) (ServiceModel, ModelReport, error)
}

type FrameworkHint string

const (
	HintChi  FrameworkHint = "chi"
	HintGin  FrameworkHint = "gin"
	HintAuto FrameworkHint = "auto"
)

type ServiceModel struct {
	SchemaVersion string     `json:"schema_version"`
	Framework     string     `json:"framework"`
	RootPath      string     `json:"root_path"`
	Endpoints     []Endpoint `json:"endpoints"`
	CreatedAt     time.Time  `json:"created_at"`
}

type Endpoint struct {
	Method      string         `json:"method"`
	Path        string         `json:"path"`
	HandlerName string         `json:"handler_name,omitempty"`
	Ref         *shared.ModelRef `json:"ref,omitempty"`
}

type ModelReport struct {
	SchemaVersion  string   `json:"schema_version"`
	EndpointsCount int      `json:"endpoints_count"`
	Confidence     float64  `json:"confidence"`
	Gaps           []string `json:"gaps,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}
