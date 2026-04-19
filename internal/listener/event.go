package listener

// Event - A signal produced by a Listener when a trigger condition is met.
type Event struct {
	// Listener is the name of the listener that produced this event.
	Listener string
	// Trigger is the name of the specific trigger that fired.
	Trigger string
	// Pipeline is the name or ID of the pipeline to execute.
	Pipeline string
	// Payload holds trigger-specific data passed to the pipeline run.
	Payload map[string]any
}

