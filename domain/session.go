package domain

import (
	"errors"
	"fmt"
	"sync"
	"time"
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
//
// Following strict DDD, the aggregate raises domain events as part of
// its state transitions. Events accumulate in pendingEvents and are
// drained by the application service via PullEvents after each call,
// then forwarded to a DomainEventPublisher. The aggregate itself does
// not know about publishers — that is infrastructure.
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

	// startedAt captures the wall-clock time at which the session became
	// Validated. Used to compute Duration on the SessionCompleted event.
	startedAt time.Time

	// resultChunks accumulates progressive output from
	// StreamingActionExecutor implementations. Ordered by assignment,
	// which is monotonic under the aggregate's mutex.
	resultChunks []ResultChunk

	// pendingEvents accumulates domain events raised during state
	// transitions; drained by PullEvents.
	pendingEvents []DomainEvent
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

// MarkValidated transitions Pending → Validated and raises SessionStarted.
func (s *ExecutionSession) MarkValidated() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.transitionTo(StatusValidated); err != nil {
		return err
	}
	now := time.Now()
	s.startedAt = now
	s.recordEvent(SessionStarted{
		SessionID:  s.id,
		ActionName: s.actionName,
		At:         now,
	})
	return nil
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

// MarkAwaitingApproval transitions Resolved → AwaitingApproval and
// raises SessionAwaitingApproval.
func (s *ExecutionSession) MarkAwaitingApproval() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status != StatusResolved {
		return fmt.Errorf("cannot transition from %s to %s", s.status, StatusAwaitingApproval)
	}
	s.status = StatusAwaitingApproval
	s.requiresApproval = true
	s.recordEvent(SessionAwaitingApproval{
		SessionID:  s.id,
		ActionName: s.actionName,
		At:         time.Now(),
	})
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

// Reject transitions AwaitingApproval → Rejected with a reason and
// raises SessionCompleted (Status: Rejected).
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
	s.recordCompletion(StatusRejected)
	return nil
}

// MarkRunning transitions Resolved → Running (skipping approval).
func (s *ExecutionSession) MarkRunning() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.transitionTo(StatusRunning)
}

// Succeed transitions Running → Succeeded and raises SessionCompleted
// (Status: Succeeded).
func (s *ExecutionSession) Succeed(result ExecutionResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.transitionTo(StatusSucceeded); err != nil {
		return err
	}
	s.result = &result
	s.recordCompletion(StatusSucceeded)
	return nil
}

// Fail transitions Running → Failed and raises SessionCompleted
// (Status: Failed).
func (s *ExecutionSession) Fail(reason FailureReason) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status != StatusRunning {
		return fmt.Errorf("cannot transition from %s to %s", s.status, StatusFailed)
	}
	s.status = StatusFailed
	s.failure = &reason
	s.recordCompletion(StatusFailed)
	return nil
}

// Emit appends a progressive-output chunk to the session and raises a
// ResultChunkEmitted event. Satisfies the ResultStream port, so the
// kernel can pass a *ExecutionSession directly to
// StreamingActionExecutor.ExecuteStream.
//
// The aggregate stamps Index (monotonic under the session's mutex,
// safe for concurrent Emit calls) and fills At with time.Now() if the
// executor left it zero. Any Index the executor pre-sets is overwritten.
func (s *ExecutionSession) Emit(chunk ResultChunk) {
	s.mu.Lock()
	defer s.mu.Unlock()

	chunk.Index = len(s.resultChunks)
	if chunk.At.IsZero() {
		chunk.At = time.Now()
	}
	s.resultChunks = append(s.resultChunks, chunk)

	s.recordEvent(ResultChunkEmitted{
		SessionID:  s.id,
		ActionName: s.actionName,
		Chunk:      chunk,
		At:         chunk.At,
	})
}

// ResultChunks returns a defensive copy of all chunks emitted so far
// by a streaming executor, in emission order. Returns an empty slice
// (never nil) when no chunks have been emitted — consistent with
// axi.md principle #5 on definitive empty states.
func (s *ExecutionSession) ResultChunks() []ResultChunk {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ResultChunk, len(s.resultChunks))
	copy(out, s.resultChunks)
	return out
}

// AppendEvidence adds an evidence record (append-only), stamps it with
// its position in the tamper-evident hash chain, and raises an
// EvidenceRecorded event.
//
// Any Hash / PreviousHash values the caller set on the record are
// overwritten — only the session may assign chain positions. If the
// record's Value cannot be canonicalised to JSON, Hash is left empty
// and the record becomes unverifiable (see VerifyEvidenceChain).
func (s *ExecutionSession) AppendEvidence(record EvidenceRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var previousHash EvidenceHash
	if n := len(s.evidence); n > 0 {
		previousHash = s.evidence[n-1].Hash
	}
	record.PreviousHash = previousHash
	record.Hash = computeEvidenceHash(record, previousHash)

	s.evidence = append(s.evidence, record)
	s.recordEvent(EvidenceRecorded{
		SessionID:    s.id,
		ActionName:   s.actionName,
		EvidenceKind: record.Kind,
		Tokens:       record.TokensUsed,
		Hash:         record.Hash,
		At:           time.Now(),
	})
}

// VerifyEvidenceChain validates the integrity of the session's evidence
// trail. It recomputes each record's hash from its canonical form and
// the declared PreviousHash and compares against the stored Hash.
//
// A record with empty Hash is treated as unverifiable and does not
// break the chain — pre-1.1 persisted records loaded from snapshots
// predate the chain and are tolerated, as are records whose Value
// failed canonical-form encoding at AppendEvidence time.
//
// Returns nil on a valid chain. Returns *ErrChainBroken pointing at
// the 0-based index of the first offending record otherwise.
func (s *ExecutionSession) VerifyEvidenceChain() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var previousHash EvidenceHash
	for i, record := range s.evidence {
		if record.Hash == "" {
			// Unverifiable record; skip without failing the chain, but
			// reset the expected previousHash since there's no anchor.
			previousHash = ""
			continue
		}
		if record.PreviousHash != previousHash {
			return &ErrChainBroken{
				Index:  i,
				Reason: fmt.Sprintf("PreviousHash mismatch: record carries %q, expected %q", record.PreviousHash, previousHash),
			}
		}
		expected := computeEvidenceHash(EvidenceRecord{
			Kind:       record.Kind,
			Source:     record.Source,
			Value:      record.Value,
			Timestamp:  record.Timestamp,
			TokensUsed: record.TokensUsed,
		}, previousHash)
		if expected != record.Hash {
			return &ErrChainBroken{
				Index:  i,
				Reason: fmt.Sprintf("Hash mismatch: record carries %q, recomputed %q", record.Hash, expected),
			}
		}
		previousHash = record.Hash
	}
	return nil
}

func (s *ExecutionSession) transitionTo(target ExecutionStatus) error {
	expected, ok := validTransitions[s.status]
	if !ok || expected != target {
		return fmt.Errorf("cannot transition from %s to %s", s.status, target)
	}
	s.status = target
	return nil
}

// recordEvent appends an event to the pending buffer. Caller must hold
// the write lock.
func (s *ExecutionSession) recordEvent(event DomainEvent) {
	s.pendingEvents = append(s.pendingEvents, event)
}

// recordCompletion raises a SessionCompleted event with Duration measured
// from the session's startedAt timestamp. If startedAt is zero (e.g. a
// session that fails before MarkValidated), Duration is reported as zero.
// Caller must hold the write lock.
func (s *ExecutionSession) recordCompletion(status ExecutionStatus) {
	now := time.Now()
	var duration time.Duration
	if !s.startedAt.IsZero() {
		duration = now.Sub(s.startedAt)
	}
	s.recordEvent(SessionCompleted{
		SessionID:  s.id,
		ActionName: s.actionName,
		Status:     status,
		Duration:   duration,
		At:         now,
	})
}

// PullEvents returns and clears the pending domain-event buffer.
// The application service calls this after each state-mutating operation
// and forwards the events to the configured DomainEventPublisher.
//
// Returns nil when the buffer is empty (the caller can range over nil
// safely; allocation is avoided on the hot path).
func (s *ExecutionSession) PullEvents() []DomainEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.pendingEvents) == 0 {
		return nil
	}
	out := s.pendingEvents
	s.pendingEvents = nil
	return out
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
