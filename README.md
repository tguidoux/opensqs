# OpenSQS

[![CI](https://github.com/tguidoux/opensqs/actions/workflows/ci.yml/badge.svg)](https://github.com/tguidoux/opensqs/actions/workflows/ci.yml)
[![Code Quality](https://github.com/tguidoux/opensqs/actions/workflows/code-quality.yml/badge.svg)](https://github.com/tguidoux/opensqs/actions/workflows/code-quality.yml)
[![Go Version](https://img.shields.io/badge/Go-1.25.5-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](https://github.com/tguidoux/opensqs/pulls)
[![GitHub Release](https://img.shields.io/github/v/release/tguidoux/opensqs)](https://github.com/tguidoux/opensqs/releases)

**An open-source, self-hosted alternative to AWS SQS.** Built in Go, compatible with the AWS SQS API, and designed to be lightweight, fast, and easy to run anywhere.

## Why OpenSQS?

AWS SQS is a fantastic managed service, but it locks you into AWS pricing, limits your control over data residency, and adds network latency for workloads running outside AWS. OpenSQS gives you the same SQS API you already know, running on your own infrastructure.

**Purpose:** Provide a drop-in SQS-compatible message queue that teams can self-host — for development, testing, air-gapped environments, or cost optimization.

**Motivation:**
- **Local development:** Test SQS-dependent code without AWS credentials or network calls
- **Air-gapped environments:** Run message queues in networks without internet access
- **Cost control:** Avoid per-request SQS pricing for high-throughput workloads
- **Data sovereignty:** Keep message data in your own infrastructure
- **Zero vendor lock-in:** Standard SQS API means no client-side changes when migrating

## Features

- **SQS-compatible API** — Supports both the Query Protocol (XML, form-urlencoded) and JSON Protocol 1.0
- **Standard queues** — At-least-once delivery with visibility timeouts
- **Message operations** — Send, receive, delete, change visibility, purge
- **Queue management** — Create, delete, list, get/set attributes
- **In-memory storage** — Fast, zero-dependency message store (pluggable backend)
- **HMAC receipt handles** — Tamper-proof receipt handles signed with a server secret
- **Strict limits** — Configurable message size, queue depth, and rate limiting
- **Health checks** — Built-in `/health` endpoint for Kubernetes probes
- **Graceful shutdown** — Clean in-flight message handling on SIGINT/SIGTERM
- **Distroless containers** — Multi-arch (arm64/amd64) images for security

## Quickstart

### Prerequisites

- [Bazelisk](https://github.com/bazelbuild/bazelisk) (manages Bazel versions automatically):
  ```bash
  brew install bazelisk  # macOS
  ```

### Run the Server

```bash
git clone https://github.com/tguidoux/opensqs.git
cd opensqs
bazel run //:clean          # Initialize workspace
bazel run //apps/go/server:opensqs-server
```

The server starts on `http://localhost:9324`.

### Test with AWS CLI

```bash
# Point AWS CLI at your local OpenSQS server
export AWS_ENDPOINT_URL=http://localhost:9324
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=us-east-1

# Create a queue
aws sqs create-queue --queue-name my-queue

# Send a message
aws sqs send-message --queue-url http://localhost:9324/123456789012/my-queue --message-body "Hello OpenSQS!"

# Receive the message
aws sqs receive-message --queue-url http://localhost:9324/123456789012/my-queue

# Delete the message (use the receipt handle from receive)
aws sqs delete-message --queue-url http://localhost:9324/123456789012/my-queue --receipt-handle "<receipt-handle>"

# List queues
aws sqs list-queues
```

### Run in Docker

```bash
bazel run //apps/go/server:opensqs_server_image_platform_transition_load_docker
docker run -p 9324:9324 opensqs_server_image
```

### Deploy with Helm

```bash
# Add the OpenSQS Helm chart
helm install opensqs deploy/helm \
  --set image.tag=v0.0.7 \
  --set sqs.serverSecret="your-secret-key"

# Or use a values file
helm install opensqs deploy/helm -f my-values.yaml
```

See [`deploy/helm/values.yaml`](deploy/helm/values.yaml) for all configurable options.

## Examples

The [`examples/`](examples/) directory contains ready-to-use configurations for various deployment scenarios:

| Example | Auth | Storage | Environment |
|---------|------|---------|-------------|
| [Docker — No Auth](examples/docker/) | Disabled | Memory | Local dev |
| [Docker — With Creds](examples/docker/) | Pre-seeded | Memory | Local dev |
| [Docker Compose — No Auth](examples/docker-compose/) | Disabled | Memory | Local dev |
| [Docker Compose — With Creds](examples/docker-compose/) | Pre-seeded | Memory | Local dev |
| [Docker Compose — SQLite](examples/docker-compose/) | Pre-seeded | SQLite (persistent) | Local dev |
| [K8s — No Auth](examples/kubernetes/) | Disabled | Memory | Staging |
| [K8s — With Creds](examples/kubernetes/) | Pre-seeded | Memory | Staging |
| [K8s — Persistent](examples/kubernetes/) | Pre-seeded | SQLite + PVC | Prod |

Each example includes its own README with setup instructions. See [`examples/README.md`](examples/README.md) for the full overview.

Quick start with Docker Compose:

```bash
# No authentication (simplest)
docker compose -f examples/docker-compose/docker-compose.no-auth.yml up

# With pre-seeded credentials
docker compose -f examples/docker-compose/docker-compose.with-creds.yml up
```

AWS CLI command examples are also available in [`examples/commands/`](examples/commands/):

```bash
# Basic queue operations (create, send, receive, delete)
./examples/commands/quick-start.sh

# FIFO queues + dead-letter queues
./examples/commands/fifo-and-dlq.sh
```

## Example Program

A complete Go example demonstrating the queue library API is in [`apps/go/playground/sqs_example`](apps/go/playground/sqs_example/main.go):

```bash
bazel run //apps/go/playground/sqs_example:sqs_example
```

This example shows how to:
- Create a `QueueManager` and queue
- Send and receive messages with attributes
- Delete messages after processing
- List queues, purge, and clean up

## Development

### Project Structure

```
opensqs/
├── apps/
│   └── go/
│       ├── server/              # SQS server (HTTP API, handlers, protocol)
│       │   ├── main.go          # Entry point
│       │   ├── config.go        # ServerConfig struct
│       │   ├── config.yaml      # Local dev configuration
│       │   ├── handlers/        # Request handlers & dispatch
│       │   ├── protocol/        # Query & JSON protocol parsers
│       │   └── health/          # Health check server
│       └── playground/          # Example programs
│           ├── sqs_example/     # Queue library usage example
│           ├── sqs_dlq_example/ # Dead-letter queue example
│           ├── sqs_fifo_example/ # FIFO queue example
│           └── sqs_phase2_example/ # Phase 2 features example
├── pkgs/v1/
│   ├── config/                  # YAML config loading with validation
│   ├── environment/             # Environment enum (LOCAL, STAGING, PROD)
│   ├── logger/                  # Structured logging
│   └── queue/                   # Queue engine, manager, store, types
│       ├── queue.go             # Queue struct & operations
│       ├── manager.go           # QueueManager (lifecycle, lookup, list)
│       ├── limits.go            # SQS limits enforcement
│       ├── attributes.go        # Queue attributes
│       ├── store/               # Pluggable storage interface
│       │   └── memory/          # In-memory store with HMAC receipts
│       └── types/                # Message, attributes, constants
├── deploy/
│   └── helm/                    # Helm chart (deployment, service, configmap, PVC, ingress, HPA, PDB)
├── examples/                   # Deployment examples (Docker, Docker Compose, Kubernetes, CLI commands)
│   ├── docker/                 # Dockerfile examples (with/without auth)
│   ├── docker-compose/         # Docker Compose examples (memory, SQLite)
│   ├── kubernetes/             # Kubernetes manifests (with/without auth, persistent)
│   └── commands/               # AWS CLI command scripts (quick-start, FIFO+DLQ, Bazel)
└── tools/                       # Custom Bazel rules & dev tools
```

### Common Commands

```bash
# Workspace cleanup (format + regenerate)
bazel run //:clean

# Go-specific: update BUILD files and dependencies
bazel run //:go.clean

# Bazel formatting
bazel run //:bazel.clean

# Regenerate BUILD files after adding Go files
bazel run //:gazelle

# Build everything
bazel build //apps/go/...

# Run all tests
bazel test //...

# Run the server
bazel run //apps/go/server:opensqs-server

# Build and load Docker image
bazel run //apps/go/server:opensqs_server_image_platform_transition_load_docker
```

### Configuration

The server reads `config.yaml` by default (override with `CONFIG_PATH` env var):

```yaml
server:
  host: "0.0.0.0"
  port: 9324

sqs:
  nodeAddress: "localhost:9324"
  accountId: "123456789012"
  region: "us-east-1"
  storageType: "memory"
  strictLimits: true
  serverSecret: "dev-only-not-for-production"

log:
  level: "info"

environment: "local"
```

Environments: `LOCAL`, `STAGING`, `PROD`. Health checks run on port 8001 for non-local environments.

### Startup Queues

You can pre-create queues at server startup by listing them under the `queues` key in `config.yaml`:

```yaml
queues:
  - name: "orders"
    attributes:
      visibilityTimeout: 60
      receiveMessageWaitTimeSeconds: 5
  - name: "notifications"
  - name: "dead-letter.fifo"
    attributes:
      fifoQueue: true
      contentBasedDeduplication: true
      visibilityTimeout: 120
```

Any attributes you omit default to standard SQS values. Available attributes:

| Attribute | Type | Default |
|-----------|------|---------|
| `visibilityTimeout` | int | 30 |
| `delaySeconds` | int | 0 |
| `maximumMessageSize` | int | 262144 |
| `messageRetentionPeriod` | int | 345600 |
| `receiveMessageWaitTimeSeconds` | int | 0 |
| `fifoQueue` | bool | false |
| `contentBasedDeduplication` | bool | false |

### Adding Go Dependencies

1. Add the import to your Go code
2. Add the dependency to `go.mod`
3. Run `bazel run //:go.clean`
4. Follow any buildozer command prompts

### Testing

Tests live in `tests/` subfolders within each package:

```bash
# Run all tests
bazel test //...

# Run specific package tests
bazel test //pkgs/v1/queue/tests:go_default_test
bazel test //apps/go/server/handlers/tests:go_default_test
```

## Shared Libraries

| Package | Description |
|---------|-------------|
| `pkgs/v1/config/` | YAML config loading with schema validation |
| `pkgs/v1/environment/` | Environment enum (LOCAL, STAGING, PROD) |
| `pkgs/v1/logger/` | Structured logging (contextual & uncontextual) |
| `pkgs/v1/queue/` | Queue engine, manager, in-memory store, types |

## Build System

OpenSQS uses **Bazel** with custom `opensqs_go_*` rules for hermetic, reproducible builds. All toolchains, dependencies, and formatters are managed through Bazel — no "works on my machine" problems.

## Contributing

Contributions are welcome! Please read our [Contributing Guide](CONTRIBUTING.md) to get started.

- 📋 [Contributing Guide](CONTRIBUTING.md) — Development setup, conventions, and PR process
- 🤝 [Code of Conduct](CODE_OF_CONDUCT.md) — Community standards
- 🔒 [Security Policy](SECURITY.md) — How to report vulnerabilities
- 📝 [Changelog](CHANGELOG.md) — Release history
- 💬 [Discussions](https://github.com/tguidoux/opensqs/discussions) — Questions and ideas
- 🐛 [Issues](https://github.com/tguidoux/opensqs/issues) — Bug reports and feature requests

### Quick Start for Contributors

1. **Fork** the repository
2. **Clone** your fork: `git clone https://github.com/<your-username>/opensqs.git`
3. **Create** a feature branch: `git checkout -b my-feature`
4. **Make** your changes, following the existing code style and patterns
5. **Test** your changes: `bazel test //...`
6. **Commit** with a clear message
7. **Push** and open a Pull Request

See [CONTRIBUTING.md](CONTRIBUTING.md) for detailed guidelines.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for the full license text.
