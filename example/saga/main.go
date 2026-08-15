// Package main is a reference "sagas as plugin" adopter pattern for axi-go.
//
// It shows how to keep durable-saga machinery OUT of axi-go core while still
// using the kernel's ActionInvoker / OrchestratorActionExecutor primitives:
//
//   - An in-process Outbox records every saga step (stand-in for Postgres /
//     Kafka). axi-go never sees this type.
//   - saga.run is an OrchestratorActionExecutor that invokes leaf actions
//     through ActionInvoker and FAIL-CLOSES when a nested write-external
//     step pauses at awaiting_approval — the parent does not Succeed while
//     a child still awaits a human.
//   - Leaf actions stay ordinary plugins (reserve = none, charge =
//     write-external). Approval remains Kernel.Approve / Reject.
//
// Run: go run ./example/saga
//
// This example is copy/vendor only — not a supported axi-go package.
package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"go.klarlabs.de/axi"
	"go.klarlabs.de/axi/domain"
)

// --- Durable log (lives in the adopter module, not axi-go) ---

// OutboxEntry is one append-only saga log record.
type OutboxEntry struct {
	At        time.Time
	SagaID    string
	Step      string
	SessionID domain.ExecutionSessionID
	Status    domain.ExecutionStatus
	Detail    string
}

// Outbox is a tiny in-process stand-in for a durable saga log / outbox.
// Production code would write to Postgres, Kafka, etc. The point is that
// this type never crosses into axi-go packages.
type Outbox struct {
	mu      sync.Mutex
	entries []OutboxEntry
}

func (o *Outbox) Append(e OutboxEntry) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if e.At.IsZero() {
		e.At = time.Now()
	}
	o.entries = append(o.entries, e)
}

func (o *Outbox) Entries() []OutboxEntry {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]OutboxEntry, len(o.entries))
	copy(out, o.entries)
	return out
}

// --- Leaf actions ---

type leafPlugin struct{}

func (leafPlugin) Contribute() (*domain.PluginContribution, error) {
	reserve, _ := domain.NewActionDefinition(
		"order.reserve", "Reserve inventory (local, no approval)",
		domain.EmptyContract(), domain.EmptyContract(), nil,
		domain.EffectProfile{Level: domain.EffectWriteLocal},
		domain.IdempotencyProfile{IsIdempotent: true},
	)
	_ = reserve.BindExecutor("exec.order.reserve")

	charge, _ := domain.NewActionDefinition(
		"order.charge", "Charge payment (write-external → approval gate)",
		domain.EmptyContract(), domain.EmptyContract(), nil,
		domain.EffectProfile{Level: domain.EffectWriteExternal},
		domain.IdempotencyProfile{IsIdempotent: false},
	)
	_ = charge.BindExecutor("exec.order.charge")

	return domain.NewPluginContribution("order.leaf.plugin",
		[]*domain.ActionDefinition{reserve, charge}, nil)
}

type reserveExec struct{}

func (reserveExec) Execute(_ context.Context, _ any, _ domain.CapabilityInvoker) (domain.ExecutionResult, []domain.EvidenceRecord, error) {
	return domain.ExecutionResult{Summary: "inventory reserved", Data: map[string]any{"sku": "SKU-1"}},
		[]domain.EvidenceRecord{{Kind: "inventory.reserved", Source: "order.reserve", Value: "SKU-1"}}, nil
}

type chargeExec struct{}

func (chargeExec) Execute(_ context.Context, _ any, _ domain.CapabilityInvoker) (domain.ExecutionResult, []domain.EvidenceRecord, error) {
	return domain.ExecutionResult{Summary: "payment charged", Data: map[string]any{"amount": 42}},
		[]domain.EvidenceRecord{{Kind: "payment.charged", Source: "order.charge", Value: 42}}, nil
}

// --- Saga orchestrator plugin ---

type sagaPlugin struct{}

func (sagaPlugin) Contribute() (*domain.PluginContribution, error) {
	run, _ := domain.NewActionDefinition(
		"saga.run", "Run a multi-step saga with fail-closed nested approval",
		domain.NewContract([]domain.ContractField{
			{Name: "saga_id", Type: "string", Required: true, Description: "Caller-supplied saga correlation id"},
			{Name: "steps", Type: "array", Required: true, Description: "Ordered action names to invoke"},
		}),
		domain.EmptyContract(), nil,
		domain.EffectProfile{Level: domain.EffectNone},
		domain.IdempotencyProfile{IsIdempotent: false},
	)
	_ = run.BindExecutor("exec.saga.run")
	return domain.NewPluginContribution("saga.plugin",
		[]*domain.ActionDefinition{run}, nil)
}

// sagaExec orchestrates leaf actions through ActionInvoker. Nested
// write-external pauses are treated as hard failures (fail-closed).
type sagaExec struct {
	outbox *Outbox
}

func (s *sagaExec) Execute(_ context.Context, _ any, _ domain.CapabilityInvoker) (domain.ExecutionResult, []domain.EvidenceRecord, error) {
	return domain.ExecutionResult{}, nil, fmt.Errorf("sagaExec.Execute: kernel should prefer ExecuteOrchestrated")
}

func (s *sagaExec) ExecuteOrchestrated(
	ctx context.Context,
	input any,
	_ domain.CapabilityInvoker,
	actions domain.ActionInvoker,
) (domain.ExecutionResult, []domain.EvidenceRecord, error) {
	in, _ := input.(map[string]any)
	sagaID, _ := in["saga_id"].(string)
	rawSteps, _ := in["steps"].([]any)
	if sagaID == "" || len(rawSteps) == 0 {
		return domain.ExecutionResult{}, nil, fmt.Errorf("saga.run requires saga_id and non-empty steps")
	}

	var evidence []domain.EvidenceRecord
	completed := make([]string, 0, len(rawSteps))

	for _, raw := range rawSteps {
		step, _ := raw.(string)
		if step == "" {
			return domain.ExecutionResult{}, nil, fmt.Errorf("saga.run: empty step name")
		}

		out, err := actions.Invoke(ctx, domain.ActionName(step), map[string]any{"saga_id": sagaID})
		if err != nil {
			s.outbox.Append(OutboxEntry{SagaID: sagaID, Step: step, Status: domain.StatusFailed, Detail: err.Error()})
			return domain.ExecutionResult{}, evidence, fmt.Errorf("saga %s step %s transport error: %w", sagaID, step, err)
		}

		s.outbox.Append(OutboxEntry{
			SagaID:    sagaID,
			Step:      step,
			SessionID: out.SessionID,
			Status:    out.Status,
			Detail:    statusDetail(out),
		})
		evidence = append(evidence, domain.EvidenceRecord{
			Kind:   "saga.step",
			Source: "saga.run",
			Value: map[string]any{
				"step":       step,
				"session_id": string(out.SessionID),
				"status":     string(out.Status),
			},
		})

		// Fail-closed: nested write-external must not let the parent Succeed.
		if out.IsAwaitingApproval() {
			s.recordCompensate(sagaID, completed)
			return domain.ExecutionResult{}, evidence, fmt.Errorf(
				"saga %s fail-closed: step %q awaits approval (session %s); approve that session out-of-band before retrying",
				sagaID, step, out.SessionID,
			)
		}
		if out.IsFailure() {
			s.recordCompensate(sagaID, completed)
			msg := "failed"
			if out.Failure != nil {
				msg = out.Failure.Message
			}
			return domain.ExecutionResult{}, evidence, fmt.Errorf("saga %s step %q %s: %s", sagaID, step, out.Status, msg)
		}
		completed = append(completed, step)
	}

	s.outbox.Append(OutboxEntry{SagaID: sagaID, Step: "*", Status: domain.StatusSucceeded, Detail: "saga completed"})
	return domain.ExecutionResult{
		Summary: fmt.Sprintf("saga %s completed (%d steps)", sagaID, len(completed)),
		Data:    map[string]any{"saga_id": sagaID, "steps": completed},
	}, evidence, nil
}

func (s *sagaExec) recordCompensate(sagaID string, completed []string) {
	// Reverse-order compensation intents — real sagas would invoke
	// compensate actions; here we only persist the intent to the outbox.
	for i := len(completed) - 1; i >= 0; i-- {
		s.outbox.Append(OutboxEntry{
			SagaID: sagaID,
			Step:   "compensate:" + completed[i],
			Status: domain.StatusPending,
			Detail: "compensation scheduled (example only)",
		})
	}
}

func statusDetail(out *domain.ActionOutcome) string {
	if out == nil {
		return ""
	}
	if out.IsAwaitingApproval() {
		return "awaiting human approval"
	}
	if out.Failure != nil {
		return out.Failure.Message
	}
	if out.Result != nil {
		return out.Result.Summary
	}
	return string(out.Status)
}

func wireKernel(outbox *Outbox) *axi.Kernel {
	kernel := axi.New()
	kernel.RegisterActionExecutor("exec.order.reserve", reserveExec{})
	kernel.RegisterActionExecutor("exec.order.charge", chargeExec{})
	kernel.RegisterActionExecutor("exec.saga.run", &sagaExec{outbox: outbox})
	_ = kernel.RegisterPlugin(leafPlugin{})
	_ = kernel.RegisterPlugin(sagaPlugin{})
	return kernel
}

func printOutbox(outbox *Outbox) {
	fmt.Println("outbox:")
	for _, e := range outbox.Entries() {
		fmt.Printf("  %-20s step=%-22s status=%-18s session=%s %s\n",
			e.SagaID, e.Step, e.Status, e.SessionID, e.Detail)
	}
}

func main() {
	ctx := context.Background()
	outbox := &Outbox{}
	kernel := wireKernel(outbox)

	fmt.Println("→ saga with only local steps (should Succeed)")
	ok, err := kernel.Execute(ctx, axi.Invocation{
		Action: "saga.run",
		Input: map[string]any{
			"saga_id": "saga-local",
			"steps":   []any{"order.reserve"},
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Execute: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  status = %s summary = %s\n\n", ok.Status, ok.Result.Summary)

	fmt.Println("→ saga including write-external charge (fail-closed)")
	paused, err := kernel.Execute(ctx, axi.Invocation{
		Action: "saga.run",
		Input: map[string]any{
			"saga_id": "saga-pay",
			"steps":   []any{"order.reserve", "order.charge"},
		},
	})
	if err != nil {
		// Domain failure surfaces as Failed session, not always a Go error.
		fmt.Printf("  Execute err = %v\n", err)
	}
	if paused != nil {
		fmt.Printf("  parent status = %s\n", paused.Status)
		if paused.Failure != nil {
			fmt.Printf("  failure = %s\n", paused.Failure.Message)
		}
	}
	fmt.Println()
	printOutbox(outbox)
}
