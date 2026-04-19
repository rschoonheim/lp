package observability

import "sync/atomic"

// Stats - Observable state snapshot for a tracked hook registry.
type Stats struct {
	Name         string
	Listeners    int
	EmitCount    uint64
	PanicsCaught uint64
}

// HookTracker - Tracks emit counts and panic recovery for a named hook registry.
// Pair one HookTracker with each hooks.Registry to add observability.
type HookTracker struct {
	name         string
	emitCount    atomic.Uint64
	panicsCaught atomic.Uint64
	onPanic      func(name string, recovered any)
}

// NewHookTracker - Creates a HookTracker for the given event name.
func NewHookTracker(name string) *HookTracker {
	return &HookTracker{name: name}
}

// Name - Returns the tracker's identifier.
func (t *HookTracker) Name() string {
	return t.name
}

// SetPanicHandler - Sets a callback invoked when RecoverPanic catches a panic.
// The handler receives the tracker name and the recovered value.
func (t *HookTracker) SetPanicHandler(fn func(name string, recovered any)) {
	t.onPanic = fn
}

// TrackEmit - Increments the emit counter. Call once per Emit() invocation.
func (t *HookTracker) TrackEmit() {
	t.emitCount.Add(1)
}

// RecoverPanic - Recovers from a panic in a listener invocation, increments
// the panic counter, and calls the configured panic handler. Use via defer
// inside the loop that invokes listeners:
//
//	tracker.TrackEmit()
//	for _, fn := range registry.Emit() {
//	    func() {
//	        defer tracker.RecoverPanic()
//	        fn(args)
//	    }()
//	}
func (t *HookTracker) RecoverPanic() {
	rec := recover()
	if rec == nil {
		return
	}
	t.panicsCaught.Add(1)
	if t.onPanic != nil {
		t.onPanic(t.name, rec)
	}
}

// Stats - Returns an observable state snapshot. The caller provides the current
// listener count (from hooks.Registry.Len()) since the tracker does not own the registry.
func (t *HookTracker) Stats(listenerCount int) Stats {
	return Stats{
		Name:         t.name,
		Listeners:    listenerCount,
		EmitCount:    t.emitCount.Load(),
		PanicsCaught: t.panicsCaught.Load(),
	}
}

// Reset - Resets all counters to zero. Intended for testing.
func (t *HookTracker) Reset() {
	t.emitCount.Store(0)
	t.panicsCaught.Store(0)
}

