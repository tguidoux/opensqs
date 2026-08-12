# SQS API Reference

This document describes the SQS API actions implemented by OpenSQS.

## Supported Protocols

| Protocol | Content-Type | Used By |
|----------|-------------|---------|
| **Query Protocol** | `application/x-www-form-urlencoded` | AWS CLI, boto3 (default), aws-sdk-go-v1 |
| **JSON Protocol 1.0** | `application/x-amz-json-1.0` | aws-sdk-go-v2, aws-sdk-js-v3, aws-sdk-java-v2 |

Protocol is auto-detected from the `X-Amz-Target` header and `Content-Type`.

## Queue URL Format

```
http://{host}:{port}/{accountId}/{queueName}
```

Example: `http://localhost:9324/123456789012/my-queue`

## Implemented Actions

### Queue Management

#### CreateQueue

Creates a new queue. If a queue with the same name already exists and has matching attributes, the existing queue is returned (idempotent). If attributes differ, a `QueueAlreadyExists` error is returned.

**FIFO queues:** Queue name must end with `.fifo`. Set `FifoQueue` attribute to `true`. Optionally set `ContentBasedDeduplication` to `true` for content-based deduplication.

**Dead-letter queues:** Set the `RedrivePolicy` attribute as a JSON string with `deadLetterTargetArn` and `maxReceiveCount`.

**Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `QueueName` | Yes | Queue name (must end with `.fifo` for FIFO queues) |
| `Attribute.N.Name` / `Attribute.N.Value` | No | Queue attributes (e.g., `FifoQueue=true`, `RedrivePolicy`) |

**Common attributes for CreateQueue:**
| Attribute | Description |
|-----------|-------------|
| `FifoQueue` | Set to `true` for FIFO queues |
| `ContentBasedDeduplication` | Enable content-based deduplication (FIFO only) |
| `RedrivePolicy` | JSON: `{"deadLetterTargetArn":"arn:...","maxReceiveCount":"3"}` |
| `VisibilityTimeout` | Visibility timeout in seconds |
| `DelaySeconds` | Default delay for messages |
| `ReceiveMessageWaitTimeSeconds` | Default long polling wait time |

**Query Protocol:**
```
POST /
Action=CreateQueue&QueueName=my-queue
```

**JSON Protocol:**
```
POST /
X-Amz-Target: AmazonSQS.CreateQueue
{"QueueName": "my-queue"}
```

**FIFO queue example (Query Protocol):**
```
POST /
Action=CreateQueue&QueueName=orders.fifo&Attribute.1.Name=FifoQueue&Attribute.1.Value=true&Attribute.2.Name=ContentBasedDeduplication&Attribute.2.Value=true
```

**Response:**
```xml
<CreateQueueResponse>
  <CreateQueueResult>
    <QueueUrl>http://localhost:9324/123456789012/my-queue</QueueUrl>
  </CreateQueueResult>
  <ResponseMetadata>
    <RequestId>00000000-0000-0000-0000-000000000000</RequestId>
  </ResponseMetadata>
</CreateQueueResponse>
```

---

#### DeleteQueue

Deletes a queue and all its messages.

**Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `QueueUrl` | Yes | URL of the queue to delete |

---

#### GetQueueUrl

Returns the URL of an existing queue.

**Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `QueueName` | Yes | Name of the queue |

**Response:** Returns the queue URL.

---

#### ListQueues

Returns a list of all queues, optionally filtered by name prefix.

**Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `QueueNamePrefix` | No | Only return queues starting with this prefix |

**Response:** List of queue URLs.

---

#### PurgeQueue

Deletes all messages from a queue without deleting the queue itself.

**Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `QueueUrl` | Yes | URL of the queue to purge |

---

#### GetQueueAttributes

Returns attribute values for a queue.

**Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `QueueUrl` | Yes | URL of the queue |
| `AttributeName.N` | No | Attribute names to retrieve. Use `All` for all attributes |

**Available attributes:**

| Attribute | Description | Default |
|-----------|-------------|---------|
| `ApproximateNumberOfMessages` | Messages available for retrieval | — |
| `ApproximateNumberOfMessagesNotVisible` | Messages in flight | — |
| `ApproximateNumberOfMessagesDelayed` | Messages awaiting delivery | — |
| `VisibilityTimeout` | Visibility timeout (seconds) | 30 |
| `MaximumMessageSize` | Max body size (bytes) | 262,144 |
| `MessageRetentionPeriod` | How long messages are kept (seconds) | 345,600 |
| `DelaySeconds` | Default delay for messages | 0 |
| `ReceiveMessageWaitTimeSeconds` | Default long polling wait | 0 |
| `QueueArn` | Queue ARN | — |
| `Policy` | Queue access policy | — |
| `RedrivePolicy` | DLQ configuration | — |
| `FifoQueue` | Whether queue is FIFO | false |
| `ContentBasedDeduplication` | Content-based dedup for FIFO | false |
| `KmsMasterKeyId` | KMS key ID | — |
| `KmsDataKeyReusePeriodSeconds` | KMS key reuse period | — |
| `DeduplicationScope` | Dedup scope | — |
| `FifoThroughputLimit` | Throughput limit | — |
| `SqsManagedSseEnabled` | SSE enabled | true |

---

#### SetQueueAttributes

Sets attribute values for a queue. Attributes are applied immediately and persist for the lifetime of the queue.

**Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `QueueUrl` | Yes | URL of the queue |
| `Attribute.N.Name` / `Attribute.N.Value` | Yes | Attribute name-value pairs |

**Settable attributes:**

| Attribute | Valid Range |
|-----------|-------------|
| `VisibilityTimeout` | 0–43,200 seconds |
| `MaximumMessageSize` | 1,024–262,144 bytes |
| `MessageRetentionPeriod` | 60–1,209,600 seconds |
| `DelaySeconds` | 0–900 seconds |
| `ReceiveMessageWaitTimeSeconds` | 0–20 seconds |
| `Policy` | JSON string |
| `RedrivePolicy` | JSON string |
| `FifoQueue` | `true` / `false` |
| `ContentBasedDeduplication` | `true` / `false` |
| `KmsMasterKeyId` | KMS key ID or ARN |
| `KmsDataKeyReusePeriodSeconds` | 60–86,400 seconds |
| `SqsManagedSseEnabled` | `true` / `false` |

**Query Protocol:**
```
POST /
Action=SetQueueAttributes&QueueUrl=http://...&Attribute.1.Name=VisibilityTimeout&Attribute.1.Value=60
```

**JSON Protocol:**
```json
POST /
X-Amz-Target: AmazonSQS.SetQueueAttributes
{"QueueUrl": "http://...", "Attributes": {"VisibilityTimeout": "60"}}
```

Returns `InvalidAttributeName` error if an unknown attribute name is provided.

---

### Message Operations

#### SendMessage

Sends a message to a queue.

**Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `QueueUrl` | Yes | URL of the queue |
| `MessageBody` | Yes | Message body (max 256 KB) |
| `DelaySeconds` | No | Delay before message becomes visible (0–900). Not supported on FIFO queues. |
| `MessageAttribute.N.Name` / `.Value` | No | Message attributes |
| `MessageSystemAttribute.N.Name` / `.Value` | No | Message system attributes (e.g., `AWSTraceHeader`) |
| `MessageDeduplicationId` | No | Deduplication ID (required for FIFO queues without content-based dedup) |
| `MessageGroupId` | No | Message group ID (required for FIFO queues) |

**FIFO queues:** `MessageGroupId` is required. `MessageDeduplicationId` is required unless `ContentBasedDeduplication` is enabled on the queue. Messages within the same group are delivered in order.

**Message Attribute Data Types:**

| Type | Field Used | Example |
|------|------------|---------|
| `String` | `StringValue` | `"application/json"` |
| `Number` | `StringValue` | `"42"` (must be valid number) |
| `Binary` | `BinaryValue` | Base64-encoded bytes |

**Response:** Returns `MessageId`, `MD5OfMessageBody`, `MD5OfMessageAttributes` (when present), `MD5OfMessageSystemAttributes` (when system attributes are present), and `SequenceNumber` (FIFO queues only).

**Query Protocol:**
```
POST /
Action=SendMessage&QueueUrl=http://...&MessageBody=hello
&MessageAttribute.1.Name=Priority&MessageAttribute.1.Value.DataType=Number&MessageAttribute.1.Value.StringValue=5
```

**FIFO example (Query Protocol):**
```
POST /
Action=SendMessage&QueueUrl=http://.../orders.fifo&MessageBody=hello
&MessageGroupId=group-1&MessageDeduplicationId=dedup-1
```

**System attributes example (Query Protocol):**
```
POST /
Action=SendMessage&QueueUrl=http://...&MessageBody=hello
&MessageSystemAttribute.1.Name=AWSTraceHeader&MessageSystemAttribute.1.Value.DataType=String&MessageSystemAttribute.1.Value.StringValue=Root=1-5759e988-bd862e3fe1be46a994272793
```

**JSON Protocol:**
```json
POST /
X-Amz-Target: AmazonSQS.SendMessage
{
  "QueueUrl": "http://...",
  "MessageBody": "hello",
  "MessageAttributes": {
    "Priority": {"DataType": "Number", "StringValue": "5"}
  }
}
```

**FIFO example (JSON Protocol):**
```json
POST /
X-Amz-Target: AmazonSQS.SendMessage
{
  "QueueUrl": "http://.../orders.fifo",
  "MessageBody": "hello",
  "MessageGroupId": "group-1",
  "MessageDeduplicationId": "dedup-1"
}
```

---

#### ReceiveMessage

Retrieves one or more messages from a queue.

**Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `QueueUrl` | Yes | URL of the queue |
| `MaxNumberOfMessages` | No | Max messages to retrieve (1–10, default 1) |
| `VisibilityTimeout` | No | Visibility timeout for received messages |
| `WaitTimeSeconds` | No | Long polling wait time (0–20 seconds) |
| `AttributeName.N` | No | Message attributes to retrieve |
| `MessageAttributeNames.N` | No | Message attribute names to retrieve |

**Response:** Returns messages with `MessageId`, `ReceiptHandle`, `MD5OfBody`, `Body`, and attributes.

**Long polling:** When `WaitTimeSeconds` > 0, the request blocks until messages are available or the wait time expires.

---

#### DeleteMessage

Deletes a previously received message using its receipt handle.

**Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `QueueUrl` | Yes | URL of the queue |
| `ReceiptHandle` | Yes | Receipt handle from ReceiveMessage |

---

#### ChangeMessageVisibility

Changes the visibility timeout of a received message.

**Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `QueueUrl` | Yes | URL of the queue |
| `ReceiptHandle` | Yes | Receipt handle from ReceiveMessage |
| `VisibilityTimeout` | Yes | New visibility timeout in seconds (0–43,200) |

Setting `VisibilityTimeout` to 0 makes the message immediately visible again.

---

### Batch Operations

#### SendMessageBatch

Sends up to 10 messages to a queue in a single request. Each entry is processed independently — successful entries return `MessageId` and `MD5OfMessageBody`, while failed entries return error details.

**Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `QueueUrl` | Yes | URL of the queue |
| `SendMessageBatchRequestEntry.N` | Yes | 1–10 batch entries |

Each entry contains:
| Field | Required | Description |
|-------|----------|-------------|
| `Id` | Yes | Client-side identifier (unique within batch) |
| `MessageBody` | Yes | Message body |
| `DelaySeconds` | No | Per-message delay (0–900). Not supported on FIFO queues. |
| `MessageAttribute.N.Name` / `.Value` | No | Per-message attributes |
| `MessageSystemAttribute.N.Name` / `.Value` | No | Per-message system attributes |
| `MessageDeduplicationId` | No | Dedup ID (FIFO queues without content-based dedup) |
| `MessageGroupId` | No | Message group ID (FIFO queues) |

**Response:** Returns `Successful` (with `Id`, `MessageId`, `MD5OfMessageBody`, `MD5OfMessageAttributes`, `MD5OfMessageSystemAttributes`, `SequenceNumber` for FIFO) and `Failed` (with `Id`, `Code`, `Message`, `SenderFault`) lists.

**Validation:**
- Max 10 entries per batch (`TooManyEntriesInBatch` error if exceeded)
- Entry IDs must be distinct (`BatchEntryIdsNotDistinct` error if duplicated)
- Each entry is validated independently; partial success is possible

---

#### DeleteMessageBatch

Deletes up to 10 messages in a single request using their receipt handles.

**Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `QueueUrl` | Yes | URL of the queue |
| `DeleteMessageBatchRequestEntry.N` | Yes | 1–10 batch entries |

Each entry contains:
| Field | Required | Description |
|-------|----------|-------------|
| `Id` | Yes | Client-side identifier (unique within batch) |
| `ReceiptHandle` | Yes | Receipt handle from `ReceiveMessage` |

**Response:** Returns `Successful` and `Failed` lists.

---

#### ChangeMessageVisibilityBatch

Changes the visibility timeout for up to 10 messages in a single request.

**Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `QueueUrl` | Yes | URL of the queue |
| `ChangeMessageVisibilityBatchRequestEntry.N` | Yes | 1–10 batch entries |

Each entry contains:
| Field | Required | Description |
|-------|----------|-------------|
| `Id` | Yes | Client-side identifier (unique within batch) |
| `ReceiptHandle` | Yes | Receipt handle from `ReceiveMessage` |
| `VisibilityTimeout` | Yes | New visibility timeout (0–43,200) |

**Response:** Returns `Successful` and `Failed` lists.

---

### Queue Tagging

#### TagQueue

Adds or overwrites tags on a queue. Tags are key-value metadata that can be used for cost allocation, access control, or organization.

**Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `QueueUrl` | Yes | URL of the queue |
| `Tag.N.Key` / `Tag.N.Value` | Yes | Tag key-value pairs |

**Query Protocol:**
```
POST /
Action=TagQueue&QueueUrl=http://...&Tag.1.Key=Environment&Tag.1.Value=production
```

**JSON Protocol:**
```json
POST /
X-Amz-Target: AmazonSQS.TagQueue
{"QueueUrl": "http://...", "Tags": {"Environment": "production"}}
```

---

#### UntagQueue

Removes specified tags from a queue.

**Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `QueueUrl` | Yes | URL of the queue |
| `TagKey.N` | Yes | Tag keys to remove |

---

#### ListQueueTags

Returns all tags associated with a queue.

**Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `QueueUrl` | Yes | URL of the queue |

**Response:** Returns a map of tag key-value pairs.

---

### Permission Management

#### AddPermission

Adds a permission to a queue's policy. **Currently a stub** — accepts the request and returns success without enforcing ACLs.

**Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `QueueUrl` | Yes | URL of the queue |
| `Label` | Yes | Unique label for the permission |
| `AWSAccountId.N` | Yes | AWS account IDs to grant access |
| `ActionName.N` | Yes | SQS actions to allow |

---

#### RemovePermission

Removes a permission from a queue's policy. **Currently a stub** — accepts the request and returns success without enforcing ACLs.

**Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `QueueUrl` | Yes | URL of the queue |
| `Label` | Yes | Label of the permission to remove |

---

### Dead-Letter Queues

#### ListDeadLetterSourceQueues

Returns the URLs of all queues that have a `RedrivePolicy` pointing to the specified dead-letter queue.

**Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `QueueUrl` | Yes | URL of the dead-letter queue |

**Response:** Returns a list of queue URLs that use the specified queue as their dead-letter target.

**Query Protocol:**
```
POST /
Action=ListDeadLetterSourceQueues&QueueUrl=http://.../my-dlq
```

**JSON Protocol:**
```json
POST /
X-Amz-Target: AmazonSQS.ListDeadLetterSourceQueues
{"QueueUrl": "http://.../my-dlq"}
```

---

## Message Move Tasks

### StartMessageMoveTask

Starts a background task to move messages from a source queue (typically a DLQ) to a destination queue. If `DestinationArn` is omitted, the server auto-discovers the destination by finding a queue whose `RedrivePolicy` points to the source.

**Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `SourceArn` | Yes | ARN of the source queue (typically the DLQ) |
| `DestinationArn` | No | ARN of the destination queue. Auto-discovered if omitted. |
| `MaxNumberOfMessagesPerSecond` | No | Rate limit for message movement (0 = unlimited) |

**Response:** Returns `TaskHandle` (unique task identifier) and `ApproximateNumberOfMessagesMoved` (initially 0).

**Query Protocol:**
```
POST /
Action=StartMessageMoveTask&SourceArn=arn:aws:sqs:us-east-1:123456789012:my-dlq
```

**JSON Protocol:**
```json
POST /
X-Amz-Target: AmazonSQS.StartMessageMoveTask
{"SourceArn": "arn:aws:sqs:us-east-1:123456789012:my-dlq"}
```

---

#### CancelMessageMoveTask

Cancels a running message move task. Returns the approximate number of messages moved before cancellation.

**Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `TaskHandle` | Yes | Task handle from `StartMessageMoveTask` |

**Response:** Returns `ApproximateNumberOfMessagesMoved`.

---

#### ListMessageMoveTasks

Lists message move tasks, optionally filtered by source ARN.

**Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `SourceArn` | No | Filter tasks by source queue ARN |

**Response:** Returns a list of task objects with `TaskHandle`, `SourceArn`, `DestinationArn`, `Status`, `ApproximateNumberOfMessagesMoved`, and `MaxNumberOfMessagesPerSecond`.

Task statuses: `RUNNING`, `COMPLETED`, `CANCELLING`, `CANCELLED`, `FAILED`.

---

## Error Responses

### Query Protocol (XML)

```xml
<ErrorResponse>
  <Error>
    <Type>Sender</Type>
    <Code>QueueDoesNotExist</Code>
    <Message>The specified queue does not exist.</Message>
  </Error>
  <RequestId>00000000-0000-0000-0000-000000000000</RequestId>
</ErrorResponse>
```

### JSON Protocol

```json
{
  "__type": "com.amazonaws.sqs#QueueDoesNotExist",
  "message": "The specified queue does not exist.",
  "RequestId": "00000000-0000-0000-0000-000000000000"
}
```

### Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `InvalidAction` | 400 | Unknown or missing action |
| `MissingParameter` | 400 | Required parameter missing |
| `InvalidParameterValue` | 400 | Invalid parameter value |
| `InvalidAttributeName` | 400 | Unknown attribute name |
| `InvalidQueryParameter` | 400 | Unknown query parameter |
| `QueueAlreadyExists` | 400 | Queue name in use with different attributes |
| `AWS.SimpleQueueService.NonExistentQueue` | 400 | Queue not found |
| `InvalidMessageContents` | 400 | Message body invalid |
| `ReceiptHandleIsInvalid` | 400 | Invalid receipt handle |
| `AWS.SimpleQueueService.TooManyEntriesInBatch` | 400 | More than 10 batch entries |
| `AWS.SimpleQueueService.BatchEntryIdsNotDistinct` | 400 | Duplicate batch entry IDs |
| `OverLimit` | 403 | Too many queues or operations |
| `InternalError` | 500 | Internal server error |
| `UnsupportedOperation` | 400 | Operation not supported |
| `InvalidParameterValue` | 400 | Invalid FIFO queue name (missing `.fifo` suffix), missing `MessageGroupId`, etc. |

## Server Configuration

### Auto-Create Queues

When `autoCreate` is enabled in the server config, the server automatically creates a queue on first access (SendMessage, ReceiveMessage, etc.) if it doesn't already exist. This is useful for development and testing.

```yaml
queues:
  autoCreate: true
```

### Storage Backends

OpenSQS supports three storage backends, selected via the `storageType` config field:

#### Memory (default)

In-memory storage with `time.AfterFunc` visibility timers. Fastest option, no persistence.

```yaml
sqs:
  storageType: "memory"
```

#### SQLite

Durable persistence using the pure-Go `modernc.org/sqlite` driver (no CGO). Uses lazy visibility timeout evaluation (no goroutines). WAL mode enabled for concurrent reads.

```yaml
sqs:
  storageType: "sqlite"
  sqlitePath: "/data/opensqs.db"
```

The SQLite store supports all features including FIFO queues, dead-letter queues, visibility timeouts, and long polling. Messages persist across server restarts.

#### BadgerDB

Durable persistence using BadgerDB v4 (`dgraph-io/badger/v4`). Lazy visibility timeout evaluation, iterator-based scanning with prefix filtering.

```yaml
sqs:
  storageType: "badger"
  badgerPath: "/data/badger"
```

The BadgerDB store supports all features including FIFO queues, dead-letter queues, visibility timeouts, and long polling. Messages persist across server restarts.

### TLS

Each server (SQS API, UI, metrics, health) can be individually configured for TLS:

```yaml
server:
  tls:
    enabled: true
    certFile: "/path/to/cert.pem"
    keyFile: "/path/to/key.pem"
```

Minimum TLS version is 1.2. When TLS is disabled, the server uses plain HTTP.

### Request Logging

When enabled, logs each HTTP request with request ID, method, path, status code, bytes written, duration, remote address, and user agent.

```yaml
requestLogging:
  enabled: true
```

### Rate Limiting

When enabled, applies token bucket rate limiting (using `golang.org/x/time/rate`). Supports global or per-queue limiting.

```yaml
rateLimit:
  enabled: true
  requestsPerSecond: 1000
  burst: 100
  perQueue: true
```

When rate limited, the server returns `429 Too Many Requests` with a `Retry-After: 1` header.

### Prometheus Metrics

When enabled, exposes Prometheus metrics at `http://localhost:9326/metrics`:

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `opensqs_messages_sent_total` | Counter | `queue` | Total messages sent |
| `opensqs_messages_received_total` | Counter | `queue` | Total messages received |
| `opensqs_messages_deleted_total` | Counter | `queue` | Total messages deleted |
| `opensqs_queue_size` | Gauge | `queue`, `type` | Queue size (available/inflight/delayed) |
| `opensqs_api_requests_total` | Counter | `action`, `protocol` | API request count |
| `opensqs_api_request_duration_seconds` | Histogram | `action`, `protocol` | API request latency |
| `opensqs_move_task_messages_moved_total` | Counter | `source_arn`, `destination_arn` | Messages moved by move tasks |
| `opensqs_move_task_active` | Gauge | — | Active move task count |

```yaml
metrics:
  enabled: true
  port: 9326
```
