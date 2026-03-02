package run

import (
	"toollab-core/internal/run/usecases/domain"
	"toollab-core/internal/shared"
	targetDomain "toollab-core/internal/target/usecases/domain"
)

type Service struct {
	repo       domain.Repository
	targetRepo targetDomain.Repository
}

func NewService(repo domain.Repository, targetRepo targetDomain.Repository) *Service {
	return &Service{repo: repo, targetRepo: targetRepo}
}

func (s *Service) Create(targetID, seed, notes string) (domain.Run, error) {
	if _, err := s.targetRepo.GetByID(targetID); err != nil {
		return domain.Run{}, err
	}
	run := domain.Run{
		ID:        shared.NewID(),
		TargetID:  targetID,
		Status:    domain.StatusCreated,
		Seed:      seed,
		Notes:     notes,
		CreatedAt: shared.Now(),
	}
	if err := s.repo.Insert(run); err != nil {
		return domain.Run{}, err
	}
	return run, nil
}

func (s *Service) Get(id string) (domain.Run, error) {
	return s.repo.GetByID(id)
}

func (s *Service) ListByTarget(targetID string) ([]domain.Run, error) {
	return s.repo.ListByTarget(targetID)
}

func (s *Service) LatestCompleted(targetID string) (domain.Run, error) {
	runs, err := s.repo.ListByTarget(targetID)
	if err != nil {
		return domain.Run{}, err
	}
	for _, r := range runs {
		if r.Status == domain.StatusCompleted {
			return r, nil
		}
	}
	return domain.Run{}, shared.ErrNotFound
}
