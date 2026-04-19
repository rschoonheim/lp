package listener

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Webhook - Listener that starts an HTTP server and fires events when
// requests match a configured set of event names. Clients POST to
// /{event_name} with an optional JSON body that becomes the event payload.
type Webhook struct {
	name     string
	pipeline string
	addr     string
	events   map[string]bool
	done     chan struct{}
	once     sync.Once
	server   *http.Server
}

// NewWebhook - Creates a Webhook listener.
// Parameters:
//   - name: listener identifier
//   - pipeline: pipeline name to trigger
//   - addr: address to bind (e.g. ":9090")
//   - events: list of accepted event names; empty means accept all
func NewWebhook(name, pipeline, addr string, events []string) (*Webhook, error) {
	if addr == "" {
		return nil, fmt.Errorf("webhook requires 'addr' in config")
	}

	accepted := make(map[string]bool, len(events))
	for _, e := range events {
		accepted[strings.TrimSpace(e)] = true
	}

	return &Webhook{
		name:     name,
		pipeline: pipeline,
		addr:     addr,
		events:   accepted,
		done:     make(chan struct{}),
	}, nil
}

// Name - Returns the listener's identifier.
func (w *Webhook) Name() string { return w.name }

// Start - Starts the HTTP server and sends events on the channel when
// a matching request is received. Blocks until the context is cancelled.
func (w *Webhook) Start(ctx context.Context, events chan<- Event) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		eventName := strings.TrimPrefix(r.URL.Path, "/")
		if eventName == "" {
			http.Error(rw, "missing event name in path", http.StatusBadRequest)
			return
		}

		if len(w.events) > 0 && !w.events[eventName] {
			http.Error(rw, fmt.Sprintf("event %q not accepted", eventName), http.StatusNotFound)
			return
		}

		payload := map[string]any{
			"event":  eventName,
			"method": r.Method,
			"remote": r.RemoteAddr,
		}

		if r.Body != nil {
			defer r.Body.Close()
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MB limit
			if err == nil && len(body) > 0 {
				var parsed map[string]any
				if json.Unmarshal(body, &parsed) == nil {
					payload["body"] = parsed
				} else {
					payload["body"] = string(body)
				}
			}
		}

		events <- Event{
			Listener: w.name,
			Trigger:  eventName,
			Pipeline: w.pipeline,
			Payload:  payload,
		}

		rw.WriteHeader(http.StatusAccepted)
		_, _ = rw.Write([]byte(`{"status":"accepted"}` + "\n"))
	})

	ln, err := net.Listen("tcp", w.addr)
	if err != nil {
		return fmt.Errorf("webhook %q listen: %w", w.name, err)
	}

	w.server = &http.Server{Handler: mux}

	go func() {
		select {
		case <-ctx.Done():
		case <-w.done:
		}
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = w.server.Shutdown(shutCtx)
	}()

	if err := w.server.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("webhook %q serve: %w", w.name, err)
	}
	return nil
}

// Stop - Signals the webhook server to shut down.
func (w *Webhook) Stop() error {
	w.once.Do(func() { close(w.done) })
	return nil
}

