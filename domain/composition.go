package domain

import (
	"fmt"
	"sync"
)

// CompositionService handles plugin registration, conflict detection, and activation.
// All mutation methods are serialized via a mutex to prevent TOCTOU races.
type CompositionService struct {
	mu             sync.Mutex
	actionRepo     ActionRepository
	capabilityRepo CapabilityRepository
	pluginRepo     PluginRepository
}

// ActionRepository is the port interface for action definition storage.
type ActionRepository interface {
	GetByName(name ActionName) (*ActionDefinition, error)
	Save(action *ActionDefinition) error
	Delete(name ActionName) error
	List() []*ActionDefinition
}

// CapabilityRepository is the port interface for capability definition storage.
type CapabilityRepository interface {
	GetByName(name CapabilityName) (*CapabilityDefinition, error)
	Save(capability *CapabilityDefinition) error
	Delete(name CapabilityName) error
	List() []*CapabilityDefinition
}

// PluginRepository is the port interface for plugin contribution storage.
type PluginRepository interface {
	Save(contribution *PluginContribution) error
	GetByID(id PluginID) (*PluginContribution, error)
	Delete(id PluginID) error
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
// RegisterBundle atomically registers a plugin's contribution and its executor
// implementations. This is the preferred registration method as it validates
// that all executor refs have matching implementations before persisting anything.
func (s *CompositionService) RegisterBundle(
	bundle *PluginBundle,
	actionExecReg ActionExecutorLookup,
	capExecReg CapabilityExecutorLookup,
) error {
	// The bundle constructor already validated ref↔executor pairing.
	// Register executors first (into the registries).
	type actionRegistrar interface {
		Register(ref ActionExecutorRef, executor ActionExecutor)
	}
	type capRegistrar interface {
		Register(ref CapabilityExecutorRef, executor CapabilityExecutor)
	}
	if reg, ok := actionExecReg.(actionRegistrar); ok {
		for ref, exec := range bundle.ActionExecutors {
			reg.Register(ref, exec)
		}
	}
	if reg, ok := capExecReg.(capRegistrar); ok {
		for ref, exec := range bundle.CapabilityExecutors {
			reg.Register(ref, exec)
		}
	}

	return s.RegisterContribution(bundle.Contribution)
}

// RegisterPlugin accepts a Plugin, optionally initializes it with config,
// calls Contribute(), and registers the result.
func (s *CompositionService) RegisterPlugin(plugin Plugin) error {
	return s.RegisterPluginWithConfig(plugin, nil)
}

// RegisterPluginWithConfig accepts a Plugin with configuration.
// If the plugin implements LifecyclePlugin, Init(config) is called before Contribute().
func (s *CompositionService) RegisterPluginWithConfig(plugin Plugin, config PluginConfig) error {
	if lp, ok := plugin.(LifecyclePlugin); ok {
		if err := lp.Init(config); err != nil {
			return fmt.Errorf("plugin init failed: %w", err)
		}
	}
	contribution, err := plugin.Contribute()
	if err != nil {
		return fmt.Errorf("plugin contribution failed: %w", err)
	}
	return s.RegisterContribution(contribution)
}

// RegisterContribution validates, detects conflicts, persists, and activates a contribution.
func (s *CompositionService) RegisterContribution(contribution *PluginContribution) error {
	s.mu.Lock()
	defer s.mu.Unlock()
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

	// Persist actions, capabilities, and the contribution.
	// Track what was saved so we can rollback on partial failure.
	var savedActions []ActionName
	var savedCaps []CapabilityName

	rollback := func() {
		for _, name := range savedActions {
			_ = s.actionRepo.Delete(name)
		}
		for _, name := range savedCaps {
			_ = s.capabilityRepo.Delete(name)
		}
	}

	for _, action := range contribution.Actions() {
		if err := s.actionRepo.Save(action); err != nil {
			rollback()
			return fmt.Errorf("failed to save action %q: %w", action.Name(), err)
		}
		savedActions = append(savedActions, action.Name())
	}
	for _, cap := range contribution.Capabilities() {
		if err := s.capabilityRepo.Save(cap); err != nil {
			rollback()
			return fmt.Errorf("failed to save capability %q: %w", cap.Name(), err)
		}
		savedCaps = append(savedCaps, cap.Name())
	}

	if err := s.pluginRepo.Save(contribution); err != nil {
		rollback()
		return fmt.Errorf("failed to save plugin contribution: %w", err)
	}

	return nil
}

// DeregisterPlugin removes a plugin and all its contributed actions and capabilities.
func (s *CompositionService) DeregisterPlugin(id PluginID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	contribution, err := s.pluginRepo.GetByID(id)
	if err != nil {
		return &ErrNotFound{Entity: "plugin", ID: string(id)}
	}

	for _, action := range contribution.Actions() {
		_ = s.actionRepo.Delete(action.Name())
	}
	for _, cap := range contribution.Capabilities() {
		_ = s.capabilityRepo.Delete(cap.Name())
	}

	return s.pluginRepo.Delete(id)
}
