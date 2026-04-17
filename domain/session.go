package domain

import (
	"errors"
	"fmt"
	"sync"
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
// Thread-safe: all mutations and reads are protected by a mutex
// to support concurrent access in async execution mode.
type ExecutionSession struct {
	mu sync.RWMutex

	id         ExecutionSessionID
	actionName ActionName
	input      any

	status ExecutionStatus

	requiresApproval     bool
	resolvedCapabilities []CapabilityName
	evidence             []EvidenceRecord

	result           *ExecutionResult
	failure          *FailureReason
	approvalDecision *ApprovalDecision
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
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.transitionTo(StatusValidated)
}

// MarkResolved transitions Validated → Resolved, recording resolved capabilities.
func (s *ExecutionSession) MarkResolved(capabilities []CapabilityName) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.transitionTo(StatusResolved); err != nil {
		return err
	}
	s.resolvedCapabilities = capabilities
	return nil
}

// MarkAwaitingApproval transitions Resolved → AwaitingApproval.
func (s *ExecutionSession) MarkAwaitingApproval() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status != StatusResolved {
		return fmt.Errorf("cannot transition from %s to %s", s.status, StatusAwaitingApproval)
	}
	s.status = StatusAwaitingApproval
	s.requiresApproval = true
	return nil
}

// Approve transitions AwaitingApproval → Running.
// The decision must include a non-empty Principal identifying who approved.
func (s *ExecutionSession) Approve(decision ApprovalDecision) error {
	if decision.Principal == "" {
		return errors.New("approval decision requires a non-empty principal")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status != StatusAwaitingApproval {
		return fmt.Errorf("cannot approve session in %s status", s.status)
	}
	s.approvalDecision = &decision
	s.status = StatusRunning
	return nil
}

// Reject transitions AwaitingApproval → Rejected with a reason.
// The decision must include a non-empty Principal identifying who rejected.
func (s *ExecutionSession) Reject(reason FailureReason, decision ApprovalDecision) error {
	if decision.Principal == "" {
		return errors.New("approval decision requires a non-empty principal")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status != StatusAwaitingApproval {
		return fmt.Errorf("cannot reject session in %s status", s.status)
	}
	s.approvalDecision = &decision
	s.status = StatusRejected
	s.failure = &reason
	return nil
}

// MarkRunning transitions Resolved → Running (skipping approval).
func (s *ExecutionSession) MarkRunning() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.transitionTo(StatusRunning)
}

// Succeed transitions Running → Succeeded with a result.
func (s *ExecutionSession) Succeed(result ExecutionResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.transitionTo(StatusSucceeded); err != nil {
		return err
	}
	s.result = &result
	return nil
}

// Fail transitions Running → Failed with a failure reason.
func (s *ExecutionSession) Fail(reason FailureReason) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status != StatusRunning {
		return fmt.Errorf("cannot transition from %s to %s", s.status, StatusFailed)
	}
	s.status = StatusFailed
	s.failure = &reason
	return nil
}

// AppendEvidence adds an evidence record (append-only).
func (s *ExecutionSession) AppendEvidence(record EvidenceRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
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

// ID returns the session identifier (set at construction; immutable).
func (s *ExecutionSession) ID() ExecutionSessionID { return s.id }

// ActionName returns the name of the action this session is executing
// (set at construction; immutable).
func (s *ExecutionSession) ActionName() ActionName { return s.actionName }

// Input returns the raw input supplied to the action (immutable).
func (s *ExecutionSession) Input() any { return s.input }

// Status returns the current execution status under a read lock,
// safe for concurrent callers.
func (s *ExecutionSession) Status() ExecutionStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// RequiresApproval reports whether this session paused (or will pause) at
// AwaitingApproval — true for actions with write-external effect level.
func (s *ExecutionSession) RequiresApproval() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.requiresApproval
}

// Result returns the execution result if the session has Succeeded,
// otherwise nil. Callers should not mutate the returned value.
func (s *ExecutionSession) Result() *ExecutionResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.result
}

// Failure returns the failure reason if the session has Failed or been
// Rejected, otherwise nil.
func (s *ExecutionSession) Failure() *FailureReason {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.failure
}

// Evidence returns a defensive copy of the append-only evidence trail
// collected during execution.
func (s *ExecutionSession) Evidence() []EvidenceRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]EvidenceRecord, len(s.evidence))
	copy(out, s.evidence)
	return out
}

// ApprovalDecision returns the recorded approval/rejection decision,
// or nil if the session has not transitioned through AwaitingApproval.
func (s *ExecutionSession) ApprovalDecision() *ApprovalDecision {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.approvalDecision
}

// ResolvedCapabilities returns a defensive copy of the capabilities
// resolved for this session by the CapabilityResolutionService.
func (s *ExecutionSession) ResolvedCapabilities() []CapabilityName {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]CapabilityName, len(s.resolvedCapabilities))
	copy(out, s.resolvedCapabilities)
	return out
}
