package inmemory

import (
	"fmt"

	"github.com/felixgeelhaar/axi-go/domain"
)

// Compile-time interface satisfaction check.
var _ domain.ContractValidator = (*ContractValidator)(nil)

// ContractValidator validates input against a contract using field-based checks.
type ContractValidator struct{}

func NewContractValidator() *ContractValidator {
	return &ContractValidator{}
}

func (v *ContractValidator) Validate(contract domain.Contract, input any) error {
	if contract.IsEmpty() {
		return nil
	}

	inputMap, ok := input.(map[string]any)
	if !ok {
		return fmt.Errorf("input must be a map[string]any when contract has fields, got %T", input)
	}

	for _, field := range contract.Fields {
		if field.Required {
			if _, exists := inputMap[field.Name]; !exists {
				return fmt.Errorf("required field %q is missing", field.Name)
			}
		}
	}

	return nil
}
