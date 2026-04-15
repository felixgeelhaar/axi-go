package domain

import (
	"context"
	"fmt"
)

// ActionExecutorFunc executes an action with access to capabilities.
type ActionExecutorFunc interface {
	Execute(ctx context.Context, input any, invoker CapabilityInvokerFunc) (ExecutionResult, []EvidenceRecord, error)
}

// CapabilityInvokerFunc is used inside action execution to invoke resolved capabilities.
type CapabilityInvokerFunc interface {
	Invoke(name CapabilityName, input any) (any, error)
}

// CapabilityExecutorFunc executes a single capability.
type CapabilityExecutorFunc interface {
	Execute(ctx context.Context, input any) (any, error)
}

// ActionExecutorLookup finds an executor for an action binding.
type ActionExecutorLookup interface {
	GetActionExecutor(ref ActionExecutorRef) (ActionExecutorFunc, error)
}

// CapabilityExecutorLookup finds an executor for a capability binding.
type CapabilityExecutorLookup interface {
	GetCapabilityExecutor(ref CapabilityExecutorRef) (CapabilityExecutorFunc, error)
}

// ContractValidatorFunc validates input against a contract.
type ContractValidatorFunc interface {
	Validate(contract Contract, input any) error
}

// ActionExecutionService orchestrates the full execution flow of an action.
type ActionExecutionService struct {
	actionRepo        ActionRepository
	resolutionService *CapabilityResolutionService
	validator         ContractValidatorFunc
	actionExecutors   ActionExecutorLookup
	capExecutors      CapabilityExecutorLookup
}

// NewActionExecutionService creates an ActionExecutionService.
func NewActionExecutionService(
	actionRepo ActionRepository,
	resolutionService *CapabilityResolutionService,
	validator ContractValidatorFunc,
	actionExecutors ActionExecutorLookup,
	capExecutors CapabilityExecutorLookup,
) *ActionExecutionService {
	return &ActionExecutionService{
		actionRepo:        actionRepo,
		resolutionService: resolutionService,
		validator:         validator,
		actionExecutors:   actionExecutors,
		capExecutors:      capExecutors,
	}
}

// Execute runs the full execution flow on a session.
// Flow: Validate → Resolve → Run → Succeed/Fail
func (s *ActionExecutionService) Execute(ctx context.Context, session *ExecutionSession) error {
	// Load the action definition.
	action, err := s.actionRepo.GetByName(session.ActionName())
	if err != nil {
		return fmt.Errorf("action %q not found: %w", session.ActionName(), err)
	}

	// Validate input against contract.
	if err := s.validator.Validate(action.InputContract(), session.Input()); err != nil {
		return fmt.Errorf("input validation failed: %w", err)
	}
	if err := session.MarkValidated(); err != nil {
		return err
	}

	// Resolve capabilities.
	resolvedCaps, err := s.resolutionService.Resolve(action.Requirements())
	if err != nil {
		return fmt.Errorf("capability resolution failed: %w", err)
	}
	capNames := make([]CapabilityName, len(resolvedCaps))
	for i, c := range resolvedCaps {
		capNames[i] = c.Name()
	}
	if err := session.MarkResolved(capNames); err != nil {
		return err
	}

	// Build capability invoker from resolved capabilities.
	invoker, err := s.buildInvoker(ctx, resolvedCaps)
	if err != nil {
		return fmt.Errorf("failed to build capability invoker: %w", err)
	}

	// Transition to running.
	if err := session.MarkRunning(); err != nil {
		return err
	}

	// Execute the action.
	executor, err := s.actionExecutors.GetActionExecutor(action.ExecutionBinding())
	if err != nil {
		failErr := session.Fail(FailureReason{Code: "EXECUTOR_NOT_FOUND", Message: err.Error()})
		if failErr != nil {
			return failErr
		}
		return err
	}

	result, evidence, execErr := executor.Execute(ctx, session.Input(), invoker)
	for _, e := range evidence {
		session.AppendEvidence(e)
	}

	if execErr != nil {
		if err := session.Fail(FailureReason{Code: "EXECUTION_ERROR", Message: execErr.Error()}); err != nil {
			return err
		}
		return nil // Failure is a valid outcome, not an error.
	}

	return session.Succeed(result)
}

// buildInvoker creates a CapabilityInvokerFunc from resolved capabilities.
func (s *ActionExecutionService) buildInvoker(ctx context.Context, capabilities []*CapabilityDefinition) (CapabilityInvokerFunc, error) {
	capMap := make(map[CapabilityName]*CapabilityDefinition, len(capabilities))
	for _, c := range capabilities {
		capMap[c.Name()] = c
	}
	return &boundInvoker{
		ctx:          ctx,
		capabilities: capMap,
		executors:    s.capExecutors,
	}, nil
}

// boundInvoker implements CapabilityInvokerFunc by dispatching to registered executors.
type boundInvoker struct {
	ctx          context.Context
	capabilities map[CapabilityName]*CapabilityDefinition
	executors    CapabilityExecutorLookup
}

func (i *boundInvoker) Invoke(name CapabilityName, input any) (any, error) {
	cap, ok := i.capabilities[name]
	if !ok {
		return nil, fmt.Errorf("capability %q not available in this execution context", name)
	}
	executor, err := i.executors.GetCapabilityExecutor(cap.ExecutionBinding())
	if err != nil {
		return nil, fmt.Errorf("executor for capability %q not found: %w", name, err)
	}
	return executor.Execute(i.ctx, input)
}
