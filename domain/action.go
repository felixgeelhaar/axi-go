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

// Name returns the unique action name.
func (a *ActionDefinition) Name() ActionName { return a.name }

// Description returns the human-readable description of the action.
func (a *ActionDefinition) Description() string { return a.description }

// InputContract returns the input contract declared by this action.
func (a *ActionDefinition) InputContract() Contract { return a.inputContract }

// OutputContract returns the output contract declared by this action.
func (a *ActionDefinition) OutputContract() Contract { return a.outputContract }

// Requirements returns a defensive copy of the capability requirements
// this action depends on.
func (a *ActionDefinition) Requirements() RequirementSet {
	out := make(RequirementSet, len(a.requirements))
	copy(out, a.requirements)
	return out
}

// EffectProfile returns the side-effect classification for this action.
func (a *ActionDefinition) EffectProfile() EffectProfile { return a.effectProfile }

// IdempotencyProfile returns the idempotency declaration for this action.
func (a *ActionDefinition) IdempotencyProfile() IdempotencyProfile { return a.idempotencyProfile }

// ExecutionBinding returns the executor reference bound to this action,
// or the empty ref if BindExecutor has not yet been called.
func (a *ActionDefinition) ExecutionBinding() ActionExecutorRef { return a.executionBinding }

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
