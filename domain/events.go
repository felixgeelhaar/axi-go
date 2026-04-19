package domain

import "time"

// DomainEvent is the marker interface for events raised by aggregates
// during their state transitions. Implementations are immutable value
// objects: once constructed, they are not mutated by anyone.
//
// Adapters classify events into observability (Prometheus counters,
// OpenTelemetry spans), audit logs, persistence outboxes, or analytics
// pipelines — the domain itself has no metric or trace concepts.
type DomainEvent interface {
	// OccurredAt is the wall-clock time at which the event happened.
	OccurredAt() time.Time
	// EventType is a stable string identifier suitable for adapter dispatch
	// (e.g. "session.started"). Stable across releases per the deprecation
	// policy in docs/ROADMAP.md.
	EventType() string
}

// DomainEventPublisher is the port for handling events raised by the
// kernel. The default implementation (NopDomainEventPublisher) discards
// every event, preserving axi-go's zero-dependency posture for callers
// who do not wire observability.
//
// Implementations MUST be safe for concurrent use and SHOULD NOT block
// the caller — Publish is invoked on the hot path of action execution.
// Adapters that need expensive work (network, disk) should buffer
// asynchronously.
type DomainEventPublisher interface {
	Publish(event DomainEvent)
}

// NopDomainEventPublisher discards all events. Used as the default so
// callers that do not configure observability incur zero cost.
type NopDomainEventPublisher struct{}

// Publish discards the event.
func (NopDomainEventPublisher) Publish(DomainEvent) {}

// InvocationOutcome classifies the result of a capability invocation.
// Strict-DDD callers may treat this as a value object; adapters often
// project it directly to a label dimension.
type InvocationOutcome string

const (
	// OutcomeSuccess indicates the capability returned without error.
	OutcomeSuccess InvocationOutcome = "success"
	// OutcomeError indicates the capability returned a non-nil error.
	OutcomeError InvocationOutcome = "error"
)

// BudgetKind identifies which execution budget was exhausted when a
// BudgetExceeded event is raised.
type BudgetKind string

const (
	// BudgetKindDuration indicates ExecutionBudget.MaxDuration was reached.
	BudgetKindDuration BudgetKind = "duration"
	// BudgetKindInvocations indicates ExecutionBudget.MaxCapabilityInvocations was reached.
	BudgetKindInvocations BudgetKind = "invocations"
	// BudgetKindTokens indicates ExecutionBudget.MaxTokens was reached.
	BudgetKindTokens BudgetKind = "tokens"
)

// SessionStarted is raised when an ExecutionSession transitions from
// Pending to Validated — i.e. when execution actually begins after
// input validation passes.
type SessionStarted struct {
	SessionID  ExecutionSessionID
	ActionName ActionName
	At         time.Time
}

// OccurredAt returns the event's wall-clock time.
func (e SessionStarted) OccurredAt() time.Time { return e.At }

// EventType returns "session.started".
func (SessionStarted) EventType() string { return "session.started" }

// SessionAwaitingApproval is raised when an ExecutionSession transitions
// from Resolved to AwaitingApproval — i.e. when an action whose effect
// level is write-external pauses for human approval.
type SessionAwaitingApproval struct {
	SessionID  ExecutionSessionID
	ActionName ActionName
	At         time.Time
}

// OccurredAt returns the event's wall-clock time.
func (e SessionAwaitingApproval) OccurredAt() time.Time { return e.At }

// EventType returns "session.awaiting_approval".
func (SessionAwaitingApproval) EventType() string { return "session.awaiting_approval" }

// SessionCompleted is raised when an ExecutionSession reaches a terminal
// state (Succeeded, Failed, or Rejected). Duration is measured from the
// SessionStarted event.
type SessionCompleted struct {
	SessionID  ExecutionSessionID
	ActionName ActionName
	Status     ExecutionStatus
	Duration   time.Duration
	At         time.Time
}

// OccurredAt returns the event's wall-clock time.
func (e SessionCompleted) OccurredAt() time.Time { return e.At }

// EventType returns "session.completed".
func (SessionCompleted) EventType() string { return "session.completed" }

// CapabilityInvoked is raised by the capability invoker once per
// invocation attempt — including successful retries. The Outcome field
// distinguishes success from error; Duration is the wall-clock cost of
// the underlying executor call.
type CapabilityInvoked struct {
	SessionID  ExecutionSessionID
	ActionName ActionName
	Capability CapabilityName
	Duration   time.Duration
	Outcome    InvocationOutcome
	At         time.Time
}

// OccurredAt returns the event's wall-clock time.
func (e CapabilityInvoked) OccurredAt() time.Time { return e.At }

// EventType returns "capability.invoked".
func (CapabilityInvoked) EventType() string { return "capability.invoked" }

// CapabilityRetried is raised once per retry attempt before the retry
// fires — Attempt is 1 for the first retry, 2 for the second, and so on.
// Only emitted for actions whose IdempotencyProfile permits retries.
type CapabilityRetried struct {
	SessionID  ExecutionSessionID
	ActionName ActionName
	Capability CapabilityName
	Attempt    int
	At         time.Time
}

// OccurredAt returns the event's wall-clock time.
func (e CapabilityRetried) OccurredAt() time.Time { return e.At }

// EventType returns "capability.retried".
func (CapabilityRetried) EventType() string { return "capability.retried" }

// BudgetExceeded is raised when an ExecutionBudget limit is reached
// during execution. Kind identifies which limit (duration, invocations,
// or tokens). The session will subsequently transition to Failed.
type BudgetExceeded struct {
	SessionID  ExecutionSessionID
	ActionName ActionName
	Kind       BudgetKind
	At         time.Time
}

// OccurredAt returns the event's wall-clock time.
func (e BudgetExceeded) OccurredAt() time.Time { return e.At }

// EventType returns "budget.exceeded".
func (BudgetExceeded) EventType() string { return "budget.exceeded" }

// ResultChunkEmitted is raised by ExecutionSession.Emit whenever a
// streaming action appends a chunk. Adapters subscribing to the
// DomainEventPublisher can forward chunks to HTTP/SSE, gRPC stream,
// or MCP SSE consumers in real time without polling the session.
//
// The Chunk value object carries the emission-time payload. At on the
// event echoes Chunk.At so adapters have a single canonical timestamp
// regardless of which field they read.
type ResultChunkEmitted struct {
	SessionID  ExecutionSessionID
	ActionName ActionName
	Chunk      ResultChunk
	At         time.Time
}

// OccurredAt returns the event's wall-clock time.
func (e ResultChunkEmitted) OccurredAt() time.Time { return e.At }

// EventType returns "result.chunk.emitted".
func (ResultChunkEmitted) EventType() string { return "result.chunk.emitted" }

// EvidenceRecorded is raised when an evidence record is appended to an
// ExecutionSession. EvidenceKind is the plugin-defined classifier carried
// on EvidenceRecord.Kind; Tokens mirrors EvidenceRecord.TokensUsed (zero
// when unreported); Hash is the tamper-evident chain hash assigned by
// the aggregate (empty if the record could not be canonicalised for
// hashing).
type EvidenceRecorded struct {
	SessionID    ExecutionSessionID
	ActionName   ActionName
	EvidenceKind string
	Tokens       int64
	Hash         EvidenceHash
	At           time.Time
}

// OccurredAt returns the event's wall-clock time.
func (e EvidenceRecorded) OccurredAt() time.Time { return e.At }

// EventType returns "evidence.recorded".
func (EvidenceRecorded) EventType() string { return "evidence.recorded" }
