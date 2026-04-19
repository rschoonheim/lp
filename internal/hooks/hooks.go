package hooks

import "sync"

// Registry - Generic, thread-safe observer registry for a single event type.
// T is the callback function signature (e.g., func(string) or func(string, error)).
type Registry[T any] struct {
	name      string
	mu        sync.RWMutex
	listeners []T
}

// NewRegistry - Creates a named Registry for identifying this event in logs and diagnostics.
func NewRegistry[T any](name string) Registry[T] {
	return Registry[T]{name: name}
}

// Name - Returns the registry's identifier.
func (r *Registry[T]) Name() string {
	return r.name
}

// On - Registers a listener callback for this event.
func (r *Registry[T]) On(fn T) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.listeners = append(r.listeners, fn)
}

// Emit - Returns a snapshot of all registered listeners for safe iteration.
// The caller invokes each listener with the appropriate arguments.
func (r *Registry[T]) Emit() []T {
	r.mu.RLock()
	defer r.mu.RUnlock()
	snapshot := make([]T, len(r.listeners))
	copy(snapshot, r.listeners)
	return snapshot
}

// Len - Returns the number of registered listeners.
func (r *Registry[T]) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.listeners)
}

// Clear - Removes all registered listeners. Intended for testing.
func (r *Registry[T]) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.listeners = nil
}
