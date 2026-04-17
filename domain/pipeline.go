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
	// Compensate, when set, is invoked to undo this step's effect if a later
	// step in the same pipeline fails. It receives the post-Transform output
	// of this step. Compensation runs in reverse order of completion and is
	// best-effort — errors are collected on PipelineFailure.CompensationErrors
	// without masking the primary failure cause. Aligned with Issue #9 phase 3
	// (saga-lite).
	Compensate func(ctx context.Context, output any) error
}

// Pipeline is a composable capability executor that chains multiple capabilities
// sequentially, piping the output of each step as input to the next.
type Pipeline struct {
	Steps []PipelineStep
}

// PipelineFailure is returned by Pipeline.ExecuteWithInvoker when a step fails
// mid-sequence. It carries the outputs of steps that succeeded so the caller
// can inspect partial state — e.g., to resume from the failed step or record
// what was already committed before the error. When steps define a Compensate
// hook, any errors raised during compensation are surfaced via
// CompensationErrors without overriding the primary Cause.
//
// Aligned with Issue #9 phases 2 (partial state) and 3 (saga-lite).
type PipelineFailure struct {
	FailedStep         int     // zero-based index of the step that errored
	CompletedOutput    []any   // outputs of steps [0, FailedStep), post-Transform
	Cause              error   // the underlying error from the failed step
	CompensationErrors []error // best-effort compensation failures, reverse order
}

func (e *PipelineFailure) Error() string {
	msg := fmt.Sprintf("pipeline failed at step %d (after %d completed): %v",
		e.FailedStep, len(e.CompletedOutput), e.Cause)
	if len(e.CompensationErrors) > 0 {
		msg += fmt.Sprintf(" (compensation raised %d error(s))", len(e.CompensationErrors))
	}
	return msg
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
// steps that completed successfully. When failed steps' predecessors define
// Compensate hooks, they are invoked in reverse order of completion (saga
// pattern).
func (p *Pipeline) ExecuteWithInvoker(ctx context.Context, input any, invoker CapabilityInvoker) (any, error) {
	current := input
	completed := make([]any, 0, len(p.Steps))
	for i, step := range p.Steps {
		result, err := invoker.Invoke(step.Capability, current)
		if err != nil {
			failure := &PipelineFailure{
				FailedStep:      i,
				CompletedOutput: completed,
				Cause:           err,
			}
			p.compensate(ctx, completed, failure)
			return nil, failure
		}
		if step.Transform != nil {
			result = step.Transform(result)
		}
		completed = append(completed, result)
		current = result
	}
	return current, nil
}

// compensate invokes Compensate on previously completed steps in reverse order.
// Errors are collected on the PipelineFailure but do not abort compensation of
// earlier steps. Context cancellation stops the walk.
func (p *Pipeline) compensate(ctx context.Context, completed []any, failure *PipelineFailure) {
	for i := len(completed) - 1; i >= 0; i-- {
		if ctx.Err() != nil {
			failure.CompensationErrors = append(failure.CompensationErrors, ctx.Err())
			return
		}
		step := p.Steps[i]
		if step.Compensate == nil {
			continue
		}
		if err := step.Compensate(ctx, completed[i]); err != nil {
			failure.CompensationErrors = append(failure.CompensationErrors,
				fmt.Errorf("compensate step %d (%s): %w", i, step.Capability, err))
		}
	}
}

// Execute satisfies CapabilityExecutor but requires an invoker — panics if called directly.
// Use ExecuteWithInvoker instead.
func (p *Pipeline) Execute(_ context.Context, _ any) (any, error) {
	return nil, &ErrValidation{Message: "pipeline must be executed with an invoker (use ExecuteWithInvoker)"}
}
