# Queue Library

The `pkgs/v1/queue` package provides the core queue engine used by the OpenSQS server. It can also be used as a standalone Go library for embedded queue functionality.

## Package Structure

```
pkgs/v1/queue/
├── queue.go          # Queue struct and methods
├── manager.go        # QueueManager for multi-queue management
├── attributes.go     # QueueAttributes and defaults
├── limits.go         # Limits validation (strict/relaxed)
├── errors.go         # SQS error types and factories
├── BUILD.bazel
├── dlq/
│   ├── redrive.go    # RedrivePolicy struct and parser
│   └── move_task.go  # MoveTaskManager for message move tasks
├── types/
│   ├── types.go      # Message, MessageAttribute, SQSError interface, ReceiptHandleInfo
│   └── constants.go  # SQS version, action names, attribute names, default/max limits
├── store/
│   ├── store.go      # Store interface, StoreFactory, StoreConfig, RedriveFunc
│   ├── memory/
│   │   └── memory.go # In-memory store (FIFO, DLQ, visibility timers)
│   ├── sqlite/
│   │   └── sqlite.go # SQLite persistent store
│   └── badger/
│       └── badger.go # BadgerDB persistent store
└── tests/
    ├── queue_test.go
    ├── manager_test.go
    └── ...
```

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/tguidoux/opensqs/pkgs/v1/queue"
    "github.com/tguidoux/opensqs/pkgs/v1/queue/store"
    "github.com/tguidoux/opensqs/pkgs/v1/queue/store/memory"
    "github.com/tguidoux/opensqs/pkgs/v1/queue/types"
)

func main() {
    // Create a store factory — determines which Store implementation is used.
    // Use memory.NewMemoryStore for in-memory, or sqlite.NewSQLiteStore for persistence.
    factory := func(queueName string, visibilityTimeout int, serverSecret []byte, cfg store.StoreConfig) store.Store {
        return memory.NewMemoryStore(queueName, visibilityTimeout, serverSecret, cfg)
    }

    // Create a queue manager
    manager := queue.NewQueueManager(
        "localhost:9324",      // nodeAddress
        "123456789012",        // accountID
        "us-east-1",           // region
        []byte("my-secret-key"), // serverSecret (for receipt handle signing)
        factory,               // storeFactory
    )

    // Create a queue
    q, err := manager.CreateQueue("orders", nil)
    if err != nil {
        panic(err)
    }

    // Send a message
    msg := &types.Message{
        MessageID: "msg-001",
        Body:      `{"orderId":"12345","total":42.99,"currency":"USD"}`,
    }
    err = q.Store().SendMessage(ctx, msg, 0)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Sent: %s\n", msg.MessageID)

    // Receive messages
    msgs, err := q.ReceiveMessages(1, 30, 5) // maxMessages, visibilityTimeout, waitTime
    if err != nil {
        panic(err)
    }
    for _, m := range msgs {
        fmt.Printf("Received: %s\n", m.Body)
        // Delete after processing
        q.DeleteMessage(m.ReceiptHandle)
    }

    // Cleanup
    manager.DeleteQueue("orders")
}
```

## Phase 2 Features

### Message Attributes

Messages can carry typed metadata via `MessageAttributes`. Supported types: `String`, `Number`, and `Binary`.

```go
msg := &types.Message{
    MessageID: "msg-001",
    Body:      `{"event":"order.created"}`,
    MessageAttributes: map[string]types.MessageAttribute{
        "ContentType": {DataType: "String", StringValue: "application/json"},
        "Priority":    {DataType: "Number", StringValue: "5"},
        "TraceId":     {DataType: "Binary", BinaryValue: []byte("trace-abc")},
    },
}
q.Store().SendMessage(ctx, msg, 0)

// When received, attributes are available on the message
received, _ := q.Store().ReceiveMessages(ctx, 1, 30, 0)
for name, attr := range received[0].MessageAttributes {
    fmt.Printf("  %s: %s (type: %s)\n", name, attr.StringValue, attr.DataType)
}
```

### SetQueueAttributes

Queue attributes can be modified at runtime via `SetAttribute`:

```go
// Change visibility timeout to 60 seconds
q.Attributes().SetAttribute(types.AttributeVisibilityTimeout, "60")

// Enable long polling by default
q.Attributes().SetAttribute(types.AttributeReceiveMessageWaitTimeSeconds, "10")

// Read it back
val, _ := q.Attributes().GetAttribute(types.AttributeVisibilityTimeout)
fmt.Println(val) // "60"
```

### Queue Tagging

Tags are key-value metadata stored directly on the queue:

```go
// Add tags
q.Tags()["Environment"] = "production"
q.Tags()["Team"] = "payments"

// List tags
for k, v := range q.Tags() {
    fmt.Printf("%s = %s\n", k, v)
}

// Remove a tag
delete(q.Tags(), "Environment")
```

### Batch Operations

Batch operations send, delete, or change visibility for up to 10 messages in a single logical operation. At the library level, iterate over entries and call the store methods:

```go
// SendMessageBatch — send multiple messages
messages := []*types.Message{
    {MessageID: "batch-001", Body: `{"item":"A"}`},
    {MessageID: "batch-002", Body: `{"item":"B"}`},
    {MessageID: "batch-003", Body: `{"item":"C"}`},
}
for _, m := range messages {
    q.Store().SendMessage(ctx, m, 0)
}

// DeleteMessageBatch — receive then delete all
received, _ := q.Store().ReceiveMessages(ctx, 10, 30, 0)
for _, m := range received {
    q.Store().DeleteMessage(ctx, m.ReceiptHandle)
}

// ChangeMessageVisibilityBatch — make messages visible again
for _, m := range received {
    q.Store().ChangeMessageVisibility(ctx, m.ReceiptHandle, 0)
}
```

> **See also:** `apps/go/playground/sqs_phase2_example/` for a complete runnable example of all Phase 2 features.

### FIFO Queues

FIFO (First-In-First-Out) queues guarantee message ordering within a message group and provide deduplication. Queue names must end with `.fifo`.

```go
// Create a FIFO queue with content-based deduplication
fifoAttrs := queue.NewDefaultQueueAttributes()
fifoAttrs.FifoQueue = true
fifoAttrs.ContentBasedDeduplication = true

q, _ := manager.CreateQueue("orders.fifo", fifoAttrs)

// Send messages with explicit group and dedup IDs
msg1 := &types.Message{
    MessageID:             "msg-001",
    Body:                  `{"order":"A"}`,
    MessageGroupID:        "group-1",
    MessageDeduplicationID: "dedup-A",
}
msg2 := &types.Message{
    MessageID:             "msg-002",
    Body:                  `{"order":"B"}`,
    MessageGroupID:        "group-1",
    MessageDeduplicationID: "dedup-B",
}
q.Store().SendMessage(ctx, msg1, 0)
q.Store().SendMessage(ctx, msg2, 0)

// Messages within the same group are delivered in order
received, _ := q.Store().ReceiveMessages(ctx, 10, 30, 0)
// received[0].Body == `{"order":"A"}` (sent first)
// received[0].SequenceNumber is set for FIFO messages
```

**FIFO behavior:**
- Only one message from a group is in-flight at a time (next message in group becomes visible only after the previous one is deleted or visibility expires)
- `MessageDeduplicationId` is required unless `ContentBasedDeduplication` is enabled (in which case the MD5 of the body is used)
- Deduplication window is 5 minutes — duplicate IDs within the window are silently dropped
- `SequenceNumber` is assigned to every FIFO message

### Dead-Letter Queues (DLQ)

A dead-letter queue receives messages that exceed `maxReceiveCount` receive attempts. Configure via `RedrivePolicy` attribute:

```go
// 1. Create the dead-letter queue
dlq, _ := manager.CreateQueue("failed-orders", nil)
dlqArn := dlq.ARN("us-east-1", "123456789012")

// 2. Create the main queue with a redrive policy
mainAttrs := queue.NewDefaultQueueAttributes()
mainAttrs.RedrivePolicy = fmt.Sprintf(
    `{"deadLetterTargetArn":"%s","maxReceiveCount":"3"}`,
    dlqArn,
)
mainQueue, _ := manager.CreateQueue("orders", mainAttrs)

// Messages that are received 3 times without being deleted
// are automatically moved to the DLQ
```

**DLQ behavior:**
- When a message's `ApproximateReceiveCount` exceeds `maxReceiveCount`, the store calls the `RedriveFunc` instead of making the message visible again
- The `RedriveFunc` is set up automatically by `QueueManager` based on the `RedrivePolicy` attribute
- Use `ListDeadLetterSourceQueues` to find all queues that redrive to a specific DLQ

### Message System Attributes

System attributes (e.g., `AWSTraceHeader`) are metadata attached to messages, separate from user-defined message attributes:

```go
msg := &types.Message{
    MessageID:  "msg-001",
    Body:       "hello",
    SystemAttributes: map[string]types.MessageSystemAttribute{
        "AWSTraceHeader": {
            DataType:    "String",
            StringValue: "Root=1-5759e988-bd862e3fe1be46a994272793;Sampled=1",
        },
    },
}
q.Store().SendMessage(ctx, msg, 0)

// When received, system attributes are available on the message
received, _ := q.Store().ReceiveMessages(ctx, 1, 30, 0)
traceHeader := received[0].SystemAttributes["AWSTraceHeader"]
```

## QueueManager

`QueueManager` manages multiple queues and provides queue-level operations.

### Creation

```go
manager := queue.NewQueueManager(
    nodeAddress   string,          // e.g. "localhost:9324"
    accountID     string,          // e.g. "123456789012"
    region        string,          // e.g. "us-east-1"
    serverSecret  []byte,          // HMAC signing key for receipt handles
    storeFactory  store.StoreFactory, // determines Store implementation
)
```

The `storeFactory` is a function that creates a new `Store` for each queue. It receives the queue name, visibility timeout, server secret, and a `StoreConfig` containing FIFO/DLQ settings derived from queue attributes.

### Methods

| Method | Description |
|--------|-------------|
| `CreateQueue(name, attrs, tags)` | Creates a queue. Idempotent if name+attrs match existing queue. Returns error if attrs differ. |
| `DeleteQueue(name)` | Deletes a queue and all its messages. |
| `LookupQueue(name)` | Returns `*Queue` or error if not found. |
| `LookupQueueByURL(url)` | Resolves a queue URL to `*Queue`. |
| `ListQueues()` | Returns `[]*Queue` (all queues). |
| `ListQueueURLs()` | Returns `[]string` (all queue URLs). |
| `PurgeQueue(name)` | Deletes all messages from a queue. |
| `QueueURL(name)` | Returns the URL for a queue name. |
| `NodeAddress()` | Returns the node address. |
| `AccountID()` | Returns the account ID. |
| `Region()` | Returns the AWS region. |

### Queue URL Format

```
http://{nodeAddress}/{accountID}/{queueName}
```

### Queue ARN Format

```
arn:aws:sqs:{region}:{accountID}:{queueName}
```

## Queue

`Queue` represents a single message queue.

### Methods

| Method | Description |
|--------|-------------|
| `Name()` | Returns the queue name. |
| `Attributes()` | Returns `*QueueAttributes`. |
| `Store()` | Returns the underlying `store.Store`. |
| `Tags()` | Returns queue tags as `map[string]string`. |
| `SetTags(tags)` | Sets queue tags. |
| `IsFifo()` | Returns `true` if the queue is a FIFO queue. |
| `URL()` | Returns the queue URL. |
| `ARN()` | Returns the queue ARN. |
| `GetAttribute(name)` | Returns a single attribute value. |
| `ApproximateNumberOfMessages()` | Available message count. |
| `ApproximateNumberOfMessagesNotVisible()` | In-flight message count. |
| `ApproximateNumberOfMessagesDelayed()` | Delayed message count. |

## QueueAttributes

`QueueAttributes` holds all configurable queue properties.

### Default Values

| Attribute | Default |
|-----------|---------|
| `VisibilityTimeout` | 30 seconds |
| `MaximumMessageSize` | 262,144 bytes (256 KB) |
| `MessageRetentionPeriod` | 345,600 seconds (4 days) |
| `DelaySeconds` | 0 |
| `ReceiveMessageWaitTimeSeconds` | 0 |
| `FifoQueue` | false |
| `ContentBasedDeduplication` | false |
| `SqsManagedSseEnabled` | true |

### Methods

| Method | Description |
|--------|-------------|
| `NewDefaultQueueAttributes()` | Returns attributes with SQS defaults. |
| `GetAttribute(name)` | Returns value for a named attribute. |
| `SetAttribute(name, value)` | Sets a named attribute. |
| `AllAttributes()` | Returns all attributes as `map[string]string`. |
| `AllAttributeNames()` | Returns all attribute name constants. |

## Limits

`Limits` validates SQS parameter constraints. Two modes are available:

- **StrictMode** (default) — Enforces all SQS limits strictly
- **RelaxedMode** — Allows values beyond SQS limits (useful for testing)

### Validation Methods

| Method | Validates |
|--------|-----------|
| `VerifyMessageSize(size)` | ≤ 256 KB |
| `VerifyBatchSize(count)` | ≤ 10 entries |
| `VerifyVisibilityTimeout(seconds)` | 0–43,200 |
| `VerifyDelaySeconds(seconds)` | 0–900 |
| `VerifyReceiveMessageWaitTime(seconds)` | 0–20 |
| `VerifyMaxNumberOfMessages(count)` | 1–10 |
| `VerifyMessageRetentionPeriod(seconds)` | 60–1,209,600 |
| `VerifyMaximumMessageSize(bytes)` | ≤ 262,144 |
| `VerifyQueueName(name)` | Valid SQS queue name |
| `VerifyDeduplicationId(id)` | Non-empty, ≤ 128 chars |
| `VerifyMessageGroupId(id)` | Non-empty, ≤ 128 chars |

## Store Interface

The `store.Store` interface defines the storage contract:

```go
type Store interface {
    SendMessage(ctx context.Context, msg *types.Message, delaySeconds int) error
    ReceiveMessages(ctx context.Context, maxMessages int, visibilityTimeout int, waitTimeSeconds int) ([]*types.Message, error)
    DeleteMessage(ctx context.Context, receiptHandle string) error
    ChangeMessageVisibility(ctx context.Context, receiptHandle string, visibilityTimeout int) error
    ApproximateNumberOfMessages() int
    ApproximateNumberOfMessagesNotVisible() int
    ApproximateNumberOfMessagesDelayed() int
    Purge(ctx context.Context) error
    Close() error
}
```

### StoreFactory

`StoreFactory` is a function type that creates a new `Store` for each queue. It receives the queue name, visibility timeout, server secret, and a `StoreConfig` containing FIFO/DLQ settings:

```go
type StoreFactory func(queueName string, visibilityTimeout int, serverSecret []byte, attrs StoreConfig) Store

type StoreConfig struct {
    IsFifo                    bool
    ContentBasedDeduplication bool
    MaxReceiveCount           int
    RedriveFunc               RedriveFunc
}
```

### MemoryStore

The default in-memory implementation:

```go
store := memory.NewMemoryStore(
    queueName         string,
    visibilityTimeout int,
    serverSecret      []byte,
    cfg               store.StoreConfig,
)
```

**Features:**
- Long polling via notification channel
- Visibility timeout via `time.AfterFunc` timers
- HMAC-SHA256 signed receipt handles
- Thread-safe (mutex-protected)
- FIFO support: message groups, deduplication cache, sequence numbers
- DLQ support: redrive callback when `maxReceiveCount` is exceeded
- `store.Now` is overridable for time-dependent tests

### SQLiteStore

A persistent storage implementation backed by SQLite:

```go
import (
    "database/sql"
    "github.com/tguidoux/opensqs/pkgs/v1/queue/store/sqlite"
    _ "modernc.org/sqlite" // pure-Go SQLite driver
)

db, err := sql.Open("sqlite", "/data/opensqs.db")
store, err := sqlite.NewSQLiteStore(
    db,
    queueName         string,
    visibilityTimeout int,
    serverSecret      []byte,
    cfg               store.StoreConfig,
)
```

**Features:**
- Durable persistence across server restarts
- Lazy visibility timeout evaluation (no goroutines)
- FIFO support: message groups, deduplication, sequence numbers
- DLQ support: redrive callback when `maxReceiveCount` is exceeded
- Thread-safe via SQL transactions
- Multiple queues can share the same database file

**Factory pattern for SQLite:**

```go
db, _ := sql.Open("sqlite", "/data/opensqs.db")
factory := func(queueName string, visibilityTimeout int, serverSecret []byte, cfg store.StoreConfig) store.Store {
    s, _ := sqlite.NewSQLiteStore(db, queueName, visibilityTimeout, serverSecret, cfg)
    return s
}
manager := queue.NewQueueManager("localhost:9324", "123456789012", "us-east-1", []byte("secret"), factory)
```

### Receipt Handle Format

Receipt handles are base64-encoded JSON containing:
```json
{
  "data": "<base64-encoded ReceiptHandleInfo JSON>",
  "signature": "<HMAC-SHA256 signature>"
}
```

`ReceiptHandleInfo` contains:
- `QueueName` — Name of the queue
- `MessageID` — Unique message identifier
- `ReceiveTimestamp` — Unix timestamp of receive
- `RandomNonce` — 8 random hex bytes

### BadgerStore

A persistent storage implementation backed by BadgerDB v4:

```go
import (
    "github.com/dgraph-io/badger/v4"
    "github.com/tguidoux/opensqs/pkgs/v1/queue/store/badger"
)

db, err := badger.Open(badger.DefaultOptions("/data/badger"))
store, err := badger.NewBadgerStore(
    db,
    queueName         string,
    visibilityTimeout int,
    serverSecret      []byte,
    cfg               store.StoreConfig,
)
```

**Features:**
- Durable persistence across server restarts
- Lazy visibility timeout evaluation (no goroutines)
- Iterator-based scanning with prefix filtering
- FIFO support: message groups, deduplication, sequence numbers
- DLQ support: redrive callback when `maxReceiveCount` is exceeded
- Thread-safe via BadgerDB transactions
- Multiple queues can share the same BadgerDB instance
- Long polling via poll-loop with configurable interval

**Factory pattern for BadgerDB:**

```go
db, _ := badger.Open(badger.DefaultOptions("/data/badger"))
factory := func(queueName string, visibilityTimeout int, serverSecret []byte, cfg store.StoreConfig) store.Store {
    s, _ := badger.NewBadgerStore(db, queueName, visibilityTimeout, serverSecret, cfg)
    return s
}
manager := queue.NewQueueManager("localhost:9324", "123456789012", "us-east-1", []byte("secret"), factory)
```

### Message Move Tasks

The `MoveTaskManager` (in `pkgs/v1/queue/dlq/move_task.go`) manages background message migration from a source queue (typically a DLQ) to a destination queue. It supports:

- **Auto-discovery**: If `DestinationArn` is omitted, the manager finds a queue whose `RedrivePolicy` points to the source.
- **Rate limiting**: `MaxNumberOfMessagesPerSecond` controls the move rate (0 = unlimited).
- **Cancellation**: Tasks can be cancelled mid-flight via `CancelMessageMoveTask`.
- **Status tracking**: Tasks transition through `RUNNING` → `COMPLETED`/`CANCELLED`/`FAILED`.

```go
import "github.com/tguidoux/opensqs/pkgs/v1/queue/dlq"

mtm := dlq.NewMoveTaskManager(manager)
handle, err := mtm.Start(sourceArn, destinationArn, maxRate)
// ... later ...
err = mtm.Cancel(handle)
tasks := mtm.List(sourceArn)
```

## Message Type

```go
type Message struct {
    MessageID                        string
    ReceiptHandle                    string
    MD5OfBody                        string
    MD5OfMessageAttributes           string
    Body                             string
    MessageAttributes                map[string]MessageAttribute
    Attributes                       map[string]string
    SystemAttributes                 map[string]MessageSystemAttribute
    SentTimestamp                    time.Time
    ReceivedTimestamp                time.Time
    FirstReceivedTimestamp           time.Time
    VisibilityDeadline               time.Time
    IsVisible                        bool
    ApproximateReceiveCount          int
    ApproximateFirstReceiveTimestamp time.Time
    SequenceNumber                   string
    MessageDeduplicationID           string
    MessageGroupID                   string
}

type MessageAttribute struct {
    DataType    string   // "String", "Number", or "Binary"
    StringValue string   // Used for String and Number types
    BinaryValue []byte   // Used for Binary type
}
```

## Error Types

All errors implement the `SQSError` interface:

```go
type SQSError interface {
    error
    Code() string
    HTTPStatusCode() int
    ErrorType() string  // "Sender" or "Receiver"
    Message() string
}
```

### Error Factory Functions

| Function | Error Code |
|----------|------------|
| `NewInvalidAction(action)` | `InvalidAction` |
| `NewQueueDoesNotExist(name)` | `AWS.SimpleQueueService.NonExistentQueue` |
| `NewInvalidParameterValue(msg)` | `InvalidParameterValue` |
| `NewInvalidAttributeName(name)` | `InvalidAttributeName` |
| `NewInvalidQueryParameter(name)` | `InvalidQueryParameter` |
| `NewMissingParameter(name)` | `MissingParameter` |
| `NewQueueNameExists(name)` | `QueueAlreadyExists` |
| `NewTooManyEntriesInBatchRequest()` | `AWS.SimpleQueueService.TooManyEntriesInBatch` |
| `NewBatchEntryIdsNotDistinct()` | `AWS.SimpleQueueService.BatchEntryIdsNotDistinct` |
| `NewInvalidMessageContents(msg)` | `InvalidMessageContents` |
| `NewReceiptHandleIsInvalid()` | `ReceiptHandleIsInvalid` |
| `NewOverLimit()` | `OverLimit` |
| `NewInternalError(msg)` | `InternalError` |
| `NewInvalidMessageID()` | `InvalidMessageId` |
| `NewUnsupportedOperation()` | `UnsupportedOperation` |
