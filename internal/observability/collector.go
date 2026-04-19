package observability

import "sync"

// Snapshot - Point-in-time observability state for a named component.
type Snapshot struct {
	Component string
	Hooks     []Stats
	Counters  map[string]uint64
}

// Collector - Aggregates observability data from components that expose Stats and counters.
type Collector struct {
	mu         sync.RWMutex
	components map[string]func() []Stats
	counters   map[string]map[string]*counter
	logger     *Logger
}

type counter struct {
	val uint64
	mu  sync.Mutex
}

// NewCollector - Creates a Collector with an optional logger for reporting.
func NewCollector(logger *Logger) *Collector {
	return &Collector{
		components: make(map[string]func() []Stats),
		counters:   make(map[string]map[string]*counter),
		logger:     logger,
	}
}

// RegisterHooksSource - Registers a named component's Stats provider for aggregation.
func (c *Collector) RegisterHooksSource(component string, fn func() []Stats) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.components[component] = fn
}

// Increment - Increments a named counter for a component.
func (c *Collector) Increment(component, name string) {
	c.mu.Lock()
	m, ok := c.counters[component]
	if !ok {
		m = make(map[string]*counter)
		c.counters[component] = m
	}
	ctr, ok := m[name]
	if !ok {
		ctr = &counter{}
		m[name] = ctr
	}
	c.mu.Unlock()

	ctr.mu.Lock()
	ctr.val++
	ctr.mu.Unlock()
}

// Snapshot - Returns a point-in-time snapshot for a single component.
func (c *Collector) Snapshot(component string) Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	snap := Snapshot{
		Component: component,
		Counters:  make(map[string]uint64),
	}

	if fn, ok := c.components[component]; ok {
		snap.Hooks = fn()
	}

	if m, ok := c.counters[component]; ok {
		for name, ctr := range m {
			ctr.mu.Lock()
			snap.Counters[name] = ctr.val
			ctr.mu.Unlock()
		}
	}

	return snap
}

// All - Returns snapshots for every registered component.
func (c *Collector) All() []Snapshot {
	c.mu.RLock()
	names := make(map[string]struct{})
	for name := range c.components {
		names[name] = struct{}{}
	}
	for name := range c.counters {
		names[name] = struct{}{}
	}
	c.mu.RUnlock()

	snapshots := make([]Snapshot, 0, len(names))
	for name := range names {
		snapshots = append(snapshots, c.Snapshot(name))
	}
	return snapshots
}

// Log - Logs the current state of all registered components at Info level.
func (c *Collector) Log() {
	if c.logger == nil {
		return
	}
	for _, snap := range c.All() {
		for _, h := range snap.Hooks {
			c.logger.Info("hook_stats",
				"source", snap.Component,
				"hook", h.Name,
				"listeners", h.Listeners,
				"emit_count", h.EmitCount,
				"panics_caught", h.PanicsCaught,
			)
		}
		for name, val := range snap.Counters {
			c.logger.Info("counter",
				"source", snap.Component,
				"counter", name,
				"value", val,
			)
		}
	}
}

