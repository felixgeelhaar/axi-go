# Repository security setup

This document records the recommended GitHub settings for the axi-go repo.
Branch protection and settings live in GitHub (not in code), so this file
documents the state the repo should converge on. Check the boxes once
each setting is live; the file is the source of truth for review.

---

## Branch protection — `main`

Settings → Branches → Branch protection rules → `main`:

- [ ] **Require a pull request before merging**
  - [ ] Required approvals: **1**
  - [ ] Dismiss stale pull request approvals when new commits are pushed
  - [ ] Require review from **Code Owners** (enforces `.github/CODEOWNERS`)
- [ ] **Require status checks to pass before merging**
  - [ ] Require branches to be up to date before merging
  - Required checks (from `.github/workflows/ci.yml`):
    - [ ] `Format`
    - [ ] `Lint (golangci-lint + gocritic + staticcheck)`
    - [ ] `go vet`
    - [ ] `Build`
    - [ ] `Test (race detector)`
    - [ ] `Coverage`
    - [ ] `Security (govulncheck)`
- [ ] **Require signed commits** (optional but recommended — pairs with
      the DCO sign-off to give cryptographic + intent attestation)
- [ ] **Require linear history** (enforces rebase/squash workflow)
- [ ] **Do not allow bypassing the above settings** (applies to admins too)
- [ ] **Restrict who can push to matching branches** — leave unchecked on
      a solo repo; enable when multiple maintainers exist

---

## General security settings

Settings → Code security and analysis:

- [x] **Dependency graph** (public repo — enabled by default)
- [x] **Dependabot alerts**
- [x] **Dependabot security updates** (backed by `.github/dependabot.yml`)
- [x] **Dependabot version updates** (gomod + github-actions, weekly)
- [ ] **Secret scanning** (GitHub Advanced Security — free for public repos)
- [ ] **Push protection** (blocks commits containing secret patterns)
- [x] **Code scanning** (CodeQL via `.github/workflows/codeql.yml`)
- [ ] **Private vulnerability reporting**
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
