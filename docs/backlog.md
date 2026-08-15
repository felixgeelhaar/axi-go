# Backlog

Forward-looking work only. Recently closed items (typed budget errors,
coverage gate, broader fuzz, adapter unit tests) are in
[CHANGELOG.md](../CHANGELOG.md).

---

## [post-1.x] Distributed saga reference plugin

Kernel primitives exist (`ActionInvoker`). Ship or link an external
example module that keeps a durable outbox/log out of axi-go core and
demonstrates fail-closed nested approval handling.

---

## [post-1.x] Emission-time evidence metering

Hash chain detects post-emission tampering only. Optional adapters that
meter or sign `TokensUsed` at the capability boundary remain out of core
(preserve zero-deps).

---

## [post-1.x] First-party MCP package (maybe never)

`example/mcp-server/` is copy/vendor. Promote only if maintainers want
an MCP schema dependency in-tree — currently declined in ROADMAP.
