package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"lp/internal/listener"
	"lp/internal/logging"
	"lp/internal/observability"
	"lp/internal/pipeline"
	"lp/internal/runners"
)

// server - Holds all shared state for the LP server lifecycle.
type server struct {
	cfg            Configuration
	logger         *observability.Logger
	collector      *observability.Collector
	ledger         *logging.Ledger
	store          *pipeline.Store
	pool           *runners.Pool
	mgr            *listener.Manager
	pipelineEvents chan runners.Event
	restAPI        *api
	activeRuns     sync.Map // pipeline name → *pipeline.Run
	ws             *wsHub
}

// newServer - Creates and wires a server from the given configuration.
func newServer(cfg Configuration) *server {
	logger := observability.NewLoggerTo("server", io.Discard)
	collector := observability.NewCollector(logger)
	ledger := logging.NewLedger()

	ledger.Info(logging.CatConfig, "server", "configuration loaded", map[string]any{
		"timeout":              cfg.Runners.Timeout,
		"max_concurrent":       cfg.Runners.MaxConcurrent,
		"pipeline_directories": cfg.Pipelines.Directories,
	})

	store := pipeline.NewStore()
	loaded, loadErrs := LoadPipelinesFromDirectories(store, cfg.Pipelines.Directories)
	ledger.Info(logging.CatConfig, "server", "pipelines loaded", map[string]any{
		"loaded": loaded,
		"errors": len(loadErrs),
	})
	for _, err := range loadErrs {
		ledger.Error(logging.CatConfig, "server", "pipeline load error", map[string]any{
			"error": err.Error(),
		})
	}

	pool := runners.NewPool(cfg.RunnersConfig())
	pipelineEvents := make(chan runners.Event, 64)
	pool.EventSink = pipelineEvents
	RegisterConfiguredHooks(pool, cfg.Runners.Hooks)

	mgr := listener.NewManager(64)
	listenersRegistered, listenerErrs := RegisterConfiguredListeners(mgr, cfg.Listeners)
	ledger.Info(logging.CatConfig, "server", "listeners configured", map[string]any{
		"registered": listenersRegistered,
		"errors":     len(listenerErrs),
	})
	for _, err := range listenerErrs {
		ledger.Error(logging.CatConfig, "server", "listener config error", map[string]any{
			"error": err.Error(),
		})
	}

	return &server{
		cfg:            cfg,
		logger:         logger,
		collector:      collector,
		ledger:         ledger,
		store:          store,
		pool:           pool,
		mgr:            mgr,
		pipelineEvents: pipelineEvents,
		ws:             newWSHub(),
	}
}

// registerCLIHooks - Registers hooks that print pipeline and step results to the CLI.
func (s *server) registerCLIHooks() {
	pipelineLog := logging.NewScoped(s.ledger, logging.CatPipeline, "runners")
	stepLog := logging.NewScoped(s.ledger, logging.CatStep, "runners")
	hookLog := logging.NewScoped(s.ledger, logging.CatHook, "runners")

	s.pool.Hooks.OnPipelineStarted(func(pipeline string) {
		out.run("pipeline %s started", pipeline)
		pipelineLog.Info("pipeline started", map[string]any{"pipeline": pipeline})
	})
	s.pool.Hooks.OnPipelineCompleted(func(pipeline string, results []runners.Result) {
		out.ok("pipeline %s completed (%d steps)", pipeline, len(results))
		pipelineLog.Info("pipeline completed", map[string]any{"pipeline": pipeline})
	})
	s.pool.Hooks.OnPipelineFailed(func(pipeline string, err error) {
		out.fail("pipeline %s: %v", pipeline, err)
		pipelineLog.Error("pipeline failed", map[string]any{"pipeline": pipeline, "error": err.Error()})
	})
	s.pool.Hooks.OnStepStarted(func(pipeline, step string) {
		out.run("step %s/%s started", pipeline, step)
		stepLog.Info("step started", map[string]any{"pipeline": pipeline, "step": step})
	})
	s.pool.Hooks.OnStepCompleted(func(pipeline, step string, result runners.Result) {
		out.ok("step %s/%s completed", pipeline, step)
		if result.Stdout != "" {
			out.output("stdout", result.Stdout)
		}
		if result.Stderr != "" {
			out.output("stderr", result.Stderr)
		}
		stepLog.Info("step completed", map[string]any{"pipeline": pipeline, "step": step})
	})
	s.pool.Hooks.OnStepFailed(func(pipeline, step string, err error) {
		out.fail("step %s/%s: %v", pipeline, step, err)
		stepLog.Error("step failed", map[string]any{"pipeline": pipeline, "step": step, "error": err.Error()})
	})

	s.pool.Hooks.SetPanicHandler(func(name string, recovered any) {
		out.fail("hook %s panicked: %v", name, recovered)
		hookLog.Error("hook panic", map[string]any{"hook": name, "recovered": fmt.Sprint(recovered)})
		s.logger.Error("hook_panic", "hook", name, "recovered", recovered)
	})
}

// registerRunTracking - Registers hooks that update Run records in the store.
func (s *server) registerRunTracking() {
	s.pool.Hooks.OnPipelineStarted(func(pipelineName string) {
		if v, ok := s.activeRuns.Load(pipelineName); ok {
			v.(*pipeline.Run).Start()
		}
	})

	s.pool.Hooks.OnStepStarted(func(pipelineName, step string) {
		if v, ok := s.activeRuns.Load(pipelineName); ok {
			run := v.(*pipeline.Run)
			for i, sr := range run.Steps {
				if sr.Status == pipeline.StatusPending {
					run.StartStep(i, step)
					break
				}
			}
		}
	})

	s.pool.Hooks.OnStepCompleted(func(pipelineName, step string, result runners.Result) {
		if v, ok := s.activeRuns.Load(pipelineName); ok {
			run := v.(*pipeline.Run)
			for i, sr := range run.Steps {
				if sr.Name == step && sr.Status == pipeline.StatusRunning {
					run.CompleteStep(i, result.Stdout)
					break
				}
			}
		}
	})

	s.pool.Hooks.OnStepFailed(func(pipelineName, step string, err error) {
		if v, ok := s.activeRuns.Load(pipelineName); ok {
			run := v.(*pipeline.Run)
			for i, sr := range run.Steps {
				if sr.Name == step && sr.Status == pipeline.StatusRunning {
					run.FailStep(i, "", err.Error())
					break
				}
			}
		}
	})

	s.pool.Hooks.OnPipelineCompleted(func(pipelineName string, results []runners.Result) {
		if v, ok := s.activeRuns.Load(pipelineName); ok {
			v.(*pipeline.Run).Finish(nil)
			s.activeRuns.Delete(pipelineName)
		}
	})

	s.pool.Hooks.OnPipelineFailed(func(pipelineName string, err error) {
		if v, ok := s.activeRuns.Load(pipelineName); ok {
			v.(*pipeline.Run).Finish(err)
			s.activeRuns.Delete(pipelineName)
		}
	})
}

// registerWSBroadcast - Registers hooks that push real-time events to WebSocket clients.
func (s *server) registerWSBroadcast() {
	s.pool.Hooks.OnPipelineStarted(func(p string) {
		s.ws.broadcast(wsEvent{Type: "pipeline_started", Pipeline: p, Status: "running"})
	})
	s.pool.Hooks.OnPipelineCompleted(func(p string, results []runners.Result) {
		s.ws.broadcast(wsEvent{Type: "pipeline_completed", Pipeline: p, Status: "completed", Steps: len(results)})
	})
	s.pool.Hooks.OnPipelineFailed(func(p string, err error) {
		s.ws.broadcast(wsEvent{Type: "pipeline_failed", Pipeline: p, Status: "failed", Error: err.Error()})
	})
	s.pool.Hooks.OnStepStarted(func(p, step string) {
		s.ws.broadcast(wsEvent{Type: "step_started", Pipeline: p, Step: step, Status: "running"})
	})
	s.pool.Hooks.OnStepCompleted(func(p, step string, result runners.Result) {
		s.ws.broadcast(wsEvent{Type: "step_completed", Pipeline: p, Step: step, Status: "completed", Stdout: result.Stdout, Stderr: result.Stderr})
	})
	s.pool.Hooks.OnStepFailed(func(p, step string, err error) {
		s.ws.broadcast(wsEvent{Type: "step_failed", Pipeline: p, Step: step, Status: "failed", Error: err.Error()})
	})
}

// registerObservability - Wires observability collectors for hooks stats.
func (s *server) registerObservability() {
	s.collector.RegisterHooksSource("runners", s.pool.Hooks.Stats)
	s.collector.RegisterHooksSource("listeners", s.mgr.Hooks.Stats)
}

// printStartupSummary - Prints loaded pipelines and grouped ledger entries to the CLI.
func (s *server) printStartupSummary() {
	for _, p := range s.store.Pipelines() {
		out.info("pipeline loaded: %s (id=%s, steps=%d)", p.Name, p.ID, len(p.Steps))
	}
	for cat, entries := range s.ledger.Grouped() {
		for _, e := range entries {
			switch e.Level {
			case logging.LevelError:
				out.fail("[%s] %s: %s", cat, e.Source, e.Message)
			default:
				out.info("[%s] %s: %s", cat, e.Source, e.Message)
			}
		}
	}
}

// dispatchPipeline - Looks up a pipeline by name, creates a run, and submits it to the pool.
func (s *server) dispatchPipeline(ctx context.Context, name, source string) {
	p, err := s.store.GetPipelineByName(name)
	if err != nil {
		s.logger.Error(source+"_lookup_failed", "pipeline", name, "error", err)
		s.ledger.Error(logging.CatPipeline, "server", source+" target not found", map[string]any{
			"pipeline": name,
			"error":    err.Error(),
		})
		return
	}

	run, err := s.store.CreateRun(p.ID)
	if err != nil {
		s.logger.Error(source+"_run_create_failed", "pipeline", name, "error", err)
		return
	}
	s.activeRuns.Store(name, run)

	if err := s.pool.Submit(ctx, p.ToRunnersPipeline()); err != nil {
		s.logger.Error(source+"_submit_failed", "pipeline", name, "error", err)
		s.ledger.Error(logging.CatPipeline, "server", source+" submit failed", map[string]any{
			"pipeline": name,
			"error":    err.Error(),
		})
		run.Finish(err)
	}
}

// run - Starts the event loops and blocks until the context is cancelled.
func (s *server) run(ctx context.Context, cancel context.CancelFunc) {
	go s.handleSignals(cancel)
	go s.handleListenerEvents(ctx)
	go s.handlePipelineEvents(ctx)
	go s.watchPipelineDirectories(ctx)

	if s.cfg.APIAddr != "" {
		s.restAPI = newAPI(s, s.cfg.APIAddr)
		s.restAPI.start()
	}

	s.logger.Info("server_started")
	s.ledger.Info(logging.CatServer, "server", "server started", nil)
	out.info("server started")

	if err := s.mgr.Start(ctx); err != nil {
		s.logger.Error("listener_manager_error", "error", err)
	}

	s.pool.Wait()

	if s.restAPI != nil {
		s.restAPI.stop()
	}

	s.logger.Info("server_stopped")
	s.ledger.Info(logging.CatServer, "server", "server stopped", nil)
	out.info("server stopped")
	s.printStats()
}

// printStats - Prints collected observability stats to the CLI.
func (s *server) printStats() {
	for _, snap := range s.collector.All() {
		for _, h := range snap.Hooks {
			out.info("stats [%s] %s: listeners=%d emits=%d panics=%d",
				snap.Component, h.Name, h.Listeners, h.EmitCount, h.PanicsCaught)
		}
		for name, val := range snap.Counters {
			out.info("stats [%s] %s: %d", snap.Component, name, val)
		}
	}
}

// handleSignals - Listens for SIGINT/SIGTERM and cancels the context.
func (s *server) handleSignals(cancel context.CancelFunc) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	s.logger.Info("shutdown", "signal", sig.String())
	s.ledger.Info(logging.CatServer, "server", "shutting down", map[string]any{"signal": sig.String()})
	cancel()
}

// handleListenerEvents - Processes events from external listeners and dispatches pipelines.
func (s *server) handleListenerEvents(ctx context.Context) {
	for event := range s.mgr.Events() {
		s.ledger.Info(logging.CatPipeline, "server", "event received", map[string]any{
			"listener": event.Listener,
			"trigger":  event.Trigger,
			"pipeline": event.Pipeline,
		})
		s.logger.Info("event_received",
			"listener", event.Listener,
			"trigger", event.Trigger,
			"pipeline", event.Pipeline,
		)
		s.dispatchPipeline(ctx, event.Pipeline, "listener_event")
	}
}

// handlePipelineEvents - Processes events emitted by pipelines for chaining.
func (s *server) handlePipelineEvents(ctx context.Context) {
	for pe := range s.pipelineEvents {
		out.event("pipeline %s fired %q → %s", pe.Source, pe.Trigger, pe.Pipeline)
		s.ledger.Info(logging.CatPipeline, "server", "pipeline event emitted", map[string]any{
			"source":   pe.Source,
			"trigger":  pe.Trigger,
			"pipeline": pe.Pipeline,
		})
		s.dispatchPipeline(ctx, pe.Pipeline, "pipeline_event")
	}
}

// watchPipelineDirectories - Polls pipeline directories for YAML file changes
// and hot-reloads pipeline definitions when modifications are detected.
func (s *server) watchPipelineDirectories(ctx context.Context) {
	dirs := s.cfg.Pipelines.Directories
	if len(dirs) == 0 {
		return
	}

	modTimes := s.snapshotPipelineFiles(dirs)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			current := s.snapshotPipelineFiles(dirs)
			if !pipelineFilesChanged(modTimes, current) {
				continue
			}
			modTimes = current

			out.info("pipeline files changed, reloading...")
			loaded, errs := ReloadPipelinesFromDirectories(s.store, dirs)
			out.ok("pipelines reloaded: %d loaded", loaded)
			s.ws.broadcast(wsEvent{Type: "pipelines_reloaded", Steps: loaded})
			s.ledger.Info(logging.CatConfig, "server", "pipelines reloaded", map[string]any{
				"loaded": loaded,
				"errors": len(errs),
			})
			for _, err := range errs {
				out.fail("pipeline reload: %v", err)
				s.ledger.Error(logging.CatConfig, "server", "pipeline reload error", map[string]any{
					"error": err.Error(),
				})
			}
		}
	}
}

// snapshotPipelineFiles - Returns a map of YAML file paths to their modification times.
func (s *server) snapshotPipelineFiles(dirs []string) map[string]time.Time {
	snapshot := make(map[string]time.Time)
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext != ".yaml" && ext != ".yml" {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			snapshot[filepath.Join(dir, entry.Name())] = info.ModTime()
		}
	}
	return snapshot
}

// pipelineFilesChanged - Returns true if the file set or any modification time differs.
func pipelineFilesChanged(old, current map[string]time.Time) bool {
	if len(old) != len(current) {
		return true
	}
	for path, modTime := range current {
		if prev, ok := old[path]; !ok || !prev.Equal(modTime) {
			return true
		}
	}
	return false
}
