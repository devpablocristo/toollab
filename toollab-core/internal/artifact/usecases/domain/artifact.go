package domain

import (
	"time"

	"toollab-core/internal/shared"
)

type Index struct {
	ID            string              `json:"id"`
	RunID         string              `json:"run_id"`
	Type          shared.ArtifactType `json:"type"`
	SchemaVersion string              `json:"schema_version"`
	Revision      int                 `json:"revision"`
	ContentHash   string              `json:"content_hash"`
	SizeBytes     int64               `json:"size_bytes"`
	StoragePath   string              `json:"storage_path"`
	CreatedAt     time.Time           `json:"created_at"`
}

type IndexRepository interface {
	Insert(idx Index) error
	GetLatest(runID string, artType shared.ArtifactType) (Index, error)
	GetByRevision(runID string, artType shared.ArtifactType, revision int) (Index, error)
	ListRevisions(runID string, artType shared.ArtifactType) ([]Index, error)
	ListByRun(runID string) ([]Index, error)
	NextRevision(runID string, artType shared.ArtifactType) (int, error)
}

type ContentStorage interface {
	Write(storagePath string, data []byte) error
	Read(storagePath string) ([]byte, error)
}
