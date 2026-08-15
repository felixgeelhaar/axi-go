# Backlog

Forward-looking work only. Recently closed items are in
[CHANGELOG.md](../CHANGELOG.md).

There are no open post-1.x backlog items. Intentional non-goals for axi-go
core remain listed under [ROADMAP — Out of scope](ROADMAP.md#out-of-scope-for-core-intentional).

---

## Done — Emission-time evidence metering (adopter pattern)

Not in axi-go core (trust boundary unchanged). Reference sketch:
[`example/metering/`](../example/metering/) — stamp `TokensUsed` from
provider usage in the action executor; observers only see reported
values. Cross-session caps remain the `DomainEventPublisher` +
`RateLimiter` composition in [`example/observability/`](../example/observability/).

---

## Done — First-party MCP package (declined)

Permanently declined for axi-go core: zero external deps and no MCP schema
in-tree. Adopters copy/vendor [`example/mcp-server/`](../example/mcp-server/)
(stdlib JSON-RPC sketch, no stability guarantee). Revisit only if the
project deliberately accepts an MCP schema dependency — not planned.

---

## Done — Distributed saga reference plugin

Shipped as [`example/saga/`](../example/saga/): in-process outbox +
`saga.run` orchestrator with fail-closed nested write-external handling.
Durable-log backends stay in the adopter module.
