package domain

import (
	"errors"
	"fmt"
)

// validTransitions defines the primary (happy-path) state transitions.
// Special cases (Fail, Reject, AwaitingApproval→Running) are handled explicitly.
var validTransitions = map[ExecutionStatus]ExecutionStatus{
	StatusPending:          StatusValidated,
	StatusValidated:        StatusResolved,
	StatusResolved:         StatusRunning,   // direct path (no approval needed)
	StatusAwaitingApproval: StatusRunning,   // after approval
	StatusRunning:          StatusSucceeded, // or StatusFailed, checked separately
}

// ExecutionSession is the aggregate root for one action execution.
type ExecutionSession struct {
	id         ExecutionSessionID
	actionName ActionName
	input      any

	status ExecutionStatus

	requiresApproval     bool
	resolvedCapabilities []CapabilityName
	evidence             []EvidenceRecord

	result  *ExecutionResult
	failure *FailureReason
}

// NewExecutionSession creates a new session in Pending status.
func NewExecutionSession(
	id ExecutionSessionID,
	actionName ActionName,
	input any,
) (*ExecutionSession, error) {
	if id == "" {
		return nil, errors.New("session ID is required")
	}
	if actionName == "" {
		return nil, errors.New("action name is required")
	}
	return &ExecutionSession{
		id:         id,
		actionName: actionName,
		input:      input,
		status:     StatusPending,
	}, nil
}

// MarkValidated transitions Pending → Validated.
func (s *ExecutionSession) MarkValidated() error {
	return s.transitionTo(StatusValidated)
}

// MarkResolved transitions Validated → Resolved, recording resolved capabilities.
func (s *ExecutionSession) MarkResolved(capabilities []CapabilityName) error {
	if err := s.transitionTo(StatusResolved); err != nil {
		return err
	}
	s.resolvedCapabilities = capabilities
	return nil
}

// MarkAwaitingApproval transitions Resolved → AwaitingApproval.
// Used for actions with external effects that require human-in-the-loop approval.
func (s *ExecutionSession) MarkAwaitingApproval() error {
	if s.status != StatusResolved {
		return fmt.Errorf("cannot transition from %s to %s", s.status, StatusAwaitingApproval)
	}
	s.status = StatusAwaitingApproval
	s.requiresApproval = true
	return nil
}

// Approve transitions AwaitingApproval → Running.
func (s *ExecutionSession) Approve() error {
	if s.status != StatusAwaitingApproval {
		return fmt.Errorf("cannot approve session in %s status", s.status)
	}
	s.status = StatusRunning
	return nil
}

// Reject transitions AwaitingApproval → Rejected with a reason.
func (s *ExecutionSession) Reject(reason FailureReason) error {
	if s.status != StatusAwaitingApproval {
		return fmt.Errorf("cannot reject session in %s status", s.status)
	}
	s.status = StatusRejected
	s.failure = &reason
	return nil
}

// MarkRunning transitions Resolved → Running (skipping approval).
func (s *ExecutionSession) MarkRunning() error {
	return s.transitionTo(StatusRunning)
}

// Succeed transitions Running → Succeeded with a result.
func (s *ExecutionSession) Succeed(result ExecutionResult) error {
	if err := s.transitionTo(StatusSucceeded); err != nil {
		return err
	}
	s.result = &result
	return nil
}

// Fail transitions Running → Failed with a failure reason.
func (s *ExecutionSession) Fail(reason FailureReason) error {
	if s.status != StatusRunning {
		return fmt.Errorf("cannot transition from %s to %s", s.status, StatusFailed)
	}
	s.status = StatusFailed
	s.failure = &reason
	return nil
}

// AppendEvidence adds an evidence record (append-only).
func (s *ExecutionSession) AppendEvidence(record EvidenceRecord) {
	s.evidence = append(s.evidence, record)
}

func (s *ExecutionSession) transitionTo(target ExecutionStatus) error {
	expected, ok := validTransitions[s.status]
	if !ok || expected != target {
		return fmt.Errorf("cannot transition from %s to %s", s.status, target)
	}
	s.status = target
	return nil
}

// Accessors.

func (s *ExecutionSession) ID() ExecutionSessionID   { return s.id }
func (s *ExecutionSession) ActionName() ActionName   { return s.actionName }
func (s *ExecutionSession) Input() any               { return s.input }
func (s *ExecutionSession) Status() ExecutionStatus  { return s.status }
func (s *ExecutionSession) RequiresApproval() bool   { return s.requiresApproval }
func (s *ExecutionSession) Result() *ExecutionResult { return s.result }
func (s *ExecutionSession) Failure() *FailureReason  { return s.failure }

func (s *ExecutionSession) Evidence() []EvidenceRecord {
	out := make([]EvidenceRecord, len(s.evidence))
	copy(out, s.evidence)
	return out
}

func (s *ExecutionSession) ResolvedCapabilities() []CapabilityName {
	out := make([]CapabilityName, len(s.resolvedCapabilities))
	copy(out, s.resolvedCapabilities)
	return out
}
