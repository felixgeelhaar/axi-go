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

// Contract represents an input or output contract for an action or capability.
type Contract struct {
	Fields []ContractField
}

// ContractField describes a single field in a contract.
type ContractField struct {
	Name        string // Field name (required).
	Type        string // Type hint: "string", "number", "boolean", "object", "array". Empty means any.
	Description string // Human/agent-readable description of what this field is for.
	Required    bool   // Whether this field must be present.
	Example     any    // Example value for documentation and agent guidance.
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
	EffectNone          EffectLevel = "none"
	EffectReadLocal     EffectLevel = "read-local"
	EffectWriteLocal    EffectLevel = "write-local"
	EffectReadExternal  EffectLevel = "read-external"
	EffectWriteExternal EffectLevel = "write-external"
)

// ValidEffectLevel returns true if the given level is a known effect level.
func ValidEffectLevel(level EffectLevel) bool {
	switch level {
	case EffectNone, EffectReadLocal, EffectWriteLocal, EffectReadExternal, EffectWriteExternal:
		return true
	default:
		return false
	}
}

// IsWriteEffect returns true if the effect level involves writes.
func (p EffectProfile) IsWriteEffect() bool {
	return p.Level == EffectWriteLocal || p.Level == EffectWriteExternal
}

// IsExternalEffect returns true if the effect level involves external systems.
func (p EffectProfile) IsExternalEffect() bool {
	return p.Level == EffectReadExternal || p.Level == EffectWriteExternal
}

type EffectProfile struct {
	Level EffectLevel
}

// IdempotencyProfile describes whether an action is idempotent.
type IdempotencyProfile struct {
	IsIdempotent bool
}

// Execution value objects.

// ApprovalDecision records who approved or rejected, and why.
type ApprovalDecision struct {
	Principal string // who approved (user ID, service account, etc.)
	Rationale string // why — free text
}

// Suggestion guides agents toward a logical next action after execution.
// Aligned with axi.md principle #9: contextual next-step suggestions.
type Suggestion struct {
	Action      string // suggested action name
	Description string // why this action makes sense next
}

type ExecutionResult struct {
	Data        any
	Summary     string
	ContentType string       // MIME type hint for agents: "text/plain", "application/json", etc.
	Suggestions []Suggestion // contextual next-step hints for agents
}

type FailureReason struct {
	Code    string
	Message string
}

type EvidenceRecord struct {
	Kind      string
	Source    string
	Value     any
	Timestamp int64 // Unix milliseconds. Zero means not set.
}

// ExecutionStatus represents the state of an execution session.
type ExecutionStatus string

const (
	StatusPending          ExecutionStatus = "pending"
	StatusValidated        ExecutionStatus = "validated"
	StatusResolved         ExecutionStatus = "resolved"
	StatusAwaitingApproval ExecutionStatus = "awaiting_approval"
	StatusRejected         ExecutionStatus = "rejected"
	StatusRunning          ExecutionStatus = "running"
	StatusSucceeded        ExecutionStatus = "succeeded"
	StatusFailed           ExecutionStatus = "failed"
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
