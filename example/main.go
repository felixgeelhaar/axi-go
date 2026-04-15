// Package main demonstrates an end-to-end axi-go setup with a sample plugin.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/felixgeelhaar/axi-go/api"
	"github.com/felixgeelhaar/axi-go/application"
	"github.com/felixgeelhaar/axi-go/domain"
	"github.com/felixgeelhaar/axi-go/inmemory"
)

// --- Sample plugin: "greeter" ---

// greeterPlugin implements domain.Plugin.
type greeterPlugin struct{}

func (p *greeterPlugin) Contribute() (*domain.PluginContribution, error) {
	// Define a "string.upper" capability.
	capName, _ := domain.NewCapabilityName("string.upper")
	cap, _ := domain.NewCapabilityDefinition(capName, "Uppercases a string",
		domain.NewContract([]domain.ContractField{
			{Name: "text", Type: "string", Description: "Text to uppercase", Required: true, Example: "hello"},
		}),
		domain.EmptyContract(),
	)
	_ = cap.BindExecutor("exec.string.upper")

	// Define a "greet" action that requires string.upper.
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

// --- Executor implementations ---

type upperExecutor struct{}

func (e *upperExecutor) Execute(_ context.Context, input any) (any, error) {
	s, ok := input.(string)
	if !ok {
		return nil, fmt.Errorf("expected string, got %T", input)
	}
	return strings.ToUpper(s), nil
}

type greetExecutor struct{}

func (e *greetExecutor) Execute(_ context.Context, input any, caps domain.CapabilityInvoker) (domain.ExecutionResult, []domain.EvidenceRecord, error) {
	m, ok := input.(map[string]any)
	if !ok {
		return domain.ExecutionResult{}, nil, fmt.Errorf("expected map input")
	}
	name, _ := m["name"].(string)

	upper, err := caps.Invoke("string.upper", name)
	if err != nil {
		return domain.ExecutionResult{}, nil, err
	}

	msg := fmt.Sprintf("Hello, %s!", upper)
	return domain.ExecutionResult{
			Data:        map[string]any{"message": msg},
			Summary:     "Greeted " + name,
			ContentType: "application/json",
		}, []domain.EvidenceRecord{
			{Kind: "invocation", Source: "greet", Value: map[string]any{"name": name, "upper": upper}},
		}, nil
}

func main() {
	// Wire infrastructure.
	actionRepo := inmemory.NewActionDefinitionRepository()
	capRepo := inmemory.NewCapabilityDefinitionRepository()
	pluginRepo := inmemory.NewPluginContributionRepository()
	sessionRepo := inmemory.NewExecutionSessionRepository()
	validator := inmemory.NewContractValidator()
	actionExecReg := inmemory.NewActionExecutorRegistry()
	capExecReg := inmemory.NewCapabilityExecutorRegistry()
	idGen := inmemory.NewSequentialIDGenerator()

	// Build domain services.
	compositionSvc := domain.NewCompositionService(actionRepo, capRepo, pluginRepo)
	resolutionSvc := domain.NewCapabilityResolutionService(capRepo)
	executionSvc := domain.NewActionExecutionService(actionRepo, resolutionSvc, validator, actionExecReg, capExecReg)

	// Register the greeter plugin using PluginBundle (atomic registration).
	plugin := &greeterPlugin{}
	contribution, err := plugin.Contribute()
	if err != nil {
		fmt.Fprintln(os.Stderr, "contribute:", err)
		os.Exit(1)
	}

	bundle, err := domain.NewPluginBundle(
		contribution,
		map[domain.ActionExecutorRef]domain.ActionExecutor{
			"exec.greet": &greetExecutor{},
		},
		map[domain.CapabilityExecutorRef]domain.CapabilityExecutor{
			"exec.string.upper": &upperExecutor{},
		},
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bundle:", err)
		os.Exit(1)
	}

	if err := compositionSvc.RegisterBundle(bundle, actionExecReg, capExecReg); err != nil {
		fmt.Fprintln(os.Stderr, "register:", err)
		os.Exit(1)
	}

	// Build use cases.
	executeUC := &application.ExecuteActionUseCase{
		SessionRepo:      sessionRepo,
		ExecutionService: executionSvc,
		IDGen:            idGen,
	}
	registerUC := &application.RegisterPluginContributionUseCase{
		CompositionService: compositionSvc,
	}

	// Start server.
	srv := api.NewServer(executeUC, registerUC, actionRepo, capRepo, sessionRepo)
	addr := ":8080"
	fmt.Println("axi-go example server listening on", addr)
	fmt.Println()
	fmt.Println("Try:")
	fmt.Println("  curl localhost:8080/health")
	fmt.Println("  curl localhost:8080/api/v1/actions")
	fmt.Println("  curl -X POST localhost:8080/api/v1/actions/execute \\")
	fmt.Println("    -H 'Content-Type: application/json' \\")
	fmt.Println("    -d '{\"action_name\":\"greet\",\"input\":{\"name\":\"world\"}}'")

	if err := srv.Run(addr); err != nil {
		fmt.Fprintln(os.Stderr, "server:", err)
		os.Exit(1)
	}
}
