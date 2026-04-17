package domain

import "errors"

// ActionDefinition is the aggregate root for a semantic action.
type ActionDefinition struct {
	name        ActionName
	description string

	inputContract  Contract
	outputContract Contract

	requirements RequirementSet

	effectProfile      EffectProfile
	idempotencyProfile IdempotencyProfile

	executionBinding ActionExecutorRef
}

// NewActionDefinition creates a validated ActionDefinition.
// Invariants: name must be valid, contracts must exist.
func NewActionDefinition(
	name ActionName,
	description string,
	inputContract Contract,
	outputContract Contract,
	requirements RequirementSet,
	effectProfile EffectProfile,
	idempotencyProfile IdempotencyProfile,
) (*ActionDefinition, error) {
	if name == "" {
		return nil, errors.New("action name is required")
	}
	return &ActionDefinition{
		name:               name,
		description:        description,
		inputContract:      inputContract,
		outputContract:     outputContract,
		requirements:       requirements,
		effectProfile:      effectProfile,
		idempotencyProfile: idempotencyProfile,
	}, nil
}

// BindExecutor sets the execution binding. Must be set before activation.
func (a *ActionDefinition) BindExecutor(ref ActionExecutorRef) error {
	if ref == "" {
		return errors.New("executor ref must not be empty")
	}
	a.executionBinding = ref
	return nil
}

// IsBound returns true if an executor is bound.
func (a *ActionDefinition) IsBound() bool {
	return a.executionBinding != ""
}

// Accessors.

func (a *ActionDefinition) Name() ActionName         { return a.name }
func (a *ActionDefinition) Description() string      { return a.description }
func (a *ActionDefinition) InputContract() Contract  { return a.inputContract }
func (a *ActionDefinition) OutputContract() Contract { return a.outputContract }
func (a *ActionDefinition) Requirements() RequirementSet {
	out := make(RequirementSet, len(a.requirements))
	copy(out, a.requirements)
	return out
}
func (a *ActionDefinition) EffectProfile() EffectProfile           { return a.effectProfile }
func (a *ActionDefinition) IdempotencyProfile() IdempotencyProfile { return a.idempotencyProfile }
func (a *ActionDefinition) ExecutionBinding() ActionExecutorRef    { return a.executionBinding }

// ActionSummary is a minimal, discovery-oriented projection of ActionDefinition.
// It carries only the fields an agent typically needs to choose between tools,
// aligned with axi.md principle #2 (minimal default schemas).
type ActionSummary struct {
	Name        string
	Description string
	Effect      EffectLevel
	Idempotent  bool
}

// Summary returns the discovery-oriented projection of this action.
func (a *ActionDefinition) Summary() ActionSummary {
	return ActionSummary{
		Name:        string(a.name),
		Description: a.description,
		Effect:      a.effectProfile.Level,
		Idempotent:  a.idempotencyProfile.IsIdempotent,
	}
}
