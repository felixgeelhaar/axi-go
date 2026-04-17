# Roadmap and versioning policy

This document is the contract between axi-go and its users about stability,
change, and what "1.0" means. If you're evaluating axi-go for a production
dependency, read this first.

---

## Current status: pre-1.0

axi-go is in active development. The public API has changed and will change
again before 1.0. Every breaking change is annotated in
[CHANGELOG.md](../CHANGELOG.md) with a `BREAKING` tag and is preceded by
a commit whose message starts with `feat!:`. Users pinning to a specific
pre-1.0 tag can upgrade at their own pace.

---

## What 1.0 means

1.0 is the point at which axi-go commits to
[Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html). A user
who depends on `v1.x.y` can upgrade to any later `v1.*.*` without expecting
source-level breakage.

To reach 1.0, axi-go must meet all of:

- [x] **API stability:** every exported type, function, and method in the
      `axi`, `domain`, `application`, `inmemory`, `jsonstore`, and `toon`
      packages is either frozen for 1.x or explicitly marked
      `// Deprecated:` with a defined removal schedule. As of the 1.0
      audit, no names are marked `// Deprecated:` — the entire exported
      surface is frozen for 1.x.
- [x] **Godoc completeness:** every exported name carries a narrative
      doc comment, and at least one `Example*` function exists for every
      top-level API surface (`Kernel.Execute`, `Kernel.Help`,
      `Kernel.ListActionSummaries`, `toon.Encode`, `axi.Truncate`,
      `domain.Pipeline`).
- [ ] **Persistence schema frozen:** `SessionSnapshot.Schema == "1"` is
      treated as the 1.x persistence format. Future incompatible changes
      bump the schema and ship with a documented migration.
- [ ] **CI quality floor:** fmt + lint + vet + race + fuzz-smoke + govulncheck
      + coverage gate (currently 60%) all green.
- [ ] **Security posture:** SECURITY.md reflects current practice, release
      workflow signs artifacts with cosign keyless, SBOM attached to every
      GitHub Release.
- [x] **Adoption signal:** the rationale path was taken. v1.0.0 ships
      without a public external-adoption claim; the decision and
      reasoning are captured in the "Adoption note for v1.0.0" section
      of [CHANGELOG.md](../CHANGELOG.md).

When all six are checked, the next tag is `v1.0.0`.

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
  upgrade. Example: changing `inmemory.NewSequentialIDGenerator` to emit
  UUIDs instead of `session-N` would break any tests that match session
  ids — MAJOR.
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

During pre-1.0, this policy is aspirational. Breaking changes may land
between minor tags as the API continues to shake out.

---

## Out of scope for 1.0

Features that have been considered and deferred past 1.0:

- **Streaming `ExecutionResult`.** Current result is all-or-nothing. MCP
  2025-06-18 supports streaming via SSE; a future iteration may add
  `StreamingActionExecutor` without breaking the synchronous path.
- **Distributed sagas.** Pipeline compensation is in-process only. Full
  sagas across service boundaries require a durable event log and
  at-least-once semantics the library does not provide.
- **Evidence integrity.** Plugins can currently forge
  `EvidenceRecord.TokensUsed`. A future release may hash-chain evidence
  or require a kernel-signed origin. For 1.0, this remains a documented
  trust boundary — see [CONCEPTS.md](CONCEPTS.md).
- **Observability ports.** A `MetricsReporter` port analogous to `Logger`
  is on the list but not in the 1.0 critical path.
- **MCP adapter as a package.** `example/mcp-server/` will stay as an
  example. Users copy-paste it or vendor it; axi-go itself will not
  import an MCP schema.

---

## Getting notified

- Watch the repo on GitHub for release notifications.
- The GitHub Release body is always the relevant CHANGELOG section for
  that tag.
- Security advisories are published via GitHub Security Advisories per
  [SECURITY.md](../.github/SECURITY.md).

Feedback on this roadmap — especially on the 1.0 checklist — is
welcome as a GitHub issue.
