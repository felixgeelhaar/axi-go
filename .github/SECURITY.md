# Security Policy

## Supported Versions

Security fixes are provided for the latest release on `main`.

| Version | Supported |
|---------|-----------|
| main / latest 1.x tag | ✅ |
| older | ❌ |

## Reporting a Vulnerability

**Do not open a public issue for security vulnerabilities.**

Report privately via:

1. **GitHub Security Advisories** (preferred): Use the
   [Report a vulnerability](https://github.com/klarlabs-studio/axi-go/security/advisories/new)
   button on the Security tab.

Email is not currently a monitored channel for this repository; use
GitHub private reporting.

### What to include

- A description of the vulnerability
- Steps to reproduce (minimal code example if possible)
- The affected components and versions
- Potential impact (data exposure, privilege escalation, DoS, etc.)
- Any suggested mitigation

### What to expect

- **Acknowledgment** within 3 business days
- **Initial assessment** within 7 days (severity rating, whether fix is needed)
- **Fix timeline** depends on severity:
  - Critical: patch within 7 days
  - High: patch within 30 days
  - Medium/Low: next scheduled release
- **Disclosure** coordinated with reporter; CVE requested for significant issues

## Security practices in this project

- **Zero external dependencies** — minimizes supply chain attack surface
- **GitHub Actions pinned to commit SHAs** — prevents mutable tag attacks
- **nox security scan + remediation** — taint analysis and automated
  dependency/action remediation (`.github/workflows/nox-remediate.yml`)
- **govulncheck** — via shared Go CI / release path
- **Race detector** — exercised in tests and release CI
- **Cosign keyless** — SBOM signing on GitHub Releases (OIDC)
- **Warden / provenance** — supply-chain provenance workflow in tree

## Hall of fame

Reporters who responsibly disclose vulnerabilities will be credited here (with consent).
