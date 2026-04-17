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
For an MCP (Model Context Protocol) adapter in ~250 lines with no external
deps, see [`example/mcp-server/`](example/mcp-server/).

If you want to understand the *why* behind the shape of the library — the
reasoning that makes actions, capabilities, effect profiles, and evidence
inevitable once you accept certain premises — read
[`docs/CONCEPTS.md`](docs/CONCEPTS.md).

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

## Configuring a kernel

The fluent builder on `axi.New()` returns a configured `*Kernel`. Chain
the `With*` methods as needed:

```go
kernel := axi.New().
    WithLogger(logger).
    WithBudget(axi.Budget{MaxDuration: 5*time.Minute, MaxCapabilityInvocations: 100}).
    WithRateLimiter(myRateLimiter).
    WithIDGenerator(uuidGen)
```

Register plugins and executors before the first `Execute`:

```go
kernel.RegisterPlugin(plugin)
kernel.RegisterBundle(bundle)  // atomic: metadata + executors together
```

Drive actions from your delivery layer:

```go
result, _ := kernel.Execute(ctx, axi.Invocation{Action: "greet", Input: inp})

// For write-external actions that paused at awaiting_approval:
result, _ := kernel.Approve(ctx, sessionID, decision)
result, _ := kernel.Reject(ctx, sessionID, decision)
```

## Kernel reference (quick)

| Method | Purpose |
|---|---|
| `New()` | Build a kernel with default in-memory adapters |
| `WithLogger`, `WithBudget`, `WithRateLimiter`, `WithIDGenerator`, `WithTimeout` | Fluent configuration |
| `RegisterPlugin`, `RegisterPluginWithConfig`, `RegisterBundle` | Add actions + capabilities |
| `RegisterActionExecutor`, `RegisterCapabilityExecutor` | Bind refs to implementations |
| `DeregisterPlugin` | Remove a plugin and everything it contributed |
| `Execute`, `ExecuteAsync` | Invoke an action synchronously or in the background |
| `Approve`, `Reject` | Resolve a session awaiting approval |
| `GetSession` | Look up a session by id |
| `ListActions`, `ListCapabilities` | Full aggregates |
| `ListActionsResult`, `ListCapabilitiesResult` | Aggregates wrapped with `TotalCount` + `IsEmpty()` |
| `ListActionSummaries`, `ListCapabilitySummaries` | Minimal-schema projections (axi.md #2) |
| `GetAction`, `Help` | Introspect one action or any name (axi.md #10) |

See the godoc on [pkg.go.dev](https://pkg.go.dev/github.com/felixgeelhaar/axi-go)
for full signatures and runnable examples.

## Safety & Control

| Feature | What it does |
|---------|--------------|
| **Effect profiles** | `none`, `read-local`, `write-local`, `read-external`, `write-external` |
| **Approval gate** | `write-external` actions pause at `awaiting_approval` — call `kernel.Approve(...)` |
| **Execution budgets** | Max duration, max capability invocations, max tokens, and idempotency-gated retries per session |
| **Rate limiting** | Pluggable `RateLimiter` checked before each execution |
| **Output validation** | Results validated against output contracts before `succeeded` |
| **Idempotency profile** | Actions declare whether they're safe to retry |
| **Evidence trail** | Append-only `EvidenceRecord`s with timestamps — full audit log |
| **Pipeline saga** | Mid-pipeline failures return a `*PipelineFailure` with partial outputs and run any `PipelineStep.Compensate` hooks in reverse order |

## Agent-facing output

axi-go draws design cues from [axi.md](https://axi.md/) — a set of principles
for agent-tool interfaces optimized for token efficiency and discoverability.

### Suggestions (axi.md #9)

Actions can emit next-step hints in their result. The agent reads them and
avoids guessing what to call next:

```go
return domain.ExecutionResult{
    Data:    map[string]any{"id": "abc-123"},
    Summary: "created resource abc-123",
    Suggestions: []domain.Suggestion{
        {Action: "resource.get", Description: "Retrieve the created resource"},
        {Action: "resource.list", Description: "List all resources"},
    },
}, nil, nil
```

### TOON encoding (axi.md #1)

The `toon` package encodes results in Token-Optimized Object Notation —
brace-free, quote-free, and ~40% shorter than equivalent JSON on uniform
arrays:

```go
import "github.com/felixgeelhaar/axi-go/toon"

out, _ := toon.Encode(map[string]any{
    "issues": []any{
        map[string]any{"number": 42, "state": "open", "title": "Fix login bug"},
        map[string]any{"number": 43, "state": "open", "title": "Add dark mode"},
    },
})
// issues[2]{number,state,title}:
//   42,open,Fix login bug
//   43,open,Add dark mode
```

### Token budget (axi.md #1)

Capabilities report token usage via `EvidenceRecord.TokensUsed`; the kernel
sums them and fails the session if the budget is exceeded:

```go
kernel := axi.New().WithBudget(axi.Budget{MaxTokens: 10_000})
// A session whose evidence sums to more than 10k tokens fails with
// FailureReason.Code = "BUDGET_EXCEEDED".
```

### Truncation (axi.md #3)

`axi.Truncate` caps strings and appends a size hint so context windows stay
bounded without silently dropping data:

```go
out, truncated := axi.Truncate(longBody, 500)
// "…first 500 chars… (truncated, 2847 chars total)"
```

### Minimal schemas and empty states (axi.md #2, #5)

`Kernel.ListActionSummaries` and `Kernel.ListCapabilitySummaries` return a
discovery-oriented projection (name, description, effect/idempotency for
actions) instead of full aggregates. All list responses share the
`ListResult[T]` shape with `TotalCount` and `IsEmpty()` so callers can
distinguish "no results" from "not queried":

```go
r := kernel.ListActionSummaries()
if r.IsEmpty() {
    fmt.Println("no actions registered")
}
for _, s := range r.Items {
    fmt.Printf("  %s  (%s, idempotent=%t) — %s\n",
        s.Name, s.Effect, s.Idempotent, s.Description)
}
```

### Help (axi.md #10)

`ActionDefinition.Help()` and `CapabilityDefinition.Help()` return a
formatted reference with contracts and capability requirements.
`Kernel.Help(name)` looks up the name as an action first, then as a
capability — a consistent fallback when contextual suggestions aren't
enough:

```go
text, _ := kernel.Help("greet")
// greet — Greet someone by name
// Effect: none  Idempotent: true
//
// Input:
//   name  (string, required)  Person to greet
//     example: world
// ...
```

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
