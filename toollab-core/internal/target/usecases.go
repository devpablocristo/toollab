// Package target contains target domain services and adapters.
package target

import (
	"fmt"

	"toollab-core/internal/shared"
	"toollab-core/internal/target/usecases/domain"
)

type Service struct{ repo domain.Repository }

func NewService(repo domain.Repository) *Service { return &Service{repo: repo} }

func (s *Service) Create(name, description string, source domain.Source, hint domain.RuntimeHint) (domain.Target, error) {
	if name == "" {
		return domain.Target{}, fmt.Errorf("%w: name is required", shared.ErrInvalidInput)
	}
	if source.Value == "" {
		return domain.Target{}, fmt.Errorf("%w: source.value is required", shared.ErrInvalidInput)
	}
	now := shared.Now()
	t := domain.Target{
		ID:          shared.NewID(),
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
		return fmt.Errorf("%w: id is required", shared.ErrInvalidInput)
	}
	return s.repo.Delete(id)
}
