package domain

import (
	"context"
	"fmt"
)

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

// PipelineFailure is returned by Pipeline.ExecuteWithInvoker when a step fails
// mid-sequence. It carries the outputs of steps that succeeded so the caller
// can inspect partial state — e.g., to resume from the failed step or record
// what was already committed before the error.
//
// Aligned with Issue #9 phase 2: sequential capability chains no longer lose
// completed work on mid-sequence failure.
type PipelineFailure struct {
	FailedStep      int   // zero-based index of the step that errored
	CompletedOutput []any // outputs of steps [0, FailedStep), post-Transform
	Cause           error // the underlying error from the failed step
}

func (e *PipelineFailure) Error() string {
	return fmt.Sprintf("pipeline failed at step %d (after %d completed): %v",
		e.FailedStep, len(e.CompletedOutput), e.Cause)
}

func (e *PipelineFailure) Unwrap() error { return e.Cause }

// NewPipeline creates a Pipeline from a sequence of capability names.
func NewPipeline(capabilities ...CapabilityName) *Pipeline {
	steps := make([]PipelineStep, len(capabilities))
	for i, name := range capabilities {
		steps[i] = PipelineStep{Capability: name}
	}
	return &Pipeline{Steps: steps}
}

// ExecuteWithInvoker runs the pipeline by invoking each step in sequence.
// On step failure it returns a *PipelineFailure carrying the outputs of the
// steps that completed successfully.
func (p *Pipeline) ExecuteWithInvoker(ctx context.Context, input any, invoker CapabilityInvoker) (any, error) {
	current := input
	completed := make([]any, 0, len(p.Steps))
	for i, step := range p.Steps {
		result, err := invoker.Invoke(step.Capability, current)
		if err != nil {
			return nil, &PipelineFailure{
				FailedStep:      i,
				CompletedOutput: completed,
				Cause:           err,
			}
		}
		if step.Transform != nil {
			result = step.Transform(result)
		}
		completed = append(completed, result)
		current = result
	}
	return current, nil
}

// Execute satisfies CapabilityExecutor but requires an invoker — panics if called directly.
// Use ExecuteWithInvoker instead.
func (p *Pipeline) Execute(_ context.Context, _ any) (any, error) {
	return nil, &ErrValidation{Message: "pipeline must be executed with an invoker (use ExecuteWithInvoker)"}
}
