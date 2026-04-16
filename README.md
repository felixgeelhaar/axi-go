# axi-go

**A safe, auditable execution kernel for AI agent tools — built in Go with zero dependencies.**

[![CI](https://github.com/felixgeelhaar/axi-go/actions/workflows/ci.yml/badge.svg)](https://github.com/felixgeelhaar/axi-go/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/felixgeelhaar/axi-go)](https://goreportcard.com/report/github.com/felixgeelhaar/axi-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

---

## Why axi-go?

When you give an AI agent a bag of tools (`search`, `send_email`, `run_sql`), you quickly hit these problems:

- **No safety** — the agent can call `send_email` a thousand times before you know it
- **No audit trail** — you can't explain *why* the agent did what it did
- **Tool sprawl** — 200 raw functions, no grouping, no dependencies, no lifecycle
- **No type information** — the agent has to guess what inputs each tool accepts
- **No approval gates** — the agent can take irreversible actions autonomously

**axi-go solves this** by splitting the world into two layers:

| Layer | Example | Answers |
|-------|---------|---------|
| **Actions** | `greet`, `send-email`, `search-docs` | *What* the agent wants to do (intent) |
| **Capabilities** | `string.upper`, `http.get`, `db.query` | *How* it gets done (mechanics) |

An action declares the capabilities it needs. axi-go resolves them, validates inputs against typed contracts, enforces effect profiles (read-only? external? writes?), pauses for human approval when required, runs with execution budgets, and produces a structured audit trail (`evidence`).

Think of it as **"Kubernetes for agent tools"**: you register capabilities once, and agents invoke high-level actions that compose them safely.

## Installation

```bash
go get github.com/felixgeelhaar/axi-go
```

Or run the example server:

```bash
git clone https://github.com/felixgeelhaar/axi-go.git
cd axi-go
go run example/main.go
```

## 60-Second Tour

Start the example server and try it:

```bash
# 1. See what actions are registered
curl localhost:8080/api/v1/actions
```

You'll see a `greet` action with a typed input contract:

```json
{
  "items": [{
    "name": "greet",
    "description": "Greet someone by name, with their name uppercased",
    "input_contract": {
      "fields": [{
        "name": "name", "type": "string",
        "description": "Person to greet", "required": true, "example": "world"
      }]
    },
    "effect_level": "none",
    "is_idempotent": true,
    "requirements": ["string.upper"]
  }],
  "count": 1
}
```

Note: the agent can see the input type, a description, an example, the side-effect level, and what capabilities this action composes.

```bash
# 2. Execute it
curl -X POST localhost:8080/api/v1/actions/execute \
  -H 'Content-Type: application/json' \
  -d '{"action_name":"greet","input":{"name":"world"}}'
```

Response includes a session ID, the result, and an evidence trail:

```json
{
  "session_id": "session-1",
  "status": "succeeded",
  "result": {
    "data": {"message": "Hello, WORLD!"},
    "summary": "Greeted world",
    "content_type": "application/json"
  },
  "evidence": [{
    "kind": "invocation",
    "source": "greet",
    "value": {"name": "world", "upper": "WORLD"}
  }]
}
```

The `evidence` array is your audit trail — every capability invocation, input, and output is captured.

## Key Concepts

### Actions express *intent*

An **Action** is what an agent wants to accomplish:

```go
action, _ := domain.NewActionDefinition(
    "send-email",
    "Send an email notification",
    inputContract,   // { to: string, subject: string, body: string }
    outputContract,  // { message_id: string }
    requirements,    // needs smtp.send capability
    domain.EffectProfile{Level: domain.EffectWriteExternal}, // !! external write !!
    domain.IdempotencyProfile{IsIdempotent: false},
)
```

Because this is `write-external`, axi-go **pauses for approval** before sending. A supervisor (human or another system) must call `POST /sessions/{id}/approve` first.

### Capabilities express *mechanics*

A **Capability** is a low-level building block:

```go
smtpCap, _ := domain.NewCapabilityDefinition(
    "smtp.send",
    "Sends an email via SMTP",
    inputContract,
    outputContract,
)
```

Capabilities have no side-effect profile of their own — they're invoked by actions, and the action's effect profile governs safety.

### Plugins bundle actions + capabilities

A **Plugin** contributes a set of actions and capabilities. Plugins are the unit of deployment:

```go
type emailPlugin struct{}

func (p *emailPlugin) Contribute() (*domain.PluginContribution, error) {
    return domain.NewPluginContribution("email.plugin",
        []*domain.ActionDefinition{sendEmailAction},
        []*domain.CapabilityDefinition{smtpCap},
    )
}
```

### Sessions track each execution

Every execution gets a **Session** with a strict state machine:

```
Pending → Validated → Resolved → [AwaitingApproval] → Running → Succeeded | Failed | Rejected
```

The session persists input, resolved capabilities, evidence, and result/failure. You can poll it anytime via `GET /sessions/{id}`.

## Safety Features

| Feature | What it does |
|---------|--------------|
| **Effect profiles** | `none`, `read-local`, `write-local`, `read-external`, `write-external` — actions declare their blast radius |
| **Approval gate** | `write-external` actions pause at `awaiting_approval`; require `POST /sessions/{id}/approve` |
| **Execution budget** | Max duration and max capability invocations per session |
| **Rate limiting** | Pluggable `RateLimiter` checked before each execution |
| **Output validation** | Results validated against output contracts before `succeeded` |
| **Idempotency profile** | Actions declare whether they're safe to retry |

## Writing a Plugin

Complete example — see [`example/main.go`](example/main.go) for a runnable version.

```go
// 1. Implement domain.Plugin with actions and capabilities
type greeterPlugin struct{}

func (p *greeterPlugin) Contribute() (*domain.PluginContribution, error) {
    cap, _ := domain.NewCapabilityDefinition("string.upper", "Uppercases",
        domain.EmptyContract(), domain.EmptyContract())
    _ = cap.BindExecutor("exec.upper")

    reqs, _ := domain.NewRequirementSet(domain.Requirement{Capability: "string.upper"})
    action, _ := domain.NewActionDefinition("greet", "Greets someone",
        domain.NewContract([]domain.ContractField{
            {Name: "name", Type: "string", Required: true, Example: "world"},
        }),
        domain.NewContract([]domain.ContractField{
            {Name: "message", Type: "string", Required: true},
        }),
        reqs,
        domain.EffectProfile{Level: domain.EffectNone},
        domain.IdempotencyProfile{IsIdempotent: true},
    )
    _ = action.BindExecutor("exec.greet")

    return domain.NewPluginContribution("greeter",
        []*domain.ActionDefinition{action},
        []*domain.CapabilityDefinition{cap})
}

// 2. Implement executors (the actual code that runs)
type upperExecutor struct{}
func (e *upperExecutor) Execute(ctx context.Context, input any) (any, error) {
    return strings.ToUpper(input.(string)), nil
}

type greetExecutor struct{}
func (e *greetExecutor) Execute(ctx context.Context, input any, caps domain.CapabilityInvoker) (domain.ExecutionResult, []domain.EvidenceRecord, error) {
    m := input.(map[string]any)
    upper, _ := caps.Invoke("string.upper", m["name"])
    return domain.ExecutionResult{
        Data:        map[string]any{"message": "Hello, " + upper.(string) + "!"},
        ContentType: "application/json",
    }, nil, nil
}

// 3. Register atomically with PluginBundle
bundle, _ := domain.NewPluginBundle(contribution,
    map[domain.ActionExecutorRef]domain.ActionExecutor{"exec.greet": &greetExecutor{}},
    map[domain.CapabilityExecutorRef]domain.CapabilityExecutor{"exec.upper": &upperExecutor{}},
)
compositionService.RegisterBundle(bundle, actionExecReg, capExecReg)
```

## API Reference

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Health check |
| `GET` | `/api/v1/actions` | List actions with contracts |
| `GET` | `/api/v1/actions/{name}` | Get action details |
| `POST` | `/api/v1/actions/execute` | Execute action (sync or `async:true`) |
| `GET` | `/api/v1/sessions/{id}` | Get/poll session state |
| `POST` | `/api/v1/sessions/{id}/approve` | Approve pending session |
| `POST` | `/api/v1/sessions/{id}/reject` | Reject pending session |
| `GET` | `/api/v1/capabilities` | List capabilities |
| `POST` | `/api/v1/plugins` | Register plugin metadata |
| `DELETE` | `/api/v1/plugins/{id}` | Deregister plugin |

Error responses include an `error_code` field: `not_found`, `conflict`, `validation_error`, `unauthorized`, `internal_error`.

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

## Authentication

Bearer-token auth is included; implement `api.Authenticator` for JWT/OAuth/etc.

```go
auth := api.NewBearerTokenAuth("my-secret-token")
srv.Engine().Use(api.AuthMiddleware(auth, "/health")) // skip auth on /health
```

## Persistence

Two adapters included, pick the one that fits:

| Adapter | Package | Use for |
|---------|---------|---------|
| In-memory | `inmemory/` | Tests, ephemeral workloads, single-process |
| JSON files | `jsonstore/` | Small deployments, dev/staging, simple persistence |

For Postgres, SQLite, Redis, etc. implement the 4 repository interfaces in `domain/`:
`ActionRepository`, `CapabilityRepository`, `PluginRepository`, `SessionRepository`.

```go
actionRepo, _ := jsonstore.NewActionStore("./data")
capRepo, _ := jsonstore.NewCapabilityStore("./data")
pluginRepo, _ := jsonstore.NewPluginStore("./data")
sessionRepo, _ := jsonstore.NewSessionStore("./data")
```

## Architecture

axi-go is built with strict [Domain-Driven Design](https://www.domainlanguage.com/ddd/):

```
domain/          Core business logic. Zero dependencies.
application/     Use cases that orchestrate the domain.
api/             HTTP delivery, config, middleware.
inmemory/        In-memory adapters + structured logger.
jsonstore/       File-based JSON persistence.
cmd/server/      Entrypoint with graceful shutdown.
example/         Working sample plugin.
```

**Dependency direction**: `domain` ← `application` ← `api`/`inmemory`/`jsonstore` ← `cmd/server`

The domain has no external imports and no knowledge of HTTP, JSON, or storage. All port interfaces live in `domain/`.

## Development

```bash
make check          # Full suite: fmt + lint + test + security
make test           # Run tests
make lint           # golangci-lint (gocritic, errcheck, staticcheck, unused)
make fmt            # Auto-fix formatting
make cover          # Tests with coverage
make security       # nox security scan
make install-hooks  # Install pre-commit git hook
go test ./... -race # Race detector (important for async execution)
```

Every commit passes through a pre-commit hook that runs fmt, lint, vet, and all 101+ tests.

## Status

**Production-ready core** — domain, API, safety, persistence, observability all shipped. Used as an execution kernel behind agent systems.

See [CONTRIBUTING.md](CONTRIBUTING.md) for contributing, [CLAUDE.md](CLAUDE.md) for a deeper architecture reference.

## License

MIT — see [LICENSE](LICENSE).
