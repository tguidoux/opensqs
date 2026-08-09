# Plan: Phase 2 — Full SQS Compatibility

## TL;DR

Implement RFC Phase 2 features: message attributes (end-to-end), SetQueueAttributes (real), batch operations (SendMessageBatch, DeleteMessageBatch, ChangeMessageVisibilityBatch), queue tagging (TagQueue, UntagQueue, ListQueueTags), and permission stubs (AddPermission, RemovePermission). FIFO queues, DLQ, and SQLite persistence are deferred to later phases to keep scope manageable.

## Current State (Phase 1 Complete)

**11 actions dispatched** (10 fully working + 1 stub):
- ✅ CreateQueue, DeleteQueue, GetQueueUrl, ListQueues, SendMessage, ReceiveMessage, DeleteMessage, ChangeMessageVisibility, GetQueueAttributes, PurgeQueue
- ⚠️ SetQueueAttributes (stub — no-op)

**Key architectural gaps blocking Phase 2:**
1. `Request` interface (`handler.go:23-35`) is too narrow — missing `GetMessageAttributes()`, `GetAttributes()`, `GetBatchEntries()`, `GetTags()`, `GetMessageDeduplicationID()`, `GetMessageGroupID()`
2. Protocol parsers already have some data (e.g., `QueryRequest.Attributes`, `JSONRequest.GetBatchEntries()`, `JSONRequest.GetAttributes()`) but adapters don't expose them
3. `handleSendMessage` doesn't populate `MessageAttributes` on the `types.Message`
4. `buildXMLMessage` / `buildJSONMessage` don't include message attributes in responses
5. No batch response types in `marshal.go`
6. No tag response types in `marshal.go`

## Steps

### Phase 2A: Message Attributes (End-to-End)
*Parallel with Phase 2B*

1. **Add `GetMessageAttributes()` to `Request` interface** in `handler.go` — returns `map[string]types.MessageAttribute`
2. **Implement `GetMessageAttributes()` on `QueryRequest`** in `query.go` — parse `MessageAttribute.N.Name`, `.N.Value.DataType`, `.N.Value.StringValue`, `.N.Value.BinaryValue` from form params (pattern already exists for batch entries in `parseBatchEntries`)
3. **Implement `GetMessageAttributes()` on `JSONRequest`** in `json.go` — extract from `MessageAttributes` JSON object
4. **Expose via adapters** — add `GetMessageAttributes()` to `QueryRequestAdapter` and `JSONRequestAdapter` in `adapter.go`
5. **Update `handleSendMessage`** in `actions.go` — populate `msg.MessageAttributes` from request, compute `MD5OfMessageAttributes`
6. **Update `buildXMLMessage` / `buildJSONMessage`** in `adapter.go` — include `MessageAttributes` and `MD5OfMessageAttributes` in response (response types `XMLMessage` and `JSONMessage` already have these fields)
7. **Add `GetMessageAttributeNames()` to `Request` interface** — for ReceiveMessage attribute filtering
8. **Update `handleReceiveMessage`** — pass requested attribute names to filter which message attributes are returned

### Phase 2B: SetQueueAttributes (Real Implementation)
*Parallel with Phase 2A*

1. **Add `GetAttributes()` to `Request` interface** in `handler.go` — returns `map[string]string`
2. **Expose via adapters** — `QueryRequestAdapter` and `JSONRequestAdapter` delegate to existing `QueryRequest.Attributes` and `JSONRequest.GetAttributes()`
3. **Update `handleSetQueueAttributes`** in `actions.go` — iterate attributes map, call `q.Attributes().SetAttribute(name, value)` for each, return success
4. **Add `SetQueueAttributes` method to `Queue`** in `queue.go` — delegates to `attributes.SetAttribute`

### Phase 2C: Queue Tagging
*Depends on Phase 2B (Request interface changes)*

1. **Add `GetTags()` and `GetTagKeys()` to `Request` interface** in `handler.go`
2. **Implement tag parsing on `QueryRequest`** in `query.go` — parse `Tag.N.Key` / `Tag.N.Value` for TagQueue, `TagKey.N` for UntagQueue
3. **Implement tag parsing on `JSONRequest`** in `json.go` — extract `Tags` map for TagQueue, `TagKeys` array for UntagQueue
4. **Expose via adapters** in `adapter.go`
5. **Add tag response types** in `marshal.go` — `ListQueueTagsResponse` (XML), `JSONListQueueTagsResponse` (JSON), `TagQueueResponse`, `UntagQueueResponse`
6. **Implement handlers** in `actions.go` — `handleTagQueue`, `handleUntagQueue`, `handleListQueueTags`
7. **Add dispatch cases** in `handler.go` — `ActionTagQueue`, `ActionUntagQueue`, `ActionListQueueTags`
8. **Add marshalling** in `adapter.go` — `marshalXMLResponse` and `marshalJSONResponse` cases for tag actions
9. **Update `Response` struct** in `handler.go` — add `Tags map[string]string` field

### Phase 2D: Batch Operations
*Depends on Phase 2A (message attributes) and Phase 2B (Request interface)*

1. **Add `GetBatchEntries()` to `Request` interface** in `handler.go` — returns `[]BatchEntry`
2. **Define `BatchEntry` struct** in `handler.go` — `ID`, `MessageBody`, `DelaySeconds`, `ReceiptHandle`, `VisibilityTimeout`, `MessageAttributes`
3. **Implement `GetBatchEntries()` on `QueryRequest`** — convert existing `QueryBatchEntry` to `BatchEntry` (already parsed by `parseBatchEntries`)
4. **Implement `GetBatchEntries()` on `JSONRequest`** — convert existing `JSONBatchEntry` to `BatchEntry` (already parsed by `GetBatchEntries()`)
5. **Expose via adapters** in `adapter.go`
6. **Add batch response types** in `marshal.go`:
   - `SendMessageBatchResponse` (XML) with `SendMessageBatchResultEntry` entries + `BatchResultErrorEntry`
   - `DeleteMessageBatchResponse` (XML) with `DeleteMessageBatchResultEntry` + `BatchResultErrorEntry`
   - `ChangeMessageVisibilityBatchResponse` (XML) with `ChangeMessageVisibilityBatchResultEntry` + `BatchResultErrorEntry`
   - JSON equivalents for all three
7. **Implement `handleSendMessageBatch`** in `actions.go` — validate batch size, check for duplicate IDs, iterate entries, call `q.Store().SendMessage` for each, collect results
8. **Implement `handleDeleteMessageBatch`** in `actions.go` — iterate entries, call `q.Store().DeleteMessage` for each, collect results
9. **Implement `handleChangeMessageVisibilityBatch`** in `actions.go` — iterate entries, call `q.Store().ChangeMessageVisibility` for each, collect results
10. **Add dispatch cases** in `handler.go` — `ActionSendMessageBatch`, `ActionDeleteMessageBatch`, `ActionChangeMessageVisibilityBatch`
11. **Add marshalling** in `adapter.go` — batch response cases for XML and JSON
12. **Update `Response` struct** in `handler.go` — add `BatchResults` and `BatchErrors` fields

### Phase 2E: Permission Stubs
*Depends on Phase 2B (Request interface)*

1. **Implement `handleAddPermission`** in `actions.go` — resolve queue, return success (no-op)
2. **Implement `handleRemovePermission`** in `actions.go` — resolve queue, return success (no-op)
3. **Add dispatch cases** in `handler.go` — `ActionAddPermission`, `ActionRemovePermission`
4. **Add response types** in `marshal.go` — `AddPermissionResponse`, `RemovePermissionResponse` (XML + JSON)
5. **Add marshalling** in `adapter.go`

### Phase 2F: Tests
*Depends on all above*

1. **Update `mockRequest`** in `handlers/tests/handlers_test.go` — add new fields for message attributes, attributes, batch entries, tags
2. **Add handler tests** — SendMessageBatch (success, too many entries, duplicate IDs, mixed success/error), DeleteMessageBatch, ChangeMessageVisibilityBatch, TagQueue, UntagQueue, ListQueueTags, SetQueueAttributes (real), AddPermission, RemovePermission, SendMessage with message attributes
3. **Add protocol parser tests** — Query parser for message attributes, tags, batch entries; JSON parser for same
4. **Add marshal tests** — XML/JSON for batch responses, tag responses, message attributes in responses
5. **Run `bazel run //:gazelle`** to update BUILD files
6. **Run all tests** — `bazel test //apps/go/server/...`

### Phase 2G: Bazel & Cleanup
*Depends on Phase 2F*

1. **Run `bazel run //:gazelle`** — update BUILD.bazel for any new files
2. **Run `bazel run //:bazel.clean`** — format Bazel files
3. **Build verification** — `bazel build //apps/go/server:opensqs-server`
4. **Full test run** — `bazel test //apps/go/server/...`

## Relevant Files

- `apps/go/server/handlers/handler.go` — `Request` interface (add methods), `Response` struct (add fields), `HandleRequest` dispatch (add cases)
- `apps/go/server/handlers/actions.go` — All action handlers (update SendMessage, SetQueueAttributes; add batch, tag, permission handlers)
- `apps/go/server/handlers/adapter.go` — Adapters (expose new methods), `MarshalResponse` (add batch/tag cases), `buildXMLMessage`/`buildJSONMessage` (add message attributes)
- `apps/go/server/protocol/query.go` — `QueryRequest` (add `GetMessageAttributes()`, `GetTags()`, `GetTagKeys()`, `GetBatchEntries()`)
- `apps/go/server/protocol/json.go` — `JSONRequest` (add `GetMessageAttributes()`, `GetTags()`, `GetTagKeys()`, convert `GetBatchEntries()` return type)
- `apps/go/server/protocol/marshal.go` — Add batch response types, tag response types, permission response types
- `apps/go/server/handlers/tests/handlers_test.go` — Update `mockRequest`, add tests
- `apps/go/server/protocol/tests/protocol_test.go` — Add parser tests for new parameters
- `pkgs/v1/queue/queue.go` — `Queue` (add `SetTags` already exists, may need `AddTags`/`RemoveTags`)
- `pkgs/v1/queue/types/types.go` — `MessageAttribute` (already has `BinaryValue []byte` — verify protocol layer handles base64)
- `pkgs/v1/queue/types/constants.go` — Action constants already defined

## Verification

1. **Unit tests pass** — `bazel test //apps/go/server/handlers/tests:handlers_test` and `bazel test //apps/go/server/protocol/tests:protocol_test`
2. **Build succeeds** — `bazel build //apps/go/server:opensqs-server`
3. **Manual AWS CLI test — message attributes:**
   ```bash
   aws --endpoint-url http://localhost:9324 sqs send-message \
     --queue-url http://localhost:9324/123456789012/test \
     --message-body "hello" \
     --message-attributes "Priority={DataType=Number,StringValue=1}"
   ```
4. **Manual AWS CLI test — batch send:**
   ```bash
   aws --endpoint-url http://localhost:9324 sqs send-message-batch \
     --queue-url http://localhost:9324/123456789012/test \
     --entries '[{"Id":"msg1","MessageBody":"hello"},{"Id":"msg2","MessageBody":"world"}]'
   ```
5. **Manual AWS CLI test — tags:**
   ```bash
   aws --endpoint-url http://localhost:9324 sqs tag-queue \
     --queue-url http://localhost:9324/123456789012/test \
     --tags "Environment=dev,Team=backend"
   aws --endpoint-url http://localhost:9324 sqs list-queue-tags \
     --queue-url http://localhost:9324/123456789012/test
   ```
6. **Manual AWS CLI test — set queue attributes:**
   ```bash
   aws --endpoint-url http://localhost:9324 sqs set-queue-attributes \
     --queue-url http://localhost:9324/123456789012/test \
     --attributes "VisibilityTimeout=60"
   ```
7. **Manual AWS CLI test — permissions:**
   ```bash
   aws --endpoint-url http://localhost:9324 sqs add-permission \
     --queue-url http://localhost:9324/123456789012/test \
     --label MyLabel --aws-account-ids 123456789012 --actions SendMessage
   ```

## Decisions

- **Scope:** Implementing message attributes, SetQueueAttributes, batch operations, tagging, and permission stubs. **Excluding** FIFO queues, DLQ, and SQLite persistence — these are complex enough to warrant separate phases.
- **BatchEntry type:** Define a unified `BatchEntry` struct in `handlers` package rather than reusing protocol-specific types, to keep the `Request` interface clean.
- **MD5OfMessageAttributes:** Will compute using the same algorithm as AWS (sorted attribute names, MD5 of concatenated name+type+value). This is important for SDK compatibility.
- **Permission stubs:** Match ElasticMQ behavior — accept and return success without enforcing anything.
- **Binary attributes:** Protocol layer uses base64 strings; `types.MessageAttribute.BinaryValue` is `[]byte`. Adapters handle encoding/decoding.

## Further Considerations

1. **FIFO queues** — Should we include FIFO in this phase? Recommendation: **No** — FIFO requires deduplication cache, message group tracking, and sequence number generation, which is a significant scope increase. Better as Phase 2.5 or Phase 3.
2. **Dead-letter queues** — Should we include DLQ? Recommendation: **No** — DLQ requires redrive policy parsing, receive count tracking with maxReceiveCount enforcement, and automatic message movement. Better as a separate phase.
3. **Batch entry ID validation** — AWS requires batch entry IDs to be alphanumeric, hyphen, underscore, max 80 chars. Should we validate? Recommendation: **Yes** — add a `VerifyBatchEntryID` method to `Limits`.
