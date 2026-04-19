# listener — Trigger Monitoring and Event Production

## Purpose

Provides the listener layer for LP: monitors triggers (file changes, time schedules, etc.) and fires events that the server routes to pipeline execution. Each listener type implements a common interface; a `Manager` runs them concurrently and merges their events into a single channel.

## Location

```
internal/listener/listener.go  # Listener interface
internal/listener/event.go     # Event type produced by listeners
internal/listener/manager.go   # Manager runs listeners concurrently, merges events
internal/listener/hooks.go     # Lifecycle hooks with observability trackers
```

## Core types

```go
listener.Listener   // Interface: Name(), Start(ctx, chan<- Event), Stop()
listener.Event      // Trigger signal: Listener, Trigger, Pipeline, Payload
listener.Manager    // Runs []Listener, collects events, exposes Hooks
listener.Hooks      // Lifecycle hooks: listener_started, listener_stopped, listener_error, event_received
```

## Listener interface

Every trigger type implements this interface:

```go
type Listener interface {
    Name() string
    Start(ctx context.Context, events chan<- Event) error
    Stop() error
}
```

- `Start` blocks until `ctx` is cancelled or a fatal error occurs.
- Events are sent on the provided channel.
- `Stop` is called during graceful shutdown.

## Event fields

```go
type Event struct {
    Listener string         // Which listener produced this
    Trigger  string         // Which trigger fired
    Pipeline string         // Pipeline name or ID to execute
    Payload  map[string]any // Trigger-specific data
}
```

## Manager API

- `NewManager(bufferSize int) *Manager` — Create with buffered event channel
- `Register(l Listener)` — Add a listener (before Start)
- `Events() <-chan Event` — Read channel for all listener events
- `Start(ctx context.Context) error` — Launch all listeners; blocks until ctx cancelled
- `Hooks` — Lifecycle hooks (exported field)

## Hooks

Four hook events: `listener_started`, `listener_stopped`, `listener_error`, `event_received`.

- `OnListenerStarted(fn func(name string))`
- `OnListenerStopped(fn func(name string))`
- `OnListenerError(fn func(name string, err error))`
- `OnEventReceived(fn func(event Event))`

## Implementing a new listener type

### 1. Create a file in your listener package (or `internal/listener/`)

```go
package listener

import "context"

type FileWatcher struct {
    name string
    path string
    done chan struct{}
}

// NewFileWatcher - Creates a FileWatcher listener for the given path.
func NewFileWatcher(name, path string) *FileWatcher {
    return &FileWatcher{name: name, path: path, done: make(chan struct{})}
}

// Name - Returns the listener's identifier.
func (w *FileWatcher) Name() string { return w.name }

// Start - Watches the path for changes and sends events.
func (w *FileWatcher) Start(ctx context.Context, events chan<- Event) error {
    for {
        select {
        case <-ctx.Done():
            return nil
        case <-w.done:
            return nil
        // ... watch for changes, send Event on events channel ...
        }
    }
}

// Stop - Signals the watcher to shut down.
func (w *FileWatcher) Stop() error {
    close(w.done)
    return nil
}
```

### 2. Register with the Manager

```go
mgr := listener.NewManager(64)
mgr.Register(listener.NewFileWatcher("code-watcher", "/src"))
```

### 3. Consume events from the server

```go
go mgr.Start(ctx)

for event := range mgr.Events() {
    // Route event.Pipeline to the runner pool
}
```

## Conventions

- Every listener type must implement the `Listener` interface.
- `Start` must respect context cancellation — never block indefinitely.
- Error wrapping: use `fmt.Errorf("listener %q: %w", name, err)` for context.
- Hooks follow the project pattern: exported `On*`, unexported `emit*`, paired with `observability.HookTracker`.
- Follow the project function comment style: `// functionName - Description.`

