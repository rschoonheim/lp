package runners

import (
	"lp/internal/hooks"
	"lp/internal/observability"
)

// Hooks contains callback registries that are invoked at specific points
// during pipeline and step execution. All callbacks are optional.
// Hooks are safe for concurrent use.
type Hooks struct {
	onPipelineStarted   hooks.Registry[func(pipeline string)]
	onPipelineCompleted hooks.Registry[func(pipeline string, results []Result)]
	onPipelineFailed    hooks.Registry[func(pipeline string, err error)]

	onStepStarted   hooks.Registry[func(pipeline, step string)]
	onStepCompleted hooks.Registry[func(pipeline, step string, result Result)]
	onStepFailed    hooks.Registry[func(pipeline, step string, err error)]

	trackers []*observability.HookTracker
}

// NewHooks - Creates a Hooks instance with named registries and observability trackers.
func NewHooks() Hooks {
	trackers := []*observability.HookTracker{
		observability.NewHookTracker("pipeline_started"),
		observability.NewHookTracker("pipeline_completed"),
		observability.NewHookTracker("pipeline_failed"),
		observability.NewHookTracker("step_started"),
		observability.NewHookTracker("step_completed"),
		observability.NewHookTracker("step_failed"),
	}
	return Hooks{
		onPipelineStarted:   hooks.NewRegistry[func(pipeline string)]("pipeline_started"),
		onPipelineCompleted: hooks.NewRegistry[func(pipeline string, results []Result)]("pipeline_completed"),
		onPipelineFailed:    hooks.NewRegistry[func(pipeline string, err error)]("pipeline_failed"),
		onStepStarted:       hooks.NewRegistry[func(pipeline, step string)]("step_started"),
		onStepCompleted:     hooks.NewRegistry[func(pipeline, step string, result Result)]("step_completed"),
		onStepFailed:        hooks.NewRegistry[func(pipeline, step string, err error)]("step_failed"),
		trackers:            trackers,
	}
}

// SetPanicHandler - Configures a panic handler on all hook trackers.
func (h *Hooks) SetPanicHandler(fn func(name string, recovered any)) {
	for _, t := range h.trackers {
		t.SetPanicHandler(fn)
	}
}

// Stats - Returns observability stats for all hook registries.
func (h *Hooks) Stats() []observability.Stats {
	return []observability.Stats{
		h.trackers[0].Stats(h.onPipelineStarted.Len()),
		h.trackers[1].Stats(h.onPipelineCompleted.Len()),
		h.trackers[2].Stats(h.onPipelineFailed.Len()),
		h.trackers[3].Stats(h.onStepStarted.Len()),
		h.trackers[4].Stats(h.onStepCompleted.Len()),
		h.trackers[5].Stats(h.onStepFailed.Len()),
	}
}

// OnPipelineStarted - Registers a listener called when a pipeline begins execution.
func (h *Hooks) OnPipelineStarted(fn func(pipeline string)) {
	h.onPipelineStarted.On(fn)
}

// OnPipelineCompleted - Registers a listener called when a pipeline finishes successfully.
func (h *Hooks) OnPipelineCompleted(fn func(pipeline string, results []Result)) {
	h.onPipelineCompleted.On(fn)
}

// OnPipelineFailed - Registers a listener called when a pipeline fails.
func (h *Hooks) OnPipelineFailed(fn func(pipeline string, err error)) {
	h.onPipelineFailed.On(fn)
}

// OnStepStarted - Registers a listener called when a step begins execution.
func (h *Hooks) OnStepStarted(fn func(pipeline, step string)) {
	h.onStepStarted.On(fn)
}

// OnStepCompleted - Registers a listener called when a step finishes successfully.
func (h *Hooks) OnStepCompleted(fn func(pipeline, step string, result Result)) {
	h.onStepCompleted.On(fn)
}

// OnStepFailed - Registers a listener called when a step fails.
func (h *Hooks) OnStepFailed(fn func(pipeline, step string, err error)) {
	h.onStepFailed.On(fn)
}

// emitPipelineStarted - Fires all registered pipeline-started listeners with panic recovery.
func (h *Hooks) emitPipelineStarted(pipeline string) {
	h.trackers[0].TrackEmit()
	for _, fn := range h.onPipelineStarted.Emit() {
		func() {
			defer h.trackers[0].RecoverPanic()
			fn(pipeline)
		}()
	}
}

// emitPipelineCompleted - Fires all registered pipeline-completed listeners with panic recovery.
func (h *Hooks) emitPipelineCompleted(pipeline string, results []Result) {
	h.trackers[1].TrackEmit()
	for _, fn := range h.onPipelineCompleted.Emit() {
		func() {
			defer h.trackers[1].RecoverPanic()
			fn(pipeline, results)
		}()
	}
}

// emitPipelineFailed - Fires all registered pipeline-failed listeners with panic recovery.
func (h *Hooks) emitPipelineFailed(pipeline string, err error) {
	h.trackers[2].TrackEmit()
	for _, fn := range h.onPipelineFailed.Emit() {
		func() {
			defer h.trackers[2].RecoverPanic()
			fn(pipeline, err)
		}()
	}
}

// emitStepStarted - Fires all registered step-started listeners with panic recovery.
func (h *Hooks) emitStepStarted(pipeline, step string) {
	h.trackers[3].TrackEmit()
	for _, fn := range h.onStepStarted.Emit() {
		func() {
			defer h.trackers[3].RecoverPanic()
			fn(pipeline, step)
		}()
	}
}

// emitStepCompleted - Fires all registered step-completed listeners with panic recovery.
func (h *Hooks) emitStepCompleted(pipeline, step string, result Result) {
	h.trackers[4].TrackEmit()
	for _, fn := range h.onStepCompleted.Emit() {
		func() {
			defer h.trackers[4].RecoverPanic()
			fn(pipeline, step, result)
		}()
	}
}

// emitStepFailed - Fires all registered step-failed listeners with panic recovery.
func (h *Hooks) emitStepFailed(pipeline, step string, err error) {
	h.trackers[5].TrackEmit()
	for _, fn := range h.onStepFailed.Emit() {
		func() {
			defer h.trackers[5].RecoverPanic()
			fn(pipeline, step, err)
		}()
	}
}
