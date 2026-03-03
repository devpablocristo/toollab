// Package artifact contains artifact services, storage, and indexing adapters.
package artifact

import (
	"encoding/json"
	"fmt"

	"toollab-core/internal/artifact/usecases/domain"
	runDomain "toollab-core/internal/run/usecases/domain"
	"toollab-core/internal/shared"
)

const maxArtifactSize = 10 * 1024 * 1024

type Service struct {
	indexRepo domain.IndexRepository
	storage   domain.ContentStorage
	runRepo   runDomain.Repository
}

func NewService(indexRepo domain.IndexRepository, storage domain.ContentStorage, runRepo runDomain.Repository) *Service {
	return &Service{indexRepo: indexRepo, storage: storage, runRepo: runRepo}
}

type PutResult struct {
	RunID         string              `json:"run_id"`
	Type          shared.ArtifactType `json:"type"`
	SchemaVersion string              `json:"schema_version"`
	Revision      int                 `json:"revision"`
	ContentHash   string              `json:"content_hash"`
	SizeBytes     int64               `json:"size_bytes"`
	CreatedAt     string              `json:"created_at"`
}

func (s *Service) Put(runID string, artType shared.ArtifactType, rawJSON []byte) (PutResult, error) {
	if !json.Valid(rawJSON) {
		return PutResult{}, fmt.Errorf("%w: body is not valid JSON", shared.ErrInvalidInput)
	}
	return s.putBytes(runID, artType, rawJSON)
}

// PutRaw stores any byte payload (CSV, YAML, plain text, etc.) without JSON validation.
func (s *Service) PutRaw(runID string, artType shared.ArtifactType, data []byte) (PutResult, error) {
	return s.putBytes(runID, artType, data)
}

func (s *Service) putBytes(runID string, artType shared.ArtifactType, data []byte) (PutResult, error) {
	if _, err := s.runRepo.GetByID(runID); err != nil {
		return PutResult{}, err
	}
	if !artType.Valid() {
		return PutResult{}, fmt.Errorf("%w: invalid artifact type %q", shared.ErrInvalidInput, artType)
	}
	if int64(len(data)) > maxArtifactSize {
		return PutResult{}, fmt.Errorf("%w: artifact exceeds max size (%d bytes)", shared.ErrInvalidInput, maxArtifactSize)
	}

	rev, err := s.indexRepo.NextRevision(runID, artType)
	if err != nil {
		return PutResult{}, err
	}

	storagePath := StoragePath(runID, string(artType), rev)
	contentHash := shared.SHA256Bytes(data)
	now := shared.Now()

	idx := domain.Index{
		ID:            shared.NewID(),
		RunID:         runID,
		Type:          artType,
		SchemaVersion: "v1",
		Revision:      rev,
		ContentHash:   contentHash,
		SizeBytes:     int64(len(data)),
		StoragePath:   storagePath,
		CreatedAt:     now,
	}

	if err := s.storage.Write(storagePath, data); err != nil {
		return PutResult{}, fmt.Errorf("writing artifact: %w", err)
	}
	if err := s.indexRepo.Insert(idx); err != nil {
		return PutResult{}, fmt.Errorf("inserting index: %w", err)
	}

	return PutResult{
		RunID:         runID,
		Type:          artType,
		SchemaVersion: idx.SchemaVersion,
		Revision:      rev,
		ContentHash:   contentHash,
		SizeBytes:     idx.SizeBytes,
		CreatedAt:     now.Format(shared.TimeFormat),
	}, nil
}

func (s *Service) GetLatest(runID string, artType shared.ArtifactType) ([]byte, domain.Index, error) {
	idx, err := s.indexRepo.GetLatest(runID, artType)
	if err != nil {
		return nil, domain.Index{}, err
	}
	data, err := s.storage.Read(idx.StoragePath)
	return data, idx, err
}

func (s *Service) GetByRevision(runID string, artType shared.ArtifactType, revision int) ([]byte, domain.Index, error) {
	idx, err := s.indexRepo.GetByRevision(runID, artType, revision)
	if err != nil {
		return nil, domain.Index{}, err
	}
	data, err := s.storage.Read(idx.StoragePath)
	return data, idx, err
}

func (s *Service) GetLatestMeta(runID string, artType shared.ArtifactType) (domain.Index, error) {
	return s.indexRepo.GetLatest(runID, artType)
}

func (s *Service) GetRevisionMeta(runID string, artType shared.ArtifactType, revision int) (domain.Index, error) {
	return s.indexRepo.GetByRevision(runID, artType, revision)
}

func (s *Service) ListRevisions(runID string, artType shared.ArtifactType) ([]domain.Index, error) {
	return s.indexRepo.ListRevisions(runID, artType)
}

func (s *Service) ListByRun(runID string) ([]domain.Index, error) {
	return s.indexRepo.ListByRun(runID)
}
