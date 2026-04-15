// Package main is the entrypoint for the axi-go HTTP server.
package main

import (
	"fmt"
	"os"

	"github.com/felixgeelhaar/axi-go/api"
	"github.com/felixgeelhaar/axi-go/application"
	"github.com/felixgeelhaar/axi-go/domain"
	"github.com/felixgeelhaar/axi-go/inmemory"
)

func main() {
	cfg := api.ConfigFromEnv()
	logger := inmemory.NewStdLogger(inmemory.LevelInfo)

	actionRepo := inmemory.NewActionDefinitionRepository()
	capRepo := inmemory.NewCapabilityDefinitionRepository()
	pluginRepo := inmemory.NewPluginContributionRepository()
	sessionRepo := inmemory.NewExecutionSessionRepository()
	validator := inmemory.NewContractValidator()
	actionExecReg := inmemory.NewActionExecutorRegistry()
	capExecReg := inmemory.NewCapabilityExecutorRegistry()
	idGen := inmemory.NewSequentialIDGenerator()

	compositionService := domain.NewCompositionService(actionRepo, capRepo, pluginRepo)
	resolutionService := domain.NewCapabilityResolutionService(capRepo)
	executionService := domain.NewActionExecutionService(
		actionRepo, resolutionService, validator, actionExecReg, capExecReg,
	)
	executionService.SetLogger(logger)
	executionService.SetDefaultBudget(cfg.DefaultBudget)

	registerUC := &application.RegisterPluginContributionUseCase{
		CompositionService: compositionService,
	}
	executeUC := &application.ExecuteActionUseCase{
		SessionRepo:      sessionRepo,
		ExecutionService: executionService,
		IDGen:            idGen,
	}

	srv := api.NewServer(executeUC, registerUC, actionRepo, capRepo, sessionRepo)

	fmt.Println("axi-go server listening on", cfg.Addr)
	if err := srv.RunWithConfig(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "server error:", err)
		os.Exit(1)
	}
}
