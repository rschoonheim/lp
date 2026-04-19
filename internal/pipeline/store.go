package pipeline

import (
	"fmt"
	"sync"
)

// Store - Thread-safe in-memory store for pipelines and their runs.
type Store struct {
	mu        sync.RWMutex
	pipelines map[string]*Pipeline
	runs      map[string]*Run
	byPipeline map[string][]string // pipeline ID → run IDs
}

// NewStore - Creates an empty Store.
func NewStore() *Store {
	return &Store{
		pipelines:  make(map[string]*Pipeline),
		runs:       make(map[string]*Run),
		byPipeline: make(map[string][]string),
	}
}

// AddPipeline - Registers a pipeline in the store.
func (s *Store) AddPipeline(p *Pipeline) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pipelines[p.ID] = p
}

// GetPipeline - Returns a pipeline by ID or an error if not found.
func (s *Store) GetPipeline(id string) (*Pipeline, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.pipelines[id]
	if !ok {
		return nil, fmt.Errorf("pipeline %q: %w", id, ErrNotFound)
	}
	return p, nil
}

// GetPipelineByName - Returns the first pipeline matching the given name or an error if not found.
func (s *Store) GetPipelineByName(name string) (*Pipeline, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.pipelines {
		if p.Name == name {
			return p, nil
		}
	}
	return nil, fmt.Errorf("pipeline name %q: %w", name, ErrNotFound)
}

// Pipelines - Returns all registered pipelines.
func (s *Store) Pipelines() []*Pipeline {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Pipeline, 0, len(s.pipelines))
	for _, p := range s.pipelines {
		out = append(out, p)
	}
	return out
}

// ReplacePipelines - Atomically replaces all pipelines in the store.
// Existing runs are preserved; run references to removed pipelines remain queryable.
func (s *Store) ReplacePipelines(newPipelines []*Pipeline) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Build a name→old-ID map so we can migrate run history to updated pipelines
	oldByName := make(map[string]string)
	for _, p := range s.pipelines {
		oldByName[p.Name] = p.ID
	}

	fresh := make(map[string]*Pipeline, len(newPipelines))
	for _, p := range newPipelines {
		fresh[p.ID] = p

		// If a pipeline with the same name existed, migrate its run history
		if oldID, ok := oldByName[p.Name]; ok && oldID != p.ID {
			if runIDs, exists := s.byPipeline[oldID]; exists {
				s.byPipeline[p.ID] = append(s.byPipeline[p.ID], runIDs...)
				delete(s.byPipeline, oldID)
				// Update run references to point to the new pipeline ID
				for _, rid := range runIDs {
					if r, ok := s.runs[rid]; ok {
						r.PipelineID = p.ID
					}
				}
			}
		}
	}

	s.pipelines = fresh
}

// CreateRun - Creates a new Run for the given pipeline ID and stores it.
func (s *Store) CreateRun(pipelineID string) (*Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.pipelines[pipelineID]
	if !ok {
		return nil, fmt.Errorf("pipeline %q: %w", pipelineID, ErrNotFound)
	}
	run := NewRun(pipelineID, len(p.Steps))
	s.runs[run.ID] = run
	s.byPipeline[pipelineID] = append(s.byPipeline[pipelineID], run.ID)
	return run, nil
}

// GetRun - Returns a run by ID or an error if not found.
func (s *Store) GetRun(id string) (*Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.runs[id]
	if !ok {
		return nil, fmt.Errorf("run %q: %w", id, ErrNotFound)
	}
	return r, nil
}

// RunsFor - Returns all runs for a pipeline in creation order.
func (s *Store) RunsFor(pipelineID string) []*Run {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.byPipeline[pipelineID]
	out := make([]*Run, 0, len(ids))
	for _, id := range ids {
		if r, ok := s.runs[id]; ok {
			out = append(out, r)
		}
	}
	return out
}

// ErrNotFound - Sentinel error for missing pipelines or runs.
var ErrNotFound = fmt.Errorf("not found")

