# Protocol Layer

This document describes the wire protocol support in OpenSQS, covering request parsing, response marshalling, and error formatting for both AWS SQS protocols.

## Overview

OpenSQS supports two AWS SQS wire protocols simultaneously. The server auto-detects which protocol a client is using and handles both transparently.

```
┌─────────────────────────────────────────────────────────┐
│                    HTTP Request                         │
│                                                         │
│  Query Protocol:          JSON Protocol:               │
│  Content-Type:            X-Amz-Target: AmazonSQS.X    │
│  application/              Content-Type:               │
│  x-www-form-urlencoded     application/x-amz-json-1.0  │
└──────────────┬──────────────────────┬──────────────────┘
               │                      │
       ┌───────▼────────┐   ┌────────▼────────┐
       │ DetectProtocol()│   │                 │
       │ → QueryProtocol │   │ → JSONProtocol  │
       └───────┬────────┘   └────────┬────────┘
               │                      │
       ┌───────▼────────┐   ┌────────▼────────┐
       │ ParseQueryReq()│   │ParseJSONRequest()│
       │ → QueryRequest │   │ → JSONRequest   │
       └───────┬────────┘   └────────┬────────┘
               │                      │
       ┌───────▼────────┐   ┌────────▼────────┐
       │QueryRequestAdpt│   │JSONRequestAdapt │
       │  implements    │   │  implements     │
       │  Request iface │   │  Request iface  │
       └───────┬────────┘   └────────┬────────┘
               │                      │
               └──────────┬───────────┘
                          │
                  ┌───────▼────────┐
                  │ Handler.       │
                  │ HandleRequest()│
                  │ (protocol-     │
                  │  agnostic)     │
                  └───────┬────────┘
                          │
                  ┌───────▼────────┐
                  │ MarshalResponse│
                  │ → XML or JSON  │
                  └────────────────┘
```

## Protocol Detection

`DetectProtocol()` in `handlers/adapter.go` determines the protocol:

1. If `X-Amz-Target` header is present → **JSON Protocol**
2. If `Content-Type` is `application/x-amz-json-1.0` → **JSON Protocol**
3. Otherwise → **Query Protocol**

## Request Interface

Both adapters implement the `handlers.Request` interface, providing a protocol-agnostic view:

```go
type Request interface {
    GetAction() string
    GetQueueName() string
    GetQueueURL() string
    GetMessageBody() string
    GetDelaySeconds() int
    GetVisibilityTimeout() int
    GetMaxNumberOfMessages() int
    GetWaitTimeSeconds() int
    GetReceiptHandle() string
    GetPrefix() string
    GetAttributeNames() []string
    GetMessageAttributes() map[string]types.MessageAttribute
    GetMessageAttributeNames() []string
    GetAttributes() map[string]string
    GetBatchEntries() []BatchEntry
    GetTags() map[string]string
    GetTagKeys() []string
    GetMessageDeduplicationID() string
    GetMessageGroupID() string
    GetMessageSystemAttributes() map[string]types.MessageSystemAttribute
}
```

## Query Protocol

**File:** `protocol/query.go`

Parses `application/x-www-form-urlencoded` request bodies (and query string parameters).

### Parsing

```go
func ParseQueryRequest(body []byte) (*QueryRequest, error)
```

### Key Features

- **Action extraction:** From `Action` parameter
- **Attribute pairs:** Handles `Attribute.N.Name` / `Attribute.N.Value` pattern
- **Batch entries:** Parses `SendMessageBatchRequestEntry.N.*`, `DeleteMessageBatchRequestEntry.N.*`, `ChangeMessageVisibilityBatchRequestEntry.N.*` formats
- **Message attributes:** Parses `MessageAttribute.N.Name`, `.Value.DataType`, `.Value.StringValue`, `.Value.BinaryValue` (including per-entry attributes in batch requests)
- **Message system attributes:** Parses `MessageSystemAttribute.N.Name`, `.Value.DataType`, `.Value.StringValue` (e.g., `AWSTraceHeader`)
- **FIFO parameters:** Parses `MessageDeduplicationId` and `MessageGroupId` (also per-entry in batch requests)
- **Queue attributes:** Parses `Attribute.N.Name` / `Attribute.N.Value` pairs for SetQueueAttributes
- **Queue tags:** Parses `Tag.N.Key` / `Tag.N.Value` for TagQueue and `TagKey.N` for UntagQueue
- **Reserved parameters:** Validates against SQS reserved parameter names (includes `MessageDeduplicationId`, `MessageGroupId`, `MessageSystemAttribute`)
- **Default values:** Returns sensible defaults for optional parameters (e.g., `MaxNumberOfMessages` defaults to 1)

### Example Request

```
POST / HTTP/1.1
Content-Type: application/x-www-form-urlencoded

Action=SendMessage&QueueUrl=http%3A%2F%2Flocalhost%3A9324%2F123456789012%2Forders&MessageBody=Hello&DelaySeconds=5
```

## JSON Protocol

**File:** `protocol/json.go`

Parses JSON request bodies following the AWS JSON Protocol 1.0 specification.

### Parsing

```go
func ParseJSONRequest(targetHeader string, body []byte) (*JSONRequest, error)
```

### Key Features

- **Action extraction:** From `X-Amz-Target` header (format: `AmazonSQS.<Action>`)
- **JSON unmarshalling:** Uses `encoding/json` with case-insensitive field matching
- **CamelCase mapping:** JSON camelCase → Go struct fields (e.g., `queueUrl` → `QueueURL`)
- **Message attributes:** Parses `MessageAttributes` object with `DataType`, `StringValue`, `BinaryValue` (base64-decoded)
- **Message system attributes:** Parses `MessageSystemAttributes` object (e.g., `AWSTraceHeader`)
- **FIFO parameters:** Parses `MessageDeduplicationId` and `MessageGroupId` (also per-entry in batch requests)
- **Message attribute names:** Parses `MessageAttributeNames` array for ReceiveMessage filtering
- **Queue attributes:** Parses `Attributes` map for SetQueueAttributes
- **Queue tags:** Parses `Tags` map for TagQueue and `TagKeys` array for UntagQueue
- **Batch entries:** Parses `Entries` array with `Id`, `MessageBody`, `DelaySeconds`, `ReceiptHandle`, `VisibilityTimeout`, per-entry `MessageAttributes`, `MessageSystemAttributes`, `MessageDeduplicationId`, and `MessageGroupId`
- **Default values:** Same defaults as Query Protocol

### Example Request

```
POST / HTTP/1.1
X-Amz-Target: AmazonSQS.SendMessage
Content-Type: application/x-amz-json-1.0

{"queueUrl": "http://localhost:9324/123456789012/orders", "messageBody": "Hello", "delaySeconds": 5}
```

## Response Marshalling

**File:** `protocol/marshal.go`

### XML Marshalling

```go
func MarshalXMLResponse(resp *handlers.Response) ([]byte, error)
```

Produces SQS-compatible XML responses:

```xml
<SendMessageResponse xmlns="http://queue.amazonaws.com/doc/2012-11-05/">
  <SendMessageResult>
    <MessageId>abc-123</MessageId>
    <MD5OfMessageBody>5d41402abc4b2a76b9719d911017c592</MD5OfMessageBody>
    <MD5OfMessageAttributes>3f7a8b2c1e9d4a6f5b0c8e7d2a1b3f4c</MD5OfMessageAttributes>
    <MD5OfMessageSystemAttributes>a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6</MD5OfMessageSystemAttributes>
    <SequenceNumber>00000000000000000001</SequenceNumber>
  </SendMessageResult>
  <ResponseMetadata>
    <RequestId>00000000-0000-0000-0000-000000000000</RequestId>
  </ResponseMetadata>
</SendMessageResponse>
```

> `MD5OfMessageSystemAttributes` is included when system attributes are present. `SequenceNumber` is included for FIFO queue messages.

### XML Response Types

| Type | Used For |
|------|----------|
| `CreateQueueResponse` | CreateQueue |
| `GetQueueURLResponse` | GetQueueURL |
| `ListQueuesResponse` | ListQueues |
| `SendMessageResponse` | SendMessage |
| `ReceiveMessageResponse` | ReceiveMessage |
| `DeleteMessageResponse` | DeleteMessage, ChangeMessageVisibility |
| `GetQueueAttributesResponse` | GetQueueAttributes |
| `SetQueueAttributesResponse` | SetQueueAttributes |
| `PurgeQueueResponse` | PurgeQueue |
| `DeleteQueueResponse` | DeleteQueue |
| `SendMessageBatchResponse` | SendMessageBatch |
| `DeleteMessageBatchResponse` | DeleteMessageBatch |
| `ChangeMessageVisibilityBatchResponse` | ChangeMessageVisibilityBatch |
| `ListQueueTagsResponse` | ListQueueTags |
| `TagQueueResponse` | TagQueue |
| `UntagQueueResponse` | UntagQueue |
| `AddPermissionResponse` | AddPermission |
| `RemovePermissionResponse` | RemovePermission |
| `ListDeadLetterSourceQueuesResponse` | ListDeadLetterSourceQueues |
| `StartMessageMoveTaskResponse` | StartMessageMoveTask |
| `CancelMessageMoveTaskResponse` | CancelMessageMoveTask |
| `ListMessageMoveTasksResponse` | ListMessageMoveTasks |
| `XMLMessage` | Individual message in ReceiveMessage |
| `XMLMsgAttribute` | Message attribute in XML responses |

### JSON Marshalling

```go
func MarshalJSONResponse(resp *handlers.Response) ([]byte, error)
```

Produces JSON responses:

```json
{
  "MessageId": "abc-123",
  "MD5OfMessageBody": "5d41402abc4b2a76b9719d911017c592"
}
```

### JSON Response Types

| Type | Used For |
|------|----------|
| `JSONCreateQueueResponse` | CreateQueue |
| `JSONGetQueueURLResponse` | GetQueueURL |
| `JSONListQueuesResponse` | ListQueues |
| `JSONSendMessageResponse` | SendMessage |
| `JSONReceiveMessageResponse` | ReceiveMessage |
| `JSONSendMessageBatchResponse` | SendMessageBatch |
| `JSONDeleteMessageBatchResponse` | DeleteMessageBatch |
| `JSONChangeMessageVisibilityBatchResponse` | ChangeMessageVisibilityBatch |
| `JSONListQueueTagsResponse` | ListQueueTags |
| `JSONTagQueueResponse` | TagQueue |
| `JSONUntagQueueResponse` | UntagQueue |
| `JSONAddPermissionResponse` | AddPermission |
| `JSONRemovePermissionResponse` | RemovePermission |
| `JSONListDeadLetterSourceQueuesResponse` | ListDeadLetterSourceQueues |
| `JSONStartMessageMoveTaskResponse` | StartMessageMoveTask |
| `JSONCancelMessageMoveTaskResponse` | CancelMessageMoveTask |
| `JSONListMessageMoveTasksResponse` | ListMessageMoveTasks |
| `JSONMessage` | Individual message in JSON ReceiveMessage |

## Error Responses

**File:** `protocol/errors.go`

### SQSErrorResponse

```go
type SQSErrorResponse struct {
    Type      string // "Sender" or "Receiver"
    Code      string // e.g., "QueueDoesNotExist"
    Message   string // Human-readable error message
    RequestID string // Request ID
}
```

### XML Error Format

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

### JSON Error Format

```json
{
  "__type": "com.amazonaws.sqs#QueueDoesNotExist",
  "message": "The specified queue does not exist.",
  "RequestId": "00000000-0000-0000-0000-000000000000"
}
```

Note: The JSON `__type` field uses the format `com.amazonaws.sqs#<Code>`.

## Content Types

| Context | Content-Type |
|---------|-------------|
| Query Protocol request | `application/x-www-form-urlencoded` |
| JSON Protocol request | `application/x-amz-json-1.0` |
| Query Protocol response | `text/xml` |
| JSON Protocol response | `application/x-amz-json-1.0` |

## Constants

**File:** `protocol/types/constants.go` (via `queue/types/constants.go`)

### SQS Version

```go
const SQSVersion = "2012-11-05"
```

### Content Type Constants

| Constant | Value |
|----------|-------|
| `QueryProtocolContentType` | `application/x-www-form-urlencoded` |
| `JSONProtocolContentType` | `application/x-amz-json-1.0` |
| `XMLContentType` | `text/xml` |
| `JSONContentType` | `application/json` |

### Action Names

27 action name constants are defined (23 implemented, 4 reserved for future):

**Implemented (23):**
`CreateQueue`, `DeleteQueue`, `GetQueueURL`, `ListQueues`, `SendMessage`, `ReceiveMessage`, `DeleteMessage`, `ChangeMessageVisibility`, `GetQueueAttributes`, `SetQueueAttributes`, `PurgeQueue`, `SendMessageBatch`, `DeleteMessageBatch`, `ChangeMessageVisibilityBatch`, `TagQueue`, `UntagQueue`, `ListQueueTags`, `AddPermission`, `RemovePermission`, `ListDeadLetterSourceQueues`, `StartMessageMoveTask`, `CancelMessageMoveTask`, `ListMessageMoveTasks`

**Reserved (4):**
Reserved for future SQS API actions.
