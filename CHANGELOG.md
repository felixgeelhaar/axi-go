# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and axi-go adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html) from 1.0.0
onwards. Pre-1.0 versions may introduce breaking changes between minor
releases; those are annotated with `BREAKING` below.

## [Unreleased]

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

[Unreleased]: https://github.com/felixgeelhaar/axi-go/compare/df0fda9...HEAD
