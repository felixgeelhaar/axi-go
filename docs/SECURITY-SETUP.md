# Repository security setup

This document records the recommended GitHub settings for the axi-go repo.
Branch protection and settings live in GitHub (not in code), so this file
documents the state the repo should converge on. Check the boxes once
each setting is live; the file is the source of truth for review.

---

## Branch protection — `main`

Applied via `gh api` on 2026-04-17. Current state:

- [x] **Require a pull request before merging** (via API flag; PRs merge
      only when CI is green)
  - [x] Required approvals: **0** (solo repo; bump to 1 when multiple
        maintainers exist)
  - [x] Dismiss stale pull request approvals when new commits are pushed
  - [x] Require review from **Code Owners** (enforces `.github/CODEOWNERS`)
- [x] **Require status checks to pass before merging**
  - [x] Require branches to be up to date before merging
  - Required checks (from `.github/workflows/ci.yml`):
    - [x] `Format`
    - [x] `Lint (golangci-lint + gocritic + staticcheck)`
    - [x] `go vet`
    - [x] `Build`
    - [x] `Test (race detector)`
    - [x] `Coverage`
    - [x] `Security (govulncheck)`
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
- [x] **Dependabot alerts**
- [x] **Dependabot security updates** (backed by `.github/dependabot.yml`)
- [x] **Dependabot version updates** (gomod + github-actions, weekly)
- [x] **Secret scanning** (GitHub Advanced Security — free for public repos)
- [x] **Push protection** (blocks commits containing secret patterns)
- [ ] **Secret scanning: generic (non-provider) patterns** — toggled
      via API but the repo reports disabled; appears to require an
      org-level setting. Revisit.
- [ ] **Secret scanning: validity checks** — same story as above.
- [x] **Code scanning** (CodeQL via `.github/workflows/codeql.yml`)
- [x] **Private vulnerability reporting**
      (enables the "Report a vulnerability" button referenced in
      [SECURITY.md](../.github/SECURITY.md))

---

## Releases

Settings → Actions → General → Workflow permissions:

- [x] **Read repository contents permission** (default)
- [ ] **Allow GitHub Actions to create and approve pull requests** —
      leave disabled; dependabot doesn't need it

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
- No API tokens (govulncheck and dependabot use public sources)

If future features need secrets, add them as
**environment-scoped** secrets (not repo-wide) and document the scope
here.

---

## Reviewing this document

Quarterly, walk the list. Settings that were once unavailable or
inappropriate may have become defaults, and vice versa. Update this file
and the repo settings together so they stay aligned.
