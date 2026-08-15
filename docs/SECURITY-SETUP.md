# Repository security setup

This document records the recommended GitHub settings for the axi-go repo.
Branch protection and settings live in GitHub (not in code), so this file
documents the state the repo should converge on. Check the boxes once
each setting is live; the file is the source of truth for review.

---

## Branch protection — `main`

Applied via `gh api` on 2026-04-17; revisit when CI job names change.

- [x] **Require a pull request before merging**
  - [x] Required approvals: **0** (solo repo; bump to 1 when multiple
        maintainers exist)
  - [x] Dismiss stale pull request approvals when new commits are pushed
  - [x] Require review from **Code Owners** (enforces `.github/CODEOWNERS`)
- [x] **Require status checks to pass before merging**
  - [x] Require branches to be up to date before merging
  - Required checks come from the shared workflow invoked by
    `.github/workflows/ci.yml` (klarlabs-studio `go-ci.yml`). Exact check
    names follow that reusable workflow (typically Lint / Test / Build /
    Security). Update this list when the shared workflow renames jobs.
- [ ] **Require signed commits** — deferred; not every environment is set
      up to produce signatures. Revisit when the project has an external
      contributor.
- [x] **Require linear history** (enforces rebase/squash workflow)
- [x] **Require conversation resolution before merging**
- [ ] **Do not allow bypassing the above settings** — intentionally off
      (`enforce_admins: false`) so the solo maintainer can push hotfixes
      directly. Flip to on when protection must apply to everyone.
- [ ] **Restrict who can push to matching branches** — leave unchecked on
      a solo repo; enable when multiple maintainers exist
- [x] **Block force pushes** and **block deletions**

---

## General security settings

Settings → Code security and analysis:

- [x] **Dependency graph** (public repo — enabled by default)
- [x] **Secret scanning** (GitHub Advanced Security — free for public repos)
- [x] **Push protection** (blocks commits containing secret patterns)
- [ ] **Secret scanning: generic (non-provider) patterns** — may require
      an org-level setting. Revisit.
- [ ] **Secret scanning: validity checks** — same story as above.
- [x] **Private vulnerability reporting**
      (enables the "Report a vulnerability" button referenced in
      [SECURITY.md](../.github/SECURITY.md))
- [x] **nox remediation** — `.github/workflows/nox-remediate.yml` replaces
      Dependabot for gomod / Actions updates
- [ ] **Dependabot alerts / version updates** — superseded by nox-remediate;
      leave disabled unless maintainers re-enable intentionally
- [ ] **CodeQL workflow** — removed when adopting shared Go CI; static
      analysis is covered by golangci-lint + nox taint analysis instead

---

## Releases

Settings → Actions → General → Workflow permissions:

- [x] **Read repository contents permission** (default)
- [ ] **Allow GitHub Actions to create and approve pull requests** —
      leave disabled unless a remediation bot needs it

Tags prefixed `v*.*.*` trigger `.github/workflows/release.yml`, which
requires:

- [x] `id-token: write` permission (cosign keyless via GitHub OIDC)
- [x] `contents: write` permission (publish GitHub Release)

No long-lived secrets needed. The signing identity is the GitHub Actions
OIDC token, bound to the release workflow and repo.

---

## Secret management

The repo intentionally requires no secrets to build, test, or release:

- No registry credentials (Go modules published via proxy)
- No signing keys (cosign keyless uses OIDC)
- No API tokens (govulncheck and nox use public sources)

If future features need secrets, add them as
**environment-scoped** secrets (not repo-wide) and document the scope
here.

---

## Reviewing this document

Quarterly, walk the list. Settings that were once unavailable or
incorrect should be corrected here in the same PR that changes workflows.
