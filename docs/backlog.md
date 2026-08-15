# Backlog

Forward-looking work only. Recently closed items are in
[CHANGELOG.md](../CHANGELOG.md).

---

## [post-1.x] First-party MCP package (maybe never)

`example/mcp-server/` is copy/vendor. Promote only if maintainers want
an MCP schema dependency in-tree — currently declined in ROADMAP.

---

## Done — Emission-time evidence metering (adopter pattern)

Not in axi-go core (trust boundary unchanged). Reference sketch:
[`example/metering/`](../example/metering/) — stamp `TokensUsed` from
provider usage in the action executor; observers only see reported
values. Cross-session caps remain the `DomainEventPublisher` +
`RateLimiter` composition in [`example/observability/`](../example/observability/).

---

## Done — Distributed saga reference plugin

See [`example/saga/`](../example/saga/) (PR) — in-process outbox +
fail-closed nested write-external handling. Durable-log backends stay
in the adopter module.
