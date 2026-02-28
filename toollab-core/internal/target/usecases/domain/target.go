package domain

import "time"

type SourceType string

const (
	SourceRepoURL SourceType = "repo_url"
	SourcePath    SourceType = "path"
	SourceZip     SourceType = "zip"
)

type Source struct {
	Type  SourceType `json:"type"`
	Value string     `json:"value"`
}

type RuntimeHint struct {
	BaseURL           string            `json:"base_url,omitempty"`
	DockerComposePath string            `json:"docker_compose_path,omitempty"`
	Cmd               string            `json:"cmd,omitempty"`
	AuthHeaders       map[string]string `json:"auth_headers,omitempty"`
}

type Target struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Source      Source      `json:"source"`
	RuntimeHint RuntimeHint `json:"runtime_hint"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

type Repository interface {
	Insert(t Target) error
	GetByID(id string) (Target, error)
	List() ([]Target, error)
	Delete(id string) error
}
