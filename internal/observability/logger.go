package observability

import (
	"io"
	"log/slog"
	"os"
)

// Logger - Structured logger scoped to a named component.
type Logger struct {
	inner *slog.Logger
}

// NewLogger - Creates a Logger with a JSON handler writing to stderr, scoped to the given component.
func NewLogger(component string) *Logger {
	return NewLoggerTo(component, os.Stderr)
}

// NewLoggerTo - Creates a Logger writing JSON to the given writer, scoped to the given component.
func NewLoggerTo(component string, w io.Writer) *Logger {
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return &Logger{
		inner: slog.New(handler).With("component", component),
	}
}

// NewLoggerWithLevel - Creates a Logger with a JSON handler at the specified level.
func NewLoggerWithLevel(component string, level slog.Level) *Logger {
	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})
	return &Logger{
		inner: slog.New(handler).With("component", component),
	}
}

// With - Returns a new Logger with additional key-value context.
func (l *Logger) With(args ...any) *Logger {
	return &Logger{inner: l.inner.With(args...)}
}

// Info - Logs an informational message with optional key-value pairs.
func (l *Logger) Info(msg string, args ...any) {
	l.inner.Info(msg, args...)
}

// Error - Logs an error message with optional key-value pairs.
func (l *Logger) Error(msg string, args ...any) {
	l.inner.Error(msg, args...)
}

// Warn - Logs a warning message with optional key-value pairs.
func (l *Logger) Warn(msg string, args ...any) {
	l.inner.Warn(msg, args...)
}

// Debug - Logs a debug message with optional key-value pairs.
func (l *Logger) Debug(msg string, args ...any) {
	l.inner.Debug(msg, args...)
}

