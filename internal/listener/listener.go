package listener

import "context"

// Listener - Interface that all trigger types must implement.
// A Listener monitors one or more triggers and sends Events on the
// provided channel when conditions are met.
type Listener interface {
	// Name - Returns the listener's identifier for logging and diagnostics.
	Name() string

	// Start - Begins monitoring triggers. Events are sent on the channel.
	// The listener must stop when the context is cancelled.
	// Start blocks until the context is done or a fatal error occurs.
	Start(ctx context.Context, events chan<- Event) error

	// Stop - Signals the listener to gracefully shut down.
	Stop() error
}

