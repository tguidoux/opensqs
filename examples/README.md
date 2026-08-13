# OpenSQS Examples

This directory contains ready-to-use examples for running OpenSQS in various environments. Each example is self-contained with configuration files, deployment manifests, and documentation.

## Directory Structure

```
examples/
├── README.md                  # This file
├── docker/                    # Dockerfile-based examples
│   ├── README.md
│   ├── Dockerfile.no-auth     # No authentication
│   ├── Dockerfile.with-creds  # With initial credentials
│   ├── config.no-auth.yaml
│   └── config.with-creds.yaml
├── docker-compose/            # Docker Compose examples
│   ├── README.md
│   ├── docker-compose.no-auth.yml
│   ├── docker-compose.with-creds.yml
│   ├── docker-compose.sqlite.yml
│   ├── config.no-auth.yaml
│   ├── config.with-creds.yaml
│   └── config.sqlite.yaml
├── kubernetes/                # Kubernetes manifests
│   ├── README.md
│   ├── k8s.no-auth.yaml       # No authentication
│   ├── k8s.with-creds.yaml    # With initial credentials
│   ├── k8s.persistent.yaml    # With SQLite persistence + credentials
│   └── k8s.secret.yaml        # Kubernetes Secret for credentials
└── commands/                  # Shell scripts with AWS CLI commands
    ├── README.md
    ├── quick-start.sh         # Basic create/send/receive/delete
    ├── fifo-and-dlq.sh        # FIFO queues + dead-letter queues
    └── run-with-bazel.sh      # Build & run from source with Bazel
```

## Quick Reference

| Example | Auth | Storage | Environment |
|---------|------|---------|-------------|
| [Docker — No Auth](docker/) | Disabled | Memory | Local dev |
| [Docker — With Creds](docker/) | Enabled (pre-seeded) | Memory | Local dev |
| [Docker Compose — No Auth](docker-compose/) | Disabled | Memory | Local dev |
| [Docker Compose — With Creds](docker-compose/) | Enabled (pre-seeded) | Memory | Local dev |
| [Docker Compose — SQLite](docker-compose/) | Enabled (pre-seeded) | SQLite (persistent) | Local dev |
| [K8s — No Auth](kubernetes/) | Disabled | Memory | Staging |
| [K8s — With Creds](kubernetes/) | Enabled (pre-seeded) | Memory | Staging |
| [K8s — Persistent](kubernetes/) | Enabled (pre-seeded) | SQLite + PVC | Prod |

## Choosing an Example

### For Local Development
- **Quickest start:** [Docker Compose — No Auth](docker-compose/docker-compose.no-auth.yml)
- **With realistic auth:** [Docker Compose — With Creds](docker-compose/docker-compose.with-creds.yml)
- **With persistent storage:** [Docker Compose — SQLite](docker-compose/docker-compose.sqlite.yml)
- **From source:** [Run with Bazel](commands/run-with-bazel.sh)

### For Kubernetes
- **Dev cluster:** [K8s — No Auth](kubernetes/k8s.no-auth.yaml)
- **Staging with auth:** [K8s — With Creds](kubernetes/k8s.with-creds.yaml)
- **Production with persistence:** [K8s — Persistent](kubernetes/k8s.persistent.yaml)
- **Helm chart:** See [`deploy/helm/`](../deploy/helm/) for the production Helm chart

### For Testing with AWS CLI
- [Quick Start commands](commands/quick-start.sh) — Basic queue operations
- [FIFO + DLQ commands](commands/fifo-and-dlq.sh) — FIFO ordering and dead-letter queues

## Credentials

Examples that use initial credentials use AWS documentation example values:

| Field | Value |
|-------|-------|
| AccessKeyId | `AKIAIOSFODNN7EXAMPLE` |
| SecretAccessKey | `wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY` |

**Replace these with your own credentials in any real deployment.** Never commit real credentials to source control.

## Ports

All examples expose the same ports:

| Port | Service | URL |
|------|---------|-----|
| 9324 | SQS API | `http://localhost:9324` |
| 9325 | Web UI | `http://localhost:9325` |
| 9326 | Metrics | `http://localhost:9326/metrics` |
| 8001 | Health | `http://localhost:8001/health` (non-local only) |

## Related Documentation

- [Configuration Reference](../docs/configuration.md)
- [API Reference](../docs/api-reference.md)
- [Architecture](../docs/architecture.md)
- [Helm Chart](../deploy/helm/)
