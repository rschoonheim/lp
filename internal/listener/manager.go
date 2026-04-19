package listener

import (
	"context"
	"fmt"
	"sync"
)

// Manager - Runs multiple Listeners concurrently and collects their events
// into a single channel. Provides lifecycle hooks for observability.
type Manager struct {
	mu        sync.Mutex
	listeners []Listener
	events    chan Event
	Hooks     Hooks
}

// NewManager - Creates a Manager with the given event buffer size.
func NewManager(bufferSize int) *Manager {
	return &Manager{
		events: make(chan Event, bufferSize),
		Hooks:  NewHooks(),
	}
}

// Register - Adds a listener to be managed. Must be called before Start.
func (m *Manager) Register(l Listener) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listeners = append(m.listeners, l)
}

// Events - Returns the channel on which all listener events are published.
func (m *Manager) Events() <-chan Event {
	return m.events
}

// Start - Launches all registered listeners in separate goroutines.
// Blocks until the context is cancelled, then stops all listeners.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	listeners := make([]Listener, len(m.listeners))
	copy(listeners, m.listeners)
	m.mu.Unlock()

	var wg sync.WaitGroup

	for _, l := range listeners {
		l := l
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Hooks.emitListenerStarted(l.Name())
			if err := l.Start(ctx, m.events); err != nil {
				m.Hooks.emitListenerError(l.Name(), fmt.Errorf("listener %q: %w", l.Name(), err))
			}
			m.Hooks.emitListenerStopped(l.Name())
		}()
	}

	<-ctx.Done()

	for _, l := range listeners {
		if err := l.Stop(); err != nil {
			m.Hooks.emitListenerError(l.Name(), fmt.Errorf("listener %q stop: %w", l.Name(), err))
		}
	}

	wg.Wait()
	close(m.events)
	return nil
}

