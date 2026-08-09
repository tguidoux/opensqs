# RFC: OpenSQS — Open-Source AWS SQS-Compatible Message Queue Server

**Author:** Theo Guidoux  
**Date:** 2026-08-09  
**Status:** Draft  
**Version:** 0.1.0

---

## 1. Abstract

OpenSQS is an open-source, lightweight, AWS SQS-compatible message queue server written in Go. It enables developers to run a fully SQS-compatible server locally or in production by simply pointing their existing AWS SDK clients to a different endpoint URL — no code changes required.

Inspired by [ElasticMQ](https://github.com/softwaremill/elasticmq) (Scala/Pekko), OpenSQS reimplements the same concept in Go for faster startup, lower memory footprint, simpler deployment as a single static binary, and a modern built-in web UI.

---

## 2. Motivation

### 2.1 Problem Statement

AWS SQS is the de facto standard for message queuing in cloud-native applications. However:

- **Local development** requires either a real AWS account (cost, network latency, shared state) or a mock that doesn't behave like SQS.
- **Testing** in CI/CD pipelines needs a deterministic, fast, self-contained SQS-compatible server.
- **Air-gapped / on-prem environments** cannot reach AWS endpoints but still need SQS semantics.
- **Existing solutions** like ElasticMQ are JVM-based (slow startup, high memory), require a separate UI process, and are not trivially embeddable in Go-based infrastructure.

### 2.2 Goals

OpenSQS aims to be:

| Goal | Description |
|------|-------------|
| **Fully SQS-compatible** | Any application using the AWS SDK (Go, Java, Python, Node.js, etc.) works by changing only the endpoint URL. |
| **Lightweight** | Single static binary, < 20MB, < 50MB RAM at idle. Sub-second startup. |
| **Easy to start** | `docker run opensqs/server` or download binary and run. Zero-config defaults. |
| **Web UI included** | Built-in dashboard for browsing queues, sending/receiving messages, viewing attributes — no separate process. |
| **Production-ready** | Persistence, metrics, graceful shutdown, health checks. Suitable for on-prem deployments. |

### 2.3 Non-Goals

- We are **not** building a distributed, replicated, multi-node message broker. OpenSQS is a single-node server (like ElasticMQ). Clustering is a future possibility.
- We are **not** reimplementing the full AWS authentication/signature stack. Credentials are accepted but not validated (any value works), matching ElasticMQ's behavior for local/dev use.
- We are **not** building SNS, Kinesis, or other AWS service compatibility.

---

## 3. Background: AWS SQS

### 3.1 What SQS Is

Amazon Simple Queue Service (SQS) is a fully managed message queuing service that enables decoupling and scaling of microservices, distributed systems, and serverless applications.

**Core workflow:**
1. **Producers** send messages to an SQS queue.
2. SQS stores messages durably.
3. **Consumers** poll the queue for messages.
4. Upon receiving, a message becomes **invisible** (visibility timeout).
5. Consumer processes the message and **deletes** it from the queue.
6. If not deleted within the visibility timeout, the message becomes visible again for redelivery.

### 3.2 Queue Types

| Feature | Standard Queue | FIFO Queue |
|---------|---------------|------------|
| Delivery guarantee | At-least-once | Exactly-once |
| Ordering | Best-effort | Strictly preserved per message group |
| Throughput | Nearly unlimited | ~3,000 msg/s (with batching) |
| Duplicates | Possible | Prevented via deduplication |
| Name suffix | (none) | `.fifo` |
| Use case | High-throughput background jobs | Transactional workflows, payments |

### 3.3 Key Concepts

- **Visibility Timeout** — Period a received message is hidden from other consumers (default: 30s, range: 0–43,200s).
- **Dead-Letter Queue (DLQ)** — Queue that receives messages after `maxReceiveCount` failed processing attempts.
- **Long Polling** — `ReceiveMessage` with `WaitTimeSeconds` > 0 blocks until messages arrive (up to 20s), reducing empty responses.
- **Message Attributes** — Structured metadata attached to messages (String, Number, Binary types).
- **Delay Seconds** — Messages can be delivered with a delay (per-message or per-queue).
- **Redrive Policy** — JSON configuration linking a queue to its DLQ.
- **Message Deduplication** — FIFO queues deduplicate messages within a 5-minute window using `MessageDeduplicationId` or content hash.
- **Message Groups** — FIFO queues use `MessageGroupId` to order messages within a group.

### 3.4 Queue Attributes

| Attribute | Description | Default |
|-----------|-------------|---------|
| `ApproximateNumberOfMessages` | Messages available for retrieval | — |
| `ApproximateNumberOfMessagesNotVisible` | Messages in flight | — |
| `ApproximateNumberOfMessagesDelayed` | Messages awaiting delivery after delay | — |
| `CreatedTimestamp` | Queue creation time | — |
| `LastModifiedTimestamp` | Last modification time | — |
| `VisibilityTimeout` | Visibility timeout (seconds) | 30 |
| `MaximumMessageSize` | Max message body size (bytes) | 262,144 (256 KB) |
| `MessageRetentionPeriod` | How long messages are kept (seconds) | 345,600 (4 days) |
| `DelaySeconds` | Default delay for messages (seconds) | 0 |
| `ReceiveMessageWaitTimeSeconds` | Default long polling wait (seconds) | 0 |
| `Policy` | Queue access policy (JSON) | — |
| `RedrivePolicy` | DLQ configuration (JSON) | — |
| `RedriveAllowPolicy` | Controls which source queues can use this as DLQ | — |
| `KmsMasterKeyId` | KMS key ID for encryption | — |
| `KmsDataKeyReusePeriodSeconds` | KMS data key reuse period | 300 |
| `FifoQueue` | Whether queue is FIFO | false |
| `ContentBasedDeduplication` | Enable content-based dedup for FIFO | false |
| `DeduplicationScope` | Dedup scope (messageGroup / queue) | — |
| `FifoThroughputLimit` | Throughput limit (perQueue / perMessageGroupId) | — |
| `QueueArn` | Queue ARN | — |

---

## 4. SQS API Compatibility

### 4.1 Supported Protocols

AWS SQS supports two wire protocols. OpenSQS will implement both:

| Protocol | Content-Type | Description |
|----------|-------------|-------------|
| **AWS Query Protocol** | `application/x-www-form-urlencoded` | Legacy protocol. Parameters in form body or query string. Responses in XML. Used by `aws-sdk-go-v1`, `boto3` (default), etc. |
| **AWS JSON Protocol 1.0** | `application/x-amz-json-1.0` | Newer protocol. Parameters in JSON body. Responses in JSON. Used by `aws-sdk-go-v2`, `aws-sdk-js-v3`, etc. |

The protocol is determined by the `Content-Type` / `X-Amz-Target` headers:
- `application/x-www-form-urlencoded` → Query Protocol (XML response)
- `application/x-amz-json-1.0` → JSON Protocol (JSON response)

### 4.2 API Actions

OpenSQS will implement the complete SQS API action set:

#### 4.2.1 Queue Management

| Action | Description | Key Parameters |
|--------|-------------|----------------|
| `CreateQueue` | Create a new queue | `QueueName`, `Attributes`, `tags` |
| `DeleteQueue` | Delete a queue and all its messages | `QueueUrl` |
| `GetQueueUrl` | Get the URL of a queue by name | `QueueName`, `QueueOwnerAWSAccountId` |
| `ListQueues` | List all queues (optionally filtered by prefix) | `QueueNamePrefix`, `MaxResults`, `NextToken` |
| `PurgeQueue` | Delete all messages from a queue | `QueueUrl` |
| `GetQueueAttributes` | Get queue attributes | `QueueUrl`, `AttributeNames` |
| `SetQueueAttributes` | Set queue attributes | `QueueUrl`, `Attributes` |
| `ListDeadLetterSourceQueues` | List queues using this queue as DLQ | `QueueUrl`, `MaxResults`, `NextToken` |

#### 4.2.2 Message Operations

| Action | Description | Key Parameters |
|--------|-------------|----------------|
| `SendMessage` | Send a single message | `QueueUrl`, `MessageBody`, `DelaySeconds`, `MessageAttributes`, `MessageSystemAttributes`, `MessageDeduplicationId`, `MessageGroupId` |
| `SendMessageBatch` | Send up to 10 messages | `QueueUrl`, `Entries[]` |
| `ReceiveMessage` | Retrieve messages from a queue | `QueueUrl`, `MaxNumberOfMessages`, `VisibilityTimeout`, `WaitTimeSeconds`, `AttributeNames`, `MessageAttributeNames`, `MessageSystemAttributeNames`, `ReceiveRequestAttemptId` |
| `DeleteMessage` | Delete a received message | `QueueUrl`, `ReceiptHandle` |
| `DeleteMessageBatch` | Delete up to 10 messages | `QueueUrl`, `Entries[]` |
| `ChangeMessageVisibility` | Change visibility timeout of a message | `QueueUrl`, `ReceiptHandle`, `VisibilityTimeout` |
| `ChangeMessageVisibilityBatch` | Change visibility for up to 10 messages | `QueueUrl`, `Entries[]` |

#### 4.2.3 Tagging

| Action | Description | Key Parameters |
|--------|-------------|----------------|
| `TagQueue` | Add tags to a queue | `QueueUrl`, `Tags` |
| `UntagQueue` | Remove tags from a queue | `QueueUrl`, `TagKeys` |
| `ListQueueTags` | List all tags for a queue | `QueueUrl` |

#### 4.2.4 Permissions (Stubbed)

| Action | Description | Key Parameters |
|--------|-------------|----------------|
| `AddPermission` | Add a permission to a queue | `QueueUrl`, `Label`, `AWSAccountIds`, `Actions` |
| `RemovePermission` | Remove a permission | `QueueUrl`, `Label` |

> **Note:** Permission actions will be accepted and return success but will not enforce access control. This matches ElasticMQ's behavior and is sufficient for local/dev use.

#### 4.2.5 Message Move Tasks

| Action | Description | Key Parameters |
|--------|-------------|----------------|
| `StartMessageMoveTask` | Start moving messages from DLQ to another queue | `SourceArn`, `DestinationArn`, `MaxNumberOfMessagesPerSecond` |
| `CancelMessageMoveTask` | Cancel a running message move task | `TaskHandle` |
| `ListMessageMoveTasks` | List message move tasks for a source queue | `SourceArn`, `MaxResults` |

### 4.3 Queue URL Format

Queue URLs follow the AWS format:

```
http://{host}:{port}/{accountId}/{queueName}
```

Example:
```
http://localhost:9324/000000000000/my-queue
http://localhost:9324/000000000000/orders.fifo
```

The `accountId` defaults to `000000000000` and is configurable. The host/port is derived from the server's configured `node-address`.

### 4.4 Error Handling

Errors follow the SQS error response format:

**Query Protocol (XML):**
```xml
<ErrorResponse>
  <Error>
    <Type>Sender</Type>
    <Code>QueueDoesNotExist</Code>
    <Message>The specified queue does not exist.</Message>
    <Detail/>
  </Error>
  <RequestId>00000000-0000-0000-0000-000000000000</RequestId>
</ErrorResponse>
```

**JSON Protocol:**
```json
{
  "__type": "com.amazonaws.sqs#QueueDoesNotExist",
  "message": "The specified queue does not exist."
}
```

**Standard error codes:**

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `InvalidAction` | 400 | Unknown or missing action |
| `MissingAction` | 400 | No action parameter |
| `InvalidParameterValue` | 400 | Invalid parameter value |
| `MissingParameter` | 400 | Required parameter missing |
| `QueueAlreadyExists` | 400 | Queue name already in use with different attributes |
| `QueueDoesNotExist` | 400 | Queue not found |
| `QueueDeletedRecently` | 400 | Queue was deleted within 60s |
| `MessageNotInFlight` | 400 | Receipt handle expired or invalid |
| `ReceiptHandleIsInvalid` | 400 | Invalid receipt handle |
| `InvalidMessageContents` | 400 | Message body invalid |
| `BatchRequestTooLong` | 400 | Batch payload exceeds limit |
| `BatchEntryIdsNotDistinct` | 400 | Duplicate batch entry IDs |
| `TooManyEntriesInBatchRequest` | 400 | More than 10 entries in batch |
| `InvalidAttributeName` | 400 | Unknown attribute name |
| `InvalidAttributeValue` | 400 | Invalid attribute value |
| `InvalidIdFormat` | 400 | Invalid batch entry ID format |
| `PurgeQueueInProgress` | 400 | Purge already in progress |
| `OverLimit` | 403 | Too many queues or operations |

---

## 5. Architecture

### 5.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────┐
│                      OpenSQS Server                       │
│                                                          │
│  ┌─────────────┐  ┌──────────────┐  ┌───────────────┐  │
│  │  SQS REST   │  │   Web UI      │  │  Health Check │  │
│  │  API Layer  │  │   (Embedded)   │  │   Server      │  │
│  │  (Port 9324)│  │  (Port 9325)  │  │  (Port 8001)  │  │
│  └──────┬──────┘  └──────┬───────┘  └───────────────┘  │
│         │                │                               │
│  ┌──────┴────────────────┴──────┐                       │
│  │       Core Queue Engine       │                       │
│  │  ┌─────────┐  ┌────────────┐  │                       │
│  │  │ Queue   │  │  Message   │  │                       │
│  │  │ Manager │  │  Store     │  │                       │
│  │  └────┬────┘  └─────┬──────┘  │                       │
│  │       │              │         │                       │
│  │  ┌────┴──────────────┴────┐   │                       │
│  │  │   Persistence Layer    │   │                       │
│  │  │  (In-Memory / SQLite)  │   │                       │
│  │  └────────────────────────┘   │                       │
│  └──────────────────────────────┘                       │
│                                                          │
│  ┌──────────────────────────────────────────────────┐   │
│  │              Shared Libraries (pkgs/v1/)          │   │
│  │  config  │  environment  │  logger  │  random_name │   │
│  └──────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

### 5.2 Component Breakdown

#### 5.2.1 SQS REST API Layer (`apps/go/server/`)

The HTTP-facing layer that handles SQS protocol parsing and response marshalling.

**Responsibilities:**
- Parse incoming requests (Query Protocol + JSON Protocol)
- Route to the correct action handler
- Validate parameters
- Marshal responses (XML or JSON based on protocol)
- Generate receipt handles and message IDs
- Handle error responses in the correct format

**Technology:** Huma v2 with chi router (per monorepo conventions).

**Request flow:**
```
HTTP Request
  → Content-Type detection (Query vs JSON protocol)
  → Action extraction (Action param or X-Amz-Target header)
  → Parameter parsing into typed request struct
  → Action handler dispatch
  → Core engine call
  → Response marshalling (XML or JSON)
  → HTTP Response
```

#### 5.2.2 Core Queue Engine (`pkgs/v1/queue/`)

The heart of OpenSQS — manages queues, messages, and all SQS semantics.

**Sub-packages:**

| Package | Responsibility |
|---------|---------------|
| `pkgs/v1/queue/` | Queue manager — create, delete, lookup queues |
| `pkgs/v1/queue/message/` | Message types, receipt handles, deduplication |
| `pkgs/v1/queue/attributes/` | Queue attribute definitions, validation, defaults |
| `pkgs/v1/queue/fifo/` | FIFO-specific logic — message groups, sequencing, dedup |
| `pkgs/v1/queue/visibility/` | Visibility timeout tracking and expiry |
| `pkgs/v1/queue/dlq/` | Dead-letter queue redrive logic |
| `pkgs/v1/queue/errors/` | SQS error types and codes |

**Queue Manager:**
```go
type QueueManager interface {
    CreateQueue(name string, attrs QueueAttributes) (*Queue, error)
    DeleteQueue(name string) error
    LookupQueue(name string) (*Queue, bool)
    ListQueues(prefix string) []string
    PurgeQueue(name string) error
}
```

**Queue:**
```go
type Queue struct {
    Name       string
    Attributes QueueAttributes
    Tags       map[string]string
    // Internal message store
    messages  MessageStore
    // FIFO-specific state
    fifoState *FifoState
}
```

**Message Store interface:**
```go
type MessageStore interface {
    SendMessage(msg Message) error
    ReceiveMessages(max int, waitTime time.Duration, visibilityTimeout time.Duration) []Message
    DeleteMessage(receiptHandle string) error
    ChangeVisibility(receiptHandle string, timeout time.Duration) error
    ApproximateCount() (visible, inFlight, delayed int)
}
```

#### 5.2.3 Persistence Layer (`pkgs/v1/queue/store/`)

Pluggable storage backends:

| Backend | Use Case | Status |
|---------|----------|--------|
| **In-Memory** | Default, fastest, dev/testing | Phase 1 |
| **SQLite** | Production single-node, persistence across restarts | Phase 2 |
| **BadgerDB** | High-performance embedded KV store (alternative) | Future |

```go
type Store interface {
    MessageStore  // Embeds message operations
    QueueStore     // Queue metadata persistence
    Close() error
}
```

#### 5.2.4 Web UI (`apps/go/server/ui/`)

A built-in web dashboard served from the same binary.

**Features:**
- List all queues with message counts (visible, in-flight, delayed)
- Create / delete queues with attribute configuration
- View queue details and attributes
- Send messages (with attributes, FIFO parameters)
- Receive and delete messages
- Purge queues
- Real-time auto-refresh (configurable)
- Dark/light theme

**Technology:** Server-side rendered HTML + minimal JavaScript (no heavy SPA framework). The UI makes SQS API calls to the same server process via the SQS REST API, ensuring it always reflects the actual server state.

**Design philosophy:** The UI is a convenience layer — it uses the same SQS API as any external client. This means:
- No separate API endpoints for the UI
- The UI works against any SQS-compatible server (including real AWS)
- UI can be disabled via config for headless deployments

#### 5.2.5 Health Check Server (`apps/go/server/health/`)

Per monorepo conventions, a separate health check server on port 8001 for non-local environments.

```
GET /health → 200 OK
```

### 5.3 Configuration

Following the monorepo's `config.ConfigI[T]` pattern:

```yaml
# config.yaml (local)
server:
  host: "0.0.0.0"
  port: 9324
  ui:
    enabled: true
    port: 9325

node:
  protocol: "http"
  host: "localhost"
  port: 9324
  context-path: ""

aws:
  region: "us-east-1"
  account-id: "000000000000"

storage:
  type: "memory"  # "memory" | "sqlite"
  sqlite:
    path: "/data/opensqs.db"

queues:
  auto-create: false
  # Pre-create queues on startup
  # - name: "my-queue"
  #   attributes:
  #     VisibilityTimeout: "30"
  #     DelaySeconds: "0"

limits:
  mode: "strict"  # "strict" | "relaxed"

logging:
  level: "info"
```

**Config struct:**
```go
type ServerConfig struct {
    config.ConfigI[ServerConfig]
    Server   ServerConfig   `yaml:"server"`
    Node     NodeConfig     `yaml:"node"`
    AWS     AWSConfig      `yaml:"aws"`
    Storage StorageConfig  `yaml:"storage"`
    Queues  QueuesConfig   `yaml:"queues"`
    Limits  LimitsConfig  `yaml:"limits"`
    Logging LoggingConfig  `yaml:"logging"`
}
```

### 5.4 SQS Limits

Two modes matching ElasticMQ's approach:

| Mode | Behavior |
|------|----------|
| `strict` | Enforce all AWS SQS limits (message size, batch size, attribute count, etc.) |
| `relaxed` | Allow exceeding limits (useful for testing) |

**Key limits (strict mode):**

| Limit | Value |
|-------|-------|
| Maximum message size | 262,144 bytes (256 KB) |
| Maximum batch entries | 10 |
| Maximum message attributes | 10 |
| Queue name length | 1–80 chars |
| FIFO queue name suffix | `.fifo` |
| Visibility timeout range | 0–43,200 seconds |
| Message retention period | 60–1,209,600 seconds |
| Delay seconds range | 0–900 seconds |
| Receive wait time | 0–20 seconds |
| Max receive messages | 1–10 |

---

## 6. Project Structure

Following the monorepo conventions:

```
opensqs/
├── apps/go/
│   ├── server/                          # Main OpenSQS server binary
│   │   ├── main.go                      # Entry point, config loading, server bootstrap
│   │   ├── config.yaml                  # Local development config
│   │   ├── values.staging.yaml          # Staging config
│   │   ├── values.prod.yaml             # Production config
│   │   ├── BUILD.bazel
│   │   ├── handlers/                    # SQS API action handlers
│   │   │   ├── create_queue.go
│   │   │   ├── delete_queue.go
│   │   │   ├── get_queue_url.go
│   │   │   ├── list_queues.go
│   │   │   ├── purge_queue.go
│   │   │   ├── send_message.go
│   │   │   ├── send_message_batch.go
│   │   │   ├── receive_message.go
│   │   │   ├── delete_message.go
│   │   │   ├── delete_message_batch.go
│   │   │   ├── change_message_visibility.go
│   │   │   ├── change_message_visibility_batch.go
│   │   │   ├── get_queue_attributes.go
│   │   │   ├── set_queue_attributes.go
│   │   │   ├── tag_queue.go
│   │   │   ├── untag_queue.go
│   │   │   ├── list_queue_tags.go
│   │   │   ├── add_permission.go
│   │   │   ├── remove_permission.go
│   │   │   ├── list_dead_letter_source_queues.go
│   │   │   ├── start_message_move_task.go
│   │   │   ├── cancel_message_move_task.go
│   │   │   ├── list_message_move_tasks.go
│   │   │   └── handler.go               # Router, protocol detection, dispatch
│   │   ├── protocol/                    # Wire protocol handling
│   │   │   ├── query.go                 # AWS Query Protocol parser (form-urlencoded → XML)
│   │   │   ├── json.go                  # AWS JSON Protocol parser (JSON → JSON)
│   │   │   ├── marshal.go              # Response marshalling (XML/JSON)
│   │   │   └── errors.go               # SQS error response formatting
│   │   ├── ui/                          # Embedded web UI
│   │   │   ├── handlers.go             # UI HTTP handlers
│   │   │   ├── templates/              # HTML templates
│   │   │   └── static/                 # CSS, JS assets
│   │   ├── health/                     # Health check server
│   │   │   └── server.go
│   │   └── tests/
│   │       ├── BUILD.bazel
│   │       └── server_test.go
│   └── playground/                     # Existing playground
│       └── cmd_hello_world/
│
├── pkgs/v1/
│   ├── config/                          # Existing — config loading
│   ├── environment/                     # Existing — env enum
│   ├── logger/                          # Existing — structured logging
│   ├── random_name/                     # Existing — ID generation
│   ├── queue/                           # NEW — Core queue engine
│   │   ├── queue.go                     # Queue type, QueueManager
│   │   ├── manager.go                   # Queue lifecycle management
│   │   ├── attributes.go               # Queue attribute definitions & defaults
│   │   ├── limits.go                    # SQS limit enforcement
│   │   ├── errors.go                    # SQS error types
│   │   ├── BUILD.bazel
│   │   └── tests/
│   │       ├── BUILD.bazel
│   │       ├── queue_test.go
│   │       └── manager_test.go
│   ├── queue/message/                   # NEW — Message types & operations
│   │   ├── message.go                   # Message struct, receipt handles
│   │   ├── attributes.go               # Message attribute types (String, Number, Binary)
│   │   ├── dedup.go                    # Deduplication logic (FIFO)
│   │   ├── sequence.go                 # Sequence number generation
│   │   ├── BUILD.bazel
│   │   └── tests/
│   │       ├── BUILD.bazel
│   │       └── message_test.go
│   ├── queue/store/                     # NEW — Persistence backends
│   │   ├── store.go                    # Store interface
│   │   ├── memory/                     # In-memory store
│   │   │   ├── memory.go
│   │   │   ├── BUILD.bazel
│   │   │   └── tests/
│   │   ├── sqlite/                     # SQLite store
│   │   │   ├── sqlite.go
│   │   │   ├── BUILD.bazel
│   │   │   └── tests/
│   │   └── BUILD.bazel
│   ├── queue/fifo/                      # NEW — FIFO queue logic
│   │   ├── fifo.go                     # Message groups, ordering, dedup
│   │   ├── message_group.go
│   │   ├── BUILD.bazel
│   │   └── tests/
│   ├── queue/visibility/                # NEW — Visibility timeout tracking
│   │   ├── tracker.go                  # In-flight message tracking
│   │   ├── BUILD.bazel
│   │   └── tests/
│   ├── queue/dlq/                       # NEW — Dead-letter queue logic
│   │   ├── redrive.go                  # Message redrive to DLQ
│   │   ├── move_task.go               # Message move task management
│   │   ├── BUILD.bazel
│   │   └── tests/
│   └── queue/types/                    # NEW — Shared types
│       ├── types.go                   # QueueAttributes, MessageAttributes, etc.
│       ├── constants.go               # Attribute name constants, action names
│       └── BUILD.bazel
│
└── tools/                              # Existing — Bazel rules
```

---

## 7. Key Design Decisions

### 7.1 Protocol Support: Both Query and JSON

AWS SDKs use different protocols depending on language and version:
- **aws-sdk-go-v2** (Go): JSON Protocol 1.0
- **aws-sdk-go-v1** (Go): Query Protocol
- **boto3** (Python): Query Protocol
- **aws-sdk-js-v3** (Node.js): JSON Protocol 1.0
- **aws-sdk-java-v2** (Java): JSON Protocol 1.0
- **aws-sdk-java-v1** (Java): Query Protocol

To be truly "just change the URL" compatible, OpenSQS must support **both** protocols simultaneously. The server detects the protocol from the `Content-Type` header and routes accordingly.

### 7.2 Single Binary with Embedded UI

Unlike ElasticMQ (separate UI process), OpenSQS embeds the UI in the server binary using Go's `embed` package. This means:
- One binary to download and run
- No Node.js runtime needed for the UI
- UI assets are compiled into the binary at build time
- UI can be disabled via config for headless deployments

### 7.3 In-Memory First, Persistence Later

Phase 1 ships with in-memory storage only — this covers 90% of local dev/testing use cases. SQLite persistence is Phase 2, enabling production on-prem deployments where data must survive restarts.

### 7.4 No Authentication Enforcement

Like ElasticMQ, OpenSQS accepts any credentials without validation. The `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` values are ignored. This is intentional for local development simplicity. Production deployments should use network-level access control (firewall, VPC, etc.).

### 7.5 Concurrency Model

Go's goroutines and channels map naturally to the SQS message queue model:
- Each queue runs its own goroutine for visibility timeout expiry checking
- Long polling uses Go's `select` with a timer channel
- Message receive is protected by per-queue mutexes to prevent race conditions
- Batch operations are processed sequentially within a queue (matching SQS semantics)

### 7.6 Receipt Handles

Receipt handles are opaque strings that encode:
- Queue name
- Message ID
- Receive timestamp
- A random component for uniqueness

They are base64-encoded and signed with a server secret to prevent forgery. A receipt handle is only valid for the visibility timeout duration of the message it was issued for.

---

## 8. Implementation Phases

### Phase 1: Core SQS Compatibility (MVP)

**Goal:** A working SQS-compatible server that handles the most common operations.

| Item | Description |
|------|-------------|
| Server bootstrap | main.go, config loading, HTTP server, health check |
| Protocol parsing | Query Protocol (form-urlencoded) + JSON Protocol 1.0 |
| Queue management | CreateQueue, DeleteQueue, GetQueueUrl, ListQueues |
| Message operations | SendMessage, ReceiveMessage, DeleteMessage |
| Visibility timeout | Basic visibility timeout tracking |
| In-memory store | All data in memory, lost on restart |
| Error responses | SQS-formatted errors in both XML and JSON |
| Queue URL format | `http://host:port/accountId/queueName` |
| Basic attributes | GetQueueAttributes, SetQueueAttributes (core attributes) |
| Long polling | ReceiveMessage with WaitTimeSeconds |
| Docker image | `opensqs_go_image` target, published to registry |

### Phase 2: Full SQS Compatibility

| Item | Description |
|------|-------------|
| Batch operations | SendMessageBatch, DeleteMessageBatch, ChangeMessageVisibilityBatch |
| ChangeMessageVisibility | Single + batch |
| PurgeQueue | Clear all messages |
| FIFO queues | Message groups, deduplication, sequencing, exactly-once |
| Dead-letter queues | RedrivePolicy, maxReceiveCount, automatic redrive |
| Message attributes | String, Number, Binary types |
| Message system attributes | AWS trace header support |
| Tags | TagQueue, UntagQueue, ListQueueTags |
| ListDeadLetterSourceQueues | Reverse DLQ lookup |
| Permissions (stub) | AddPermission, RemovePermission (no-op) |
| SQS limits | Strict and relaxed modes |
| SQLite persistence | Optional persistence across restarts |
| Auto-create queues | Create queues on first access |
| Pre-create queues | Config-based queue creation on startup |

### Phase 3: Web UI

| Item | Description |
|------|-------------|
| Queue list view | All queues with message counts |
| Queue detail view | Attributes, tags, DLQ info |
| Create queue form | With all attributes, FIFO support |
| Send message form | With attributes, FIFO parameters |
| Receive messages view | Poll, display, delete messages |
| Purge queue action | With confirmation |
| Auto-refresh | Configurable polling interval |
| Dark/light theme | CSS-based, no framework |

### Phase 4: Message Move Tasks & Polish

| Item | Description |
|------|-------------|
| StartMessageMoveTask | Move messages from DLQ to another queue |
| CancelMessageMoveTask | Cancel running move task |
| ListMessageMoveTasks | List move tasks for a source queue |
| Queue persistence config | Persist queue metadata to file |
| Metrics endpoint | Prometheus-compatible metrics |
| Graceful shutdown | Drain in-flight messages on SIGTERM |
| Performance benchmarks | Throughput testing and optimization |

### Phase 5: Production Hardening

| Item | Description |
|------|-------------|
| BadgerDB backend | Alternative high-performance storage |
| TLS support | HTTPS endpoints |
| Rate limiting | Per-queue and global rate limits |
| Request logging | Structured access logs |
| Multi-arch Docker | amd64 + arm64 images |
| Helm chart | Kubernetes deployment |
| Integration test suite | Test against real AWS SDKs (Go, Python, Node.js, Java) |

---

## 9. Client Compatibility

OpenSQS is designed to work with any AWS SDK. The only change required is the endpoint URL:

### Go (aws-sdk-go-v2)
```go
sqsClient := sqs.NewFromConfig(cfg, func(o *sqs.Options) {
    o.BaseEndpoint = aws.String("http://localhost:9324")
    o.Region = "us-east-1"
})
```

### Python (boto3)
```python
sqs = boto3.client(
    'sqs',
    endpoint_url='http://localhost:9324',
    region_name='us-east-1',
    aws_access_key_id='x',
    aws_secret_access_key='x'
)
```

### Node.js (aws-sdk-js-v3)
```javascript
const sqs = new SQSClient({
    endpoint: 'http://localhost:9324',
    region: 'us-east-1',
    credentials: { accessKeyId: 'x', secretAccessKey: 'x' }
});
```

### Java (aws-sdk-java-v2)
```java
SqsClient sqs = SqsClient.builder()
    .endpointOverride(URI.create("http://localhost:9324"))
    .region(Region.US_EAST_1)
    .credentialsProvider(StaticCredentialsProvider.create(
        AwsBasicCredentials.create("x", "x")
    ))
    .build();
```

### AWS CLI

The AWS CLI is installed locally and is the fastest way to interact with OpenSQS during development. It uses the Query Protocol (form-urlencoded → XML), making it ideal for testing that protocol path.

```bash
aws --endpoint-url http://localhost:9324 sqs create-queue --queue-name my-queue
```

Beyond manual testing, the CLI is a valuable **reference tool**:
- `aws sqs help` — Lists all available SQS actions with full parameter documentation
- `aws sqs create-queue help` — Documents exact parameter names, types, and constraints for a specific action
- `aws sqs create-queue --generate-cli-skeleton` — Outputs a JSON template of all input parameters, useful for building Go request structs
- `--output json` / `--output text` — Inspect exact response shapes to verify compatibility

---

## 10. Testing Strategy

### 10.1 Unit Tests

- Each package has its own `tests/` subfolder following monorepo conventions
- Use `testify/assert` for assertions
- Test all SQS action handlers with both protocols
- Test queue engine: message lifecycle, visibility timeout, FIFO ordering, deduplication
- Test error cases: invalid parameters, missing queues, expired receipt handles

### 10.2 Integration Tests

- Test against real AWS SDKs (Go v2, Python boto3, Node.js v3)
- Verify wire-level compatibility: correct XML/JSON response formats
- Test FIFO semantics: ordering, deduplication, exactly-once delivery
- Test DLQ: message redrive after maxReceiveCount
- Test long polling behavior
- Test batch operations

### 10.3 AWS CLI Manual Testing

The AWS CLI is installed locally and serves as the primary manual testing tool during development. It provides:

- **Quick smoke tests** — Verify an action works end-to-end with a single command
- **Response structure verification** — `--output json` or `--output text` to inspect exact response shapes
- **Parameter edge cases** — Test invalid values, missing parameters, and boundary conditions
- **Live API reference** — `aws sqs <action> help` documents exact parameter names and constraints
- **Skeleton generation** — `aws sqs <action> --generate-cli-skeleton` outputs a JSON template for any action's input parameters, useful for building Go request structs

Example manual test workflow:
```bash
# Create queue and capture the URL
QUEUE_URL=$(aws --endpoint-url http://localhost:9324 sqs create-queue \
    --queue-name test-queue --query 'QueueUrl' --output text)

# Send, receive, delete cycle
aws --endpoint-url http://localhost:9324 sqs send-message \
    --queue-url "$QUEUE_URL" --message-body "test"

RECEIPT=$(aws --endpoint-url http://localhost:9324 sqs receive-message \
    --queue-url "$QUEUE_URL" --query 'Messages[0].ReceiptHandle' --output text)

aws --endpoint-url http://localhost:9324 sqs delete-message \
    --queue-url "$QUEUE_URL" --receipt-handle "$RECEIPT"
```

### 10.4 Compatibility Test Suite

A dedicated test suite that runs the same operations against both OpenSQS and real AWS SQS, comparing responses for structural compatibility.

---

## 11. Deployment

### 11.1 Docker

```bash
docker run -p 9324:9324 -p 9325:9325 registry.opensqs.io/server:latest
```

Built using the monorepo's `opensqs_go_image` macro with distroless base image.

### 11.2 Binary

```bash
./opensqs-server --config config.yaml
```

Single static binary, no runtime dependencies.

### 11.3 Kubernetes

Health check on port 8001 (`/health` endpoint) for liveness/readiness probes.

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8001
readinessProbe:
  httpGet:
    path: /health
    port: 8001
```

---

## 12. Open Questions

| # | Question | Status |
|---|----------|--------|
| 1 | Should we support the `X-Amz-Target` header for JSON protocol routing, or rely solely on `Content-Type`? | Resolved: Use `Content-Type` + `X-Amz-Target` (AWS uses both) |
| 2 | Should the UI be a separate Go binary or embedded in the server? | Resolved: Embedded via `go:embed` |
| 3 | SQLite vs BadgerDB for Phase 2 persistence? | Open: Start with SQLite (simpler, more familiar), evaluate BadgerDB later |
| 4 | Should we implement KMS encryption attributes? | Open: Accept but no-op in Phase 1, real encryption in future |
| 5 | Should we support cross-account queue access? | Open: Not in Phase 1, evaluate based on demand |
| 6 | Should the UI use server-side rendering or a SPA? | Resolved: Server-side rendered with minimal JS for simplicity |

---

## 13. References

- [AWS SQS API Reference](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/welcome.html)
- [AWS SQS Developer Guide](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/welcome.html)
- [aws-sdk-go-v2 SQS package](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/sqs)
- [ElasticMQ](https://github.com/softwaremill/elasticmq) — Scala/Pekko SQS-compatible server (primary inspiration)
- [AWS SQS Landing Page](https://aws.amazon.com/sqs/)
- [Mastering AWS Messaging Services — SQS Deep Dive](https://medium.com/@aahana.khanal11/mastering-aws-messaging-services-deep-dive-into-amazon-sqs-simple-queue-service-5256e15d7919)

---

## 14. Glossary

| Term | Definition |
|------|-----------|
| **Visibility Timeout** | Period during which a received message is hidden from other consumers |
| **Receipt Handle** | Opaque token returned by ReceiveMessage, required to delete or change visibility |
| **DLQ** | Dead-Letter Queue — receives messages that exceed maxReceiveCount |
| **FIFO Queue** | First-In-First-Out queue with exactly-once delivery and strict ordering |
| **Message Group** | Subset of messages in a FIFO queue that maintain order |
| **Deduplication ID** | Token used to deduplicate messages in FIFO queues (5-min window) |
| **Long Polling** | ReceiveMessage that blocks until messages are available or timeout |
| **Redrive** | Moving messages from a DLQ back to a source queue or another queue |
| **Query Protocol** | AWS legacy wire protocol using form-urlencoded requests and XML responses |
| **JSON Protocol 1.0** | AWS newer wire protocol using JSON requests and responses |
