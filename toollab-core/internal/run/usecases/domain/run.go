package domain

import "time"

type Status string

const (
	StatusCreated   Status = "created"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

type Run struct {
	ID          string     `json:"id"`
	TargetID    string     `json:"target_id"`
	Status      Status     `json:"status"`
	Seed        string     `json:"seed,omitempty"`
	Notes       string     `json:"notes,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type Repository interface {
	Insert(r Run) error
	GetByID(id string) (Run, error)
	ListByTarget(targetID string) ([]Run, error)
	UpdateStatus(id string, status Status) error
	UpdateStatusCompleted(id string, status Status, completedAt time.Time) error
}
