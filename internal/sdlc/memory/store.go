package memory

import (
	"context"
	"errors"
	"sync"

	"agenticsdlc/internal/sdlc"
)

var ErrRunNotFound = errors.New("workflow run not found")

type Store struct {
	mu   sync.RWMutex
	runs map[string]sdlc.WorkflowRun
}

func NewStore() *Store {
	return &Store{runs: map[string]sdlc.WorkflowRun{}}
}

func (s *Store) CreateRun(_ context.Context, run sdlc.WorkflowRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[run.WorkflowID] = run
	return nil
}

func (s *Store) UpdateRun(_ context.Context, run sdlc.WorkflowRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.runs[run.WorkflowID]; !ok {
		return ErrRunNotFound
	}
	s.runs[run.WorkflowID] = run
	return nil
}

func (s *Store) FindRun(_ context.Context, workflowID string) (sdlc.WorkflowRun, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	run, ok := s.runs[workflowID]
	if !ok {
		return sdlc.WorkflowRun{}, ErrRunNotFound
	}
	return run, nil
}
