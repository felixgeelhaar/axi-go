package domain_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.klarlabs.de/axi/domain"
)

func TestBudgetEnforcer_MaxInvocations(t *testing.T) {
	// Test via the full execution flow — register an action with a capability,
	// set a budget of 2 invocations, and call the capability 3 times.
	execSvc, actionRepo, capRepo, actionExecs, capExecs := setupExecution(t)
	execSvc.SetDefaultBudget(domain.ExecutionBudget{MaxCapabilityInvocations: 2})

	cap, _ := domain.NewCapabilityDefinition("counter", "Counts", domain.EmptyContract(), domain.EmptyContract())
	_ = cap.BindExecutor("exec.counter")
	_ = capRepo.Save(cap)
	capExecs.executors["exec.counter"] = &fakeCapExecutor{
		fn: func(_ context.Context, input any) (any, error) { return input, nil },
	}

	reqs, _ := domain.NewRequirementSet(domain.Requirement{Capability: "counter"})
	action, _ := domain.NewActionDefinition("budget-test", "Tests budget",
		domain.EmptyContract(), domain.EmptyContract(), reqs,
		domain.EffectProfile{Level: domain.EffectNone}, domain.IdempotencyProfile{},
	)
	_ = action.BindExecutor("exec.budget")
	_ = actionRepo.Save(action)

	callCount := 0
	actionExecs.executors["exec.budget"] = &fakeActionExecutor{
		fn: func(_ context.Context, _ any, invoker domain.CapabilityInvoker) (domain.ExecutionResult, []domain.EvidenceRecord, error) {
			// Try 3 invocations — third should fail.
			for i := 0; i < 3; i++ {
				_, err := invoker.Invoke("counter", i)
				if err != nil {
					return domain.ExecutionResult{}, nil, err
				}
				callCount++
			}
			return domain.ExecutionResult{Data: "done"}, nil, nil
		},
	}

	session, _ := domain.NewExecutionSession("s1", "budget-test", nil)
	err := execSvc.Execute(context.Background(), session)

	// Execution should complete (failure is a valid outcome).
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.Status() != domain.StatusFailed {
		t.Errorf("expected Failed (budget exceeded), got %s", session.Status())
	}
	if callCount != 2 {
		t.Errorf("expected 2 successful invocations, got %d", callCount)
	}
}

func TestBudgetEnforcer_MaxDuration(t *testing.T) {
	execSvc, actionRepo, _, actionExecs, _ := setupExecution(t)
	execSvc.SetDefaultBudget(domain.ExecutionBudget{MaxDuration: 1 * time.Millisecond})

	action, _ := domain.NewActionDefinition("slow-action", "Slow",
		domain.EmptyContract(), domain.EmptyContract(), nil,
		domain.EffectProfile{Level: domain.EffectNone}, domain.IdempotencyProfile{},
	)
	_ = action.BindExecutor("exec.slow")
	_ = actionRepo.Save(action)

	actionExecs.executors["exec.slow"] = &fakeActionExecutor{
		fn: func(_ context.Context, _ any, _ domain.CapabilityInvoker) (domain.ExecutionResult, []domain.EvidenceRecord, error) {
			// Budget is checked on capability invocations, not on the action itself.
			// With no capability invocations, duration budget isn't checked.
			return domain.ExecutionResult{Data: "ok"}, nil, nil
		},
	}

	session, _ := domain.NewExecutionSession("s1", "slow-action", nil)
	err := execSvc.Execute(context.Background(), session)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Without capability invocations, budget isn't enforced (by design).
	if session.Status() != domain.StatusSucceeded {
		t.Errorf("expected Succeeded, got %s", session.Status())
	}
}

func TestBudgetEnforcer_MaxTokens_Exceeded(t *testing.T) {
	execSvc, actionRepo, _, actionExecs, _ := setupExecution(t)
	execSvc.SetDefaultBudget(domain.ExecutionBudget{MaxTokens: 100})

	action, _ := domain.NewActionDefinition("token-test", "Tokens",
		domain.EmptyContract(), domain.EmptyContract(), nil,
		domain.EffectProfile{Level: domain.EffectNone}, domain.IdempotencyProfile{},
	)
	_ = action.BindExecutor("exec.tokens")
	_ = actionRepo.Save(action)

	actionExecs.executors["exec.tokens"] = &fakeActionExecutor{
		fn: func(_ context.Context, _ any, _ domain.CapabilityInvoker) (domain.ExecutionResult, []domain.EvidenceRecord, error) {
			return domain.ExecutionResult{Data: "ok"},
				[]domain.EvidenceRecord{
					{Kind: "llm", Source: "model-a", TokensUsed: 60},
					{Kind: "llm", Source: "model-b", TokensUsed: 50}, // total 110 > 100
				}, nil
		},
	}

	session, _ := domain.NewExecutionSession("s1", "token-test", nil)
	if err := execSvc.Execute(context.Background(), session); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.Status() != domain.StatusFailed {
		t.Fatalf("expected Failed (budget exceeded), got %s", session.Status())
	}
	if f := session.Failure(); f == nil || f.Code != "BUDGET_EXCEEDED" {
		t.Errorf("expected BUDGET_EXCEEDED failure, got %+v", f)
	}
}

func TestBudgetEnforcer_MaxTokens_WithinBudget(t *testing.T) {
	execSvc, actionRepo, _, actionExecs, _ := setupExecution(t)
	execSvc.SetDefaultBudget(domain.ExecutionBudget{MaxTokens: 100})

	action, _ := domain.NewActionDefinition("token-ok", "Tokens ok",
		domain.EmptyContract(), domain.EmptyContract(), nil,
		domain.EffectProfile{Level: domain.EffectNone}, domain.IdempotencyProfile{},
	)
	_ = action.BindExecutor("exec.tokens.ok")
	_ = actionRepo.Save(action)

	actionExecs.executors["exec.tokens.ok"] = &fakeActionExecutor{
		fn: func(_ context.Context, _ any, _ domain.CapabilityInvoker) (domain.ExecutionResult, []domain.EvidenceRecord, error) {
			return domain.ExecutionResult{Data: "ok"},
				[]domain.EvidenceRecord{{Kind: "llm", Source: "m", TokensUsed: 40}}, nil
		},
	}

	session, _ := domain.NewExecutionSession("s1", "token-ok", nil)
	if err := execSvc.Execute(context.Background(), session); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.Status() != domain.StatusSucceeded {
		t.Errorf("expected Succeeded, got %s", session.Status())
	}
}

func TestBudgetEnforcer_NoBudget(t *testing.T) {
	// Zero budget means no limit.
	execSvc, actionRepo, capRepo, actionExecs, capExecs := setupExecution(t)
	// No budget set (default zero values).

	cap, _ := domain.NewCapabilityDefinition("unlimited", "No limit", domain.EmptyContract(), domain.EmptyContract())
	_ = cap.BindExecutor("exec.unlimited")
	_ = capRepo.Save(cap)
	capExecs.executors["exec.unlimited"] = &fakeCapExecutor{
		fn: func(_ context.Context, input any) (any, error) { return input, nil },
	}

	reqs, _ := domain.NewRequirementSet(domain.Requirement{Capability: "unlimited"})
	action, _ := domain.NewActionDefinition("many-calls", "Many",
		domain.EmptyContract(), domain.EmptyContract(), reqs,
		domain.EffectProfile{Level: domain.EffectNone}, domain.IdempotencyProfile{},
	)
	_ = action.BindExecutor("exec.many")
	_ = actionRepo.Save(action)

	actionExecs.executors["exec.many"] = &fakeActionExecutor{
		fn: func(_ context.Context, _ any, invoker domain.CapabilityInvoker) (domain.ExecutionResult, []domain.EvidenceRecord, error) {
			for i := 0; i < 100; i++ {
				if _, err := invoker.Invoke("unlimited", i); err != nil {
					return domain.ExecutionResult{}, nil, err
				}
			}
			return domain.ExecutionResult{Data: "done"}, nil, nil
		},
	}

	session, _ := domain.NewExecutionSession("s1", "many-calls", nil)
	_ = execSvc.Execute(context.Background(), session)
	if session.Status() != domain.StatusSucceeded {
		t.Errorf("expected Succeeded with no budget, got %s", session.Status())
	}
}

func TestDefaultBudget_WithTimeoutStyleMerge(t *testing.T) {
	execSvc, _, _, _, _ := setupExecution(t)
	execSvc.SetDefaultBudget(domain.ExecutionBudget{
		MaxCapabilityInvocations: 7,
		MaxTokens:                99,
		MaxRetries:               2,
	})
	b := execSvc.DefaultBudget()
	b.MaxDuration = 3 * time.Second
	execSvc.SetDefaultBudget(b)

	got := execSvc.DefaultBudget()
	if got.MaxCapabilityInvocations != 7 {
		t.Errorf("MaxCapabilityInvocations = %d, want 7", got.MaxCapabilityInvocations)
	}
	if got.MaxTokens != 99 {
		t.Errorf("MaxTokens = %d, want 99", got.MaxTokens)
	}
	if got.MaxRetries != 2 {
		t.Errorf("MaxRetries = %d, want 2", got.MaxRetries)
	}
	if got.MaxDuration != 3*time.Second {
		t.Errorf("MaxDuration = %v, want 3s", got.MaxDuration)
	}
}

func TestErrBudgetExceeded_FromMaxInvocations(t *testing.T) {
	execSvc, actionRepo, capRepo, actionExecs, capExecs := setupExecution(t)
	execSvc.SetDefaultBudget(domain.ExecutionBudget{MaxCapabilityInvocations: 1})

	cap, _ := domain.NewCapabilityDefinition("counter", "Counts", domain.EmptyContract(), domain.EmptyContract())
	_ = cap.BindExecutor("exec.counter")
	_ = capRepo.Save(cap)
	capExecs.executors["exec.counter"] = &fakeCapExecutor{
		fn: func(_ context.Context, _ any) (any, error) { return 1, nil },
	}

	reqs, _ := domain.NewRequirementSet(domain.Requirement{Capability: "counter"})
	action, _ := domain.NewActionDefinition("budgeted", "b",
		domain.EmptyContract(), domain.EmptyContract(),
		reqs,
		domain.EffectProfile{Level: domain.EffectNone}, domain.IdempotencyProfile{},
	)
	_ = action.BindExecutor("exec.budgeted")
	_ = actionRepo.Save(action)

	var invokeErr error
	actionExecs.executors["exec.budgeted"] = &fakeActionExecutor{
		fn: func(_ context.Context, _ any, inv domain.CapabilityInvoker) (domain.ExecutionResult, []domain.EvidenceRecord, error) {
			_, _ = inv.Invoke("counter", nil)
			_, invokeErr = inv.Invoke("counter", nil)
			return domain.ExecutionResult{}, nil, invokeErr
		},
	}

	pub := &budgetEventRecorder{}
	execSvc.SetDomainEventPublisher(pub)
	session, _ := domain.NewExecutionSession("s-typed-budget", "budgeted", nil)
	_ = execSvc.Execute(context.Background(), session)

	if invokeErr == nil {
		t.Fatal("expected second Invoke to fail")
	}
	var exceeded *domain.ErrBudgetExceeded
	if !errors.As(invokeErr, &exceeded) {
		t.Fatalf("Invoke error type = %T (%v), want *ErrBudgetExceeded", invokeErr, invokeErr)
	}
	if exceeded.Kind != domain.BudgetKindInvocations {
		t.Errorf("Kind = %s, want invocations", exceeded.Kind)
	}

	found := false
	for _, ev := range pub.events {
		if be, ok := ev.(domain.BudgetExceeded); ok {
			found = true
			if be.Kind != domain.BudgetKindInvocations {
				t.Errorf("BudgetExceeded.Kind = %s, want invocations", be.Kind)
			}
		}
	}
	if !found {
		t.Fatal("expected BudgetExceeded domain event")
	}
}

func TestErrBudgetExceeded_DurationKind(t *testing.T) {
	execSvc, actionRepo, capRepo, actionExecs, capExecs := setupExecution(t)
	execSvc.SetDefaultBudget(domain.ExecutionBudget{MaxDuration: time.Nanosecond})

	cap, _ := domain.NewCapabilityDefinition("slow", "s", domain.EmptyContract(), domain.EmptyContract())
	_ = cap.BindExecutor("exec.slow")
	_ = capRepo.Save(cap)
	capExecs.executors["exec.slow"] = &fakeCapExecutor{
		fn: func(_ context.Context, _ any) (any, error) {
			time.Sleep(2 * time.Millisecond)
			return "ok", nil
		},
	}

	reqs, _ := domain.NewRequirementSet(domain.Requirement{Capability: "slow"})
	action, _ := domain.NewActionDefinition("timed", "t",
		domain.EmptyContract(), domain.EmptyContract(),
		reqs,
		domain.EffectProfile{Level: domain.EffectNone}, domain.IdempotencyProfile{},
	)
	_ = action.BindExecutor("exec.timed")
	_ = actionRepo.Save(action)

	var invokeErr error
	actionExecs.executors["exec.timed"] = &fakeActionExecutor{
		fn: func(_ context.Context, _ any, inv domain.CapabilityInvoker) (domain.ExecutionResult, []domain.EvidenceRecord, error) {
			_, invokeErr = inv.Invoke("slow", nil)
			return domain.ExecutionResult{}, nil, invokeErr
		},
	}

	pub := &budgetEventRecorder{}
	execSvc.SetDomainEventPublisher(pub)
	session, _ := domain.NewExecutionSession("s-budget-duration", "timed", nil)
	_ = execSvc.Execute(context.Background(), session)

	if invokeErr == nil {
		t.Fatal("expected Invoke to fail on duration budget")
	}
	var exceeded *domain.ErrBudgetExceeded
	if !errors.As(invokeErr, &exceeded) {
		t.Fatalf("error type = %T (%v), want *ErrBudgetExceeded", invokeErr, invokeErr)
	}
	if exceeded.Kind != domain.BudgetKindDuration {
		t.Errorf("Kind = %s, want duration", exceeded.Kind)
	}

	found := false
	for _, ev := range pub.events {
		if be, ok := ev.(domain.BudgetExceeded); ok {
			found = true
			if be.Kind != domain.BudgetKindDuration {
				t.Errorf("BudgetExceeded.Kind = %s, want duration", be.Kind)
			}
		}
	}
	if !found {
		t.Fatal("expected BudgetExceeded domain event with duration kind")
	}
}

type budgetEventRecorder struct {
	events []domain.DomainEvent
}

func (r *budgetEventRecorder) Publish(event domain.DomainEvent) {
	r.events = append(r.events, event)
}

func TestErrBudgetExceeded_ErrorMethod(t *testing.T) {
	err := &domain.ErrBudgetExceeded{Kind: domain.BudgetKindTokens, Message: "token budget blown"}
	if err.Error() != "token budget blown" {
		t.Errorf("Error() = %q", err.Error())
	}
	empty := &domain.ErrBudgetExceeded{Kind: domain.BudgetKindTokens}
	if empty.Error() == "" {
		t.Error("Error() should fall back to kind when Message empty")
	}
	var nilErr *domain.ErrBudgetExceeded
	if nilErr.Error() == "" {
		t.Error("nil receiver Error() should be non-empty")
	}
}
