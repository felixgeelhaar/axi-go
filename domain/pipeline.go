package domain

import "context"

// PipelineStep defines one step in a capability pipeline.
type PipelineStep struct {
	Capability CapabilityName
	// Transform optionally transforms the output of this step before passing
	// to the next. If nil, output is passed directly.
	Transform func(output any) any
}

// Pipeline is a composable capability executor that chains multiple capabilities
// sequentially, piping the output of each step as input to the next.
type Pipeline struct {
	Steps []PipelineStep
}

// NewPipeline creates a Pipeline from a sequence of capability names.
func NewPipeline(capabilities ...CapabilityName) *Pipeline {
	steps := make([]PipelineStep, len(capabilities))
	for i, name := range capabilities {
		steps[i] = PipelineStep{Capability: name}
	}
	return &Pipeline{Steps: steps}
}

// ExecuteWithInvoker runs the pipeline by invoking each step in sequence.
func (p *Pipeline) ExecuteWithInvoker(ctx context.Context, input any, invoker CapabilityInvoker) (any, error) {
	current := input
	for _, step := range p.Steps {
		result, err := invoker.Invoke(step.Capability, current)
		if err != nil {
			return nil, err
		}
		if step.Transform != nil {
			result = step.Transform(result)
		}
		current = result
	}
	return current, nil
}

// Execute satisfies CapabilityExecutor but requires an invoker — panics if called directly.
// Use ExecuteWithInvoker instead.
func (p *Pipeline) Execute(_ context.Context, _ any) (any, error) {
	return nil, &ErrValidation{Message: "pipeline must be executed with an invoker (use ExecuteWithInvoker)"}
}
