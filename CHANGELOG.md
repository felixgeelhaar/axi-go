# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and axi-go adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html) from 1.0.0
onwards. Pre-1.0 versions may introduce breaking changes between minor
releases; those are annotated with `BREAKING` below.

## [Unreleased]

### Changed — BREAKING

- **Default session IDs are UUIDv7** — `axi.New()` now defaults to
  `inmemory.UUIDv7Generator` instead of `SequentialIDGenerator`
  (`session-N`). Sequential IDs collide across process restarts when
  paired with a durable `SessionRepository` (#40). Override with
  `WithIDGenerator` only if you accept that risk. Tests that matched
  `session-1` style IDs must be updated.

### Fixed

- **`ExecuteAsync` stuck sessions** — panics before `Running` and early
  Execute errors (missing action, rate limit, validation) no longer leave
  sessions in a non-terminal state. Recovery uses `ExecutionSession.Abort`
  when `Fail` is not legal; pollers observe `Failed` with `PANIC` or
  `EXECUTION_ERROR`.
- **`WithTimeout` budget clobber** — now merges into the existing default
  budget (preserves invocations / tokens / retries) instead of replacing
  the whole `ExecutionBudget`.
- **`RegisterBundle` executor rollback** — on contribution conflict,
  previously registered executors are unregistered when the registry
  supports `Unregister` (inmemory adapters do).
- **`LifecyclePlugin.Close`** — retained on register and invoked on
  `DeregisterPlugin`.
- **Rate limit on `Resume`** — approved write-external work is subject to
  the same rate limiter as initial `Execute`.
- **Unknown snapshot schemas** — `SessionFromSnapshot` rejects non-empty
  schemas other than `CurrentSessionSchema` (`"1"`); empty remains legacy.
- **Typed budget errors** — `budgetEnforcer` returns `*ErrBudgetExceeded`
  with a `Kind`; `BudgetExceeded` events no longer parse error strings.

### Added

- **`ExecutionSession.Abort`** — force-fail from any non-terminal status.
- **`ExecutionStatus.IsTerminal`** and **`ActionOutcome.IsAwaitingApproval`**
  helpers. Orchestrator godoc documents the fail-closed recommendation for
  nested write-external pauses.
- **`SessionApproved` domain event** — raised on `Approve`.
- **jsonstore round-trips** for evidence hash chains and `result_chunks`.
- **Coverage gate** — `.coverctl.yaml` (module min 70%, exclude
  `./example/...`); CI `coverage: true`. Example packages gained smoke
  tests so `go test -cover ./...` no longer fails on missing `covdata`.
- **Domain fuzz harnesses** — `FuzzNewActionName`, `FuzzNewCapabilityName`,
  `FuzzSessionFromSnapshot`; weekly fuzz workflow covers them alongside
  `toon.Encode`.
- **Direct adapter tests** — `inmemory` repos/registries/logger;
  `application` Approve/Reject/Deregister error paths.

### Documentation

- ROADMAP / backlog / SECURITY* updated for post-1.0 reality (tags through
  v1.4.0, nox-remediate instead of Dependabot/CodeQL). CHANGELOG backfilled
  for 1.2.1–1.4.0.

## [1.4.0] - 2026-06-06

### Changed — BREAKING

- **Module path** migrated to `go.klarlabs.de/axi` (was
  `github.com/klarlabs-studio/axi-go`). Update imports accordingly.

## [1.3.0] - 2026-06-04

### Added

- **`Kernel.WithSessionRepository`** — swap the default in-memory session
  store for a durable adapter (e.g. Postgres) so write-external sessions
  paused at `AwaitingApproval` survive process restarts.

## [1.2.1] - 2026-06-02

### Fixed

- Release workflow: pin cosign CLI so SBOM signing succeeds.

## [1.2.0] - 2026-04-19

Additive release. Introduces the compositional primitive that lets
third-party plugins invoke other registered actions — the shape the
"sagas as plugin" direction needs. No breaking changes; the new
`OrchestratorActionExecutor` is an optional companion to the existing
`ActionExecutor` interface, same additive pattern as 1.1's
`StreamingActionExecutor`.

### Added

- **`domain.ActionInvoker` port + `OrchestratorActionExecutor`**
  optional interface. Lets plugin code invoke other registered actions
  as part of a composite action's work — the primitive needed for
  sagas, fan-out/fan-in, aggregation, and pipeline-of-actions patterns
  to ship as plugins rather than as kernel extensions.

  `ActionInvoker` is a narrow domain port with a single method
  `Invoke(ctx, action, input) (*ActionOutcome, error)`. Transport-level
  failures (action not registered, invoker not wired) return a Go
  error; domain-level failures (sub-action failed, rejected, or awaits
  approval) return a non-nil `ActionOutcome` whose `Status` + `Failure`
  fields carry the reason — axi-go's "failure is a valid outcome"
  contract extends to sub-invocations.

  `OrchestratorActionExecutor` is an additive companion to
  `ActionExecutor`; executors that need to invoke other actions
  implement both and the kernel prefers `ExecuteOrchestrated` when
  a `ActionInvoker` has been wired (the default when built with
  `axi.New()`). Each sub-invocation runs as a fresh `ExecutionSession`
  with its own SessionID, evidence chain, events, budget, approval
  flow, and output contract — sub-sessions never inherit or collide
  with parent state.

  This is the primitive the "sagas as plugin" pattern needs:
  `axi-go-saga` or any third-party module can now ship a plugin that
  contributes `saga.orchestrate`, `saga.step`, etc. as semantic
  actions, using the kernel's existing lifecycle for each step. The
  saga's durable-log backend stays in the plugin module (Postgres
  outbox, Kafka, in-memory), not in axi-go core — zero-deps preserved.

## [1.1.0] - 2026-04-19

Additive release along the strict-DDD spine of the library. Three of
the four `[post-1.0]` items on the deferred list from v1.0.0 are now
landed; distributed sagas remains deferred as it needs a durable event
log and at-least-once semantics that are a larger-than-library lift.
No breaking changes — all three new capabilities compose with the
existing surface.

### Added

- **Streaming `ExecutionResult` via `StreamingActionExecutor`.**
  Optional companion interface to `ActionExecutor` for actions that
  emit progressive output (LLM tokens, large-file reads, row-stream
  queries). Executors that stream implement both interfaces; the kernel
  prefers `ExecuteStream` when available and falls back to `Execute`
  otherwise — no breaking change to existing executors.

  New `ResultChunk` value object carries `Index`, `Kind`, `Data`,
  `ContentType`, `At`. New `ResultStream` domain port has a single
  `Emit(chunk)` method; `*domain.ExecutionSession` satisfies it
  directly, so the kernel passes the session through to executors with
  no adapter layer. The aggregate stamps `Index` monotonically under
  its existing mutex (safe under concurrent `Emit` calls from composed
  executors) and fills `At` with `time.Now()` when the executor leaves
  it zero. New `ResultChunkEmitted` domain event flows through the
  existing `DomainEventPublisher` — HTTP/SSE, gRPC-stream, and MCP-SSE
  adapters subscribe for live delivery without polling. New
  `ExecutionSession.ResultChunks()` accessor returns a defensive copy
  for poll-based consumers.

  `SessionSnapshot` gains an optional `result_chunks` field so streamed
  output round-trips through persistence; pre-1.1 snapshots omit the
  field entirely and load as sessions with zero chunks. Schema stays
  `"1"` per the MINOR-for-optional-fields policy.

- **Evidence integrity via tamper-evident hash chain.** Closes the
  "post-emission tampering" half of the Evidence trust boundary
  documented in docs/CONCEPTS.md. Each `EvidenceRecord` now carries
  optional `Hash` and `PreviousHash` fields (hex-encoded SHA-256 over
  the record's canonical JSON form concatenated with the previous
  hash). The `ExecutionSession` aggregate assigns these at
  `AppendEvidence` time — plugins cannot forge chain positions even if
  they construct records with hashes pre-set; the aggregate overwrites
  them. New aggregate method `session.VerifyEvidenceChain() error`
  returns `*ErrChainBroken` on any mismatch (recomputed hash differs,
  or linkage pointer is wrong). Legacy records (empty `Hash`, e.g.
  pre-1.1 persisted snapshots) verify as unverifiable-but-not-broken,
  so existing sessions continue to load cleanly. The `EvidenceRecorded`
  domain event gained a `Hash` field so audit sinks see the chain.
  Plugins reporting `TokensUsed: 0` at emission time remains the
  documented trust boundary — that half of the vector is orthogonal to
  hashing and stays per-deployment trust.
- **`domain.DomainEventPublisher` port** — observability hook in the
  shape of a strict-DDD domain event channel rather than a pre-classified
  metrics interface. The kernel raises immutable event value objects
  (`SessionStarted`, `SessionAwaitingApproval`, `SessionCompleted`,
  `CapabilityInvoked`, `CapabilityRetried`, `BudgetExceeded`,
  `EvidenceRecorded`); adapters classify them into Prometheus counters,
  OpenTelemetry spans, audit logs, or any other downstream concern. The
  domain itself imports nothing vendor-specific. Default is
  `NopDomainEventPublisher`, preserving the zero-deps story for callers
  that don't wire observability. Wire-up via `kernel.WithDomainEventPublisher(p)`.
  Following strict DDD: events are raised by the `ExecutionSession`
  aggregate inside its state-transition methods, accumulated in a
  per-session buffer, and drained by `ActionExecutionService` after
  each step. The `boundInvoker` (a domain service) publishes
  capability-level events directly since there is no aggregate to
  attach them to.

## [1.0.0] - 2026-04-18

First stable release. All six items on the
[docs/ROADMAP.md](docs/ROADMAP.md) 1.0 checklist are met: API stability,
Godoc completeness, persistence schema frozen at v1, CI quality floor,
security posture, and adoption signal. From this tag onwards axi-go
follows [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html)
and the deprecation policy documented in docs/ROADMAP.md.

### Added — axi.md alignment (principles 1–5, 9, 10)

- **TOON encoder** (`toon/`) — Token-Optimized Object Notation for result
  payloads, ~40% token savings over JSON on uniform-object arrays
  (axi.md #1). Supports scalars, maps, uniform-map arrays (tabular form),
  scalar arrays, and a numbered-entry fallback for heterogeneous slices.
- **Token budget** — `ExecutionBudget.MaxTokens` and
  `EvidenceRecord.TokensUsed` (axi.md #1). Sessions whose evidence token
  sum exceeds the budget transition to `Failed` with
  `FailureReason.Code = "BUDGET_EXCEEDED"`.
- **Summary views** — `ActionSummary` / `CapabilitySummary` + new
  `Kernel.ListActionSummaries` / `Kernel.ListCapabilitySummaries`
  returning minimal projections (axi.md #2).
- **Truncation helper** — `axi.Truncate(s, max)` caps strings and appends
  a size hint such as `"… (truncated, 2847 chars total)"` (axi.md #3).
- **ListResult** — Generic `ListResult[T]{Items, TotalCount}` wrapper
  with `IsEmpty()` and non-nil `Items` for definitive empty states
  (axi.md #4, #5).
- **Suggestions** — `ExecutionResult.Suggestions []Suggestion` for
  contextual next-step hints (axi.md #9).
- **Help** — `ActionDefinition.Help()`, `CapabilityDefinition.Help()`,
  and `Kernel.Help(name)` for unified human-readable introspection
  (axi.md #10).

### Added — reliability (Issue #9, all 3 phases)

- **Idempotency-aware retries** — `ExecutionBudget.MaxRetries` and
  `RetryBackoff`. Retries fire only when the action's
  `IdempotencyProfile.IsIdempotent` is true; non-idempotent actions
  continue to fail on first error. Exponential backoff; respects
  context cancellation.
- **`PipelineFailure`** — When `Pipeline.ExecuteWithInvoker` fails
  mid-sequence, it returns `*PipelineFailure{FailedStep,
  CompletedOutput, Cause, CompensationErrors}` carrying the outputs of
  completed steps. Implements `error` and `errors.Unwrap` so existing
  callers keep working.
- **Saga-lite compensation** — Optional `PipelineStep.Compensate` is
  invoked in reverse order when a later step fails. Compensation
  errors surface via `PipelineFailure.CompensationErrors` without
  masking the primary cause. Context cancellation halts the
  compensation walk.

### Changed — BREAKING

- `Kernel.Approve` and `Kernel.Reject` now require a
  `domain.ApprovalDecision` argument carrying a non-empty `Principal`
  and optional `Rationale`. Empty principal is rejected at the domain
  layer — the audit guarantee is enforced at the type level.

### Fixed

- `ContractValidator` now enforces `ContractField.Type`. Previously
  only field presence was validated; type hints were ignored at
  runtime.
- `ExecuteAsync` now propagates context values through the background
  execution goroutine (via `context.WithoutCancel`), preserving
  tracing, logging correlation, and user-supplied context keys while
  detaching cancellation.

### Docs

- README gained an "Agent-facing output" section covering every
  axi.md-aligned feature with runnable code samples.
- `example/main.go` rewritten to showcase Suggestions, TOON,
  token-tracking evidence, `Help`, Summary listings, and
  idempotency-gated retries end-to-end.

### Infrastructure

- Zero new external runtime dependencies. axi-go remains standard
  library only.
- All 40+ new tests pass under `go test -race`. Linter clean.

### License

- **Relicensed from MIT to Apache License 2.0.** Adds an explicit
  patent grant (§3) and aligns with the dominant license in the
  adjacent AI-tooling ecosystem (MCP, OpenTelemetry-Go, Kubernetes,
  containerd). No external contributors had landed at the time of the
  switch, so no third-party consent was required. Previous MIT-licensed
  tags remain available under MIT; future releases are Apache 2.0.
  See [NOTICE](NOTICE).

### Adoption note for v1.0.0

axi-go ships 1.0 without a public external-adoption claim. The
[ROADMAP](docs/ROADMAP.md) 1.0 checklist explicitly permits this path
when the rationale is captured, and the rationale is:

1. **The stability contract stands on its own.** SemVer, a written
   deprecation policy, a frozen persistence schema, and an audited
   exported surface give downstream users the guarantees they need to
   build on the library. None of those depend on a public reference
   adopter.
2. **Zero external dependencies.** axi-go uses only the Go standard
   library. Adoption signals matter most as a proxy for "does this
   library survive contact with real systems and real upstream churn" —
   a question that applies less sharply to a zero-dependency kernel.
3. **Shape is validated by design constraint, not by telemetry.** The
   API was shaped against the axi.md principles and the reliability
   scenarios in Issue #9 (partial-state pipelines, saga-lite
   compensation, idempotency-aware retries). These are concrete
   stress tests of the surface, not a stand-in for adoption but a
   different kind of evidence.
4. **Shipping enables adoption it cannot precede.** Teams evaluating a
   kernel for production use reasonably wait for a 1.0 tag. Deferring
   1.0 until such teams adopt us creates a deadlock the spec is
   designed to break.

The library will not retroactively promote any specific adopter into a
"reference user" marketing claim. When external production users do
adopt axi-go, that will be reflected in future release notes or README
content, but 1.0 does not gate on it.

[Unreleased]: https://github.com/klarlabs-studio/axi-go/compare/v1.2.0...HEAD
[1.2.0]: https://github.com/klarlabs-studio/axi-go/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/klarlabs-studio/axi-go/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/klarlabs-studio/axi-go/compare/df0fda9...v1.0.0
