package pipeline

import "lp/internal/runners"

// Pipeline - A named, YAML-defined pipeline with a unique ID and ordered steps.
type Pipeline struct {
	ID    string    `yaml:"-"`
	Name  string    `yaml:"name"`
	Steps []StepDef `yaml:"steps"`
	Emit  []EmitDef `yaml:"emit"`
}

// NewPipeline - Creates a Pipeline with a generated UUID.
func NewPipeline(name string, steps []StepDef) *Pipeline {
	return &Pipeline{
		ID:    newID(),
		Name:  name,
		Steps: steps,
	}
}

// ToRunnersPipeline - Converts to a runners.Pipeline for execution by the pool.
func (p *Pipeline) ToRunnersPipeline() *runners.Pipeline {
	steps := make([]runners.Step, len(p.Steps))
	for i := range p.Steps {
		steps[i] = p.Steps[i].toRunnerStep()
	}
	emit := make([]runners.EmitDef, len(p.Emit))
	for i, e := range p.Emit {
		emit[i] = runners.EmitDef{Event: e.Event, Pipeline: e.Pipeline}
	}
	rp := runners.NewPipeline(p.Name, steps...)
	rp.Emit = emit
	return rp
}

