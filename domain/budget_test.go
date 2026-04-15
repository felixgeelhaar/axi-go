package domain_test

import (
	"context"
	"testing"
	"time"

	"github.com/felixgeelhaar/axi-go/domain"
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
