// Package main demonstrates axi-go embedded in a Go program.
//
// This is not a server — axi-go is a library. Run it to see the kernel
// exercise actions, capability composition, approval gates, and evidence.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/felixgeelhaar/axi-go"
	"github.com/felixgeelhaar/axi-go/domain"
	"github.com/felixgeelhaar/axi-go/inmemory"
)

// --- Sample plugin: "greeter" ---

type greeterPlugin struct{}

func (p *greeterPlugin) Contribute() (*domain.PluginContribution, error) {
	capName, _ := domain.NewCapabilityName("string.upper")
	cap, _ := domain.NewCapabilityDefinition(capName, "Uppercases a string",
		domain.NewContract([]domain.ContractField{
			{Name: "text", Type: "string", Description: "Text to uppercase", Required: true, Example: "hello"},
		}),
		domain.EmptyContract(),
	)
	_ = cap.BindExecutor("exec.string.upper")

	actionName, _ := domain.NewActionName("greet")
	reqs, _ := domain.NewRequirementSet(domain.Requirement{Capability: capName})
	action, _ := domain.NewActionDefinition(
		actionName,
		"Greet someone by name, with their name uppercased",
		domain.NewContract([]domain.ContractField{
			{Name: "name", Type: "string", Description: "Person to greet", Required: true, Example: "world"},
		}),
		domain.NewContract([]domain.ContractField{
			{Name: "message", Type: "string", Description: "The greeting message", Required: true},
		}),
		reqs,
		domain.EffectProfile{Level: domain.EffectNone},
		domain.IdempotencyProfile{IsIdempotent: true},
	)
	_ = action.BindExecutor("exec.greet")

	return domain.NewPluginContribution("greeter.plugin",
		[]*domain.ActionDefinition{action},
		[]*domain.CapabilityDefinition{cap},
	)
}

// --- Executors ---

type upperExecutor struct{}

func (e *upperExecutor) Execute(_ context.Context, input any) (any, error) {
	s, _ := input.(string)
	return strings.ToUpper(s), nil
}

type greetExecutor struct{}

func (e *greetExecutor) Execute(_ context.Context, input any, caps domain.CapabilityInvoker) (domain.ExecutionResult, []domain.EvidenceRecord, error) {
	m := input.(map[string]any)
	name, _ := m["name"].(string)

	upper, err := caps.Invoke("string.upper", name)
	if err != nil {
		return domain.ExecutionResult{}, nil, err
	}

	return domain.ExecutionResult{
			Data:        map[string]any{"message": fmt.Sprintf("Hello, %s!", upper)},
			Summary:     "Greeted " + name,
			ContentType: "application/json",
		}, []domain.EvidenceRecord{
			{Kind: "invocation", Source: "greet", Value: map[string]any{"name": name, "upper": upper}},
		}, nil
}

func main() {
	// 1. Build the kernel with a fluent, chainable builder API.
	kernel := axi.New().
		WithLogger(inmemory.NewStdLogger(inmemory.LevelInfo)).
		WithBudget(axi.Budget{MaxCapabilityInvocations: 100})

	// 2. Register executors, then the plugin metadata.
	kernel.RegisterActionExecutor("exec.greet", &greetExecutor{})
	kernel.RegisterCapabilityExecutor("exec.string.upper", &upperExecutor{})
	if err := kernel.RegisterPlugin(&greeterPlugin{}); err != nil {
		fmt.Fprintln(os.Stderr, "register:", err)
		os.Exit(1)
	}

	// 3. Inspect what's registered — agents use this to discover tools.
	fmt.Println("=== Registered actions ===")
	for _, a := range kernel.ListActions() {
		fmt.Printf("  - %s: %s\n", a.Name(), a.Description())
		fmt.Printf("    effect: %s  idempotent: %v\n", a.EffectProfile().Level, a.IdempotencyProfile().IsIdempotent)
	}
	fmt.Println()

	// 4. Execute an action.
	fmt.Println("=== Executing 'greet' ===")
	result, err := kernel.Execute(context.Background(), axi.Invocation{
		Action: "greet",
		Input:  map[string]any{"name": "world"},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "execute:", err)
		os.Exit(1)
	}

	// 5. Inspect the result.
	fmt.Printf("Status: %s\n", result.Status)
	data, _ := json.MarshalIndent(result.Result, "", "  ")
	fmt.Printf("Result: %s\n", data)
	fmt.Printf("Evidence: %d record(s)\n", len(result.Evidence))
	for _, ev := range result.Evidence {
		fmt.Printf("  - [%s] from %s: %v\n", ev.Kind, ev.Source, ev.Value)
	}
	fmt.Println()

	// 6. Poll the session.
	fmt.Println("=== Session state ===")
	session, _ := kernel.GetSession(string(result.SessionID))
	fmt.Printf("Session %s is %s\n", session.ID(), session.Status())
}
