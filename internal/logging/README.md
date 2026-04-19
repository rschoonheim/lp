# logging — Structured User Feedback Logging

## Purpose

Collects categorized log entries from runners, listeners, and other components into a single queryable ledger. Entries are grouped by semantic **category** (their "meaning") so consumers can build targeted user feedback — e.g., show all pipeline events, or all errors, or all hook activity.

This package is for **user-facing feedback**. For operational metrics and JSON system logging, see `internal/observability`.

## Location

```
internal/logging/ledger.go  # Ledger, Entry, Level, Category
internal/logging/scoped.go  # Scoped writer (bound category + source)
```

## Core types

```go
logging.Ledger    // Thread-safe, append-only log store with query methods
logging.Scoped    // Convenience writer bound to a category and source
logging.Entry     // Single log record
logging.Level     // Severity: LevelDebug, LevelInfo, LevelWarn, LevelError
logging.Category  // Semantic group: CatPipeline, CatStep, CatHook, CatConfig, CatServer
```

## Ledger API

- `NewLedger() *Ledger` — Create an empty ledger
- `Log(level, category, source, message, fields)` — Append a full entry
- `Info(category, source, message, fields)` — Shorthand for info level
- `Error(category, source, message, fields)` — Shorthand for error level
- `Warn(category, source, message, fields)` — Shorthand for warn level
- `Debug(category, source, message, fields)` — Shorthand for debug level
- `Append(entry)` — Add a pre-built Entry
- `All() []Entry` — All entries in insertion order
- `ByCategory(cat) []Entry` — Filter by semantic category
- `ByLevel(min) []Entry` — Filter by minimum severity
- `BySource(source) []Entry` — Filter by source identifier
- `Grouped() map[Category][]Entry` — All entries grouped by category
- `Len() int` — Total entry count
- `Clear()` — Remove all entries (for testing)

## Scoped API

- `NewScoped(ledger, category, source) *Scoped` — Create a bound writer
- `Info(message, fields)` / `Error(message, fields)` / `Warn(message, fields)` / `Debug(message, fields)`

## Built-in categories

| Constant      | Value        | Use for                                  |
|---------------|--------------|------------------------------------------|
| `CatPipeline` | `"pipeline"` | Pipeline start, complete, fail           |
| `CatStep`     | `"step"`     | Step start, complete, fail               |
| `CatHook`     | `"hook"`     | Hook command execution, panics           |
| `CatConfig`   | `"config"`   | Configuration loading, validation        |
| `CatServer`   | `"server"`   | Server lifecycle, startup, shutdown      |

Custom categories can be defined as `logging.Category("my_category")`.

## Usage pattern

### 1. Create a shared ledger at startup

```go
ledger := logging.NewLedger()
```

### 2. Create scoped writers per component

```go
pipelineLog := logging.NewScoped(ledger, logging.CatPipeline, "runners")
stepLog := logging.NewScoped(ledger, logging.CatStep, "runners")
hookLog := logging.NewScoped(ledger, logging.CatHook, "events")
```

### 3. Log events from hooks

```go
pool.Hooks.OnPipelineStarted(func(pipeline string) {
    pipelineLog.Info("pipeline started", map[string]any{"pipeline": pipeline})
})

pool.Hooks.OnStepFailed(func(pipeline, step string, err error) {
    stepLog.Error("step failed", map[string]any{
        "pipeline": pipeline,
        "step":     step,
        "error":    err.Error(),
    })
})
```

### 4. Query logs for user feedback

```go
// Get all pipeline-related logs
pipelineEntries := ledger.ByCategory(logging.CatPipeline)

// Get all errors across all categories
errors := ledger.ByLevel(logging.LevelError)

// Get everything grouped by meaning
grouped := ledger.Grouped()
for cat, entries := range grouped {
    fmt.Printf("=== %s (%d entries) ===\n", cat, len(entries))
}

// Get logs from a specific source
runnerLogs := ledger.BySource("runners")
```

## Conventions

- One `Ledger` per server instance — pass it to components that need logging.
- Use `Scoped` writers to avoid repeating category/source in hot paths.
- Use built-in `Cat*` constants for standard categories; define new `Category` values for new components.
- `Fields` map holds structured context — always include identifying keys (`"pipeline"`, `"step"`, `"error"`).
- Follow the project function comment style: `// functionName - Description.`
- Use `Clear()` only in tests to reset state between cases.

