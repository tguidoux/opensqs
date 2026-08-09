# OpenSQS Documentation

OpenSQS is an open-source, self-hosted AWS SQS-compatible message queue server written in Go.

## Documentation Index

| Document | Description |
|----------|-------------|
| [Architecture](architecture.md) | Server architecture, startup flow, request lifecycle, and design decisions |
| [API Reference](api-reference.md) | All implemented SQS API actions, parameters, and error codes |
| [Queue Library](queue-library.md) | Using the queue engine as a Go library (QueueManager, Queue, Store) |
| [Configuration](configuration.md) | Server configuration file format and options |
| [Protocol Layer](protocol.md) | Wire protocol support (Query and JSON), parsing, and marshalling |
| [Shared Packages](shared-packages.md) | Cross-cutting packages: config, environment, logger |
| [Benchmarks](benchmarks.md) | Performance benchmarks for MemoryStore, SQLiteStore, and handler pipeline |
| [RFC-001](rfc-001-opensqs-server.md) | Original design specification and RFC |

## Quick Start

### Run the Server

```bash
bazel run //apps/go/server:opensqs-server
```

The server starts on `http://localhost:9324`.

When the Web UI is enabled (default for local), it starts on `http://localhost:9325`.

When metrics are enabled (default for local), they are available at `http://localhost:9326/metrics`.

Health checks are available at `http://localhost:8001/health` in non-local environments.

### Run the Example

```bash
# Basic example (create, send, receive, delete)
bazel run //apps/go/playground/sqs_example:sqs_example

# Phase 2 example (message attributes, batch ops, tagging, SetQueueAttributes)
bazel run //apps/go/playground/sqs_phase2_example:sqs_phase2_example

# FIFO queue example (message groups, deduplication, ordered receiving)
bazel run //apps/go/playground/sqs_fifo_example:sqs_fifo_example

# Dead-letter queue example (redrive policy, maxReceiveCount, message redrive)
bazel run //apps/go/playground/sqs_dlq_example:sqs_dlq_example
```

### Using with AWS CLI

```bash
aws --endpoint-url http://localhost:9324 sqs create-queue --queue-name my-queue
aws --endpoint-url http://localhost:9324 sqs send-message --queue-url http://localhost:9324/123456789012/my-queue --message-body "Hello"
aws --endpoint-url http://localhost:9324 sqs receive-message --queue-url http://localhost:9324/123456789012/my-queue
```

### Using as a Go Library

```go
import (
    "github.com/tguidoux/opensqs/pkgs/v1/queue"
    "github.com/tguidoux/opensqs/pkgs/v1/queue/store/memory"
)

// Create a store factory (memory store by default)
factory := func(queueName string, visibilityTimeout int, serverSecret []byte, cfg store.StoreConfig) store.Store {
    return memory.NewMemoryStore(queueName, visibilityTimeout, serverSecret, cfg)
}

manager := queue.NewQueueManager("localhost:9324", "123456789012", "us-east-1", []byte("my-secret"), factory)
q, _ := manager.CreateQueue("my-queue", nil)
```

## Features

- **SQS-compatible API** — 23 actions implemented:
  - Queue management: `CreateQueue`, `DeleteQueue`, `GetQueueUrl`, `ListQueues`, `PurgeQueue`
  - Message operations: `SendMessage`, `ReceiveMessage`, `DeleteMessage`, `ChangeMessageVisibility`
  - Queue attributes: `GetQueueAttributes`, `SetQueueAttributes`
  - Batch operations: `SendMessageBatch`, `DeleteMessageBatch`, `ChangeMessageVisibilityBatch`
  - Queue tagging: `TagQueue`, `UntagQueue`, `ListQueueTags`
  - Permissions: `AddPermission`, `RemovePermission` (stubbed)
  - Dead-letter queues: `ListDeadLetterSourceQueues`
  - Message move tasks: `StartMessageMoveTask`, `CancelMessageMoveTask`, `ListMessageMoveTasks`
- **FIFO queues** — `.fifo` suffix queues with message groups, deduplication (content-based or explicit), and sequence numbers
- **Dead-letter queues** — `RedrivePolicy` with `maxReceiveCount`, automatic message redrive after threshold
- **Message move tasks** — Background migration of messages from DLQ back to source or custom destination, with rate limiting and cancellation
- **Message attributes** — String, Number, and Binary types with MD5 checksums
- **Message system attributes** — `AWSTraceHeader` and other system-level attributes with MD5 checksums
- **Dual protocol support** — AWS Query Protocol (XML) and JSON Protocol 1.0 simultaneously
- **Long polling** — `WaitTimeSeconds` support with notification-based blocking (MemoryStore) or poll-loop (SQLite/BadgerDB)
- **Visibility timeout** — Automatic message re-visibility via `time.AfterFunc` timers (MemoryStore) or lazy evaluation (SQLiteStore/BadgerStore)
- **Auto-create queues** — Optionally auto-create queues on first access (configurable)
- **Pluggable storage** — `Store` interface with `MemoryStore`, `SQLiteStore`, and `BadgerStore` implementations
- **BadgerDB persistence** — Durable message storage with `BadgerStore` using BadgerDB v4
- **SQLite persistence** — Durable message storage with `SQLiteStore` using pure-Go `modernc.org/sqlite` driver
- **Store factory pattern** — `StoreFactory` type for custom store implementations
- **Signed receipt handles** — HMAC-SHA256 signed, preventing forgery
- **Startup queues** — Define queues in config to auto-create at boot
- **Request logging middleware** — Structured per-request logging with request IDs
- **Rate limiting middleware** — Global or per-queue token bucket rate limiting (`golang.org/x/time/rate`)
- **TLS support** — Per-server TLS (SQS API, UI, metrics, health) with configurable certificates
- **Prometheus metrics** — Message counts, queue sizes, API request rates/latency, move task metrics
- **Web UI** — Built-in dashboard for browsing queues, viewing messages, sending/purging, and creating queues (including FIFO) at `http://localhost:9325`
- **Configurable health checks** — Port configurable, non-local only, Kubernetes-ready
- **Graceful shutdown** — SIGINT/SIGTERM with 10s drain
- **Helm chart** — Production-ready Kubernetes deployment with ConfigMap, PVC, Ingress, HPA, PDB
- **Docker support** — Multi-arch (arm64/amd64) distroless images

## Project Structure

```
opensqs/
├── apps/go/
│   ├── server/              # SQS server application
│   │   ├── main.go          # Entry point
│   │   ├── config.go        # ServerConfig
│   │   ├── config.yaml      # Local dev config
│   │   ├── request_handler.go
│   │   ├── error_response.go
│   │   ├── startup_queues.go
│   │   ├── handlers/        # Action dispatcher + adapters
│   │   ├── protocol/        # Wire protocol parsers + marshallers
│   │   ├── middleware/      # Request logger, rate limiter, middleware chain
│   │   ├── tls/             # TLS config loader
│   │   ├── health/          # Health check server
│   │   ├── metrics/         # Prometheus metrics server
│   │   ├── ui/              # Web UI server (templates, static assets, handlers)
│   │   └── integration/     # Integration tests
│   └── playground/
│       ├── sqs_example/             # Basic example (create, send, receive, delete)
│       ├── sqs_phase2_example/      # Phase 2 example (attributes, batch, tagging)
│       ├── sqs_fifo_example/        # FIFO queue example (groups, dedup, ordering)
│       └── sqs_dlq_example/         # Dead-letter queue example (redrive policy)
├── pkgs/v1/
│   ├── queue/               # Queue engine library
│   │   ├── types/           # Message, SQSError, constants
│   │   ├── dlq/             # RedrivePolicy parsing, MoveTaskManager
│   │   └── store/
│   │       ├── store.go     # Store interface, StoreFactory, StoreConfig
│   │       ├── memory/      # In-memory store (FIFO, DLQ, visibility timers)
│   │       ├── sqlite/      # SQLite persistent store
│   │       └── badger/      # BadgerDB persistent store
│   ├── config/              # Config loading
│   ├── environment/         # Environment enum
│   └── logger/             # Structured logging
├── deploy/
│   └── helm/                # Helm chart (deployment, service, configmap, PVC, ingress, HPA, PDB)
├── tools/                   # Bazel rules and tools
└── docs/                    # This documentation
```
