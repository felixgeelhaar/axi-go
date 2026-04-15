package domain

import (
	"errors"
	"fmt"
)

// Plugin is the interface that plugin authors implement to contribute
// actions and capabilities to the system.
type Plugin interface {
	Contribute() (*PluginContribution, error)
}

// LifecyclePlugin is an optional interface for plugins that need
// initialization with configuration and cleanup on shutdown.
type LifecyclePlugin interface {
	Plugin
	Init(config map[string]any) error
	Close() error
}

// PluginConfig holds configuration passed to a LifecyclePlugin during Init.
type PluginConfig = map[string]any

// PluginContribution is the aggregate root for a contributed extension.
type PluginContribution struct {
	pluginID     PluginID
	actions      []*ActionDefinition
	capabilities []*CapabilityDefinition
	status       ContributionStatus
}

// NewPluginContribution creates a validated PluginContribution.
// Invariants: PluginID required, no duplicate names within contribution.
func NewPluginContribution(
	pluginID PluginID,
	actions []*ActionDefinition,
	capabilities []*CapabilityDefinition,
) (*PluginContribution, error) {
	if pluginID == "" {
		return nil, errors.New("plugin ID is required")
	}

	// Check for duplicate action names within contribution.
	actionNames := make(map[ActionName]struct{}, len(actions))
	for _, a := range actions {
		if _, exists := actionNames[a.Name()]; exists {
			return nil, fmt.Errorf("duplicate action name %q in plugin %q", a.Name(), pluginID)
		}
		actionNames[a.Name()] = struct{}{}
	}

	// Check for duplicate capability names within contribution.
	capNames := make(map[CapabilityName]struct{}, len(capabilities))
	for _, c := range capabilities {
		if _, exists := capNames[c.Name()]; exists {
			return nil, fmt.Errorf("duplicate capability name %q in plugin %q", c.Name(), pluginID)
		}
		capNames[c.Name()] = struct{}{}
	}

	return &PluginContribution{
		pluginID:     pluginID,
		actions:      actions,
		capabilities: capabilities,
		status:       ContributionPending,
	}, nil
}

// Activate transitions the contribution to active status.
// Invariant: all actions and capabilities must be bound before activation.
func (p *PluginContribution) Activate() error {
	if p.status == ContributionActive {
		return errors.New("contribution is already active")
	}
	for _, a := range p.actions {
		if !a.IsBound() {
			return fmt.Errorf("action %q must have an executor binding before activation", a.Name())
		}
	}
	for _, c := range p.capabilities {
		if !c.IsBound() {
			return fmt.Errorf("capability %q must have an executor binding before activation", c.Name())
		}
	}
	p.status = ContributionActive
	return nil
}

// Accessors.

func (p *PluginContribution) PluginID() PluginID         { return p.pluginID }
func (p *PluginContribution) Status() ContributionStatus { return p.status }

func (p *PluginContribution) Actions() []*ActionDefinition {
	out := make([]*ActionDefinition, len(p.actions))
	copy(out, p.actions)
	return out
}

func (p *PluginContribution) Capabilities() []*CapabilityDefinition {
	out := make([]*CapabilityDefinition, len(p.capabilities))
	copy(out, p.capabilities)
	return out
}
