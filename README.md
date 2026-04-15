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
```

Response:
```json
{
  "session_id": "session-1",
  "status": "succeeded",
  "result": {
    "data": "Hello, WORLD!",
    "summary": "Greeted world",
    "content_type": "text/plain"
  },
  "evidence": [
    {"kind": "invocation", "source": "greet", "value": {"name": "world", "upper": "WORLD"}}
  ]
}
```

## Core Concepts

| Concept | Description |
|---------|-------------|
| **Action** | A semantic, agent-facing operation (e.g. "greet", "search", "send-email") with typed contracts |
| **Capability** | A low-level executable mechanic (e.g. "string.upper", "http.get") that actions compose |
| **Plugin** | A bundle of actions and capabilities contributed to the system |
| **Session** | Tracks one action execution through its lifecycle with evidence |

Actions express **intent**. Capabilities express **mechanics**. The kernel resolves which capabilities an action needs, validates inputs against contracts, executes through a strict state machine, and produces structured results with evidence.

## Writing a Plugin

```go
// 1. Implement domain.Plugin
type myPlugin struct{}

func (p *myPlugin) Contribute() (*domain.PluginContribution, error) {
    action, _ := domain.NewActionDefinition(
        "greet", "Greet someone",
        domain.NewContract([]domain.ContractField{
            {Name: "name", Type: "string", Required: true},
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
    return domain.ExecutionResult{Data: "Hello, " + m["name"].(string)}, nil, nil
}

// 3. Register atomically with PluginBundle
bundle, _ := domain.NewPluginBundle(contribution,
    map[domain.ActionExecutorRef]domain.ActionExecutor{"exec.greet": &greetExec{}},
    nil,
)
compositionService.RegisterBundle(bundle, actionExecReg, capExecReg)
```

See [`example/main.go`](example/main.go) for a complete working example.

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Health check |
| `GET` | `/api/v1/actions` | List all actions |
| `GET` | `/api/v1/actions/{name}` | Get action by name |
| `POST` | `/api/v1/actions/execute` | Execute an action |
| `GET` | `/api/v1/sessions/{id}` | Get execution session |
| `GET` | `/api/v1/capabilities` | List all capabilities |
| `POST` | `/api/v1/plugins` | Register a plugin |

Action execution returns **200** for both success and failure outcomes — domain-level failure is a valid result, not an HTTP error. Check the `status` field (`succeeded` or `failed`).

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
