// Package store maneja el almacenamiento en memoria de runs y artefactos v2.
package store

import (
	"errors"
	"sync"
	"time"

	"toollab-v2/internal/model"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	mu              sync.RWMutex
	runs            map[string]model.Run
	snapshots       map[string]model.RepoSnapshot
	services        map[string]model.ServiceModel
	summaries       map[string]model.Summary
	audits          map[string]model.AuditReport
	scenarios       map[string][]model.Scenario
	interpretations map[string]model.LLMInterpretation
	logs            map[string][]string
}

func New() *Store {
	return &Store{
		runs:            map[string]model.Run{},
		snapshots:       map[string]model.RepoSnapshot{},
		services:        map[string]model.ServiceModel{},
		summaries:       map[string]model.Summary{},
		audits:          map[string]model.AuditReport{},
		scenarios:       map[string][]model.Scenario{},
		interpretations: map[string]model.LLMInterpretation{},
		logs:            map[string][]string{},
	}
}

func (s *Store) InsertRun(run model.Run) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[run.ID] = run
}

func (s *Store) UpdateRun(id string, fn func(*model.Run)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[id]
	if !ok {
		return ErrNotFound
	}
	fn(&run)
	s.runs[id] = run
	return nil
}

func (s *Store) GetRun(id string) (model.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	run, ok := s.runs[id]
	if !ok {
		return model.Run{}, ErrNotFound
	}
	return run, nil
}

func (s *Store) ListRuns() []model.Run {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Run, 0, len(s.runs))
	for _, run := range s.runs {
		out = append(out, run)
	}
	return out
}

func (s *Store) DeleteRun(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.runs[id]; !ok {
		return ErrNotFound
	}
	delete(s.runs, id)
	delete(s.snapshots, id)
	delete(s.services, id)
	delete(s.summaries, id)
	delete(s.audits, id)
	delete(s.scenarios, id)
	delete(s.interpretations, id)
	delete(s.logs, id)
	return nil
}

func (s *Store) SaveSnapshot(runID string, snap model.RepoSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots[runID] = snap
}

func (s *Store) GetSnapshot(runID string) (model.RepoSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.snapshots[runID]
	if !ok {
		return model.RepoSnapshot{}, ErrNotFound
	}
	return v, nil
}

func (s *Store) SaveServiceModel(runID string, m model.ServiceModel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.services[runID] = m
}

func (s *Store) GetServiceModel(runID string) (model.ServiceModel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.services[runID]
	if !ok {
		return model.ServiceModel{}, ErrNotFound
	}
	return v, nil
}

func (s *Store) SaveSummary(runID string, v model.Summary) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.summaries[runID] = v
}

func (s *Store) GetSummary(runID string) (model.Summary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.summaries[runID]
	if !ok {
		return model.Summary{}, ErrNotFound
	}
	return v, nil
}

func (s *Store) SaveAudit(runID string, v model.AuditReport) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.audits[runID] = v
}

func (s *Store) GetAudit(runID string) (model.AuditReport, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.audits[runID]
	if !ok {
		return model.AuditReport{}, ErrNotFound
	}
	return v, nil
}

func (s *Store) SaveScenarios(runID string, v []model.Scenario) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scenarios[runID] = v
}

func (s *Store) GetScenarios(runID string) ([]model.Scenario, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.scenarios[runID]
	if !ok {
		return nil, ErrNotFound
	}
	return v, nil
}

func (s *Store) SaveInterpretation(runID string, v model.LLMInterpretation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.interpretations[runID] = v
}

func (s *Store) GetInterpretation(runID string) (model.LLMInterpretation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.interpretations[runID]
	if !ok {
		return model.LLMInterpretation{}, ErrNotFound
	}
	return v, nil
}

func (s *Store) AppendLog(runID, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	line := time.Now().Format(time.RFC3339) + " " + msg
	s.logs[runID] = append(s.logs[runID], line)
}

func (s *Store) GetLogs(runID string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string(nil), s.logs[runID]...)
}
