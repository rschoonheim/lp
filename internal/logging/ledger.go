package logging

import (
	"sync"
	"time"
)

// Level - Severity level for a log entry.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// String - Returns the human-readable name of the level.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Category - Semantic grouping for log entries so they can be filtered by meaning.
type Category string

const (
	CatPipeline Category = "pipeline"
	CatStep     Category = "step"
	CatHook     Category = "hook"
	CatConfig   Category = "config"
	CatServer   Category = "server"
)

// Entry - A single log record with semantic metadata for grouping.
type Entry struct {
	Time     time.Time
	Level    Level
	Category Category
	Source   string
	Message  string
	Fields   map[string]any
}

// Ledger - Thread-safe, append-only log collector. Entries are stored in order
// and can be retrieved all at once or filtered by category.
type Ledger struct {
	mu      sync.RWMutex
	entries []Entry
}

// NewLedger - Creates an empty Ledger.
func NewLedger() *Ledger {
	return &Ledger{}
}

// Append - Adds a pre-built Entry to the ledger.
func (l *Ledger) Append(e Entry) {
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	l.mu.Lock()
	l.entries = append(l.entries, e)
	l.mu.Unlock()
}

// Log - Convenience method to append an entry with the given parameters.
func (l *Ledger) Log(level Level, category Category, source, message string, fields map[string]any) {
	l.Append(Entry{
		Time:     time.Now(),
		Level:    level,
		Category: category,
		Source:   source,
		Message:  message,
		Fields:   fields,
	})
}

// Info - Appends an info-level entry.
func (l *Ledger) Info(category Category, source, message string, fields map[string]any) {
	l.Log(LevelInfo, category, source, message, fields)
}

// Error - Appends an error-level entry.
func (l *Ledger) Error(category Category, source, message string, fields map[string]any) {
	l.Log(LevelError, category, source, message, fields)
}

// Warn - Appends a warn-level entry.
func (l *Ledger) Warn(category Category, source, message string, fields map[string]any) {
	l.Log(LevelWarn, category, source, message, fields)
}

// Debug - Appends a debug-level entry.
func (l *Ledger) Debug(category Category, source, message string, fields map[string]any) {
	l.Log(LevelDebug, category, source, message, fields)
}

// All - Returns a copy of all entries in insertion order.
func (l *Ledger) All() []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]Entry, len(l.entries))
	copy(out, l.entries)
	return out
}

// ByCategory - Returns entries matching the given category.
func (l *Ledger) ByCategory(cat Category) []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	var out []Entry
	for _, e := range l.entries {
		if e.Category == cat {
			out = append(out, e)
		}
	}
	return out
}

// ByLevel - Returns entries at or above the given severity level.
func (l *Ledger) ByLevel(min Level) []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	var out []Entry
	for _, e := range l.entries {
		if e.Level >= min {
			out = append(out, e)
		}
	}
	return out
}

// BySource - Returns entries from the given source.
func (l *Ledger) BySource(source string) []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	var out []Entry
	for _, e := range l.entries {
		if e.Source == source {
			out = append(out, e)
		}
	}
	return out
}

// Grouped - Returns all entries grouped by category.
func (l *Ledger) Grouped() map[Category][]Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	groups := make(map[Category][]Entry)
	for _, e := range l.entries {
		groups[e.Category] = append(groups[e.Category], e)
	}
	return groups
}

// Len - Returns the total number of entries.
func (l *Ledger) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.entries)
}

// Clear - Removes all entries. Intended for testing.
func (l *Ledger) Clear() {
	l.mu.Lock()
	l.entries = nil
	l.mu.Unlock()
}

