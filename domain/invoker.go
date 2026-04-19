package domain

import "context"

// ActionInvoker is the domain port that lets one action invoke another
// registered action through the kernel. Implementations create a fresh
// ExecutionSession for each call, run it through the full execution
// pipeline (rate limit, validation, resolution, approval gate, budget,
// output contract), and surface the outcome as an ActionOutcome value
// object.
//
// The port is provided to orchestrator executors so that compositional
// domain logic — sagas, fan-out/fan-in, aggregation, pipelines of
// whole actions — can live inside plugin code without holding a
// reference to the kernel aggregate. The narrow interface is a
// deliberate anti-corruption boundary: plugins cannot reach into
// session state, registry internals, or the publisher.
type ActionInvoker interface {
	// Invoke runs the named action with the given input and returns
	// its outcome. The returned error is reserved for transport-level
	// failures (action not found, invoker not wired, context cancelled
	// before start); domain-level failures come back as a non-nil
	// ActionOutcome whose Status is Failed or Rejected.
	//
	// Actions with write-external effects return immediately with
	// Status == AwaitingApproval; the orchestrator decides whether
	// to treat that as a failure, surface it to the caller, or wait
	// for approval out-of-band. Orchestrators are NOT permitted to
	// approve on behalf of humans — approval is always an explicit
	// caller action via Kernel.Approve / Reject.
	Invoke(ctx context.Context, action ActionName, input any) (*ActionOutcome, error)
}

// ActionOutcome is the value-object result of an ActionInvoker.Invoke
// call. Immutable — orchestrators inspect fields to decide their next
// step.
//
// A successful outcome has Status == StatusSucceeded and a non-nil
// Result; a failed outcome has Status == StatusFailed and a non-nil
// Failure; a rejected outcome has Status == StatusRejected and a
// non-nil Failure whose Message carries the rejection rationale; an
// awaiting-approval outcome has Status == StatusAwaitingApproval and
// both Result and Failure nil.
type ActionOutcome struct {
	SessionID ExecutionSessionID
	Status    ExecutionStatus
	Result    *ExecutionResult
	Failure   *FailureReason
	Evidence  []EvidenceRecord
}

// IsSuccess is a convenience: true iff Status == StatusSucceeded.
func (o *ActionOutcome) IsSuccess() bool {
	return o != nil && o.Status == StatusSucceeded
}

// IsFailure is a convenience: true iff Status == StatusFailed or
// StatusRejected.
func (o *ActionOutcome) IsFailure() bool {
	return o != nil && (o.Status == StatusFailed || o.Status == StatusRejected)
}

// OrchestratorActionExecutor is the optional companion to ActionExecutor
// for executors that need to invoke other registered actions as part
// of their work. Implementations that orchestrate MUST also implement
// ActionExecutor so the kernel can fall back to the synchronous path
// when no invoker is wired (e.g. unit tests that don't need real
// composition). The OrchestratorActionExecutor interface adds a single
// method and the kernel prefers ExecuteOrchestrated when both are
// implemented and an ActionInvoker has been provided to the service.
//
// The invoker argument lets the orchestrator compose whole actions
// (with their full lifecycle: validation, evidence, budget, events).
// The caps argument is the usual CapabilityInvoker for low-level
// capability calls — orchestrators often use both.
type OrchestratorActionExecutor interface {
	ExecuteOrchestrated(
		ctx context.Context,
		input any,
		caps CapabilityInvoker,
		actions ActionInvoker,
	) (ExecutionResult, []EvidenceRecord, error)
}
