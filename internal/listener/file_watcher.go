package listener

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// FileWatcher - Listener that polls a directory for file changes matching a pattern.
type FileWatcher struct {
	name     string
	path     string
	pattern  string
	pipeline string
	interval time.Duration
	done     chan struct{}
	modTimes map[string]time.Time
}

// NewFileWatcher - Creates a FileWatcher listener.
// Config keys: path (required), pattern (optional, default "*"), interval (optional, seconds, default 2).
func NewFileWatcher(name, pipeline, path, pattern string, interval time.Duration) (*FileWatcher, error) {
	if path == "" {
		return nil, fmt.Errorf("file_watcher requires 'path' in config")
	}
	if pattern == "" {
		pattern = "*"
	}
	if interval == 0 {
		interval = 2 * time.Second
	}
	return &FileWatcher{
		name:     name,
		path:     path,
		pattern:  pattern,
		pipeline: pipeline,
		interval: interval,
		done:     make(chan struct{}),
		modTimes: make(map[string]time.Time),
	}, nil
}

// Name - Returns the listener's identifier.
func (w *FileWatcher) Name() string { return w.name }

// Start - Polls the configured path for file changes and sends events.
func (w *FileWatcher) Start(ctx context.Context, events chan<- Event) error {
	// Initial scan to populate modTimes without firing events
	_ = w.scan(nil)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-w.done:
			return nil
		case <-ticker.C:
			if err := w.scan(events); err != nil {
				return fmt.Errorf("file_watcher %q scan: %w", w.name, err)
			}
		}
	}
}

// Stop - Signals the watcher to shut down.
func (w *FileWatcher) Stop() error {
	select {
	case <-w.done:
	default:
		close(w.done)
	}
	return nil
}

// scan - Walks the path, checks for new or modified files matching the pattern,
// and sends events for changes. Pass nil events channel for initial population.
func (w *FileWatcher) scan(events chan<- Event) error {
	entries, err := os.ReadDir(w.path)
	if err != nil {
		return fmt.Errorf("reading directory %q: %w", w.path, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matched, _ := filepath.Match(w.pattern, entry.Name())
		if !matched {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		fullPath := filepath.Join(w.path, entry.Name())
		prevMod, seen := w.modTimes[fullPath]
		w.modTimes[fullPath] = info.ModTime()

		if events != nil && (!seen || info.ModTime().After(prevMod)) {
			events <- Event{
				Listener: w.name,
				Trigger:  "file_changed",
				Pipeline: w.pipeline,
				Payload: map[string]any{
					"path": fullPath,
					"file": entry.Name(),
				},
			}
		}
	}

	return nil
}

