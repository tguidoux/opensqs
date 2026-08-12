# Security Policy

## Supported Versions

OpenSQS is in active development. Security updates are applied to the latest
release and the `main` branch.

| Version | Supported          |
|---------|--------------------|
| latest  | ✅ Security fixes  |
| < latest | ❌ Not supported  |

## Reporting a Vulnerability

We take security vulnerabilities seriously. If you discover a security issue,
please report it responsibly.

### How to Report

**Do NOT open a public GitHub issue for security vulnerabilities.**

Instead, please report vulnerabilities via one of these channels:

1. **GitHub Security Advisories** (preferred):
   - Go to the [Security tab](https://github.com/tguidoux/opensqs/security/advisories/new)
   - Click "Report a vulnerability"
   - Fill in the details

2. **Email**: Send details to **security@opensqs.io**

### What to Include

Please include as much of the following as possible:

- Description of the vulnerability and its impact
- Steps to reproduce the issue
- Affected versions (if known)
- Proof of concept (if available)
- Suggested fix (if any)

### Response Timeline

| Step | Target |
|------|--------|
| Acknowledgment of report | Within 48 hours |
| Initial assessment | Within 5 business days |
| Fix or mitigation | Within 30 days (severity-dependent) |
| Public disclosure | After a fix is released, coordinated with reporter |

### What to Expect

- We will acknowledge your report promptly.
- We will investigate and validate the issue.
- We will work on a fix and coordinate disclosure with you.
- We will credit you in the security advisory (unless you prefer to remain anonymous).

## Security Best Practices for Deployments

When deploying OpenSQS in production:

- **Change the default server secret** — The `serverSecret` in `config.yaml` must
  be changed from the default value. Use AWS SSM Parameter Store or a secrets
  manager for production.
- **Enable TLS** — Configure TLS termination at the load balancer or use the
  built-in TLS support.
- **Restrict network access** — Use Kubernetes NetworkPolicies or firewall rules
  to limit access to trusted clients only.
- **Use distroless images** — The provided container images are distroless for
  minimal attack surface.
- **Run as non-root** — The container images run as a non-root user by default.
- **Keep dependencies updated** — We use Dependabot to monitor dependency
  vulnerabilities. Review and merge dependency updates promptly.

## Disclosure Policy

- We follow coordinated disclosure.
- Vulnerabilities are disclosed after a fix is available.
- We request a 90-day window to fix issues before public disclosure, but this
  is flexible based on severity and reporter preference.
