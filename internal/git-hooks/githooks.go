package githooks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"lp/internal/listener"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const shimTemplate = `#!/bin/sh
# Installed by lp git-hook listener (%s). Do not edit.
# Original hook (if any) backed up as %s.lp-backup
curl -s -X POST "http://127.0.0.1:%d/%s" \
  --header "Content-Type: application/json" \
  --data "{\"args\": \"$*\"}" \
  --max-time 5 >/dev/null 2>&1 || true
# Run the backed-up original hook if it exists.
if [ -x "%s.lp-backup" ]; then
  exec "%s.lp-backup" "$@"
fi
`

// GitHook - Listener that installs shim scripts into a Git repository's
// .git/hooks directory. When Git invokes a hook, the shim calls a local
// HTTP server that emits an LP event to trigger the configured pipeline.
type GitHook struct {
	name     string
	pipeline string
	repo     string
	hooks    []string
	port     int
	done     chan struct{}
	once     sync.Once
	server   *http.Server
}

// New - Creates a GitHook listener.
// Parameters:
//   - name: listener identifier
//   - pipeline: pipeline name to trigger
//   - repo: absolute path to the Git repository root
//   - hooks: list of Git hook names to install shims for (e.g. "pre-commit")
func New(name, pipeline, repo string, hooks []string) (*GitHook, error) {
	if repo == "" {
		return nil, fmt.Errorf("git_hook requires 'repo' in config")
	}
	if len(hooks) == 0 {
		return nil, fmt.Errorf("git_hook requires at least one hook name in 'hooks'")
	}

	gitDir := filepath.Join(repo, ".git")
	if info, err := os.Stat(gitDir); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("git_hook: %q does not appear to be a git repository (no .git directory)", repo)
	}

	return &GitHook{
		name:     name,
		pipeline: pipeline,
		repo:     repo,
		hooks:    hooks,
		done:     make(chan struct{}),
	}, nil
}

// Name - Returns the listener's identifier.
func (g *GitHook) Name() string { return g.name }

// Start - Installs hook shims, starts the local HTTP server, and sends
// events on the channel when Git triggers an installed hook.
func (g *GitHook) Start(ctx context.Context, events chan<- listener.Event) error {
	// Bind to a random port on localhost.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("git_hook %q listen: %w", g.name, err)
	}
	g.port = ln.Addr().(*net.TCPAddr).Port

	// Install shim scripts.
	if err := g.installShims(); err != nil {
		_ = ln.Close()
		return fmt.Errorf("git_hook %q install shims: %w", g.name, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", g.handler(events))
	g.server = &http.Server{Handler: mux}

	go func() {
		select {
		case <-ctx.Done():
		case <-g.done:
		}
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = g.server.Shutdown(shutCtx)
	}()

	if err := g.server.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("git_hook %q serve: %w", g.name, err)
	}
	return nil
}

// Stop - Removes installed shim scripts, restores backups, and stops the server.
func (g *GitHook) Stop() error {
	g.removeShims()
	g.once.Do(func() { close(g.done) })
	return nil
}

// handler - Returns an http.HandlerFunc that converts hook invocations to events.
func (g *GitHook) handler(events chan<- listener.Event) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		hookName := strings.TrimPrefix(r.URL.Path, "/")
		if hookName == "" {
			http.Error(rw, "missing hook name", http.StatusBadRequest)
			return
		}

		payload := map[string]any{
			"hook": hookName,
			"repo": g.repo,
		}

		if r.Body != nil {
			defer r.Body.Close()
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			if err == nil && len(body) > 0 {
				var parsed map[string]any
				if json.Unmarshal(body, &parsed) == nil {
					for k, v := range parsed {
						payload[k] = v
					}
				}
			}
		}

		events <- listener.Event{
			Listener: g.name,
			Trigger:  hookName,
			Pipeline: g.pipeline,
			Payload:  payload,
		}

		rw.WriteHeader(http.StatusAccepted)
		_, _ = rw.Write([]byte(`{"status":"accepted"}` + "\n"))
	}
}

// installShims - Writes shim scripts into .git/hooks for each configured hook.
// Existing hooks are backed up to <hook>.lp-backup.
func (g *GitHook) installShims() error {
	hooksDir := filepath.Join(g.repo, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return fmt.Errorf("creating hooks dir: %w", err)
	}

	for _, hook := range g.hooks {
		hookPath := filepath.Join(hooksDir, hook)
		backupPath := hookPath + ".lp-backup"

		// Back up existing hook if present.
		if _, err := os.Stat(hookPath); err == nil {
			if err := os.Rename(hookPath, backupPath); err != nil {
				return fmt.Errorf("backing up existing hook %q: %w", hook, err)
			}
		}

		shim := fmt.Sprintf(shimTemplate, g.name, hookPath, g.port, hook, hookPath, hookPath)
		if err := os.WriteFile(hookPath, []byte(shim), 0o755); err != nil {
			return fmt.Errorf("writing shim for %q: %w", hook, err)
		}
	}

	return nil
}

// removeShims - Removes installed shim scripts and restores backups.
func (g *GitHook) removeShims() {
	hooksDir := filepath.Join(g.repo, ".git", "hooks")

	for _, hook := range g.hooks {
		hookPath := filepath.Join(hooksDir, hook)
		backupPath := hookPath + ".lp-backup"

		_ = os.Remove(hookPath)

		// Restore backup if it exists.
		if _, err := os.Stat(backupPath); err == nil {
			_ = os.Rename(backupPath, hookPath)
		}
	}
}

