package main

import (
	"context"
	"strings"
	"testing"

	"go.klarlabs.de/axi"
	"go.klarlabs.de/axi/domain"
)

func TestSaga_LocalStepsSucceed(t *testing.T) {
	outbox := &Outbox{}
	kernel := wireKernel(outbox)

	result, err := kernel.Execute(context.Background(), axi.Invocation{
		Action: "saga.run",
		Input: map[string]any{
			"saga_id": "s-local",
			"steps":   []any{"order.reserve"},
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Status != domain.StatusSucceeded {
		t.Fatalf("status = %s, want succeeded", result.Status)
	}

	entries := outbox.Entries()
	if len(entries) < 2 {
		t.Fatalf("outbox entries = %d, want >= 2", len(entries))
	}
	if entries[0].Step != "order.reserve" || entries[0].Status != domain.StatusSucceeded {
		t.Errorf("first entry = %+v", entries[0])
	}
	last := entries[len(entries)-1]
	if last.Step != "*" || last.Status != domain.StatusSucceeded {
		t.Errorf("completion entry = %+v", last)
	}
}

func TestSaga_FailClosedOnNestedWriteExternal(t *testing.T) {
	outbox := &Outbox{}
	kernel := wireKernel(outbox)

	result, err := kernel.Execute(context.Background(), axi.Invocation{
		Action: "saga.run",
		Input: map[string]any{
			"saga_id": "s-pay",
			"steps":   []any{"order.reserve", "order.charge"},
		},
	})
	if err != nil {
		t.Fatalf("Execute returned Go error %v (want Failed outcome)", err)
	}
	if result.Status != domain.StatusFailed {
		t.Fatalf("parent status = %s, want failed (fail-closed)", result.Status)
	}
	if result.Failure == nil || !strings.Contains(result.Failure.Message, "fail-closed") {
		t.Fatalf("failure = %+v, want fail-closed message", result.Failure)
	}

	var sawAwaiting, sawCompensate bool
	var childSession domain.ExecutionSessionID
	for _, e := range outbox.Entries() {
		if e.Step == "order.charge" && e.Status == domain.StatusAwaitingApproval {
			sawAwaiting = true
			childSession = e.SessionID
		}
		if strings.HasPrefix(e.Step, "compensate:") {
			sawCompensate = true
		}
	}
	if !sawAwaiting {
		t.Fatal("outbox missing awaiting_approval entry for order.charge")
	}
	if !sawCompensate {
		t.Fatal("outbox missing compensation intent for completed reserve step")
	}

	// Child session is still pollable and awaiting approval — human must
	// Approve it out-of-band; the saga orchestrator refused to Succeed.
	session, err := kernel.GetSession(string(childSession))
	if err != nil {
		t.Fatalf("GetSession(%s): %v", childSession, err)
	}
	if session.Status() != domain.StatusAwaitingApproval {
		t.Fatalf("child status = %s, want awaiting_approval", session.Status())
	}
}

func TestSaga_RequiresInput(t *testing.T) {
	kernel := wireKernel(&Outbox{})
	result, err := kernel.Execute(context.Background(), axi.Invocation{
		Action: "saga.run",
		Input:  map[string]any{"saga_id": "", "steps": []any{}},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Status != domain.StatusFailed {
		t.Fatalf("status = %s, want failed", result.Status)
	}
}
