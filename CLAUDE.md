# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
make check          # Full suite: fmt + lint + test + security
make test           # Run tests
make lint           # golangci-lint (gocritic, errcheck, govet, staticcheck, unused)
make fmt            # Auto-fix formatting (gofmt + goimports via golangci-lint)
make cover          # Tests with coverage + coverctl record
make security       # nox security scan
make install-hooks  # Install pre-commit git hook
```

Single test: `go test ./domain/... -run TestSession -v -count=1`

Zero external dependencies — standard library only.

## What axi-go Is

A domain-driven execution kernel for semantic actions. Actions express intent, capabilities express mechanics, plugins contribute both, and execution sessions track the full lifecycle with evidence and safety controls.

## Architecture

```
domain/          Zero deps. Aggregates, services, port interfaces.
application/     Use cases: RegisterPlugin, ExecuteAction.
api/             Gin-like HTTP API on stdlib net/http.
inmemory/        Thread-safe in-memory adapters.
cmd/server/      Minimal entrypoint.
example/         Working example with sample greeter plugin.
```

Dependency direction: `domain` <- `application` <- `api`/`inmemory` <- `cmd/server`

### `domain/` — Core

**Aggregates** (unexported fields, constructor-enforced invariants):
- **ActionDefinition** — semantic action with typed contracts, requirements, effect/idempotency profiles, executor binding
- **CapabilityDefinition** — executable capability with contracts and executor binding
- **PluginContribution** — groups actions + capabilities; must activate before use
- **ExecutionSession** — state machine: `Pending -> Validated -> Resolved -> [AwaitingApproval] -> Running -> Succeeded|Failed|Rejected`

**Domain services:**
- **CompositionService** — registers/deregisters plugins, enforces uniqueness, rollback on partial failure
- **CapabilityResolutionService** — resolves RequirementSet into CapabilityDefinitions
- **ActionExecutionService** — orchestrates validate -> resolve -> [approve] -> execute -> evidence -> result/failure

**Key types:**
- **Plugin** / **LifecyclePlugin** (`plugin.go`) — contribute actions/capabilities, optional Init(config)/Close()
- **PluginBundle** (`bundle.go`) — pairs contribution with executors for atomic registration
- **Contract** (`values.go`) — fields with `Name`, `Type`, `Description`, `Required`, `Example`
- **EffectProfile** — 5 levels: `none`, `read-local`, `write-local`, `read-external`, `write-external`
- **ExecutionBudget** (`budget.go`) — max duration, max capability invocations
- **RateLimiter** (`budget.go`) — port interface for action-level rate limiting
- **Pipeline** (`pipeline.go`) — sequential capability chaining primitive
- **ComposableCapabilityExecutor** (`execution.go`) — capabilities that invoke other capabilities
- **EvidenceRecord** — includes `Timestamp` (unix ms)
- **ExecutionResult** — includes `ContentType` for agent interpretation
- **Typed errors** (`errors.go`) — `ErrNotFound`, `ErrConflict`, `ErrValidation`

**Port interfaces** (all in domain):
- Repositories: `ActionRepository`, `CapabilityRepository`, `PluginRepository`, `SessionRepository`
- Executors: `ActionExecutor`, `CapabilityInvoker`, `CapabilityExecutor`, `ComposableCapabilityExecutor`
- Lookups: `ActionExecutorLookup`, `CapabilityExecutorLookup`
- Validation: `ContractValidator`
- Safety: `RateLimiter`

### `api/` — HTTP API

Gin-like router (`Engine`/`Context`/`RouterGroup`) on Go 1.22+ `http.ServeMux`. No external deps.

Routes:
- `GET /health` — health check
- `GET /api/v1/actions` — list (envelope: `{items, count}`)
- `GET /api/v1/actions/{name}` — get action
- `POST /api/v1/actions/execute` — execute (sync: 200, async: 202, approval needed: 200 with `requires_approval`)
- `GET /api/v1/sessions/{id}` — poll session status
- `POST /api/v1/sessions/{id}/approve` — approve pending session
- `POST /api/v1/sessions/{id}/reject` — reject pending session
- `GET /api/v1/capabilities` — list (envelope)
- `POST /api/v1/plugins` — register (201/409)
- `DELETE /api/v1/plugins/{id}` — deregister (204)

Error responses include `error_code` field (`not_found`, `conflict`, `validation_error`, `internal_error`).

## Key Design Rules

- **Domain has zero imports** outside stdlib
- **Aggregates enforce invariants** — unexported fields, defensive slice copies
- **All port interfaces live in domain**
- **PluginBundle** is the preferred registration method
- **Action failure is a valid outcome**, not a Go error
- **write-external actions require approval** before execution
- **Names must match** `^[a-zA-Z][a-zA-Z0-9._-]*$`
- **Executor code must be compiled in** — HTTP API registers metadata only

## Execution Flow

`ActionExecutionService.Execute`:
1. Rate limit check
2. Load ActionDefinition -> validate input -> `Validated`
3. Resolve RequirementSet -> `Resolved`
4. If write-external: pause at `AwaitingApproval` (requires Approve/Reject)
5. Build budget-enforced `CapabilityInvoker` -> execute -> `Running`
6. Collect evidence -> `Succeeded` (with result) or `Failed` (with reason)

Async mode: `ExecuteAsync` returns immediately with session in `Pending`, executes in background goroutine. Poll via `GET /sessions/{id}`.
