package domain

import "errors"

// CapabilityDefinition is the aggregate root for a low-level executable capability.
type CapabilityDefinition struct {
	name        CapabilityName
	description string

	inputContract  Contract
	outputContract Contract

	executionBinding CapabilityExecutorRef
}

// NewCapabilityDefinition creates a validated CapabilityDefinition.
func NewCapabilityDefinition(
	name CapabilityName,
	description string,
	inputContract Contract,
	outputContract Contract,
) (*CapabilityDefinition, error) {
	if name == "" {
		return nil, errors.New("capability name is required")
	}
	return &CapabilityDefinition{
		name:           name,
		description:    description,
		inputContract:  inputContract,
		outputContract: outputContract,
	}, nil
}

// BindExecutor sets the execution binding.
func (c *CapabilityDefinition) BindExecutor(ref CapabilityExecutorRef) error {
	if ref == "" {
		return errors.New("executor ref must not be empty")
	}
	c.executionBinding = ref
	return nil
}

// IsBound returns true if an executor is bound.
func (c *CapabilityDefinition) IsBound() bool {
	return c.executionBinding != ""
}

// Name returns the unique capability name.
func (c *CapabilityDefinition) Name() CapabilityName { return c.name }

// Description returns the human-readable description of the capability.
func (c *CapabilityDefinition) Description() string { return c.description }

// InputContract returns the input contract declared by this capability.
func (c *CapabilityDefinition) InputContract() Contract { return c.inputContract }

// OutputContract returns the output contract declared by this capability.
func (c *CapabilityDefinition) OutputContract() Contract { return c.outputContract }

// ExecutionBinding returns the executor reference bound to this capability,
// or the empty ref if BindExecutor has not yet been called.
func (c *CapabilityDefinition) ExecutionBinding() CapabilityExecutorRef { return c.executionBinding }

// CapabilitySummary is a minimal, discovery-oriented projection of
// CapabilityDefinition, aligned with axi.md principle #2.
type CapabilitySummary struct {
	Name        string
	Description string
}

// Summary returns the discovery-oriented projection of this capability.
func (c *CapabilityDefinition) Summary() CapabilitySummary {
	return CapabilitySummary{
		Name:        string(c.name),
		Description: c.description,
	}
}
