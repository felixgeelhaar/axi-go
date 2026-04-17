
## API stability for 1.0

Every exported type, function, and method in the axi, domain, application, inmemory, jsonstore, and toon packages is either frozen for 1.x or explicitly marked `// Deprecated:` with a defined removal schedule. No silent removals between minor versions post-1.0. Deprecation policy documented in docs/ROADMAP.md.

---

## Godoc completeness

Every exported name carries a narrative doc comment, and at least one Example* function exists for every top-level API surface (Kernel methods, toon.Encode, axi.Truncate, domain.Pipeline). Examples appear in go doc and at pkg.go.dev.

---

## Persistence schema frozen at v1

SessionSnapshot.Schema == "1" is the 1.x persistence format. Future incompatible changes bump the schema and ship with a documented migration. jsonstore round-trip tests cover every persisted field (Suggestions, TokensUsed, ApprovalDecision, schema version, legacy-without-schema loading).

---

## CI quality floor

Every CI gate green on main before 1.0: fmt, lint (golangci-lint v2), go vet, build, test with race detector, coverage >= 60%, govulncheck, nightly fuzz-smoke on toon.Encode (-fuzztime=5m). Coverage regressions fail the build.

---

## Security posture for 1.0

SECURITY.md reflects current practice. Release workflow signs SBOM with cosign keyless (GitHub OIDC). CycloneDX SBOM attached to every GitHub Release. Branch protection live on main. CODEOWNERS enforced. Dependabot + secret scanning + push protection enabled. Private vulnerability reporting enabled. Documented in docs/SECURITY-SETUP.md.

---

## Adoption signal

At least one production use case outside the author's own systems, OR an explicit decision documented in the v1.0.0 release notes to ship 1.0 without one. The decision-with-rationale path is acceptable; the box is checked either way once the rationale is captured.

---

## [post-1.0] Streaming ExecutionResult

Deferred past 1.0. Current ExecutionResult is all-or-nothing. MCP 2025-06-18 supports streaming via SSE; future StreamingActionExecutor interface can be added additively without breaking the synchronous path.

---

## [post-1.0] Distributed saga semantics

Deferred past 1.0. Pipeline compensation is in-process only. Full distributed sagas across service boundaries require a durable event log and at-least-once semantics the library does not provide. In-process saga shape is intentional as a migration path.

---

## [post-1.0] Evidence integrity

Deferred past 1.0. Plugins can currently forge EvidenceRecord.TokensUsed — documented trust boundary in CONCEPTS.md. A future release may hash-chain evidence or require kernel-signed origin for production-grade audit integrity.

---

## [post-1.0] Observability ports

Deferred past 1.0. A MetricsReporter port analogous to domain.Logger would let adopters plumb Prometheus/OpenTelemetry without axi-go importing vendor clients. Zero-impl by default to preserve the zero-deps story.

---
