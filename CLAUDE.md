# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
make check          # Full suite: fmt + lint + test + security
make test           # Run tests
make lint           # golangci-lint
make fmt            # Auto-fix formatting
make cover          # Tests with coverage + coverctl record
make security       # nox security scan
make install-hooks  # Install pre-commit git hook
go test ./... -race # Race detector (important for async execution)
```

Single test: `go test ./domain/... -run TestBudget -v -count=1`

Zero external dependencies — standard library only.

## Architecture

```
domain/          Zero deps. Aggregates, services, port interfaces, snapshot types.
application/     Use cases: RegisterPlugin, ExecuteAction (sync + async), Approve/Reject.
api/             Gin-like HTTP API on stdlib net/http. Config, middleware, auth.
inmemory/        Thread-safe in-memory adapters. StdLogger.
jsonstore/       File-based JSON persistence adapter.
cmd/server/      Entrypoint with config, logging, graceful shutdown.
example/         Working example with sample greeter plugin.
```

Dependency direction: `domain` <- `application` <- `api`/`inmemory`/`jsonstore` <- `cmd/server`

### `domain/` — Core (zero deps)

**Aggregates** (unexported fields, thread-safe via sync.RWMutex):
- **ActionDefinition** — semantic action with typed contracts, requirements, effect/idempotency profiles
- **CapabilityDefinition** — executable capability with contracts
- **PluginContribution** — groups actions + capabilities
- **ExecutionSession** — state machine: `Pending -> Validated -> Resolved -> [AwaitingApproval] -> Running -> Succeeded|Failed|Rejected`

**Domain services:**
- **CompositionService** — register/deregister plugins with mutex serialization, rollback on failure
- **CapabilityResolutionService** — resolves requirements
- **ActionExecutionService** — validate -> resolve -> [approve] -> execute with budget/rate-limit enforcement, output validation

**Key types:**
- **Plugin** / **LifecyclePlugin** — contribute actions/capabilities, optional Init(config)/Close()
- **PluginBundle** — atomic registration of contribution + executors
- **Contract** — fields with Name, Type, Description, Required, Example
- **EffectProfile** — 5 levels: none, read-local, write-local, read-external, write-external
- **ExecutionBudget** — max duration, max capability invocations
- **RateLimiter** — port interface for action-level rate limiting
- **Pipeline** — sequential capability chaining
- **ComposableCapabilityExecutor** — capabilities that invoke other capabilities
- **Logger** — structured logging port (Debug/Info/Warn/Error with fields)
- **Typed errors** — ErrNotFound, ErrConflict, ErrValidation
- **Snapshot types** — serializable forms for persistence adapters

### `api/` — HTTP API

Gin-like router on Go 1.22+ `http.ServeMux`. Graceful shutdown, configurable timeouts.

Routes: `/health`, `/api/v1/actions`, `/api/v1/actions/{name}`, `/api/v1/actions/execute` (sync/async), `/api/v1/sessions/{id}`, `/api/v1/sessions/{id}/approve`, `/api/v1/sessions/{id}/reject`, `/api/v1/capabilities`, `/api/v1/plugins`, `DELETE /api/v1/plugins/{id}`

Features: error_code in responses, list envelopes, ConfigFromEnv (AXI_* env vars), AuthMiddleware + BearerTokenAuth.

### `jsonstore/` — File-based persistence

JSON file adapter implementing all repository interfaces. Stores data as `{dir}/actions/*.json`, `capabilities/*.json`, etc. Uses domain snapshot types for serialization.

### `inmemory/` — In-memory adapters + StdLogger

Thread-safe repos, executor registries, validator, ID generator, structured logger.

## Key Design Rules

- **Domain has zero imports** outside stdlib
- **ExecutionSession is thread-safe** (sync.RWMutex) for async execution
- **CompositionService is serialized** (sync.Mutex) preventing TOCTOU races
- **write-external actions require approval** before execution
- **Output contracts are validated** — results checked before Succeeded
- **Budget enforced per invocation** — max invocations and max duration
- **Context cancellation checked** between execution phases

## Configuration

Environment variables (all optional, sensible defaults):
- `AXI_ADDR` — listen address (default `:8080`)
- `AXI_READ_TIMEOUT_SECS` — HTTP read timeout (default 15)
- `AXI_WRITE_TIMEOUT_SECS` — HTTP write timeout (default 30)
- `AXI_IDLE_TIMEOUT_SECS` — HTTP idle timeout (default 60)
- `AXI_MAX_DURATION_SECS` — max execution duration (default 300)
- `AXI_MAX_INVOCATIONS` — max capability invocations per session (default 100)
