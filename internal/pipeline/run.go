package pipeline

import (
	"lp/internal/logging"
	"lp/internal/observability"
	"sync"
	"time"
)

// Status - Lifecycle state of a Run.
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

// Run - A single execution of a Pipeline. Each run has its own UUID,
// captured step results, a dedicated Ledger for grouped logs, and an
// observability Collector for per-run metrics.
type Run struct {
	mu         sync.RWMutex
	ID         string
	PipelineID string
	Status     Status
	StartedAt  *time.Time
	FinishedAt *time.Time
	Error      string
	Steps      []StepResult
	Ledger     *logging.Ledger
	Collector  *observability.Collector
}

// NewRun - Creates a pending Run for the given pipeline with its own
// Ledger and Collector for isolated per-run observability.
func NewRun(pipelineID string, stepCount int) *Run {
	ledger := logging.NewLedger()
	logger := observability.NewLogger("run")
	collector := observability.NewCollector(logger)

	steps := make([]StepResult, stepCount)
	for i := range steps {
		steps[i].Status = StatusPending
	}

	return &Run{
		ID:         newID(),
		PipelineID: pipelineID,
		Status:     StatusPending,
		Steps:      steps,
		Ledger:     ledger,
		Collector:  collector,
	}
}

// Start - Marks the run as running and records the start time.
func (r *Run) Start() {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	r.Status = StatusRunning
	r.StartedAt = &now
	r.Ledger.Info(logging.CatPipeline, "run", "run started", map[string]any{
		"run_id":      r.ID,
		"pipeline_id": r.PipelineID,
	})
}

// StartStep - Marks a step as running within this run.
func (r *Run) StartStep(index int, name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if index < len(r.Steps) {
		r.Steps[index].Name = name
		r.Steps[index].Status = StatusRunning
	}
	r.Ledger.Info(logging.CatStep, "run", "step started", map[string]any{
		"run_id": r.ID,
		"step":   name,
		"index":  index,
	})
}

// CompleteStep - Records a successful step result.
func (r *Run) CompleteStep(index int, output string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if index < len(r.Steps) {
		r.Steps[index].Status = StatusCompleted
		r.Steps[index].Output.Stdout = output
	}
	r.Ledger.Info(logging.CatStep, "run", "step completed", map[string]any{
		"run_id": r.ID,
		"step":   r.Steps[index].Name,
		"index":  index,
	})
}

// FailStep - Records a failed step result.
func (r *Run) FailStep(index int, output string, err string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if index < len(r.Steps) {
		r.Steps[index].Status = StatusFailed
		r.Steps[index].Output.Stderr = output
		r.Steps[index].Error = err
	}
	r.Ledger.Error(logging.CatStep, "run", "step failed", map[string]any{
		"run_id": r.ID,
		"step":   r.Steps[index].Name,
		"index":  index,
		"error":  err,
	})
}

// Finish - Marks the run as completed or failed and records the finish time.
func (r *Run) Finish(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	r.FinishedAt = &now
	if err != nil {
		r.Status = StatusFailed
		r.Error = err.Error()
		r.Ledger.Error(logging.CatPipeline, "run", "run failed", map[string]any{
			"run_id":      r.ID,
			"pipeline_id": r.PipelineID,
			"error":       err.Error(),
		})
	} else {
		r.Status = StatusCompleted
		r.Ledger.Info(logging.CatPipeline, "run", "run completed", map[string]any{
			"run_id":      r.ID,
			"pipeline_id": r.PipelineID,
		})
	}
}

// Duration - Returns the run duration, or zero if not yet started.
func (r *Run) Duration() time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.StartedAt == nil {
		return 0
	}
	end := time.Now()
	if r.FinishedAt != nil {
		end = *r.FinishedAt
	}
	return end.Sub(*r.StartedAt)
}

