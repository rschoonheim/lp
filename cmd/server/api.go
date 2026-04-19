package main

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
	"time"
)

// api - REST API server for querying LP server state and history.
type api struct {
	srv    *server
	httpSrv *http.Server
}

// newAPI - Creates an API bound to the given server and listen address.
func newAPI(srv *server, addr string) *api {
	a := &api{srv: srv}

	mux := http.NewServeMux()

	// Dashboard
	dashboardFile, _ := fs.ReadFile(dashboardFS, "dashboard.html")
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(dashboardFile)
	})

	mux.HandleFunc("/api/status", a.handleStatus)
	mux.HandleFunc("/api/history", a.handleHistory)
	mux.HandleFunc("/api/pipelines", a.handlePipelines)
	mux.HandleFunc("/api/pipelines/", a.handlePipelineRuns)
	mux.HandleFunc("/api/listeners", a.handleListeners)
	mux.HandleFunc("/ws", a.srv.ws.handleUpgrade)

	a.httpSrv = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return a
}

// start - Starts the HTTP server in a goroutine. Logs the listen address.
func (a *api) start() {
	out.info("api listening on %s", a.httpSrv.Addr)
	go func() {
		if err := a.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			out.fail("api server error: %v", err)
		}
	}()
}

// stop - Gracefully shuts down the HTTP server.
func (a *api) stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = a.httpSrv.Shutdown(ctx)
}

// writeJSON - Writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// handleStatus - GET /api/status — returns current server context.
func (a *api) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pipelines := a.srv.store.Pipelines()
	pipelineNames := make([]string, len(pipelines))
	for i, p := range pipelines {
		pipelineNames[i] = p.Name
	}

	stats := make([]map[string]any, 0)
	for _, snap := range a.srv.collector.All() {
		for _, h := range snap.Hooks {
			stats = append(stats, map[string]any{
				"component":     snap.Component,
				"hook":          h.Name,
				"listeners":     h.Listeners,
				"emit_count":    h.EmitCount,
				"panics_caught": h.PanicsCaught,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "running",
		"pipelines": pipelineNames,
		"config": map[string]any{
			"timeout":        a.srv.cfg.Runners.Timeout,
			"max_concurrent": a.srv.cfg.Runners.MaxConcurrent,
		},
		"stats": stats,
	})
}

// handleHistory - GET /api/history — returns ledger entries.
// Optional query params: ?category=pipeline&level=error
func (a *api) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	entries := a.srv.ledger.All()

	catFilter := r.URL.Query().Get("category")
	levelFilter := r.URL.Query().Get("level")

	type entryJSON struct {
		Time     time.Time      `json:"time"`
		Level    string         `json:"level"`
		Category string         `json:"category"`
		Source   string         `json:"source"`
		Message  string         `json:"message"`
		Fields   map[string]any `json:"fields,omitempty"`
	}

	result := make([]entryJSON, 0, len(entries))
	for _, e := range entries {
		if catFilter != "" && string(e.Category) != catFilter {
			continue
		}
		if levelFilter != "" && !strings.EqualFold(e.Level.String(), levelFilter) {
			continue
		}
		result = append(result, entryJSON{
			Time:     e.Time,
			Level:    e.Level.String(),
			Category: string(e.Category),
			Source:   e.Source,
			Message:  e.Message,
			Fields:   e.Fields,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total":   len(result),
		"entries": result,
	})
}

// handlePipelines - GET /api/pipelines — returns all registered pipelines.
func (a *api) handlePipelines(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Only match exact /api/pipelines, not /api/pipelines/...
	if r.URL.Path != "/api/pipelines" {
		http.NotFound(w, r)
		return
	}

	pipelines := a.srv.store.Pipelines()

	type stepJSON struct {
		Name    string `json:"name"`
		Command string `json:"command"`
	}
	type pipelineJSON struct {
		ID    string     `json:"id"`
		Name  string     `json:"name"`
		Steps []stepJSON `json:"steps"`
	}

	result := make([]pipelineJSON, 0, len(pipelines))
	for _, p := range pipelines {
		steps := make([]stepJSON, len(p.Steps))
		for i, s := range p.Steps {
			steps[i] = stepJSON{Name: s.Name, Command: s.Command}
		}
		result = append(result, pipelineJSON{
			ID:    p.ID,
			Name:  p.Name,
			Steps: steps,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

// handlePipelineRuns - GET /api/pipelines/{name}/runs — returns runs for a pipeline.
func (a *api) handlePipelineRuns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse /api/pipelines/{name}/runs
	path := strings.TrimPrefix(r.URL.Path, "/api/pipelines/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[1] != "runs" || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	name := parts[0]

	p, err := a.srv.store.GetPipelineByName(name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	runs := a.srv.store.RunsFor(p.ID)

	type stepResultJSON struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Stdout string `json:"stdout,omitempty"`
		Stderr string `json:"stderr,omitempty"`
		Error  string `json:"error,omitempty"`
	}
	type runJSON struct {
		ID         string           `json:"id"`
		PipelineID string           `json:"pipeline_id"`
		Status     string           `json:"status"`
		StartedAt  *time.Time       `json:"started_at,omitempty"`
		FinishedAt *time.Time       `json:"finished_at,omitempty"`
		Duration   string           `json:"duration,omitempty"`
		Error      string           `json:"error,omitempty"`
		Steps      []stepResultJSON `json:"steps"`
	}

	result := make([]runJSON, 0, len(runs))
	for _, run := range runs {
		steps := make([]stepResultJSON, len(run.Steps))
		for i, s := range run.Steps {
			steps[i] = stepResultJSON{
				Name:   s.Name,
				Status: string(s.Status),
				Stdout: s.Output.Stdout,
				Stderr: s.Output.Stderr,
				Error:  s.Error,
			}
		}
		rj := runJSON{
			ID:         run.ID,
			PipelineID: run.PipelineID,
			Status:     string(run.Status),
			StartedAt:  run.StartedAt,
			FinishedAt: run.FinishedAt,
			Error:      run.Error,
			Steps:      steps,
		}
		if d := run.Duration(); d > 0 {
			rj.Duration = d.String()
		}
		result = append(result, rj)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"pipeline": name,
		"total":    len(result),
		"runs":     result,
	})
}

// handleListeners - GET /api/listeners — returns all configured listeners.
func (a *api) handleListeners(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	type listenerJSON struct {
		Name     string         `json:"name"`
		Type     string         `json:"type"`
		Pipeline string         `json:"pipeline"`
		Config   map[string]any `json:"config,omitempty"`
	}

	entries := a.srv.cfg.Listeners
	result := make([]listenerJSON, 0, len(entries))
	for _, e := range entries {
		result = append(result, listenerJSON{
			Name:     e.Name,
			Type:     e.Type,
			Pipeline: e.Pipeline,
			Config:   e.Config,
		})
	}

	writeJSON(w, http.StatusOK, result)
}
