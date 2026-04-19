# LP - Local Pipeline: Agent Guide

## Project structure

```
cmd/server/              # Server entrypoint and YAML configuration wiring
internal/hooks/          # Generic, reusable event hook registry (Go generics)
internal/listener/       # Trigger monitoring and event production
internal/logging/        # Categorized user-feedback log collection
internal/observability/  # Structured logging and metrics collection
internal/pipeline/       # YAML-defined pipelines with per-run logs and observability
internal/runners/        # Core pipeline execution engine (Pool, Pipeline, Step, Hooks)
```

- `cmd/server/main.go` — CLI entrypoint; reads a YAML config file path from `os.Args[1]`
- `cmd/server/configuration.go` — `Configuration` struct with YAML tags; converts to `runners.Config`
- `cmd/server/events.go` — `HookEntry` YAML model and `RegisterConfiguredHooks` wiring
- `cmd/server/listeners.go` — `ListenerFactory` registry and `RegisterConfiguredListeners` wiring
- `cmd/server/pipelines.go` — `LoadPipelinesFromDirectories` scans dirs for pipeline YAML files
- `internal/hooks/hooks.go` — `Registry[T]` generic thread-safe observer (pure registry, no observability); see `internal/hooks/README.md`
- `internal/listener/listener.go` — `Listener` interface (`Name`, `Start`, `Stop`); see `internal/listener/README.md`
- `internal/listener/event.go` — `Event` type produced by listeners (Listener, Trigger, Pipeline, Payload)
- `internal/listener/manager.go` — `Manager` runs listeners concurrently, merges events into a single channel
- `internal/listener/hooks.go` — Listener lifecycle hooks with observability trackers
- `internal/logging/ledger.go` — `Ledger` append-only log store with category/level/source queries; see `internal/logging/README.md`
- `internal/logging/scoped.go` — `Scoped` convenience writer bound to a category and source
- `internal/observability/logger.go` — `Logger` component-scoped structured JSON logger (`log/slog`); see `internal/observability/README.md`
- `internal/observability/tracker.go` — `HookTracker` emit counting and panic recovery for hook registries
- `internal/observability/collector.go` — `Collector` aggregates `Stats` and custom counters
- `internal/pipeline/pipeline.go` — `Pipeline` (UUID, name, YAML-defined `[]StepDef`); see `internal/pipeline/README.md`
- `internal/pipeline/step.go` — `StepDef` (YAML step definition) and `StepResult` (execution output)
- `internal/pipeline/run.go` — `Run` (UUID, status, per-run `Ledger` + `Collector`, `[]StepResult`)
- `internal/pipeline/store.go` — `Store` thread-safe in-memory registry of pipelines and runs
- `internal/pipeline/id.go` — UUID v4 generation (no external deps)
- `internal/runners/pool.go` — Concurrency-limited `Pool` that runs pipelines via a semaphore channel
- `internal/runners/pipeline.go` — `Pipeline` (name + ordered `[]Step`)
- `internal/runners/step.go` — `Step.Execute` runs an OS command via `os/exec`
- `internal/runners/hooks.go` — Runner-specific `Hooks` struct composed of `hooks.Registry[T]` fields
- `internal/runners/config.go` — `Config` struct (`Timeout`, `MaxConcurrent`)

## Build and run

```bash
go build -o lp-server ./cmd/server
./lp-server <configuration.yaml>
```

## Dependencies

- Go 1.26 (`go.mod`)
- `gopkg.in/yaml.v3` — YAML config parsing

## Conventions

### Function comment style

Each function should have a comment block that describes its purpose, parameters, and return value. The comment block
should be placed immediately above the function definition. It should have the following structure:
```go
// functionName - Description of the function's purpose.
```

### Exported vs unexported

- Public registration methods are exported (e.g., `OnPipelineStarted`); internal emit helpers are unexported (e.g., `emitPipelineStarted`). Follow this pattern when adding new hook events.

### Error wrapping

Wrap errors with `fmt.Errorf` and `%w` to preserve the error chain. Include identifying context (pipeline name, step index). See `pool.go:run`.

### Hook lifecycle events

Six hook events exist: `pipeline_started`, `pipeline_completed`, `pipeline_failed`, `step_started`, `step_completed`, `step_failed`. When adding a new hook:
1. Add a `hooks.Registry[T]` field to the component's `Hooks` struct (e.g., `internal/runners/hooks.go`)
2. Add exported `On*` registration method and unexported `emit*` method
3. Wire it in `cmd/server/events.go` `RegisterConfiguredHooks` switch
4. For new components, see `internal/hooks/README.md` for the full implementation pattern

### YAML configuration

Config structs use `yaml:"snake_case"` tags. The server-level `Configuration` struct in `cmd/server/configuration.go` owns the YAML shape; `internal/runners` stays YAML-agnostic (plain Go types).
