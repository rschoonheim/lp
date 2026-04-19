package pipeline

import "lp/internal/runners"

// StepDef - Declarative definition of a pipeline step, parsed from YAML.
type StepDef struct {
	Name    string     `yaml:"name"`
	Command string     `yaml:"command"`
	Args    []string   `yaml:"args"`
	Dir     string     `yaml:"dir"`
	Env     []string   `yaml:"env"`
	Emit    []EmitDef  `yaml:"emit"`
}

// EmitDef - Declarative event emission, parsed from YAML.
// Fires an event that can trigger another pipeline.
type EmitDef struct {
	// Event is the trigger name for the emitted event.
	Event string `yaml:"event"`
	// Pipeline is the target pipeline name to trigger.
	Pipeline string `yaml:"pipeline"`
}

// toRunnerStep - Converts a StepDef to a runners.Step for execution.
func (d *StepDef) toRunnerStep() runners.Step {
	emit := make([]runners.EmitDef, len(d.Emit))
	for i, e := range d.Emit {
		emit[i] = runners.EmitDef{Event: e.Event, Pipeline: e.Pipeline}
	}
	return runners.Step{
		Name:    d.Name,
		Command: d.Command,
		Args:    d.Args,
		Dir:     d.Dir,
		Env:     d.Env,
		Emit:    emit,
	}
}

// StepResult - Captured output and status of a single step execution within a Run.
type StepResult struct {
	Name   string
	Status Status
	Output runners.Result
	Error  string
}

