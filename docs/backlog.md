# Backlog

Forward-looking work only. Landed 1.1/1.2 items (streaming, evidence hash
chain, domain events, action orchestration) live in
[CHANGELOG.md](../CHANGELOG.md) and [ROADMAP.md](ROADMAP.md).

---

## Re-enable CI coverage gate

`.github/workflows/ci.yml` currently passes `coverage: false` into the
shared Go CI workflow. Turn the gate back on (target ≥60% with regression
fail) once the shared workflow threshold is confirmed green on main.

---

## Typed budget errors

`budgetEnforcer` still encodes limit kind in error strings;
`budgetKindFromError` parses them. Replace with typed errors so
`BudgetExceeded` classification cannot drift from message text.

---

## Broader fuzz surface

Fuzz today covers `toon.Encode`. Candidates: `SessionFromSnapshot`,
name validators, contract validation inputs.

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
