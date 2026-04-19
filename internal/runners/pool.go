package runners

import (
	"context"
	"fmt"
	"sync"
)

// Pool manages concurrent pipeline runners with configurable limits.
type Pool struct {
	cfg       Config
	sem       chan struct{}
	mu        sync.Mutex
	wg        sync.WaitGroup
	Hooks     Hooks
	EventSink chan<- Event
}

// Event - An event emitted by a pipeline or step during execution.
type Event struct {
	// Source is the pipeline that emitted this event.
	Source string
	// Trigger is the event name.
	Trigger string
	// Pipeline is the target pipeline to execute.
	Pipeline string
}

// NewPool - Creates a runner pool from the given configuration.
func NewPool(cfg Config) *Pool {
	return &Pool{
		cfg:   cfg,
		sem:   make(chan struct{}, cfg.MaxConcurrent),
		Hooks: NewHooks(),
	}
}

// Submit - Schedules a pipeline for execution. It blocks if MaxConcurrent runners are already active.
func (p *Pool) Submit(ctx context.Context, pipeline *Pipeline) error {
	p.wg.Add(1)

	select {
	case p.sem <- struct{}{}:
	case <-ctx.Done():
		p.wg.Done()
		return ctx.Err()
	}

	go func() {
		defer p.wg.Done()
		defer func() { <-p.sem }()
		_ = p.run(ctx, pipeline)
	}()

	return nil
}

// Wait - Blocks until all submitted pipelines have finished.
func (p *Pool) Wait() {
	p.wg.Wait()
}

// run - Executes a pipeline's steps sequentially within a timeout context.
func (p *Pool) run(ctx context.Context, pipeline *Pipeline) error {
	ctx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer cancel()

	p.Hooks.emitPipelineStarted(pipeline.Name)

	var results []Result
	for i, step := range pipeline.Steps {
		p.Hooks.emitStepStarted(pipeline.Name, step.Name)

		result, err := step.Execute(ctx)
		if err != nil {
			wrapped := fmt.Errorf("pipeline %q step %d (%s): %w", pipeline.Name, i, step.Name, err)
			p.Hooks.emitStepFailed(pipeline.Name, step.Name, wrapped)
			p.Hooks.emitPipelineFailed(pipeline.Name, wrapped)
			return wrapped
		}

		results = append(results, result)
		p.Hooks.emitStepCompleted(pipeline.Name, step.Name, result)

		p.emitEvents(pipeline.Name, step.Emit)
	}

	p.Hooks.emitPipelineCompleted(pipeline.Name, results)
	p.emitEvents(pipeline.Name, pipeline.Emit)
	return nil
}

// emitEvents - Sends declared events to the EventSink if one is configured.
func (p *Pool) emitEvents(source string, defs []EmitDef) {
	if p.EventSink == nil {
		return
	}
	for _, d := range defs {
		p.EventSink <- Event{
			Source:   source,
			Trigger:  d.Event,
			Pipeline: d.Pipeline,
		}
	}
}

