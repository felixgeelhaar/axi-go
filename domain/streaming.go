package domain

import (
	"context"
	"time"
)

// ResultChunk is one piece of progressively-emitted output from a
// streaming action — a token batch, a row, a progress update, anything
// the executor wants to stream before its final ExecutionResult is ready.
//
// Chunks are immutable value objects. The aggregate (ExecutionSession)
// stamps Index as chunks are appended; any Index the executor pre-sets
// is overwritten so ordering is monotonic under concurrent Emit calls.
// At is populated from time.Now() if the executor leaves it zero.
type ResultChunk struct {
	// Index is the 0-based position of this chunk in the session's
	// chunk trail. Assigned by ExecutionSession.Emit — executors
	// should leave this zero; any non-zero value will be overwritten.
	Index int
	// Kind is a plugin-defined classifier — "text", "json", "progress",
	// "row", or anything else that helps adapters route the chunk.
	// Free-form by design; not validated by the kernel.
	Kind string
	// Data is the chunk's payload, typed however the executor sees fit.
	Data any
	// ContentType is a MIME hint for the payload: "text/plain",
	// "application/json", "text/event-stream", etc. Adapters use it to
	// pick encoding on delivery.
	ContentType string
	// At is the wall-clock time at which the chunk was produced.
	// If zero, ExecutionSession.Emit fills it with time.Now().
	At time.Time
}

// StreamingActionExecutor is the optional companion to ActionExecutor
// for actions that emit progressive output. Implementations that can
// stream MUST also implement ActionExecutor so the kernel can fall
// back to the synchronous path when streaming is not wired; the
// StreamingActionExecutor interface adds a single method and the
// kernel prefers ExecuteStream when both are implemented.
//
// The stream argument is a narrow port — the executor can only call
// Emit on it. Chunks appended via Emit are buffered on the session
// aggregate and raised as ResultChunkEmitted domain events for adapter
// delivery. The final ExecutionResult returned by ExecuteStream is the
// aggregate summary (often carrying the last chunk or a metadata blob);
// poll-based callers consume it, live-stream consumers read chunks.
type StreamingActionExecutor interface {
	ExecuteStream(
		ctx context.Context,
		input any,
		invoker CapabilityInvoker,
		stream ResultStream,
	) (ExecutionResult, []EvidenceRecord, error)
}

// ResultStream is the domain port a StreamingActionExecutor uses to
// emit chunks. The kernel passes a session-backed implementation so
// chunks accumulate on the session and raise ResultChunkEmitted events.
//
// Emit is synchronous and non-blocking — implementations MUST NOT block
// the caller. Back-pressure, delivery guarantees, and buffering limits
// are adapter concerns, not domain ones; adapters consuming the event
// stream handle their own flow control.
type ResultStream interface {
	Emit(chunk ResultChunk)
}
