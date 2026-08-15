# Backlog

Forward-looking work only. Recently closed items are in
[CHANGELOG.md](../CHANGELOG.md).

---

## [post-1.x] Emission-time evidence metering

Hash chain detects post-emission tampering only. Optional adapters that
meter or sign `TokensUsed` at the capability boundary remain out of core
(preserve zero-deps).

---

## [post-1.x] First-party MCP package (maybe never)

`example/mcp-server/` is copy/vendor. Promote only if maintainers want
an MCP schema dependency in-tree — currently declined in ROADMAP.

---

## Done — Distributed saga reference plugin

Shipped as [`example/saga/`](../example/saga/): in-process outbox +
`saga.run` orchestrator with fail-closed nested write-external handling.
Durable-log backends stay in the adopter module.
