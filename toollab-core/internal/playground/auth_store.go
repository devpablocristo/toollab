package playground

import (
	"sync"

	d "toollab-core/internal/domain"
)

// AuthStore is a simple in-memory store for auth profiles.
// In production this would be encrypted storage; for now it's per-process.
type AuthStore struct {
	mu       sync.RWMutex
	profiles map[string][]d.AuthProfile // keyed by runID
}

func NewAuthStore() *AuthStore {
	return &AuthStore{profiles: make(map[string][]d.AuthProfile)}
}

func (s *AuthStore) Put(p d.AuthProfile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.profiles[p.RunID] = append(s.profiles[p.RunID], p)
}

func (s *AuthStore) Get(runID, profileID string) (d.AuthProfile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.profiles[runID] {
		if p.ID == profileID {
			return p, true
		}
	}
	return d.AuthProfile{}, false
}

func (s *AuthStore) List(runID string) []d.AuthProfile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.profiles[runID]
}

func (s *AuthStore) Delete(runID, profileID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	profiles := s.profiles[runID]
	for i, p := range profiles {
		if p.ID == profileID {
			s.profiles[runID] = append(profiles[:i], profiles[i+1:]...)
			return
		}
	}
}
