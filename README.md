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

# List registered actions
curl localhost:8080/api/v1/actions

# Execute the "greet" action
curl -X POST localhost:8080/api/v1/actions/execute \
  -H 'Content-Type: application/json' \
  -d '{"action_name":"greet","input":{"name":"world"}}'

# Async execution (returns immediately, poll for result)
curl -X POST localhost:8080/api/v1/actions/execute \
  -H 'Content-Type: application/json' \
  -d '{"action_name":"greet","input":{"name":"world"},"async":true}'
```

## Core Concepts

| Concept | Description |
|---------|-------------|
| **Action** | A semantic, agent-facing operation (e.g. "greet", "send-email") with typed contracts |
| **Capability** | A low-level executable mechanic (e.g. "string.upper", "http.get") that actions compose |
| **Plugin** | A bundle of actions and capabilities contributed to the system |
| **Session** | Tracks one action execution through its lifecycle with evidence |
| **Pipeline** | A composable chain of capabilities that execute sequentially |

Actions express **intent**. Capabilities express **mechanics**. The kernel resolves which capabilities an action needs, validates inputs against contracts, executes through a strict state machine, and produces structured results with evidence.

## Safety Features

- **Effect profiles** — Actions declare their side-effect level: `none`, `read-local`, `write-local`, `read-external`, `write-external`
- **Human-in-the-loop** — Actions with `write-external` effects pause at `awaiting_approval` and require explicit approval via `POST /sessions/{id}/approve` before execution proceeds
- **Execution budgets** — Limit max duration and max capability invocations per session
- **Rate limiting** — Pluggable rate limiter interface checked before each execution
- **Idempotency profiles** — Actions declare whether they are safe to retry

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
type myPlugin struct{ config map[string]any }

func (p *myPlugin) Init(config map[string]any) error { p.config = config; return nil }
func (p *myPlugin) Close() error                     { return nil }
func (p *myPlugin) Contribute() (*domain.PluginContribution, error) { /* ... */ }

// Register with config:
compositionService.RegisterPluginWithConfig(plugin, map[string]any{"api_key": "..."})
```

See [`example/main.go`](example/main.go) for a complete working example.

## Capability Composition

Capabilities can invoke other capabilities by implementing `ComposableCapabilityExecutor`:

```go
type enricher struct{}

func (e *enricher) ExecuteWithInvoker(ctx context.Context, input any, invoker domain.CapabilityInvoker) (any, error) {
    upper, _ := invoker.Invoke("string.upper", input)
    return "enriched: " + upper.(string), nil
}
```

Or use the built-in `Pipeline` for sequential chaining:

```go
pipeline := domain.NewPipeline("string.upper", "string.trim")
// Register as a composable capability executor
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Health check |
| `GET` | `/api/v1/actions` | List all actions |
| `GET` | `/api/v1/actions/{name}` | Get action by name |
| `POST` | `/api/v1/actions/execute` | Execute an action (sync or async) |
| `GET` | `/api/v1/sessions/{id}` | Get execution session |
| `POST` | `/api/v1/sessions/{id}/approve` | Approve a pending session |
| `POST` | `/api/v1/sessions/{id}/reject` | Reject a pending session |
| `GET` | `/api/v1/capabilities` | List all capabilities |
| `POST` | `/api/v1/plugins` | Register a plugin |
| `DELETE` | `/api/v1/plugins/{id}` | Deregister a plugin |

Execution returns **200** for completed actions (success or domain failure), **202** for async submissions, **409** for conflicts, **404** for not found.

## Architecture

```
domain/          Zero dependencies. Aggregates, services, port interfaces.
application/     Use cases: RegisterPlugin, ExecuteAction.
api/             Gin-like HTTP API on stdlib net/http.
inmemory/        Thread-safe in-memory adapters.
cmd/server/      Minimal entrypoint.
example/         Working example with sample plugin.
```

## Development

```bash
make check          # Full suite: fmt + lint + test + security
make test           # Run tests
make lint           # golangci-lint
make fmt            # Auto-fix formatting
make install-hooks  # Install pre-commit git hook
```
