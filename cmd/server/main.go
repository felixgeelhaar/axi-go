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

	registerUC := &application.RegisterPluginContributionUseCase{
		CompositionService: compositionService,
	}
	executeUC := &application.ExecuteActionUseCase{
		SessionRepo:      sessionRepo,
		ExecutionService: executionService,
		IDGen:            idGen,
	}

	srv := api.NewServer(executeUC, registerUC, actionRepo, capRepo, sessionRepo)

	addr := ":8080"
	fmt.Println("axi-go server listening on", addr) //nolint:errcheck
	if err := srv.Run(addr); err != nil {
		fmt.Fprintln(os.Stderr, "server error:", err)
		os.Exit(1)
	}
}
