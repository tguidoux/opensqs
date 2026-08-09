# Plan: Complete Remaining Phase 2 Features

## TL;DR

Implement all 7 remaining Phase 2 items from the RFC: (1) FIFO queues with message groups, deduplication, and sequencing, (2) Dead-letter queues with RedrivePolicy and automatic redrive, (3) ListDeadLetterSourceQueues action, (4) Message system attributes (AWSTraceHeader), (5) SQLite persistence via pluggable store factory, (6) Auto-create queues on first access, (7) Fix CreateQueue to apply request attributes. The work spans 6 phases with clear dependency ordering — store factory refactoring comes first (shared by DLQ + SQLite), then FIFO (largest), then DLQ, then the smaller items, then tests + docs.

## Current State

**Phase 1: ✅ Complete** — 11 core actions, in-memory store, both protocols, long polling, visibility timeout, Docker image.

**Phase 2 (prior batch): ✅ Complete** — Batch operations (SendMessageBatch, DeleteMessageBatch, ChangeMessageVisibilityBatch), message attributes (String/Number/Binary with MD5), SetQueueAttributes (real), queue tagging (TagQueue/UntagQueue/ListQueueTags), permission stubs (AddPermission/RemovePermission), SQS limits (strict/relaxed), pre-create queues on startup.

**Phase 2 (remaining): ❌ 7 items** — FIFO, DLQ, ListDeadLetterSourceQueues, message system attributes, SQLite persistence, auto-create queues, plus a CreateQueue bug fix.

## Steps

### Phase 2H: Store Factory Refactoring
*Foundation for DLQ + SQLite — must come first*

1. **Define `StoreFactory` type** in `pkgs/v1/queue/store/store.go` — `type StoreFactory func(queueName string, visibilityTimeout int, serverSecret []byte) Store`
2. **Add `storeFactory StoreFactory` field** to `QueueManager` struct in `pkgs/v1/queue/manager.go`
3. **Update `NewQueueManager` signature** — add `storeFactory StoreFactory` as final parameter
4. **Replace hardcoded `memory.NewMemoryStore(...)` call** at `manager.go:57` with `qm.storeFactory(name, attrs.VisibilityTimeout, qm.serverSecret)`
5. **Update `apps/go/server/main.go`** — create default memory factory and pass to `NewQueueManager`
6. **Update all tests** that call `NewQueueManager` — `pkgs/v1/queue/tests/manager_test.go` needs the factory param
7. **Run `bazel run //:gazelle`** then **build + test** to verify no regression

### Phase 2I: FIFO Queues
*Depends on Phase 2H (store factory). Largest and most complex item.*

1. **Update `NewMemoryStore` constructor** in `pkgs/v1/queue/store/memory/memory.go` — add `isFifo bool, contentBasedDeduplication bool` parameters
2. **Update `StoreFactory` type** — add `isFifo bool, contentBasedDeduplication bool` to factory signature (or pass a `StoreConfig` struct)
3. **Update `QueueManager.CreateQueue`** — pass `attrs.FifoQueue` and `attrs.ContentBasedDeduplication` to the store factory
4. **Add deduplication cache to `MemoryStore`** — `dedupCache map[string]time.Time` keyed by dedup ID, 5-minute TTL, cleanup of expired entries on `SendMessage`
5. **Add message group tracking to `MemoryStore`** — `messageGroups map[string][]*memoryMessage` tracking per-group message queues. Only one message per group in-flight at a time.
6. **Add sequence number counter to `MemoryStore`** — `sequenceCounter int64`, incremented per FIFO message, stored in `msg.SequenceNumber`
7. **Update `SendMessage`** — if FIFO: compute dedup key (explicit `MessageDeduplicationID` or SHA-256 of body if `ContentBasedDeduplication`), check dedup cache, reject if duplicate within 5min window (return existing message ID silently), assign sequence number, add to message group queue
8. **Update `ReceiveMessages`** — if FIFO: iterate message groups, skip groups with an in-flight message, return oldest visible message from each eligible group up to `maxMessages`
9. **Update `DeleteMessage`** — if FIFO: remove from message group queue, allow next message in that group to become deliverable
10. **Update visibility timer callback** — if FIFO: when message becomes visible again (not deleted), it stays in its group queue and becomes the next deliverable for that group
11. **Add `GetMessageDeduplicationId()` and `GetMessageGroupId()` to `QueryRequest`** in `protocol/query.go` — simple `GetParam()` calls; add to `isReservedQueryParam`
12. **Add `GetMessageDeduplicationId()` and `GetMessageGroupId()` to `JSONRequest`** in `protocol/json.go` — `getString()` calls
13. **Add `MessageDeduplicationID` and `MessageGroupID` fields to `QueryBatchEntry` and `JSONBatchEntry`** — parse in `parseBatchEntries` (query) and `GetBatchEntries` (JSON)
14. **Add `GetMessageDeduplicationID()` and `GetMessageGroupID()` to `Request` interface** in `handler.go`
15. **Add `MessageDeduplicationID` and `MessageGroupID` to `BatchEntry` struct** in `handler.go`
16. **Implement adapter methods** in `adapter.go` — `QueryRequestAdapter` and `JSONRequestAdapter` delegate to protocol parsers; propagate FIFO fields in `GetBatchEntries()`
17. **Update `handleSendMessage`** in `actions.go` — set `MessageDeduplicationID` and `MessageGroupID` on the message; add FIFO validation: `MessageGroupId` required for FIFO, `MessageDeduplicationId` required unless `ContentBasedDeduplication`, reject FIFO params on non-FIFO queues
18. **Update `handleSendMessageBatch`** — same FIFO validation per entry, propagate dedup/group IDs
19. **Fix `handleCreateQueue`** — apply `req.GetAttributes()` to the attributes before creating the queue (currently ignores request attributes — `FifoQueue=true` is silently dropped). Add validation: if `FifoQueue=true`, queue name must end in `.fifo`; if `FifoQueue=false`, name must not end in `.fifo`
20. **Add `SequenceNumber` to response types** in `marshal.go` — `SendMessageResponse`, `XMLMessage`, `JSONSendMessageResponse`, `JSONMessage`, `SendMessageBatchResultEntry`, `JSONBatchResultEntry`
21. **Add `SequenceNumber` to `BatchResult` struct** in `handler.go`
22. **Propagate `SequenceNumber` in response marshalling** — `buildXMLMessage`, `buildJSONMessage`, `marshalXMLResponse` (SendMessage + batch), `marshalJSONResponse` (same)
23. **Add `VerifyDeduplicationId` and `VerifyMessageGroupId` to `Limits`** in `limits.go` — max 128 chars, alphanumeric + hyphens + underscores + periods
24. **Add `FifoQueue` immutability check to `handleSetQueueAttributes`** — reject `FifoQueue` attribute changes with `InvalidAttributeName`
25. **Update `mockRequest`** in `handlers/tests/handlers_test.go` — add `dedupID`, `groupID` fields + getter methods
26. **Run `bazel run //:gazelle`** then **build + test**

### Phase 2J: Dead-Letter Queues
*Depends on Phase 2H (store factory). Can run parallel with Phase 2I.*

1. **Create `pkgs/v1/queue/dlq/` package** — `redrive.go` with `RedrivePolicy` struct (`DeadLetterTargetArn string`, `MaxReceiveCount int`), `ParseRedrivePolicy(jsonStr string) (*RedrivePolicy, error)` function
2. **Add `RedriveFunc` type** to `store/store.go` — `type RedriveFunc func(msg *types.Message)` — called by store when a message exceeds `maxReceiveCount`
3. **Update `MemoryStore`** — add `maxReceiveCount int` and `redriveFunc store.RedriveFunc` fields; update `NewMemoryStore` (or factory) to accept them
4. **Update visibility timer callback in `MemoryStore`** — when message becomes visible again (visibility timeout expires), check `mm.receiveCount >= maxReceiveCount`; if true, call `redriveFunc(msg)` and remove from store instead of making visible
5. **Update `QueueManager.CreateQueue`** — parse `RedrivePolicy` from attrs, extract `maxReceiveCount`, pass redrive callback that looks up DLQ by ARN and sends the message to it
6. **Add `handleListDeadLetterSourceQueues`** in `actions.go` — iterate all queues via `manager.ListQueues("")`, parse each queue's `RedrivePolicy`, match `deadLetterTargetArn` to the target queue's ARN, return matching queue URLs
7. **Add dispatch case** for `ActionListDeadLetterSourceQueues` in `handler.go`
8. **Add response types** in `marshal.go` — `ListDeadLetterSourceQueuesResponse` (XML) + `JSONListDeadLetterSourceQueuesResponse` (JSON), with `QueueURLs []string` field
9. **Add marshalling** in `adapter.go` — `marshalXMLResponse` and `marshalJSONResponse` cases for `ListDeadLetterSourceQueues`
10. **Add `RedrivePolicy` validation to `SetAttribute`** in `attributes.go` — validate JSON structure when setting `RedrivePolicy`
11. **Run `bazel run //:gazelle`** then **build + test**

### Phase 2K: SQLite Persistence
*Depends on Phase 2H (store factory). Can run parallel with Phase 2I/2J.*

1. **Add `modernc.org/sqlite` to `go.mod`** — pure Go SQLite driver, no CGO (aligns with distroless containers)
2. **Run `bazel run //:go.clean`** — update Bazel deps for the new Go module
3. **Create `pkgs/v1/queue/store/sqlite/sqlite.go`** — implement all 9 `Store` interface methods:
   - `SendMessage` — INSERT row with `visible_at` computed from delay
   - `ReceiveMessages` — SELECT visible rows (`visible_at <= now`), UPDATE with receipt handle + new `visible_at`, handle long polling via polling loop (sleep + retry until messages or deadline)
   - `DeleteMessage` — DELETE by receipt handle
   - `ChangeMessageVisibility` — UPDATE `visible_at` by receipt handle
   - `ApproximateNumberOfMessages` — COUNT where `visible_at <= now`
   - `ApproximateNumberOfMessagesNotVisible` — COUNT where `visible_at > now AND receipt_handle != ''`
   - `ApproximateNumberOfMessagesDelayed` — COUNT where `visible_at > now AND receipt_handle = ''`
   - `Purge` — DELETE all rows for this queue
   - `Close` — close the prepared statements / DB handle
4. **Schema:** `messages` table with columns: `id`, `queue_name`, `body`, `md5_of_body`, `message_attributes` (JSON), `system_attributes` (JSON), `sent_timestamp`, `visible_at`, `receipt_handle`, `receive_count`, `first_received_at`, `sequence_number`, `message_dedup_id`, `message_group_id`, `is_visible`. Indexes on `(queue_name, visible_at)` and `receipt_handle`.
5. **Handle visibility timeout without goroutines** — SQLite store uses lazy evaluation: no `time.AfterFunc` timers. Messages become visible when `visible_at <= now` is checked on next `ReceiveMessages` call. Long polling uses a polling loop with short sleeps.
6. **Add SQLite config fields** to `config.go` — `SQLitePath string` under `SQSConfig`; update `config.yaml` with `sqlitePath` example
7. **Wire up factory selection in `main.go`** — if `storageType == "sqlite"`, open `*sql.DB` and create SQLite store factory; else use memory factory
8. **Run `bazel run //:gazelle`** then **build + test**

### Phase 2L: Auto-Create Queues + Message System Attributes
*Depends on Phase 2H. Small items, can run parallel with 2I/2J/2K.*

1. **Add `QueuesConfig` struct** to `config.go` — `AutoCreate bool` + `Startup []StartupQueue`; change `ServerConfig.Queues` from `[]StartupQueue` to `QueuesConfig`
2. **Update `config.yaml`** — add `autoCreate: false` under `queues:`
3. **Update `startup_queues.go`** — adapt to new `QueuesConfig` type
4. **Add `autoCreate bool` field to `Handler`** in `handler.go`; update `NewHandler` signature
5. **Update `resolveQueue`** in `handler.go` — if `LookupQueueByURL` fails and `autoCreate` is true, call `manager.CreateQueue(name, NewDefaultQueueAttributes())`
6. **Update `main.go`** — pass `cfg.Queues.AutoCreate` to `NewHandler`
7. **Add `GetMessageSystemAttributes()` to `QueryRequest`** in `query.go` — parse `MessageSystemAttribute.N.Name` / `.N.Value.DataType` / `.N.Value.StringValue` (same pattern as message attributes); add `MessageSystemAttribute` to `isReservedQueryParam`
8. **Add `GetMessageSystemAttributes()` to `JSONRequest`** in `json.go` — extract from `MessageSystemAttributes` JSON key
9. **Add `GetMessageSystemAttributes()` to `Request` interface** in `handler.go`
10. **Implement adapter methods** in `adapter.go`
11. **Update `handleSendMessage`** — populate `msg.SystemAttributes` and compute `MD5OfMessageSystemAttributes` (same MD5 algorithm as message attributes)
12. **Update `handleSendMessageBatch`** — same per entry
13. **Add `MD5OfMessageSystemAttributes` to response types** in `marshal.go` — `SendMessageResponse`, `XMLMessage`, `JSONSendMessageResponse`, `JSONMessage`, batch result entries
14. **Propagate in response marshalling** — `buildXMLMessage`, `buildJSONMessage`, `marshalXMLResponse`, `marshalJSONResponse`
15. **Update `mockRequest`** — add `systemAttributes` field + getter
16. **Run `bazel run //:gazelle`** then **build + test**

### Phase 2M: Tests
*Depends on all above*

1. **FIFO store tests** in `pkgs/v1/queue/store/memory/tests/memory_test.go` — message group ordering (messages within group delivered in order), one in-flight per group, dedup within 5min window, dedup with `ContentBasedDeduplication` (body hash), sequence numbers monotonic, FIFO queue name validation
2. **DLQ tests** — message redrive after `maxReceiveCount` receives, redrive to correct DLQ, `ListDeadLetterSourceQueues` returns correct queues
3. **SQLite store tests** in `pkgs/v1/queue/store/sqlite/tests/sqlite_test.go` — all 9 Store methods, long polling, visibility timeout (lazy), concurrent access
4. **Handler tests** — CreateQueue with FIFO attributes, SendMessage with dedup/group IDs, SendMessageBatch FIFO, ListDeadLetterSourceQueues, auto-create on send to non-existent queue, SendMessage with system attributes
5. **Protocol parser tests** — Query parser for `MessageDeduplicationId`, `MessageGroupId`, `MessageSystemAttribute`; JSON parser for same
6. **Marshal tests** — SequenceNumber in SendMessage/batch responses, MD5OfMessageSystemAttributes in responses, ListDeadLetterSourceQueues response
7. **Run `bazel run //:gazelle`** then **`bazel test //...`**

### Phase 2N: Documentation + Example Programs
*Depends on Phase 2M*

1. **Update `docs/api-reference.md`** — document FIFO queue creation, SendMessage with `MessageDeduplicationId`/`MessageGroupId`, `SequenceNumber` in responses, `ListDeadLetterSourceQueues` action, `RedrivePolicy` attribute, message system attributes, auto-create config, SQLite storage config
2. **Update `docs/queue-library.md`** — FIFO queue usage, DLQ configuration, store factory pattern, SQLite backend
3. **Update `docs/README.md`** — update features list (all Phase 2 complete), add FIFO + DLQ to features
4. **Update `docs/protocol.md`** — FIFO parameter parsing, system attribute parsing, ListDeadLetterSourceQueues response types
5. **Create `apps/go/playground/sqs_fifo_example/main.go`** — demonstrate FIFO queue creation, message groups, deduplication, ordered receiving
6. **Create `apps/go/playground/sqs_dlq_example/main.go`** — demonstrate DLQ setup, maxReceiveCount, message redrive
7. **Run `bazel run //:gazelle`** + **`bazel run //:bazel.clean`** + final build/test

## Relevant Files

**Store layer:**
- `pkgs/v1/queue/store/store.go` — Add `StoreFactory` type, `RedriveFunc` type
- `pkgs/v1/queue/store/memory/memory.go` — FIFO logic (dedup cache, message groups, sequence numbers), DLQ redrive callback, constructor changes
- `pkgs/v1/queue/store/sqlite/sqlite.go` — NEW: SQLite Store implementation
- `pkgs/v1/queue/store/sqlite/tests/sqlite_test.go` — NEW: SQLite tests

**Queue engine:**
- `pkgs/v1/queue/manager.go` — Store factory field, pass FIFO/DLQ config to factory, redrive callback setup
- `pkgs/v1/queue/queue.go` — No changes expected (IsFifo already exists)
- `pkgs/v1/queue/attributes.go` — RedrivePolicy JSON validation in SetAttribute
- `pkgs/v1/queue/limits.go` — VerifyDeduplicationId, VerifyMessageGroupId
- `pkgs/v1/queue/dlq/redrive.go` — NEW: RedrivePolicy struct + parser
- `pkgs/v1/queue/types/types.go` — No changes (all fields exist)
- `pkgs/v1/queue/types/constants.go` — No changes (all action constants exist)

**Server handlers:**
- `apps/go/server/handlers/handler.go` — Request interface (add GetMessageDeduplicationID, GetMessageGroupID, GetMessageSystemAttributes), BatchEntry struct (add FIFO fields), BatchResult struct (add SequenceNumber), Handler struct (add autoCreate), resolveQueue (auto-create logic), dispatch (add ListDeadLetterSourceQueues)
- `apps/go/server/handlers/actions.go` — handleSendMessage (FIFO validation, system attrs), handleSendMessageBatch (same), handleCreateQueue (fix: apply request attrs, FIFO name validation), handleSetQueueAttributes (FifoQueue immutability), handleListDeadLetterSourceQueues (NEW)
- `apps/go/server/handlers/adapter.go` — Adapter methods for new Request interface methods, buildXMLMessage/buildJSONMessage (SequenceNumber, system attrs), marshalXMLResponse/marshalJSONResponse (ListDeadLetterSourceQueues, SequenceNumber, MD5OfMessageSystemAttributes)

**Protocol layer:**
- `apps/go/server/protocol/query.go` — GetMessageDeduplicationId, GetMessageGroupId, GetMessageSystemAttributes; add to isReservedQueryParam; parse in batch entries
- `apps/go/server/protocol/json.go` — GetMessageDeduplicationId, GetMessageGroupId, GetMessageSystemAttributes; add to batch entries
- `apps/go/server/protocol/marshal.go` — SequenceNumber on SendMessageResponse/XMLMessage/JSONMessage/batch entries; MD5OfMessageSystemAttributes on same; ListDeadLetterSourceQueuesResponse + JSON variant

**Server config:**
- `apps/go/server/config.go` — QueuesConfig struct (AutoCreate), SQLitePath field
- `apps/go/server/config.yaml` — autoCreate, sqlitePath example
- `apps/go/server/startup_queues.go` — Adapt to QueuesConfig
- `apps/go/server/main.go` — Store factory selection, pass autoCreate to Handler

**Tests:**
- `apps/go/server/handlers/tests/handlers_test.go` — mockRequest updates, new handler tests
- `apps/go/server/protocol/tests/protocol_test.go` — Parser tests for FIFO params, system attrs
- `pkgs/v1/queue/store/memory/tests/memory_test.go` — FIFO store tests, DLQ tests
- `pkgs/v1/queue/tests/manager_test.go` — Update NewQueueManager calls with factory

**Docs + Examples:**
- `docs/api-reference.md`, `docs/queue-library.md`, `docs/README.md`, `docs/protocol.md`
- `apps/go/playground/sqs_fifo_example/main.go` — NEW
- `apps/go/playground/sqs_dlq_example/main.go` — NEW

## Verification

1. **Unit tests pass** — `bazel test //apps/go/server/... //pkgs/v1/queue/...`
2. **Build succeeds** — `bazel build //apps/go/server:opensqs-server`
3. **FIFO CLI test — create + send with group + receive in order:**
   ```bash
   aws --endpoint-url http://localhost:9324 sqs create-queue \
     --queue-name orders.fifo --attributes FifoQueue=true
   aws --endpoint-url http://localhost:9324 sqs send-message \
     --queue-url http://localhost:9324/123456789012/orders.fifo \
     --message-body "msg1" --message-group-id "groupA" --message-deduplication-id "d1"
   aws --endpoint-url http://localhost:9324 sqs send-message \
     --queue-url http://localhost:9324/123456789012/orders.fifo \
     --message-body "msg2" --message-group-id "groupA" --message-deduplication-id "d2"
   # Receive should return msg1 first, then msg2 after deletion
   ```
4. **FIFO dedup test — same dedup ID within 5min returns same message ID**
5. **DLQ test — message redrive after maxReceiveCount**
6. **ListDeadLetterSourceQueues test**
7. **Auto-create test — send to non-existent queue with auto-create enabled**
8. **System attributes test — send with AWSTraceHeader**
9. **SQLite persistence test — restart server, verify messages persist**
10. **Example programs run** — `bazel run //apps/go/playground/sqs_fifo_example:sqs_fifo_example` and `bazel run //apps/go/playground/sqs_dlq_example:sqs_dlq_example`

## Decisions

- **Store factory pattern:** `StoreFactory` function type injected into `QueueManager`, replacing hardcoded `memory.NewMemoryStore`. This enables both SQLite and DLQ callback injection cleanly.
- **SQLite driver:** `modernc.org/sqlite` (pure Go, no CGO) — aligns with distroless container strategy and Bazel build constraints.
- **FIFO in MemoryStore:** Dedup cache as `map[string]time.Time` with 5min TTL. Message groups as `map[string][]*memoryMessage` with one-in-flight-per-group rule. Sequence numbers as `int64` counter.
- **DLQ redrive:** Store-level callback (`RedriveFunc`) called from visibility timer expiry when `receiveCount >= maxReceiveCount`. Manager provides the callback that looks up DLQ by ARN and sends to it.
- **Auto-create:** Handler-level logic in `resolveQueue`, controlled by config flag. Not in QueueManager (domain layer stays pure).
- **CreateQueue bug fix:** Apply `req.GetAttributes()` to default attrs before creating queue. This is required for FIFO to work (FifoQueue attribute must be set at creation).
- **FifoQueue immutability:** `SetQueueAttributes` rejects `FifoQueue` changes — AWS doesn't allow changing FIFO status after creation.
- **Config change:** `ServerConfig.Queues` changes from `[]StartupQueue` to `QueuesConfig{AutoCreate bool, Startup []StartupQueue}`. This is a breaking config change — `config.yaml` needs updating.
- **ListDeadLetterSourceQueues pagination:** Implement without pagination for now (return all results). AWS supports `MaxResults`/`NextToken` but this can be added later.
- **Message move tasks** (StartMessageMoveTask, etc.) are Phase 4 per RFC — NOT included in this plan.

## Further Considerations

1. **SQLite + FIFO interaction:** SQLite store will also need FIFO logic (dedup, message groups, sequence numbers). Should FIFO logic be extracted into a shared middleware/wrapper that wraps any Store? Recommendation: For now, implement FIFO only in MemoryStore. SQLite FIFO can be a follow-up — most SQLite use cases are production standard queues, not FIFO.
2. **Relaxed mode:** `Limits.RelaxedMode` is defined but all checks enforce regardless of mode. Should relaxed mode actually skip limit checks? Recommendation: Yes — add `if l.Mode == RelaxedMode { return nil }` at the top of each Verify method. Low effort, should include in this phase.
3. **Message retention:** `MessageRetentionPeriod` attribute exists but MemoryStore never expires messages. Should we add a cleanup goroutine? Recommendation: Yes — add a background sweeper in MemoryStore that removes messages older than retention period. Low effort, important for correctness.
