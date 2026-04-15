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

// Accessors.

func (c *CapabilityDefinition) Name() CapabilityName                    { return c.name }
func (c *CapabilityDefinition) Description() string                     { return c.description }
func (c *CapabilityDefinition) InputContract() Contract                 { return c.inputContract }
func (c *CapabilityDefinition) OutputContract() Contract                { return c.outputContract }
func (c *CapabilityDefinition) ExecutionBinding() CapabilityExecutorRef { return c.executionBinding }
