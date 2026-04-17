package domain

import (
	"context"
	"fmt"
	"time"
)

// ActionExecutor executes an action with access to capabilities.
type ActionExecutor interface {
	Execute(ctx context.Context, input any, invoker CapabilityInvoker) (ExecutionResult, []EvidenceRecord, error)
}

// CapabilityInvoker is used inside action execution to invoke resolved capabilities.
type CapabilityInvoker interface {
	Invoke(name CapabilityName, input any) (any, error)
}

// CapabilityExecutor executes a single capability.
type CapabilityExecutor interface {
	Execute(ctx context.Context, input any) (any, error)
}

// ComposableCapabilityExecutor is an optional interface for capabilities
// that need to invoke other capabilities during execution.
type ComposableCapabilityExecutor interface {
	ExecuteWithInvoker(ctx context.Context, input any, invoker CapabilityInvoker) (any, error)
}

// ActionExecutorLookup finds an executor for an action binding.
type ActionExecutorLookup interface {
	GetActionExecutor(ref ActionExecutorRef) (ActionExecutor, error)
}

// CapabilityExecutorLookup finds an executor for a capability binding.
type CapabilityExecutorLookup interface {
	GetCapabilityExecutor(ref CapabilityExecutorRef) (CapabilityExecutor, error)
}

// ContractValidator validates input against a contract.
type ContractValidator interface {
	Validate(contract Contract, input any) error
}

// ActionExecutionService orchestrates the full execution flow of an action.
type ActionExecutionService struct {
	actionRepo        ActionRepository
	resolutionService *CapabilityResolutionService
	validator         ContractValidator
	actionExecutors   ActionExecutorLookup
	capExecutors      CapabilityExecutorLookup
	rateLimiter       RateLimiter
	defaultBudget     ExecutionBudget
	logger            Logger
}

// NewActionExecutionService creates an ActionExecutionService.
func NewActionExecutionService(
	actionRepo ActionRepository,
	resolutionService *CapabilityResolutionService,
	validator ContractValidator,
	actionExecutors ActionExecutorLookup,
	capExecutors CapabilityExecutorLookup,
) *ActionExecutionService {
	return &ActionExecutionService{
		actionRepo:        actionRepo,
		resolutionService: resolutionService,
		validator:         validator,
		actionExecutors:   actionExecutors,
		capExecutors:      capExecutors,
		rateLimiter:       &NoopRateLimiter{},
		logger:            &NopLogger{},
	}
}

// SetLogger configures a logger for the execution service.
func (s *ActionExecutionService) SetLogger(logger Logger) {
	s.logger = logger
}

// SetRateLimiter configures a rate limiter for action execution.
func (s *ActionExecutionService) SetRateLimiter(rl RateLimiter) {
	s.rateLimiter = rl
}

// SetDefaultBudget configures the default execution budget for all sessions.
func (s *ActionExecutionService) SetDefaultBudget(budget ExecutionBudget) {
	s.defaultBudget = budget
}

// Execute runs the execution flow on a session.
// For actions with write-external effects, the session pauses at AwaitingApproval
// and must be resumed via Resume() after approval. Otherwise runs to completion.
func (s *ActionExecutionService) Execute(ctx context.Context, session *ExecutionSession) error {
	s.logger.Info("executing action",
		F("session_id", string(session.ID())),
		F("action", string(session.ActionName())),
	)

	// Rate limit check.
	if err := s.rateLimiter.Allow(session.ActionName()); err != nil {
		return &ErrValidation{Message: fmt.Sprintf("rate limited: %v", err)}
	}

	action, err := s.actionRepo.GetByName(session.ActionName())
	if err != nil {
		return &ErrNotFound{Entity: "action", ID: string(session.ActionName())}
	}

	// Check context between phases.
	if err := ctx.Err(); err != nil {
		return err
	}

	// Validate input.
	if err := s.validator.Validate(action.InputContract(), session.Input()); err != nil {
		return &ErrValidation{Message: fmt.Sprintf("input validation failed: %v", err)}
	}
	if err := session.MarkValidated(); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
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

	// Check if approval is required for write effects on external systems.
	if action.EffectProfile().IsWriteEffect() && action.EffectProfile().IsExternalEffect() {
		if err := session.MarkAwaitingApproval(); err != nil {
			return err
		}
		return nil // Paused — caller must approve then call Resume().
	}

	// No approval needed — execute immediately.
	return s.run(ctx, session, action)
}

// Resume continues execution of a session that was approved.
// The session must be in Running status (after Approve() was called).
func (s *ActionExecutionService) Resume(ctx context.Context, session *ExecutionSession) error {
	if session.Status() != StatusRunning {
		return fmt.Errorf("cannot resume session in %s status", session.Status())
	}

	action, err := s.actionRepo.GetByName(session.ActionName())
	if err != nil {
		return &ErrNotFound{Entity: "action", ID: string(session.ActionName())}
	}

	return s.run(ctx, session, action)
}

// run executes the action (Resolved/Approved → Running → Succeeded/Failed).
func (s *ActionExecutionService) run(ctx context.Context, session *ExecutionSession, action *ActionDefinition) error {
	// Build invoker with budget enforcement.
	resolvedCaps, err := s.resolutionService.Resolve(action.Requirements())
	if err != nil {
		return fmt.Errorf("capability resolution failed: %w", err)
	}
	invoker, err := s.buildInvoker(ctx, resolvedCaps, action.IdempotencyProfile().IsIdempotent)
	if err != nil {
		return fmt.Errorf("failed to build capability invoker: %w", err)
	}

	// Transition to running (if not already — approved sessions are already Running).
	if session.Status() != StatusRunning {
		if err := session.MarkRunning(); err != nil {
			return err
		}
	}

	// Execute.
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
		return nil
	}

	// Token budget check — post-hoc, based on reported evidence.
	if s.defaultBudget.MaxTokens > 0 {
		var total int64
		for _, e := range evidence {
			total += e.TokensUsed
		}
		if total > s.defaultBudget.MaxTokens {
			if failErr := session.Fail(FailureReason{
				Code:    "BUDGET_EXCEEDED",
				Message: fmt.Sprintf("token budget %d exceeded: used %d", s.defaultBudget.MaxTokens, total),
			}); failErr != nil {
				return failErr
			}
			return nil
		}
	}

	// Validate output against contract.
	if !action.OutputContract().IsEmpty() {
		if err := s.validator.Validate(action.OutputContract(), result.Data); err != nil {
			if failErr := session.Fail(FailureReason{Code: "OUTPUT_VALIDATION_ERROR", Message: err.Error()}); failErr != nil {
				return failErr
			}
			return nil
		}
	}

	return session.Succeed(result)
}

// buildInvoker creates a CapabilityInvoker from resolved capabilities.
// idempotent is the action's idempotency flag and gates retry eligibility.
func (s *ActionExecutionService) buildInvoker(ctx context.Context, capabilities []*CapabilityDefinition, idempotent bool) (CapabilityInvoker, error) {
	capMap := make(map[CapabilityName]*CapabilityDefinition, len(capabilities))
	for _, c := range capabilities {
		capMap[c.Name()] = c
	}
	return &boundInvoker{
		ctx:          ctx,
		capabilities: capMap,
		executors:    s.capExecutors,
		budget:       newBudgetEnforcer(s.defaultBudget),
		idempotent:   idempotent,
		maxRetries:   s.defaultBudget.MaxRetries,
		retryBackoff: s.defaultBudget.RetryBackoff,
	}, nil
}

type boundInvoker struct {
	ctx          context.Context
	capabilities map[CapabilityName]*CapabilityDefinition
	executors    CapabilityExecutorLookup
	budget       *budgetEnforcer
	idempotent   bool
	maxRetries   int
	retryBackoff time.Duration
}

func (i *boundInvoker) Invoke(name CapabilityName, input any) (any, error) {
	if err := i.budget.checkInvocation(); err != nil {
		return nil, err
	}
	cap, ok := i.capabilities[name]
	if !ok {
		return nil, fmt.Errorf("capability %q not available in this execution context", name)
	}
	executor, err := i.executors.GetCapabilityExecutor(cap.ExecutionBinding())
	if err != nil {
		return nil, fmt.Errorf("executor for capability %q not found: %w", name, err)
	}

	// Retries apply only when the action is idempotent and a retry budget is
	// configured. Retries do not consume additional MaxCapabilityInvocations
	// slots, but MaxDuration still bounds total wall-clock cost.
	attempt := 0
	for {
		result, err := i.invokeOnce(executor, input)
		if err == nil {
			return result, nil
		}
		if !i.idempotent || attempt >= i.maxRetries {
			return nil, err
		}
		if ctxErr := i.ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		// Exponential backoff: RetryBackoff, 2x, 4x, ...
		delay := i.retryBackoff << attempt //nolint:gosec // attempt bounded by maxRetries
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-timer.C:
			case <-i.ctx.Done():
				timer.Stop()
				return nil, i.ctx.Err()
			}
		}
		attempt++
	}
}

func (i *boundInvoker) invokeOnce(executor CapabilityExecutor, input any) (any, error) {
	if composable, ok := executor.(ComposableCapabilityExecutor); ok {
		return composable.ExecuteWithInvoker(i.ctx, input, i)
	}
	return executor.Execute(i.ctx, input)
}
