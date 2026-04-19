package main

import (
	"fmt"
	"lp/internal/listener"
	"time"
)

// ListenerFactory - Function type that creates a Listener from a config entry.
type ListenerFactory func(entry ListenerEntry) (listener.Listener, error)

// listenerFactories - Registry of known listener type constructors.
var listenerFactories = map[string]ListenerFactory{
	"file_watcher": newFileWatcherFromConfig,
	"webhook":      newWebhookFromConfig,
}

// RegisterListenerFactory - Registers a factory for a listener type.
// Call during init() or before RegisterConfiguredListeners.
func RegisterListenerFactory(typeName string, factory ListenerFactory) {
	listenerFactories[typeName] = factory
}

// RegisterConfiguredListeners - Creates listeners from YAML config entries
// and registers them with the Manager. Returns count registered and any errors.
func RegisterConfiguredListeners(mgr *listener.Manager, entries []ListenerEntry) (int, []error) {
	var registered int
	var errs []error

	for _, entry := range entries {
		if entry.Name == "" {
			errs = append(errs, fmt.Errorf("listener entry missing 'name'"))
			continue
		}
		if entry.Type == "" {
			errs = append(errs, fmt.Errorf("listener %q: missing 'type'", entry.Name))
			continue
		}

		factory, ok := listenerFactories[entry.Type]
		if !ok {
			errs = append(errs, fmt.Errorf("listener %q: unknown type %q", entry.Name, entry.Type))
			continue
		}

		l, err := factory(entry)
		if err != nil {
			errs = append(errs, fmt.Errorf("listener %q (%s): %w", entry.Name, entry.Type, err))
			continue
		}

		mgr.Register(l)
		registered++
	}

	return registered, errs
}

// newFileWatcherFromConfig - Creates a FileWatcher from a ListenerEntry's config map.
func newFileWatcherFromConfig(entry ListenerEntry) (listener.Listener, error) {
	path, _ := entry.Config["path"].(string)
	pattern, _ := entry.Config["pattern"].(string)

	var interval time.Duration
	if v, ok := entry.Config["interval"]; ok {
		switch iv := v.(type) {
		case int:
			interval = time.Duration(iv) * time.Second
		case float64:
			interval = time.Duration(iv) * time.Second
		}
	}

	return listener.NewFileWatcher(entry.Name, entry.Pipeline, path, pattern, interval)
}

// newWebhookFromConfig - Creates a Webhook listener from a ListenerEntry's config map.
func newWebhookFromConfig(entry ListenerEntry) (listener.Listener, error) {
	addr, _ := entry.Config["addr"].(string)

	var events []string
	if v, ok := entry.Config["events"]; ok {
		switch ev := v.(type) {
		case []any:
			for _, item := range ev {
				if s, ok := item.(string); ok {
					events = append(events, s)
				}
			}
		case []string:
			events = ev
		}
	}

	return listener.NewWebhook(entry.Name, entry.Pipeline, addr, events)
}
