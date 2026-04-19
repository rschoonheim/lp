package listener

import (
	"lp/internal/hooks"
	"lp/internal/observability"
)

// Hooks - Lifecycle event registries for the listener Manager.
type Hooks struct {
	onListenerStarted hooks.Registry[func(name string)]
	onListenerStopped hooks.Registry[func(name string)]
	onListenerError   hooks.Registry[func(name string, err error)]
	onEventReceived   hooks.Registry[func(event Event)]

	trackers []*observability.HookTracker
}

// NewHooks - Creates a Hooks instance with named registries and trackers.
func NewHooks() Hooks {
	trackers := []*observability.HookTracker{
		observability.NewHookTracker("listener_started"),
		observability.NewHookTracker("listener_stopped"),
		observability.NewHookTracker("listener_error"),
		observability.NewHookTracker("event_received"),
	}
	return Hooks{
		onListenerStarted: hooks.NewRegistry[func(name string)]("listener_started"),
		onListenerStopped: hooks.NewRegistry[func(name string)]("listener_stopped"),
		onListenerError:   hooks.NewRegistry[func(name string, err error)]("listener_error"),
		onEventReceived:   hooks.NewRegistry[func(event Event)]("event_received"),
		trackers:          trackers,
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
		h.trackers[0].Stats(h.onListenerStarted.Len()),
		h.trackers[1].Stats(h.onListenerStopped.Len()),
		h.trackers[2].Stats(h.onListenerError.Len()),
		h.trackers[3].Stats(h.onEventReceived.Len()),
	}
}

// OnListenerStarted - Registers a callback for when a listener begins.
func (h *Hooks) OnListenerStarted(fn func(name string)) {
	h.onListenerStarted.On(fn)
}

// OnListenerStopped - Registers a callback for when a listener stops.
func (h *Hooks) OnListenerStopped(fn func(name string)) {
	h.onListenerStopped.On(fn)
}

// OnListenerError - Registers a callback for when a listener encounters an error.
func (h *Hooks) OnListenerError(fn func(name string, err error)) {
	h.onListenerError.On(fn)
}

// OnEventReceived - Registers a callback for when an event is produced.
func (h *Hooks) OnEventReceived(fn func(event Event)) {
	h.onEventReceived.On(fn)
}

// emitListenerStarted - Fires all registered listener-started callbacks.
func (h *Hooks) emitListenerStarted(name string) {
	h.trackers[0].TrackEmit()
	for _, fn := range h.onListenerStarted.Emit() {
		func() {
			defer h.trackers[0].RecoverPanic()
			fn(name)
		}()
	}
}

// emitListenerStopped - Fires all registered listener-stopped callbacks.
func (h *Hooks) emitListenerStopped(name string) {
	h.trackers[1].TrackEmit()
	for _, fn := range h.onListenerStopped.Emit() {
		func() {
			defer h.trackers[1].RecoverPanic()
			fn(name)
		}()
	}
}

// emitListenerError - Fires all registered listener-error callbacks.
func (h *Hooks) emitListenerError(name string, err error) {
	h.trackers[2].TrackEmit()
	for _, fn := range h.onListenerError.Emit() {
		func() {
			defer h.trackers[2].RecoverPanic()
			fn(name, err)
		}()
	}
}

// emitEventReceived - Fires all registered event-received callbacks.
func (h *Hooks) emitEventReceived(event Event) {
	h.trackers[3].TrackEmit()
	for _, fn := range h.onEventReceived.Emit() {
		func() {
			defer h.trackers[3].RecoverPanic()
			fn(event)
		}()
	}
}

