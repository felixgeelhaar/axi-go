# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What axi-go Is

A domain-driven execution kernel for semantic actions. **It is a library you embed in Go programs, not a service.** Actions express intent, capabilities express mechanics, plugins contribute both, and execution sessions track the full lifecycle with evidence and safety controls.

**axi-go has no HTTP API, no daemon, no protocol assumptions.** Delivery is the caller's choice — build an HTTP, gRPC, CLI, or MCP adapter on top of the kernel.

## Build & Test Commands

```bash
make check          # Full suite: fmt + lint + test + security
make test           # Run tests
make lint           # golangci-lint
make fmt            # Auto-fix formatting
make cover          # Tests with coverage
make install-hooks  # Install pre-commit git hook
go test ./... -race # Race detector (important for async execution)
```

Single test: `go test ./domain/... -run TestBudget -v -count=1`

Zero external dependencies — standard library only.

## Architecture

```
axi (root)       Fluent SDK facade — what consumers import.
domain/          Aggregates, services, port interfaces. Zero deps.
application/     Use cases that orchestrate the domain.
inmemory/        In-memory adapters + StdLogger.
jsonstore/       File-based JSON persistence adapter.
example/         Programmatic usage example.
```

Dependency direction: `domain` ← `application` ← `inmemory`/`jsonstore` ← `axi` (root) ← consumer code.

### Root `axi` package

Fluent SDK facade. The ONLY package consumers typically import.

```go
kernel := axi.New().
    WithLogger(logger).
    WithBudget(axi.Budget{MaxCapabilityInvocations: 100}).
    WithRateLimiter(limiter)

kernel.RegisterPlugin(plugin)
kernel.RegisterActionExecutor("exec.greet", executor)
kernel.RegisterBundle(bundle)  // atomic: metadata + executors

result, err := kernel.Execute(ctx, axi.Invocation{Action: "x", Input: ...})
result, err := kernel.ExecuteAsync(ctx, inv)
result, err := kernel.Approve(ctx, sessionID)
result, err := kernel.Reject(sessionID, reason)

actions := kernel.ListActions()
session, _ := kernel.GetSession(sessionID)
```

### `domain/` — Core (zero deps)

**Aggregates** (unexported fields, thread-safe via sync.RWMutex):
- **ActionDefinition** — semantic action with typed contracts, requirements, effect/idempotency profiles
- **CapabilityDefinition** — executable capability
- **PluginContribution** — groups actions + capabilities
- **ExecutionSession** — state machine: `Pending → Validated → Resolved → [AwaitingApproval] → Running → Succeeded|Failed|Rejected`

**Domain services:**
- **CompositionService** — register/deregister plugins with mutex serialization, rollback on failure
- **CapabilityResolutionService** — resolves requirements
- **ActionExecutionService** — validate → resolve → [approve] → execute with budget/rate-limit enforcement, output validation

**Key types:**
- **Plugin** / **LifecyclePlugin** — contribute actions/capabilities, optional Init(config)/Close()
- **PluginBundle** — atomic registration of contribution + executors
- **Contract** — fields with Name, Type, Description, Required, Example
- **EffectProfile** — 5 levels: none, read-local, write-local, read-external, write-external
- **ExecutionBudget** — max duration, max capability invocations
- **RateLimiter** — port interface for action-level rate limiting
- **Pipeline** — sequential capability chaining
- **ComposableCapabilityExecutor** — capabilities that invoke other capabilities
- **Logger** — structured logging port
- **Typed errors** — ErrNotFound, ErrConflict, ErrValidation
- **Snapshot types** — serializable forms for persistence adapters

**Port interfaces** (all in domain, all implemented by adapters):
- Repositories: `ActionRepository`, `CapabilityRepository`, `PluginRepository`, `SessionRepository`
- Executors: `ActionExecutor`, `CapabilityInvoker`, `CapabilityExecutor`, `ComposableCapabilityExecutor`
- Lookups: `ActionExecutorLookup`, `CapabilityExecutorLookup`
- Validation: `ContractValidator`
- Safety: `RateLimiter`

### `inmemory/` — Default adapters

Thread-safe repos, executor registries, validator, ID generator, StdLogger. Used by `axi.New()` as defaults.

### `jsonstore/` — File persistence

JSON file adapter implementing all repository interfaces. Uses domain snapshot types for serialization.

## Key Design Rules

- **No HTTP API, no daemon** — axi-go is a library only. Delivery adapters live outside this repo.
- **Domain has zero imports** outside stdlib.
- **Aggregates enforce invariants** via constructors, unexported fields, defensive slice copies.
- **ExecutionSession is thread-safe** (sync.RWMutex) for async execution.
- **CompositionService is serialized** (sync.Mutex) preventing TOCTOU races.
- **write-external actions require approval** before execution.
- **Output contracts are validated** before marking Succeeded.
- **Action failure is a valid outcome**, not a Go error.
- **Names must match** `^[a-zA-Z][a-zA-Z0-9._-]*$`.

## Execution Flow

`ActionExecutionService.Execute`:
1. Rate limit check
2. Load ActionDefinition → validate input → `Validated`
3. Resolve RequirementSet → `Resolved`
4. If write-external: pause at `AwaitingApproval` (requires Approve/Reject)
5. Build budget-enforced `CapabilityInvoker` → execute → `Running`
6. Collect evidence → validate output → `Succeeded` (with result) or `Failed` (with reason)

Async mode (`ExecuteAsync`): returns immediately with session in `Pending`, executes in background goroutine with panic recovery. Poll via `kernel.GetSession(id)`.
