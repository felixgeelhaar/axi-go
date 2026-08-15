package main

import (
	"context"
	"testing"

	"go.klarlabs.de/axi"
	"go.klarlabs.de/axi/domain"
)

func TestMeteredExecutor_StampsProviderUsage(t *testing.T) {
	kernel, ledger := wire()
	result, err := kernel.Execute(context.Background(), axi.Invocation{
		Action: "chat.metered",
		Input:  map[string]any{"prompt": "hi"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Status != domain.StatusSucceeded {
		t.Fatalf("status = %s", result.Status)
	}
	if got := sumEvidenceTokens(result.Evidence); got != 100 {
		t.Fatalf("evidence TokensUsed sum = %d, want 100", got)
	}
	if ledger.Spent("chat.metered") != 100 {
		t.Fatalf("ledger = %d, want 100", ledger.Spent("chat.metered"))
	}
}

func TestNaiveExecutor_UnderReports(t *testing.T) {
	kernel, ledger := wire()
	result, err := kernel.Execute(context.Background(), axi.Invocation{
		Action: "chat.naive",
		Input:  map[string]any{"prompt": "hi"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Status != domain.StatusSucceeded {
		t.Fatalf("status = %s", result.Status)
	}
	if got := sumEvidenceTokens(result.Evidence); got != 0 {
		t.Fatalf("naive evidence tokens = %d, want 0 (under-report)", got)
	}
	if ledger.Spent("chat.naive") != 0 {
		t.Fatalf("ledger saw %d; observers cannot invent tokens the executor omitted", ledger.Spent("chat.naive"))
	}
}

func TestMeteredVsNaive_SameProviderDifferentEvidence(t *testing.T) {
	kernel, _ := wire()
	ctx := context.Background()

	metered, _ := kernel.Execute(ctx, axi.Invocation{
		Action: "chat.metered", Input: map[string]any{"prompt": "x"},
	})
	naive, _ := kernel.Execute(ctx, axi.Invocation{
		Action: "chat.naive", Input: map[string]any{"prompt": "x"},
	})

	if sumEvidenceTokens(metered.Evidence) <= sumEvidenceTokens(naive.Evidence) {
		t.Fatalf("metered (%d) should report more than naive (%d)",
			sumEvidenceTokens(metered.Evidence), sumEvidenceTokens(naive.Evidence))
	}
}
