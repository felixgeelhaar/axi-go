package domain

import "fmt"

// CompositionService handles plugin registration, conflict detection, and activation.
type CompositionService struct {
	actionRepo     ActionRepository
	capabilityRepo CapabilityRepository
	pluginRepo     PluginRepository
}

// ActionRepository is the port interface for action definition storage.
type ActionRepository interface {
	GetByName(name ActionName) (*ActionDefinition, error)
	Save(action *ActionDefinition) error
	List() []*ActionDefinition
}

// CapabilityRepository is the port interface for capability definition storage.
type CapabilityRepository interface {
	GetByName(name CapabilityName) (*CapabilityDefinition, error)
	Save(capability *CapabilityDefinition) error
	List() []*CapabilityDefinition
}

// PluginRepository is the port interface for plugin contribution storage.
type PluginRepository interface {
	Save(contribution *PluginContribution) error
	GetByID(id PluginID) (*PluginContribution, error)
	Exists(id PluginID) bool
}

// SessionRepository is the port interface for execution session storage.
type SessionRepository interface {
	Save(session *ExecutionSession) error
	Get(id ExecutionSessionID) (*ExecutionSession, error)
}

// NewCompositionService creates a CompositionService.
func NewCompositionService(
	actionRepo ActionRepository,
	capabilityRepo CapabilityRepository,
	pluginRepo PluginRepository,
) *CompositionService {
	return &CompositionService{
		actionRepo:     actionRepo,
		capabilityRepo: capabilityRepo,
		pluginRepo:     pluginRepo,
	}
}

// RegisterPlugin accepts a Plugin, calls Contribute(), and registers the result.
func (s *CompositionService) RegisterPlugin(plugin Plugin) error {
	contribution, err := plugin.Contribute()
	if err != nil {
		return fmt.Errorf("plugin contribution failed: %w", err)
	}
	return s.RegisterContribution(contribution)
}

// RegisterContribution validates, detects conflicts, persists, and activates a contribution.
func (s *CompositionService) RegisterContribution(contribution *PluginContribution) error {
	if s.pluginRepo.Exists(contribution.PluginID()) {
		return &ErrConflict{Message: fmt.Sprintf("plugin %q is already registered", contribution.PluginID())}
	}

	// Check global action name uniqueness.
	for _, action := range contribution.Actions() {
		_, err := s.actionRepo.GetByName(action.Name())
		if err == nil {
			return &ErrConflict{Message: fmt.Sprintf("action name %q conflicts with existing registration", action.Name())}
		}
	}

	// Check global capability name uniqueness.
	for _, cap := range contribution.Capabilities() {
		_, err := s.capabilityRepo.GetByName(cap.Name())
		if err == nil {
			return &ErrConflict{Message: fmt.Sprintf("capability name %q conflicts with existing registration", cap.Name())}
		}
	}

	// Activate the contribution.
	if err := contribution.Activate(); err != nil {
		return fmt.Errorf("cannot activate contribution: %w", err)
	}

	// Persist actions and capabilities.
	for _, action := range contribution.Actions() {
		if err := s.actionRepo.Save(action); err != nil {
			return fmt.Errorf("failed to save action %q: %w", action.Name(), err)
		}
	}
	for _, cap := range contribution.Capabilities() {
		if err := s.capabilityRepo.Save(cap); err != nil {
			return fmt.Errorf("failed to save capability %q: %w", cap.Name(), err)
		}
	}

	// Persist the contribution itself.
	if err := s.pluginRepo.Save(contribution); err != nil {
		return fmt.Errorf("failed to save plugin contribution: %w", err)
	}

	return nil
}
