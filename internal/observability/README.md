# observability — Structured Logging and Metrics Collection

## Purpose

Centralized observability for all LP components. Provides structured JSON logging via `log/slog`, hook tracking (emit counts, panic recovery), and a metrics collector that aggregates stats and custom counters from any component.

## Location

```
internal/observability/logger.go     # Structured logger (slog wrapper)
internal/observability/tracker.go    # HookTracker for emit counting and panic recovery
internal/observability/collector.go  # Metrics aggregation from stats and custom counters
```

## Core types

```go
observability.Logger       // Component-scoped structured JSON logger
observability.HookTracker  // Emit counting and panic recovery for a hooks.Registry
observability.Stats        // Observable state snapshot for a tracked registry
observability.Collector    // Aggregates Stats and counters across components
observability.Snapshot     // Point-in-time state for a single component
```

## Logger

Wraps `log/slog` with a JSON handler writing to stderr. Every logger is scoped to a component name.

### API

- `NewLogger(component string) *Logger` — Create with default `Info` level
- `NewLoggerWithLevel(component string, level slog.Level) *Logger` — Create with custom level
- `With(args ...any) *Logger` — Derive a child logger with additional context
- `Info(msg string, args ...any)` — Log at info level
- `Error(msg string, args ...any)` — Log at error level
- `Warn(msg string, args ...any)` — Log at warn level
- `Debug(msg string, args ...any)` — Log at debug level

### Usage

```go
import "lp/internal/observability"

logger := observability.NewLogger("runners")
logger.Info("pipeline_started", "pipeline", name)
logger.Error("step_failed", "pipeline", name, "step", stepName, "error", err)

// Derive a sub-logger with persistent context
pipelineLog := logger.With("pipeline", name)
pipelineLog.Info("step_completed", "step", stepName)
```

## HookTracker

Pairs with a `hooks.Registry` to add observability without polluting the registry itself. Tracks emit counts, recovers panics, and reports stats.

### API

- `NewHookTracker(name string) *HookTracker` — Create a tracker for a named event
- `TrackEmit()` — Increment the emit counter (call once per `Emit()`)
- `RecoverPanic()` — Defer inside listener loops for panic safety
- `SetPanicHandler(fn func(name string, recovered any))` — Handle panics with context
- `Stats(listenerCount int) Stats` — Get snapshot (pass `registry.Len()` for listener count)
- `Name() string` — Get tracker identifier
- `Reset()` — Reset counters to zero (for testing)

### Stats fields

```go
type Stats struct {
    Name         string  // Tracker/registry identifier
    Listeners    int     // Current listener count (from registry)
    EmitCount    uint64  // Total TrackEmit() calls
    PanicsCaught uint64  // Panics recovered via RecoverPanic
}
```

### Usage — pairing with hooks.Registry

```go
import (
    "lp/internal/hooks"
    "lp/internal/observability"
)

type Hooks struct {
    onStarted hooks.Registry[func(name string)]
    tracker   *observability.HookTracker
}

func NewHooks() Hooks {
    return Hooks{
        onStarted: hooks.NewRegistry[func(name string)]("started"),
        tracker:   observability.NewHookTracker("started"),
    }
}

// emitStarted - Fires all registered started listeners with tracking and panic recovery.
func (h *Hooks) emitStarted(name string) {
    h.tracker.TrackEmit()
    for _, fn := range h.onStarted.Emit() {
        func() {
            defer h.tracker.RecoverPanic()
            fn(name)
        }()
    }
}

// Stats - Returns observability stats.
func (h *Hooks) Stats() []observability.Stats {
    return []observability.Stats{
        h.tracker.Stats(h.onStarted.Len()),
    }
}
```

## Collector

Aggregates observability data from components that expose `Stats` and/or custom counters.

### API

- `NewCollector(logger *Logger) *Collector` — Create with an optional logger for `Log()` output
- `RegisterHooksSource(component string, fn func() []Stats)` — Register a component's stats provider
- `Increment(component, name string)` — Increment a named counter for a component
- `Snapshot(component string) Snapshot` — Get point-in-time state for one component
- `All() []Snapshot` — Get snapshots for all registered components
- `Log()` — Log all component states at Info level

### Snapshot fields

```go
type Snapshot struct {
    Component string             // Component identifier
    Hooks     []Stats            // Hook tracker stats
    Counters  map[string]uint64  // Custom counters
}
```

### Usage

```go
import (
    "lp/internal/observability"
    "lp/internal/runners"
)

logger := observability.NewLogger("server")
collector := observability.NewCollector(logger)

pool := runners.NewPool(cfg)

// Register the pool's hook stats
collector.RegisterHooksSource("runners", pool.Hooks.Stats)

// Track custom events
collector.Increment("runners", "pipelines_submitted")

// Dump all stats to logs
collector.Log()

// Inspect a single component
snap := collector.Snapshot("runners")
```

## Integration pattern for new components

### 1. Create a logger for the component

```go
logger := observability.NewLogger("mycomponent")
```

### 2. Create HookTrackers alongside registries

Pair one `HookTracker` per `hooks.Registry` in your component's `Hooks` struct (see example in HookTracker section above).

### 3. Register hooks with the collector

If your component has a `Stats() []observability.Stats` method:

```go
collector.RegisterHooksSource("mycomponent", myComponent.Hooks.Stats)
```

### 4. Use custom counters for non-hook metrics

```go
collector.Increment("mycomponent", "requests_processed")
collector.Increment("mycomponent", "cache_misses")
```

### 5. Wire panic handlers to the logger

```go
myComponent.Hooks.SetPanicHandler(func(name string, recovered any) {
    logger.Error("hook_panic", "hook", name, "recovered", recovered)
})
```

## Conventions

- One `Logger` per component — pass the component name to `NewLogger` so all log lines include it.
- One `HookTracker` per `hooks.Registry` — keeps tracking paired 1:1 with event types.
- Use `With()` to add persistent context (e.g., pipeline name) rather than repeating key-value pairs.
- Register hooks sources in `cmd/server/main.go` during startup — keep `internal/` packages unaware of the collector.
- Counter names use `snake_case` — e.g., `"pipelines_submitted"`, `"steps_failed"`.
- `Log()` is designed for periodic or on-demand diagnostics, not per-event logging — use `Logger` directly for per-event output.
- Follow the project function comment style: `// functionName - Description.`
