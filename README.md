# axi-go

**A domain-driven execution kernel for AI agent tools — a Go library you embed, not a service you run.**

[![CI](https://github.com/felixgeelhaar/axi-go/actions/workflows/ci.yml/badge.svg)](https://github.com/felixgeelhaar/axi-go/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/felixgeelhaar/axi-go)](https://goreportcard.com/report/github.com/felixgeelhaar/axi-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/felixgeelhaar/axi-go.svg)](https://pkg.go.dev/github.com/felixgeelhaar/axi-go)

**Zero external dependencies.** Standard library only.

---

## Why axi-go?

When you give an AI agent a bag of tools (`search`, `send_email`, `run_sql`), you quickly hit these problems:

- **No safety** — the agent can call `send_email` a thousand times before you know it
- **No audit trail** — you can't explain *why* the agent did what it did
- **Tool sprawl** — 200 raw functions, no grouping, no dependencies, no lifecycle
- **No type information** — the agent has to guess what inputs each tool accepts
- **No approval gates** — the agent can take irreversible actions autonomously

**axi-go solves this** with a two-layer model:

| Layer | Example | Answers |
|-------|---------|---------|
| **Actions** | `greet`, `send-email`, `search-docs` | *What* the agent wants to do (intent) |
| **Capabilities** | `string.upper`, `http.get`, `db.query` | *How* it gets done (mechanics) |

An action declares the capabilities it needs. axi-go resolves them, validates inputs against typed contracts, enforces effect profiles (read-only? writes? external?), pauses for human approval when required, runs within execution budgets, and produces a structured audit trail.

You embed axi-go in your Go program. It has **no HTTP API, no daemon, no protocol assumptions** — those are delivery concerns for you to choose (HTTP, gRPC, CLI, MCP, whatever fits your stack).

## Install

```bash
go get github.com/felixgeelhaar/axi-go
```

## 60-Second Tour

```go
package main

import (
    "context"
    "fmt"
    "github.com/felixgeelhaar/axi-go"
)

func main() {
    // 1. Build a kernel with fluent configuration.
    kernel := axi.New().
        WithBudget(axi.Budget{MaxCapabilityInvocations: 100})

    // 2. Wire executors and register your plugin.
    kernel.RegisterActionExecutor("exec.greet", &greetExecutor{})
    kernel.RegisterCapabilityExecutor("exec.upper", &upperExecutor{})
    _ = kernel.RegisterPlugin(&greeterPlugin{})

    // 3. Execute an action.
    result, _ := kernel.Execute(context.Background(), axi.Invocation{
        Action: "greet",
        Input:  map[string]any{"name": "world"},
    })
    fmt.Println(result.Result.Data)  // → {"message": "Hello, WORLD!"}
}
```

See [`example/main.go`](example/main.go) for a complete runnable example.

## Core Concepts

### Actions express *intent*

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

Because this is `write-external`, axi-go **pauses for approval**:

```go
result, _ := kernel.Execute(ctx, axi.Invocation{Action: "send-email", Input: ...})
// result.Status == "awaiting_approval"

// A supervisor approves (or rejects):
final, _ := kernel.Approve(ctx, string(result.SessionID))
// final.Status == "succeeded"
```

### Capabilities express *mechanics*

```go
smtpCap, _ := domain.NewCapabilityDefinition(
    "smtp.send",
    "Sends an email via SMTP",
    inputContract, outputContract,
)
```

Capabilities are building blocks. Actions compose them. Capabilities themselves have no effect profile — the action's profile governs safety.

### Plugins bundle actions + capabilities

```go
type emailPlugin struct{}

func (p *emailPlugin) Contribute() (*domain.PluginContribution, error) {
    return domain.NewPluginContribution("email.plugin",
        []*domain.ActionDefinition{sendEmailAction},
        []*domain.CapabilityDefinition{smtpCap},
    )
}

kernel.RegisterPlugin(&emailPlugin{})
```

### Sessions track each execution

Every execution gets a session with a strict state machine:

```
Pending → Validated → Resolved → [AwaitingApproval] → Running → Succeeded | Failed | Rejected
```

The session persists input, resolved capabilities, evidence, and result/failure. Poll it anytime:

```go
session, _ := kernel.GetSession(sessionID)
fmt.Println(session.Status(), session.Evidence())
```

## The SDK

The root `axi` package provides a fluent, descriptive API:

```go
kernel := axi.New().
    WithLogger(logger).
    WithBudget(axi.Budget{MaxDuration: 5*time.Minute, MaxCapabilityInvocations: 100}).
    WithRateLimiter(myRateLimiter).
    WithIDGenerator(uuidGen)

// Register
kernel.RegisterPlugin(plugin)
kernel.RegisterPluginWithConfig(plugin, map[string]any{"api_key": "..."})
kernel.RegisterBundle(bundle)  // atomic: metadata + executors
kernel.DeregisterPlugin("my.plugin")

// Execute
result, _ := kernel.Execute(ctx, axi.Invocation{Action: "x", Input: ...})
result, _ := kernel.ExecuteAsync(ctx, axi.Invocation{Action: "x", Input: ...})

// Approval flow
result, _ := kernel.Approve(ctx, sessionID)
result, _ := kernel.Reject(sessionID, "too risky")

// Introspection
actions := kernel.ListActions()
caps    := kernel.ListCapabilities()
session, _ := kernel.GetSession(sessionID)
```

## Safety & Control

| Feature | What it does |
|---------|--------------|
| **Effect profiles** | `none`, `read-local`, `write-local`, `read-external`, `write-external` |
| **Approval gate** | `write-external` actions pause at `awaiting_approval` — call `kernel.Approve(...)` |
| **Execution budgets** | Max duration and max capability invocations per session |
| **Rate limiting** | Pluggable `RateLimiter` checked before each execution |
| **Output validation** | Results validated against output contracts before `succeeded` |
| **Idempotency profile** | Actions declare whether they're safe to retry |
| **Evidence trail** | Append-only `EvidenceRecord`s with timestamps — full audit log |

## Persistence

Two adapters included. Pick one, or implement the repository interfaces in `domain/` for Postgres, SQLite, Redis, etc.

| Adapter | Package | Use for |
|---------|---------|---------|
| In-memory | `inmemory/` | Tests, single-process, ephemeral |
| JSON files | `jsonstore/` | Small deployments, simple persistence |

By default, `axi.New()` uses `inmemory/`. Swap the repositories by implementing the 4 ports in `domain/`: `ActionRepository`, `CapabilityRepository`, `PluginRepository`, `SessionRepository`.

## Architecture

axi-go is built with strict [Domain-Driven Design](https://www.domainlanguage.com/ddd/):

```
axi (root)       Fluent SDK facade — what you import.
domain/          Aggregates, services, port interfaces. Zero deps.
application/     Use cases that orchestrate the domain.
inmemory/        In-memory adapters + StdLogger.
jsonstore/       File-based JSON persistence adapter.
example/         Working sample plugin.
```

**Dependency direction**: `domain` ← `application` ← `inmemory`/`jsonstore` ← `axi` ← your code

The domain has no external imports and no knowledge of JSON, HTTP, or any delivery mechanism. All port interfaces live in `domain/`.

## Building a delivery adapter

axi-go is a kernel. If you need HTTP, gRPC, MCP, or a CLI, build it as a thin adapter on top:

```go
// Your HTTP handler (you own this, it's not in axi-go)
func executeHandler(kernel *axi.Kernel) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var req ExecuteRequest
        _ = json.NewDecoder(r.Body).Decode(&req)

        result, err := kernel.Execute(r.Context(), axi.Invocation{
            Action: req.Action, Input: req.Input,
        })
        if err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }
        _ = json.NewEncoder(w).Encode(result)
    }
}
```

An MCP server adapter, a gRPC service, or a Cobra CLI would all follow the same pattern: translate protocol → kernel calls → translate response.

## Development

```bash
make check          # Full suite: fmt + lint + test + security
make test           # Run tests
make lint           # golangci-lint
make fmt            # Auto-fix formatting
make install-hooks  # Install pre-commit git hook
go test ./... -race # Race detector
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidelines and [CLAUDE.md](CLAUDE.md) for a deeper architecture reference.

## License

MIT — see [LICENSE](LICENSE).
