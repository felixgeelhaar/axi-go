# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
make build          # Build all packages
make test           # Run all tests
make lint           # golangci-lint (gocritic, errcheck, govet, staticcheck, unused)
make fmt            # Auto-fix formatting (gofmt + goimports via golangci-lint)
make cover          # Tests with coverage + coverctl record
make security       # nox security scan
make check          # Full suite: fmt + lint + test + security
make install-hooks  # Install pre-commit git hook
```

Single test: `go test ./domain/... -run TestSession -v -count=1`

Zero external dependencies — standard library only.

## What axi-go Is

A domain-driven execution kernel for semantic actions. Actions express intent (e.g. "greet"), capabilities express mechanics (e.g. "string.upper"), plugins contribute both, and execution sessions track the full lifecycle with evidence.

**This is a pure domain library.** No external frameworks, no reflection, no global registries.

## Architecture

Five packages with a strict dependency direction:

```
domain/  (stdlib only)
    ↑
application/  (imports domain/)
    ↑         \
inmemory/    api/  (imports domain/, application/)
    ↑          ↑
  cmd/server/main.go  (wires everything)
```

### `domain/` — Core (depends on nothing)

Four **aggregate roots** with unexported fields and constructor-enforced invariants:

- **ActionDefinition** — semantic action with input/output contracts, requirements, effect/idempotency profiles, and an executor binding
- **CapabilityDefinition** — low-level executable capability with contracts and an executor binding
- **PluginContribution** — groups actions + capabilities from one plugin; must activate (all bindings set) before use
- **ExecutionSession** — state machine: `Pending → Validated → Resolved → Running → Succeeded|Failed`. No backward transitions. Result XOR Failure. Evidence append-only.

Three **domain services**:

- **CompositionService** (`composition.go`) — registers plugins/contributions, enforces global name uniqueness. Accepts `Plugin` interface or raw `*PluginContribution`.
- **CapabilityResolutionService** (`resolution.go`) — resolves RequirementSet into CapabilityDefinitions
- **ActionExecutionService** (`execution.go`) — orchestrates the full execution flow

All **port interfaces** live in domain: repositories in `composition.go`, executor/validator interfaces in `execution.go`.

### `application/` — Use cases

- **RegisterPluginContributionUseCase** — wraps CompositionService. Supports `Execute()` and `ExecutePlugin()`.
- **ExecuteActionUseCase** — creates session, delegates to ActionExecutionService, persists result.

### `api/` — Gin-like HTTP API (stdlib net/http)

Custom Gin-style router (`Engine`, `Context`, `RouterGroup`) built on Go 1.22+ `http.ServeMux` path params. Zero external dependencies.

Routes under `/api/v1`:
- `GET /actions` — list actions
- `GET /actions/{name}` — get action by name
- `POST /actions/execute` — execute action (200 even on action failure — it's a valid domain outcome)
- `GET /sessions/{id}` — get session
- `GET /capabilities` — list capabilities
- `POST /plugins` — register plugin (201 on success, 409 on conflict)

DTOs in `dto.go` convert between JSON and domain aggregates (which have unexported fields).

### `inmemory/` — In-memory adapters

Thread-safe (`sync.RWMutex`) repositories, executor registries, contract validator, ID generator. Compile-time interface checks.

### `cmd/server/` — Entrypoint

Wires inmemory adapters into the API server. `go run cmd/server/main.go` starts on `:8080`.

## Quality Tooling

- **`.golangci.yml`** — golangci-lint v2 config with gocritic, errcheck, govet, staticcheck, unused, gofmt, goimports
- **`Makefile`** — all quality targets; `make check` is the full suite
- **`scripts/pre-commit`** — git hook running fmt check, lint, vet, tests. Installed via `make install-hooks`. Skips nox/coverctl for speed.
- **Nox baseline** — `.nox-baseline.yml` suppresses known false positives

## Key Design Rules

- **Domain has zero imports** outside stdlib
- **Aggregates enforce their own invariants** via constructors; fields are unexported
- **All port interfaces live in domain** — no outward dependencies from domain services
- **Plugins only contribute domain objects** — implement `domain.Plugin` with `Contribute()`
- **Action failure is a valid outcome**, not a Go error
- **Names must match** `^[a-zA-Z][a-zA-Z0-9._-]*$`
- **Executor code must be compiled in** — the HTTP API registers plugin metadata/bindings, not executable code

## Execution Flow

`ActionExecutionService.Execute`:

1. Load ActionDefinition by name
2. Validate input against InputContract → `Validated`
3. Resolve RequirementSet → `Resolved`
4. Build `boundInvoker` scoped to resolved capabilities
5. Look up ActionExecutor by binding ref → `Running`
6. Execute, collecting evidence
7. `Succeeded` (with result) or `Failed` (with reason)
