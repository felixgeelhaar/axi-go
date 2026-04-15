# axi-go

A domain-driven execution kernel for semantic actions. Define high-level agent-facing actions backed by composable capabilities, with contract validation, dependency resolution, and full execution audit trails.

**Zero external dependencies.** Standard library only.

## Quick Start

```bash
go run example/main.go
```

Then in another terminal:

```bash
# Health check
curl localhost:8080/health

# List registered actions (with typed contracts)
curl localhost:8080/api/v1/actions

# Execute the "greet" action
curl -X POST localhost:8080/api/v1/actions/execute \
  -H 'Content-Type: application/json' \
  -d '{"action_name":"greet","input":{"name":"world"}}'

# Async execution (returns 202 immediately, poll for result)
curl -X POST localhost:8080/api/v1/actions/execute \
  -H 'Content-Type: application/json' \
  -d '{"action_name":"greet","input":{"name":"world"},"async":true}'

# Poll session status
curl localhost:8080/api/v1/sessions/session-1
```

## Core Concepts

| Concept | Description |
|---------|-------------|
| **Action** | A semantic, agent-facing operation with typed input/output contracts |
| **Capability** | A low-level executable mechanic that actions compose |
| **Plugin** | A bundle of actions and capabilities contributed to the system |
| **Session** | Tracks one action execution through its lifecycle with evidence |
| **Pipeline** | A composable chain of capabilities that execute sequentially |

Actions express **intent**. Capabilities express **mechanics**. The kernel resolves which capabilities an action needs, validates inputs and outputs against contracts, executes through a strict state machine, and produces structured results with evidence.

## Safety & Control

- **Effect profiles** — `none`, `read-local`, `write-local`, `read-external`, `write-external`
- **Human-in-the-loop** — `write-external` actions pause at `awaiting_approval`; require `POST /sessions/{id}/approve` to proceed
- **Execution budgets** — max duration and max capability invocations per session
- **Rate limiting** — pluggable `RateLimiter` interface checked before each execution
- **Output validation** — results validated against output contracts before marking succeeded
- **Idempotency profiles** — actions declare whether they are safe to retry

## Writing a Plugin

```go
// 1. Implement domain.Plugin
type myPlugin struct{}

func (p *myPlugin) Contribute() (*domain.PluginContribution, error) {
    action, _ := domain.NewActionDefinition(
        "greet", "Greet someone",
        domain.NewContract([]domain.ContractField{
            {Name: "name", Type: "string", Description: "Person to greet", Required: true},
        }),
        domain.EmptyContract(), nil,
        domain.EffectProfile{Level: domain.EffectNone},
        domain.IdempotencyProfile{IsIdempotent: true},
    )
    _ = action.BindExecutor("exec.greet")
    return domain.NewPluginContribution("my.plugin",
        []*domain.ActionDefinition{action}, nil)
}

// 2. Implement the executor
type greetExec struct{}

func (e *greetExec) Execute(ctx context.Context, input any, caps domain.CapabilityInvoker) (domain.ExecutionResult, []domain.EvidenceRecord, error) {
    m := input.(map[string]any)
    return domain.ExecutionResult{Data: "Hello, " + m["name"].(string), ContentType: "text/plain"}, nil, nil
}

// 3. Register atomically with PluginBundle
bundle, _ := domain.NewPluginBundle(contribution,
    map[domain.ActionExecutorRef]domain.ActionExecutor{"exec.greet": &greetExec{}},
    nil,
)
compositionService.RegisterBundle(bundle, actionExecReg, capExecReg)
```

For plugins that need configuration and cleanup, implement `LifecyclePlugin`:

```go
func (p *myPlugin) Init(config map[string]any) error { /* setup */ }
func (p *myPlugin) Close() error                     { /* cleanup */ }

compositionService.RegisterPluginWithConfig(plugin, map[string]any{"api_key": "..."})
```

See [`example/main.go`](example/main.go) for a complete working example.

## Capability Composition

Capabilities can invoke other capabilities by implementing `ComposableCapabilityExecutor`:

```go
func (e *enricher) ExecuteWithInvoker(ctx context.Context, input any, invoker domain.CapabilityInvoker) (any, error) {
    upper, _ := invoker.Invoke("string.upper", input)
    return "enriched: " + upper.(string), nil
}
```

Or use the built-in `Pipeline` for sequential chaining:

```go
pipeline := domain.NewPipeline("string.upper", "string.trim")
```

## API Endpoints

| Method | Path | Description | Status |
|--------|------|-------------|--------|
| `GET` | `/health` | Health check | 200 |
| `GET` | `/api/v1/actions` | List all actions | 200 |
| `GET` | `/api/v1/actions/{name}` | Get action by name | 200/404 |
| `POST` | `/api/v1/actions/execute` | Execute (sync or async) | 200/202 |
| `GET` | `/api/v1/sessions/{id}` | Get session status | 200/404 |
| `POST` | `/api/v1/sessions/{id}/approve` | Approve pending session | 200/400 |
| `POST` | `/api/v1/sessions/{id}/reject` | Reject pending session | 200/400 |
| `GET` | `/api/v1/capabilities` | List capabilities | 200 |
| `POST` | `/api/v1/plugins` | Register plugin | 201/409 |
| `DELETE` | `/api/v1/plugins/{id}` | Deregister plugin | 204/404 |

All error responses include `error_code` (`not_found`, `conflict`, `validation_error`, `unauthorized`).

## Authentication

Enable token-based auth with middleware:

```go
auth := api.NewBearerTokenAuth("my-secret-token")
srv.Engine().Use(api.AuthMiddleware(auth, "/health")) // skip auth on /health
```

Implement `api.Authenticator` for custom auth (JWT, OAuth, API keys).

## Configuration

Environment variables (all optional):

| Variable | Default | Description |
|----------|---------|-------------|
| `AXI_ADDR` | `:8080` | Listen address |
| `AXI_READ_TIMEOUT_SECS` | `15` | HTTP read timeout |
| `AXI_WRITE_TIMEOUT_SECS` | `30` | HTTP write timeout |
| `AXI_IDLE_TIMEOUT_SECS` | `60` | HTTP idle timeout |
| `AXI_MAX_DURATION_SECS` | `300` | Max execution duration |
| `AXI_MAX_INVOCATIONS` | `100` | Max capability invocations per session |

## Persistence

Two adapters included:

- **`inmemory/`** — in-memory (default, ephemeral)
- **`jsonstore/`** — file-based JSON (survives restarts)

```go
// Use JSON file persistence instead of in-memory
actionRepo, _ := jsonstore.NewActionStore("./data")
capRepo, _ := jsonstore.NewCapabilityStore("./data")
pluginRepo, _ := jsonstore.NewPluginStore("./data")
sessionRepo, _ := jsonstore.NewSessionStore("./data")
```

Implement the repository interfaces (`domain.ActionRepository`, etc.) for Postgres, SQLite, or any other store.

## Architecture

```
domain/          Zero dependencies. Aggregates, services, port interfaces, snapshots.
application/     Use cases: RegisterPlugin, ExecuteAction, Approve/Reject.
api/             Gin-like HTTP API, config, middleware, auth.
inmemory/        Thread-safe in-memory adapters, StdLogger.
jsonstore/       File-based JSON persistence adapter.
cmd/server/      Entrypoint with config, logging, graceful shutdown.
example/         Working example with sample greeter plugin.
```

## Development

```bash
make check          # Full suite: fmt + lint + test + security
make test           # Run tests (99 tests across 6 packages)
make lint           # golangci-lint (gocritic, errcheck, staticcheck, etc.)
make fmt            # Auto-fix formatting
make cover          # Tests with coverage
make install-hooks  # Install pre-commit git hook
go test ./... -race # Race detector
```
