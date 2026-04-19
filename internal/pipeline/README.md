# pipeline — YAML-Defined Pipelines with Per-Run Observability

## Purpose

Defines the user-facing pipeline model: YAML-parseable pipeline definitions with unique UUIDs, a collection of runs per pipeline, and per-run isolated output capture, logging, and observability.

This package owns the **data model**. Execution is handled by `internal/runners`.

## Location

```
internal/pipeline/pipeline.go  # Pipeline type (ID, Name, Steps)
internal/pipeline/step.go      # StepDef (YAML definition) and StepResult (execution output)
internal/pipeline/run.go       # Run lifecycle (pending → running → completed/failed)
internal/pipeline/store.go     # Thread-safe in-memory store for pipelines and runs
internal/pipeline/id.go        # UUID v4 generation (no external deps)
```

## Core types

```go
pipeline.Pipeline    // YAML-defined pipeline with UUID and []StepDef
pipeline.StepDef     // Step definition with yaml tags
pipeline.StepResult  // Captured output and status of a step execution
pipeline.Run         // Single execution with own Ledger and Collector
pipeline.Store       // In-memory registry of pipelines and runs
pipeline.Status      // Lifecycle enum: pending, running, completed, failed
```

## YAML shape

```yaml
pipelines:
  - name: build
    steps:
      - name: compile
        command: go
        args: [build, ./...]
        dir: /project
        env: [CGO_ENABLED=0]
      - name: test
        command: go
        args: [test, ./...]
```

## Pipeline API

- `NewPipeline(name string, steps []StepDef) *Pipeline` — Create with generated UUID
- `ToRunnersPipeline() *runners.Pipeline` — Convert to runners model for execution

## Run API

- `NewRun(pipelineID string, stepCount int) *Run` — Create pending run with own Ledger and Collector
- `Start()` — Mark running, record start time, log to run's Ledger
- `StartStep(index int, name string)` — Mark step running
- `CompleteStep(index int, output string)` — Record step success with stdout
- `FailStep(index int, output string, err string)` — Record step failure with stderr
- `Finish(err error)` — Mark completed/failed, record finish time
- `Duration() time.Duration` — Elapsed time

Each Run has:
- `Ledger *logging.Ledger` — isolated log entries grouped by category (`CatPipeline`, `CatStep`)
- `Collector *observability.Collector` — isolated metrics per run
- `Steps []StepResult` — captured output per step

## Store API

- `NewStore() *Store` — Create empty store
- `AddPipeline(p *Pipeline)` — Register a pipeline
- `GetPipeline(id string) (*Pipeline, error)` — Lookup by ID
- `Pipelines() []*Pipeline` — List all
- `CreateRun(pipelineID string) (*Run, error)` — Create and store a new run
- `GetRun(id string) (*Run, error)` — Lookup by run ID
- `RunsFor(pipelineID string) []*Run` — All runs for a pipeline in order

## Usage pattern

### 1. Parse pipelines from YAML config

```go
var cfg Configuration
// ... unmarshal YAML ...
store := pipeline.NewStore()
for _, def := range cfg.Pipelines {
    p := pipeline.NewPipeline(def.Name, def.Steps)
    store.AddPipeline(p)
}
```

### 2. Create a run and execute

```go
run, _ := store.CreateRun(p.ID)
run.Start()

rp := p.ToRunnersPipeline()
pool.Submit(ctx, rp)  // runners executes it

// Hook listeners update the run:
pool.Hooks.OnStepCompleted(func(pname, step string) {
    run.CompleteStep(i, stdout)
})
```

### 3. Query per-run logs

```go
run, _ := store.GetRun(runID)

// All logs for this run, grouped by meaning
grouped := run.Ledger.Grouped()

// Just step logs
stepLogs := run.Ledger.ByCategory(logging.CatStep)

// Just errors
errors := run.Ledger.ByLevel(logging.LevelError)
```

### 4. Query per-run observability

```go
run.Collector.Log()  // dump all stats for this run
snap := run.Collector.Snapshot("run")
```

## Conventions

- Pipelines get UUIDs on creation via `NewPipeline` — never set `ID` manually.
- Runs get UUIDs on creation via `NewRun` or `Store.CreateRun` — never set `ID` manually.
- YAML tags live on `StepDef` and `Pipeline` — `internal/runners` stays YAML-agnostic.
- Use `errors.Is(err, pipeline.ErrNotFound)` to check for missing pipelines/runs.
- Run mutation methods (`Start`, `CompleteStep`, `FailStep`, `Finish`) auto-log to the run's own Ledger.
- Follow the project function comment style: `// functionName - Description.`

