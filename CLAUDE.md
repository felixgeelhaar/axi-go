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

A domain-driven execution kernel for semantic actions. Actions express intent (e.g. "greet"), capabilities express mechanics (e.g. "string.upper"), plugins contribute both, and execution sessions track the full lifecycle with evidence.

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
- **ExecutionSession** — state machine: `Pending -> Validated -> Resolved -> Running -> Succeeded|Failed`

**Domain services:**
- **CompositionService** — registers plugins/contributions/bundles, enforces global name uniqueness, rollback on partial failure
- **CapabilityResolutionService** — resolves RequirementSet into CapabilityDefinitions
- **ActionExecutionService** — orchestrates validate -> resolve -> execute -> evidence -> result/failure

**Key types:**
- **Plugin** interface (`plugin.go`) — implement `Contribute()` to provide actions/capabilities
- **PluginBundle** (`bundle.go`) — pairs contribution with executor implementations for atomic registration
- **Contract** (`values.go`) — fields with `Name`, `Type`, `Description`, `Required`, `Example`
- **EvidenceRecord** — includes `Timestamp` (unix ms)
- **ExecutionResult** — includes `ContentType` for agent interpretation
- **Typed errors** (`errors.go`) — `ErrNotFound`, `ErrConflict`, `ErrValidation`

**Port interfaces** (all in domain):
- Repositories: `ActionRepository`, `CapabilityRepository`, `PluginRepository`, `SessionRepository`
- Executors: `ActionExecutor`, `CapabilityInvoker`, `CapabilityExecutor`
- Lookups: `ActionExecutorLookup`, `CapabilityExecutorLookup`
- Validation: `ContractValidator`

### `api/` — HTTP API

Gin-like router (`Engine`/`Context`/`RouterGroup`) on Go 1.22+ `http.ServeMux`. No external deps.

Routes:
- `GET /health` — health check
- `GET /api/v1/actions` — list actions (envelope: `{items, count}`)
- `GET /api/v1/actions/{name}` — get action
- `POST /api/v1/actions/execute` — execute (200 even on domain failure)
- `GET /api/v1/sessions/{id}` — get session
- `GET /api/v1/capabilities` — list capabilities (envelope)
- `POST /api/v1/plugins` — register plugin (201/409)

Error responses include `error_code` field (`not_found`, `conflict`, `validation_error`, `internal_error`).

### `inmemory/` — Adapters

Thread-safe repos with typed domain errors (`ErrNotFound`). Compile-time interface checks.

## Key Design Rules

- **Domain has zero imports** outside stdlib
- **Aggregates enforce invariants** — unexported fields, defensive slice copies
- **All port interfaces live in domain**
- **PluginBundle** is the preferred registration method — validates executor refs match implementations
- **Action failure is a valid outcome**, not a Go error
- **Names must match** `^[a-zA-Z][a-zA-Z0-9._-]*$`
- **Executor code must be compiled in** — HTTP API registers metadata only

## Execution Flow

`ActionExecutionService.Execute`:
1. Load ActionDefinition -> validate input -> `Validated`
2. Resolve RequirementSet -> `Resolved`
3. Build scoped `CapabilityInvoker` -> execute -> `Running`
4. Collect evidence -> `Succeeded` (with result) or `Failed` (with reason)
