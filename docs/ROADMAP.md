# Roadmap and versioning policy

This document is the contract between axi-go and its users about stability,
change, and what "1.x" means. If you're evaluating axi-go for a production
dependency, read this first.

---

## Current status: 1.x (post-1.0)

axi-go shipped `v1.0.0` and has continued with additive and occasional
breaking minor/patch tags through **v1.5.0** (UUIDv7 default session IDs,
kernel hardening, saga/metering examples). The project follows
[Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html) and the
deprecation policy below. Breaking changes are annotated in
[CHANGELOG.md](../CHANGELOG.md) with a `BREAKING` tag and are typically
preceded by a commit message starting with `feat!:` or `fix!:`.

Latest released tag on the line of this document: **v1.5.0** — see Git
tags / [CHANGELOG.md](../CHANGELOG.md). New work on `main` lands under
`[Unreleased]` until the next tag.

---

## 1.0 checklist (historical)

These were the gates for the first stable tag. Status reflects current
practice, not a remaining pre-release blocker:

- [x] **API stability:** exported surface in `axi`, `domain`, `application`,
      `inmemory`, `jsonstore`, and `toon` is under SemVer; deprecations use
      `// Deprecated:` with the policy below.
- [x] **Godoc completeness:** narrative docs + `Example*` for top-level
      surfaces (`Kernel.Execute`, `Kernel.Help`, `Kernel.ListActionSummaries`,
      `toon.Encode`, `axi.Truncate`, `domain.Pipeline`).
- [x] **Persistence schema:** `SessionSnapshot.Schema == "1"`
      (`CurrentSessionSchema`). Empty schema loads as legacy. Unknown
      non-empty schemas are rejected. Future incompatible formats bump the
      schema and ship with a documented migration.
- [x] **CI quality floor:** shared Go CI runs fmt/lint/vet/build/test
      (race) + nox security + **coverctl coverage gate** (`.coverctl.yaml`,
      module floor 72%, examples excluded). `make cover` also runs
      `--ratchet` against committed `.cover/history.json`. Weekly fuzz
      covers `toon` and domain name/snapshot harnesses.
- [x] **Security posture:** cosign keyless SBOM signing on release; nox
      remediation replaces Dependabot; provenance/warden workflows in
      tree. See [SECURITY.md](../.github/SECURITY.md) and
      [SECURITY-SETUP.md](SECURITY-SETUP.md).
- [x] **Adoption signal:** rationale path documented in the v1.0.0
      CHANGELOG section.

---

## What qualifies as a MAJOR (breaking) change

Post-1.0, a MAJOR version bump is required whenever any of these changes:

- **Signature changes** on any exported type, function, method, or field.
  Adding a new required field to a struct that callers initialize
  positionally is a MAJOR change; adding an optional field with a safe
  zero value is MINOR.
- **Behavioral contracts** that existed in godoc or tests: return-value
  shape, error types, side effects, panic semantics, ordering guarantees.
- **Persistence schema** in any snapshot format. A schema bump is always
  a MAJOR change and ships with a documented migration path.
- **Default adapter behavior** when it would silently break users on
  upgrade. Example: changing the default session ID generator scheme
  (as with UUIDv7 vs `session-N`) is breaking for tests that match IDs.
- **Tightening** a previously permissive contract. If `Kernel.Execute`
  starts rejecting inputs it used to accept, that's breaking even if the
  rejection is "correct."

What does **not** require MAJOR:

- Adding new methods or types.
- Adding optional fields to structs, so long as zero values preserve
  existing behavior.
- Performance improvements that don't change observable outputs.
- Fixes to bugs whose prior behavior no user could reasonably have
  depended on. (Judgment call; document in the changelog.)
- Documentation, logging, and test-only changes.

---

## Deprecation policy

From 1.0 onwards:

1. A deprecated name is marked with `// Deprecated: <what to use instead>`
   in godoc.
2. Deprecated names are kept working for at least **one full MINOR
   release cycle** after the deprecation ships. Example: a name
   deprecated in `v1.5.0` is removable in `v1.7.0` at the earliest.
3. Every deprecation appears in the CHANGELOG under a **Deprecated**
   section, with a code snippet showing the migration.
4. Removals happen only in MAJOR releases (i.e. `v2.0.0`) — never in a
   MINOR or PATCH release.

---

## Out of scope for core (intentional)

- **Distributed sagas.** Pipeline compensation is in-process only. As of
  1.2 the kernel exposes `ActionInvoker` / `OrchestratorActionExecutor`
  so a saga engine can ship as a plugin with its own durable log. See
  [`example/saga/`](../example/saga/) for a fail-closed reference.
- **Emission-time evidence honesty.** The SHA-256 evidence hash chain
  detects post-emission tampering; plugins can still report untruthful
  `TokensUsed` at emit time — documented trust boundary in
  [CONCEPTS.md](CONCEPTS.md). Meter at the provider boundary in an
  adopter adapter; see [`example/metering/`](../example/metering/).
- **First-party MCP package.** Declined: axi-go stays zero-deps and will
  not import an MCP schema. Use / copy
  [`example/mcp-server/`](../example/mcp-server/) (no stability
  guarantee — see that example's README).
- **Vendor metrics clients.** Use `DomainEventPublisher` adapters
  (`example/observability/`) rather than importing Prometheus/OTel into
  core.

---

## Getting notified

- Watch the repo on GitHub for release notifications.
- The GitHub Release body is always the relevant CHANGELOG section for
  that tag.
- Security advisories are published via GitHub Security Advisories per
  [SECURITY.md](../.github/SECURITY.md).

Feedback on this roadmap is welcome as a GitHub issue.
