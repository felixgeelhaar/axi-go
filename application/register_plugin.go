// Package application contains the use cases for axi-go.
package application

import "github.com/felixgeelhaar/axi-go/domain"

// RegisterPluginContributionUseCase handles plugin registration.
type RegisterPluginContributionUseCase struct {
	CompositionService *domain.CompositionService
}

// Execute registers a plugin contribution through the composition service.
func (uc *RegisterPluginContributionUseCase) Execute(contribution *domain.PluginContribution) error {
	return uc.CompositionService.RegisterContribution(contribution)
}

// ExecutePlugin registers a plugin via the Plugin interface.
func (uc *RegisterPluginContributionUseCase) ExecutePlugin(plugin domain.Plugin) error {
	return uc.CompositionService.RegisterPlugin(plugin)
}
