package runners

// Pipeline represents a sequence of steps to execute.
type Pipeline struct {
	Name  string
	Steps []Step
	// Emit lists events to fire after the pipeline completes successfully.
	Emit []EmitDef
}

// NewPipeline - Creates a pipeline with the given name and steps.
func NewPipeline(name string, steps ...Step) *Pipeline {
	return &Pipeline{Name: name, Steps: steps}
}

