// Package target contains target domain services and adapters.
package target

import (
	"time"

	"github.com/devpablocristo/core/errors/go/domainerr"
	"github.com/google/uuid"

	"toollab-core/internal/target/usecases/domain"
)

type Service struct{ repo domain.Repository }

func NewService(repo domain.Repository) *Service { return &Service{repo: repo} }

func (s *Service) Create(name, description string, source domain.Source, hint domain.RuntimeHint) (domain.Target, error) {
	if name == "" {
		return domain.Target{}, domainerr.Validation("name is required")
	}
	if source.Value == "" {
		return domain.Target{}, domainerr.Validation("source.value is required")
	}
	now := time.Now().UTC()
	t := domain.Target{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Source:      source,
		RuntimeHint: hint,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.Insert(t); err != nil {
		return domain.Target{}, err
	}
	return t, nil
}

func (s *Service) Get(id string) (domain.Target, error) {
	return s.repo.GetByID(id)
}

func (s *Service) List() ([]domain.Target, error) {
	return s.repo.List()
}

func (s *Service) Delete(id string) error {
	if id == "" {
		return domainerr.Validation("id is required")
	}
	return s.repo.Delete(id)
}
