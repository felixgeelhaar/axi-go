// Package main is a reference adopter pattern for TokensUsed metering.
//
// axi-go does NOT verify that plugins report TokensUsed truthfully — the
// evidence hash chain only detects post-emission tampering. Honest counts
// belong in the adopter stack, next to the LLM/provider client.
//
// Two patterns are shown here:
//
//  1. Emission honesty — the action executor owns the provider call and
//     stamps EvidenceRecord.TokensUsed from the provider's usage field.
//     A "naive" executor that hard-codes TokensUsed: 0 is shown alongside
//     so the difference is obvious.
//
//  2. Spend observer — a DomainEventPublisher accumulates reported tokens
//     per action (same composition idea as example/observability/). This
//     does not create honesty; it only acts on whatever was reported.
//
// Run: go run ./example/metering
//
// Copy/vendor only — not a supported axi-go package.
package main

import (
	"context"
	"fmt"
	"os"
	"sync"

	"go.klarlabs.de/axi"
	"go.klarlabs.de/axi/domain"
)

// --- Fake provider (stand-in for OpenAI / Anthropic / …) ---

// llmResponse is what a real SDK returns: content plus authoritative usage.
type llmResponse struct {
	Text             string
	PromptTokens     int64
	CompletionTokens int64
}

func (r llmResponse) TotalTokens() int64 {
	return r.PromptTokens + r.CompletionTokens
}

// fakeLLM pretends to be a provider SDK. Usage is the source of truth.
type fakeLLM struct {
	promptTokens     int64
	completionTokens int64
}

func (f fakeLLM) Complete(_ context.Context, prompt string) (llmResponse, error) {
	return llmResponse{
		Text:             "reply to: " + prompt,
		PromptTokens:     f.promptTokens,
		CompletionTokens: f.completionTokens,
	}, nil
}

// --- Pattern 1a: honest (metered) action executor ---

type meteredChatPlugin struct{}

func (meteredChatPlugin) Contribute() (*domain.PluginContribution, error) {
	action, _ := domain.NewActionDefinition(
		"chat.metered", "LLM call that stamps TokensUsed from provider usage",
		domain.NewContract([]domain.ContractField{
			{Name: "prompt", Type: "string", Required: true},
		}),
		domain.EmptyContract(), nil,
		domain.EffectProfile{Level: domain.EffectReadExternal},
		domain.IdempotencyProfile{IsIdempotent: true},
	)
	_ = action.BindExecutor("exec.chat.metered")
	return domain.NewPluginContribution("chat.metered.plugin",
		[]*domain.ActionDefinition{action}, nil)
}

type meteredChatExec struct {
	client fakeLLM
}

func (e meteredChatExec) Execute(ctx context.Context, input any, _ domain.CapabilityInvoker) (domain.ExecutionResult, []domain.EvidenceRecord, error) {
	prompt, _ := input.(map[string]any)["prompt"].(string)
	resp, err := e.client.Complete(ctx, prompt)
	if err != nil {
		return domain.ExecutionResult{}, nil, err
	}
	tokens := resp.TotalTokens()
	return domain.ExecutionResult{
			Data:    map[string]any{"text": resp.Text, "tokens": tokens},
			Summary: "metered completion",
		}, []domain.EvidenceRecord{{
			Kind:   "llm.completion",
			Source: "chat.metered",
			Value: map[string]any{
				"prompt_tokens":     resp.PromptTokens,
				"completion_tokens": resp.CompletionTokens,
			},
			// Honesty: usage comes from the provider response, not a guess.
			TokensUsed: tokens,
		}}, nil
}

// --- Pattern 1b: naive executor that under-reports (the anti-pattern) ---

type naiveChatPlugin struct{}

func (naiveChatPlugin) Contribute() (*domain.PluginContribution, error) {
	action, _ := domain.NewActionDefinition(
		"chat.naive", "LLM call that forgets to report TokensUsed",
		domain.NewContract([]domain.ContractField{
			{Name: "prompt", Type: "string", Required: true},
		}),
		domain.EmptyContract(), nil,
		domain.EffectProfile{Level: domain.EffectReadExternal},
		domain.IdempotencyProfile{IsIdempotent: true},
	)
	_ = action.BindExecutor("exec.chat.naive")
	return domain.NewPluginContribution("chat.naive.plugin",
		[]*domain.ActionDefinition{action}, nil)
}

type naiveChatExec struct {
	client fakeLLM
}

func (e naiveChatExec) Execute(ctx context.Context, input any, _ domain.CapabilityInvoker) (domain.ExecutionResult, []domain.EvidenceRecord, error) {
	prompt, _ := input.(map[string]any)["prompt"].(string)
	resp, err := e.client.Complete(ctx, prompt)
	if err != nil {
		return domain.ExecutionResult{}, nil, err
	}
	// Anti-pattern: provider burned tokens, evidence says zero.
	return domain.ExecutionResult{
			Data:    map[string]any{"text": resp.Text},
			Summary: "naive completion (TokensUsed omitted)",
		}, []domain.EvidenceRecord{{
			Kind:       "llm.completion",
			Source:     "chat.naive",
			Value:      map[string]any{"text": resp.Text},
			TokensUsed: 0,
		}}, nil
}

// --- Pattern 2: spend observer (does not create honesty) ---

// spendLedger watches EvidenceRecorded and sums reported tokens.
// Same idea as example/observability's tokenBudgetGuard — kept smaller
// here to focus on the metering story.
type spendLedger struct {
	mu    sync.Mutex
	spent map[domain.ActionName]int64
}

func newSpendLedger() *spendLedger {
	return &spendLedger{spent: map[domain.ActionName]int64{}}
}

func (l *spendLedger) Publish(event domain.DomainEvent) {
	ev, ok := event.(domain.EvidenceRecorded)
	if !ok {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.spent[ev.ActionName] += ev.Tokens
}

func (l *spendLedger) Spent(action domain.ActionName) int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.spent[action]
}

func wire() (*axi.Kernel, *spendLedger) {
	ledger := newSpendLedger()
	client := fakeLLM{promptTokens: 40, completionTokens: 60} // 100 total

	kernel := axi.New().WithDomainEventPublisher(ledger)
	kernel.RegisterActionExecutor("exec.chat.metered", meteredChatExec{client: client})
	kernel.RegisterActionExecutor("exec.chat.naive", naiveChatExec{client: client})
	_ = kernel.RegisterPlugin(meteredChatPlugin{})
	_ = kernel.RegisterPlugin(naiveChatPlugin{})
	return kernel, ledger
}

func main() {
	kernel, ledger := wire()
	ctx := context.Background()

	fmt.Println("→ chat.metered (TokensUsed stamped from provider usage)")
	metered, err := kernel.Execute(ctx, axi.Invocation{
		Action: "chat.metered",
		Input:  map[string]any{"prompt": "hello"},
	})
	if err != nil {
		fatal("metered: %v", err)
	}
	fmt.Printf("  status=%s evidence_tokens=%d ledger=%d\n",
		metered.Status, sumEvidenceTokens(metered.Evidence), ledger.Spent("chat.metered"))

	fmt.Println("\n→ chat.naive (same provider burn, TokensUsed=0)")
	naive, err := kernel.Execute(ctx, axi.Invocation{
		Action: "chat.naive",
		Input:  map[string]any{"prompt": "hello"},
	})
	if err != nil {
		fatal("naive: %v", err)
	}
	fmt.Printf("  status=%s evidence_tokens=%d ledger=%d\n",
		naive.Status, sumEvidenceTokens(naive.Evidence), ledger.Spent("chat.naive"))

	fmt.Println("\nTakeaway: MaxTokens / spend observers only see what executors report.")
	fmt.Println("Meter at the provider boundary in YOUR adapter — not in axi-go core.")
}

func sumEvidenceTokens(ev []domain.EvidenceRecord) int64 {
	var n int64
	for _, e := range ev {
		n += e.TokensUsed
	}
	return n
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
