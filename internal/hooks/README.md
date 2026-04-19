# hooks — Generic Event Hook Registry

## Purpose

Reusable, thread-safe observer pattern for any component that needs lifecycle event hooks. Built on Go generics so each `Registry[T]` is type-safe for a specific callback signature. This package is a pure registry — observability (stats, panic recovery) lives in `internal/observability`.

## Location

`internal/hooks/hooks.go`

## Core type

```go
hooks.Registry[T any]   // Generic observer registry
```

### Registry methods

- `NewRegistry[T](name string) Registry[T]` — Create a named registry
- `On(fn T)` — Register a listener
- `Emit() []T` — Get a snapshot of listeners to invoke
- `Len() int` — Count registered listeners
- `Name() string` — Get registry identifier
- `Clear()` — Remove all listeners (for testing)

## Usage pattern

### 1. Define a hooks struct in your component

Compose one `Registry` per event, parameterized by the callback signature:

```go
package mycomponent

import "lp/internal/hooks"

type Hooks struct {
    onStarted   hooks.Registry[func(name string)]
    onFailed    hooks.Registry[func(name string, err error)]
    onCompleted hooks.Registry[func(name string)]
}
```

### 2. Create a constructor with named registries

```go
// NewHooks - Creates a Hooks instance with named registries.
func NewHooks() Hooks {
    return Hooks{
        onStarted:   hooks.NewRegistry[func(name string)]("started"),
        onFailed:    hooks.NewRegistry[func(name string, err error)]("failed"),
        onCompleted: hooks.NewRegistry[func(name string)]("completed"),
    }
}
```

### 3. Expose exported `On*` registration methods

```go
// OnStarted - Registers a listener called when processing begins.
func (h *Hooks) OnStarted(fn func(name string)) {
    h.onStarted.On(fn)
}
```

### 4. Add unexported `emit*` helpers

Iterate the snapshot returned by `Emit()` and call each listener. For panic safety and tracking, pair with `observability.HookTracker` (see `internal/observability/README.md`):

```go
// emitStarted - Fires all registered started listeners.
func (h *Hooks) emitStarted(name string) {
    for _, fn := range h.onStarted.Emit() {
        fn(name)
    }
}
```

### 5. Wire in your component's execution path

```go
func (c *Component) run() {
    c.Hooks.emitStarted(c.Name)
    // ... do work ...
    c.Hooks.emitCompleted(c.Name)
}
```

## Conventions

- **Exported `On*`** for registration, **unexported `emit*`** for firing — keeps the public API clean.
- One `Registry` field per event — never share a `Registry` across different event types.
- Always use `NewRegistry` with a descriptive name — enables meaningful logs and diagnostics.
- Listeners run synchronously within `emit*`. Launch a goroutine inside the listener if it must not block.
- Follow the project function comment style: `// functionName - Description.`
- Use `Clear()` only in tests to reset state between cases.
- For observability (emit counting, panic recovery, stats), use `observability.HookTracker` — do not add tracking to this package.
