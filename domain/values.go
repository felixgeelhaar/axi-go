// Package domain defines the core domain model for axi-go,
// a domain-driven execution kernel for semantic actions.
package domain

import (
	"errors"
	"fmt"
	"regexp"
)

// Identity value objects.

type ActionName string
type CapabilityName string
type PluginID string
type ExecutionSessionID string

var validNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9._-]*$`)

func NewActionName(s string) (ActionName, error) {
	if s == "" {
		return "", errors.New("action name must not be empty")
	}
	if !validNamePattern.MatchString(s) {
		return "", fmt.Errorf("action name %q is invalid: must match %s", s, validNamePattern.String())
	}
	return ActionName(s), nil
}

func NewCapabilityName(s string) (CapabilityName, error) {
	if s == "" {
		return "", errors.New("capability name must not be empty")
	}
	if !validNamePattern.MatchString(s) {
		return "", fmt.Errorf("capability name %q is invalid: must match %s", s, validNamePattern.String())
	}
	return CapabilityName(s), nil
}

func NewPluginID(s string) (PluginID, error) {
	if s == "" {
		return "", errors.New("plugin ID must not be empty")
	}
	return PluginID(s), nil
}

// Contract represents an input or output contract (abstract, can be JSON Schema or struct-based).
type Contract struct {
	Fields []ContractField
}

type ContractField struct {
	Name     string
	Required bool
}

func NewContract(fields []ContractField) Contract {
	return Contract{Fields: fields}
}

// EmptyContract returns a contract with no fields.
func EmptyContract() Contract {
	return Contract{}
}

// IsEmpty returns true if the contract has no fields.
func (c Contract) IsEmpty() bool {
	return len(c.Fields) == 0
}

// Requirement represents a capability dependency.
type Requirement struct {
	Capability CapabilityName
}

// RequirementSet is a set of requirements with no duplicates.
type RequirementSet []Requirement

func NewRequirementSet(reqs ...Requirement) (RequirementSet, error) {
	seen := make(map[CapabilityName]struct{}, len(reqs))
	for _, r := range reqs {
		if _, exists := seen[r.Capability]; exists {
			return nil, fmt.Errorf("duplicate requirement for capability %q", r.Capability)
		}
		seen[r.Capability] = struct{}{}
	}
	result := make(RequirementSet, len(reqs))
	copy(result, reqs)
	return result, nil
}

// EffectProfile describes the side-effect level of an action.
type EffectLevel string

const (
	EffectNone     EffectLevel = "none"
	EffectLocal    EffectLevel = "local"
	EffectExternal EffectLevel = "external"
)

// ValidEffectLevel returns true if the given level is a known effect level.
func ValidEffectLevel(level EffectLevel) bool {
	switch level {
	case EffectNone, EffectLocal, EffectExternal:
		return true
	default:
		return false
	}
}

type EffectProfile struct {
	Level EffectLevel
}

// IdempotencyProfile describes whether an action is idempotent.
type IdempotencyProfile struct {
	IsIdempotent bool
}

// Execution value objects.

// InvocationInput represents the input to an action execution.
type InvocationInput = any

type ExecutionResult struct {
	Data    any
	Summary string
}

type FailureReason struct {
	Code    string
	Message string
}

type EvidenceRecord struct {
	Kind   string
	Source string
	Value  any
}

// ExecutionStatus represents the state of an execution session.
type ExecutionStatus string

const (
	StatusPending   ExecutionStatus = "pending"
	StatusValidated ExecutionStatus = "validated"
	StatusResolved  ExecutionStatus = "resolved"
	StatusRunning   ExecutionStatus = "running"
	StatusSucceeded ExecutionStatus = "succeeded"
	StatusFailed    ExecutionStatus = "failed"
)

// ContributionStatus represents the state of a plugin contribution.
type ContributionStatus string

const (
	ContributionPending ContributionStatus = "pending"
	ContributionActive  ContributionStatus = "active"
)

// Executor references (binding identifiers).

type ActionExecutorRef string
type CapabilityExecutorRef string
