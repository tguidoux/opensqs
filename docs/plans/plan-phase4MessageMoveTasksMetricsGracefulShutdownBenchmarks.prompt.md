# Plan: Phase 4 — Message Move Tasks, Metrics, Graceful Shutdown & Benchmarks

## TL;DR

Phase 4 adds four capabilities to OpenSQS: (1) SQS Message Move Task API (`StartMessageMoveTask`, `CancelMessageMoveTask`, `ListMessageMoveTasks`) for moving messages from DLQs to other queues, (2) Prometheus-compatible metrics endpoint with per-queue and per-action counters, (3) graceful shutdown that drains in-flight messages and closes all stores on SIGTERM, and (4) performance benchmarks for both memory and SQLite stores. The implementation follows existing patterns: action constants already exist in `types/constants.go`, the handler switch in `handler.go` needs new cases, the `Request` interface needs new getters, and the protocol layer needs new parsers/marshallers.

---

## Steps

### Phase A: Message Move Tasks (SQS API)

**A1.** Create `pkgs/v1/queue/dlq/move_task.go` — defines `MoveTask` struct (TaskHandle string, SourceArn string, DestinationArn string, Status string, MaxNumberOfMessagesPerSecond int, MovedMessages int, StartedAt time.Time, Cancelled chan struct{}), `MoveTaskManager` struct (mu sync.RWMutex, tasks map[string]*MoveTask, manager *queue.QueueManager), and methods: `NewMoveTaskManager()`, `StartTask(sourceArn, destinationArn string, maxRate int) (*MoveTask, error)`, `CancelTask(taskHandle string) error`, `ListTasks(sourceArn string) []*MoveTask`, `runTask(ctx, task)` (goroutine that receives from source queue and sends to destination with optional rate limiting via `time.Ticker`). *Depends on nothing — new file.*

**A2.** Add new getters to `Request` interface in `handlers/handler.go`: `GetSourceArn() string`, `GetDestinationArn() string`, `GetTaskHandle() string`, `GetMaxNumberOfMessagesPerSecond() int`. Add corresponding `MoveTaskResult` field to `Response` struct. *Depends on A1.*

**A3.** Implement getters in `protocol/query.go` (`GetSourceArn`, `GetDestinationArn`, `GetTaskHandle`, `GetMaxNumberOfMessagesPerSecond` — read from `Params` / form values) and `protocol/json.go` (same getters reading from JSON body map). *Parallel with A2.*

**A4.** Implement adapter methods in `handlers/adapter.go`: `QueryRequestAdapter.GetSourceArn()` etc. delegating to `protocol.QueryRequest`, and `JSONRequestAdapter.GetSourceArn()` etc. delegating to `protocol.JSONRequest`. *Depends on A2, A3.*

**A5.** Add response structs to `protocol/marshal.go`: `StartMessageMoveTaskResponse` / `JSONStartMessageMoveTaskResponse` (fields: TaskHandle, RequestId), `CancelMessageMoveTaskResponse` / `JSONCancelMessageMoveTaskResponse` (fields: ApproximateNumberOfMessagesMoved, RequestId), `ListMessageMoveTasksResponse` / `JSONListMessageMoveTasksResponse` (fields: Results []MoveTaskResult, NextToken, RequestId). Add `MoveTaskResult` struct with XML/JSON tags. *Parallel with A2.*

**A6.** Add marshal cases in `handlers/adapter.go` `marshalXMLResponse()` and `marshalJSONResponse()` for `ActionStartMessageMoveTask`, `ActionCancelMessageMoveTask`, `ActionListMessageMoveTasks`. *Depends on A5.*

**A7.** Implement handler methods in `handlers/actions.go`: `handleStartMessageMoveTask`, `handleCancelMessageMoveTask`, `handleListMessageMoveTasks`. These use a `MoveTaskManager` that the `Handler` struct needs to hold. *Depends on A1, A2, A6.*

**A8.** Add `MoveTaskManager` field to `Handler` struct in `handler.go`, initialize in `NewHandler()`. Add switch cases for the three new actions in `HandleRequest()`. *Depends on A1, A7.*

**A9.** Wire `MoveTaskManager` in `main.go` — pass `manager` (QueueManager) to `MoveTaskManager` creation, pass to `handlers.NewHandler()`. *Depends on A8.*

**A10.** Write tests in `apps/go/server/handlers/tests/move_task_test.go` — test StartMessageMoveTask (creates task, returns task handle), CancelMessageMoveTask (cancels running task), ListMessageMoveTasks (lists tasks for source ARN), error cases (source queue not found, task handle not found). Use `mockRequest` pattern from existing tests, add move task fields. *Depends on A8.*

**A11.** Write tests in `pkgs/v1/queue/dlq/tests/move_task_test.go` — test `MoveTaskManager` directly: start task moves messages from source to destination, cancel stops the task, rate limiting works, list returns correct tasks. Use `newTestManager()` pattern from `pkgs/v1/queue/tests/manager_test.go`. *Depends on A1.*

### Phase B: Prometheus Metrics

**B1.** Add `github.com/prometheus/client_golang` to `go.mod`, run `bazel run //:go.clean` to update Bazel deps. *No code dependency — can start immediately.*

**B2.** Create `apps/go/server/metrics/metrics.go` — defines Prometheus metric collectors: `messagesSentTotal` (counter, labels: queue, fifo), `messagesReceivedTotal` (counter, labels: queue), `messagesDeletedTotal` (counter, labels: queue), `queueSize` (gauge, labels: queue, type=available|inflight|delayed), `apiRequestsTotal` (counter, labels: action, protocol), `apiRequestDuration` (histogram, labels: action, protocol), `moveTaskMessagesMoved` (counter, labels: source_arn, destination_arn). Register all with default Prometheus registry. Provide a `Collector` struct with methods to increment counters. *Depends on B1.*

**B3.** Create `apps/go/server/metrics/server.go` — HTTP server on configurable port serving `/metrics` endpoint via `promhttp.Handler()`. Follow the `health.Server` pattern (Start/Stop lifecycle, configurable port). *Depends on B1.*

**B4.** Add metrics config to `apps/go/server/config.go` — `MetricsConfig` struct with `Enabled bool` and `Port int` (default 9326), add `Metrics MetricsConfig` field to `ServerConfig`. Add to `config.yaml`. *Parallel with B2.*

**B5.** Wire metrics into `handlers/handler.go` — add optional `*metrics.Collector` field to `Handler`, wrap each action handler call with timing and counter increments. Use a middleware-style approach: in `HandleRequest()`, record start time, dispatch, increment `apiRequestsTotal` and observe `apiRequestDuration` on return. For message operations, increment `messagesSentTotal` etc. *Depends on B2.*

**B6.** Wire metrics into store operations — add metrics increment calls in `handlers/actions.go` after successful `SendMessage`, `ReceiveMessage`, `DeleteMessage` calls. For queue size gauges, add a periodic refresh goroutine in `metrics/server.go` that queries `QueueManager.ListQueues("")` and updates `queueSize` gauges every 15 seconds. *Depends on B2, B5.*

**B7.** Wire metrics server in `main.go` — if `cfg.Metrics.Enabled`, start metrics server on configured port. Pass `metrics.Collector` to `handlers.NewHandler()`. *Depends on B3, B4, B5.*

**B8.** Write tests in `apps/go/server/metrics/tests/metrics_test.go` — test that metrics are registered, counters increment correctly, `/metrics` endpoint returns Prometheus-format text. *Depends on B2, B3.*

### Phase C: Graceful Shutdown

**C1.** Add `Shutdown(ctx context.Context) error` method to `QueueManager` in `pkgs/v1/queue/manager.go` — iterates all queues, calls `Store().Close()` on each, waits for in-flight operations to complete (use a `sync.WaitGroup` that tracks active operations). Add `sync.WaitGroup` field to `QueueManager`, increment in `CreateQueue`/`PurgeQueue`/`redriveMessage`, decrement on completion. *No dependency — can start immediately.*

**C2.** Update `main.go` shutdown sequence — after `httpServer.Shutdown(ctx)`, call `manager.Shutdown(ctx)` to close all stores. Also stop any active move tasks. Add logging for shutdown progress (number of stores closed, in-flight messages drained). *Depends on C1, A8.*

**C3.** Add context cancellation to long-polling `ReceiveMessages` — both `MemoryStore` and `SQLiteStore` already check `ctx.Done()` in their receive loops, so cancelling the shutdown context will unblock long-polling receivers. Verify this works by passing a cancellable context. *Depends on C2.*

**C4.** Write tests in `pkgs/v1/queue/tests/manager_test.go` — test `Shutdown()` closes all stores, test that in-flight operations complete before shutdown returns, test context cancellation unblocks long polling. *Depends on C1.*

### Phase D: Performance Benchmarks

**D1.** Create `pkgs/v1/queue/store/memory/tests/bench_test.go` — `BenchmarkSendMessage`, `BenchmarkReceiveMessage`, `BenchmarkDeleteMessage`, `BenchmarkSendMessageBatch` (10 messages), `BenchmarkConcurrentSendReceive` (parallel goroutines). Use `b.ResetTimer()`, `b.ReportAllocs()`. *Depends on nothing — can start immediately.*

**D2.** Create `pkgs/v1/queue/store/sqlite/tests/bench_test.go` — same benchmarks as D1 but for SQLite store. Use `newTestDB(t)` pattern adapted for benchmarks (`b.TempDir()` instead of `t.TempDir()`). *Parallel with D1.*

**D3.** Create `apps/go/server/handlers/tests/bench_test.go` — `BenchmarkHandleRequest_SendMessage`, `BenchmarkHandleRequest_ReceiveMessage` — benchmarks through the full handler → manager → store pipeline using `mockRequest`. *Depends on nothing — can start immediately.*

**D4.** Run benchmarks and record baseline results. Create `docs/benchmarks.md` with baseline numbers for memory and SQLite stores. *Depends on D1, D2, D3.*

### Phase E: Integration & Verification

**E1.** Run `bazel run //:gazelle` to generate BUILD.bazel files for all new packages and test files. *Depends on all above.*

**E2.** Run `bazel run //:bazel.clean` to format all Bazel files. *Depends on E1.*

**E3.** Run `bazel build //apps/go/server:opensqs-server` to verify the server builds. *Depends on E1, E2.*

**E4.** Run `bazel test //apps/go/server/...` and `bazel test //pkgs/v1/queue/...` to verify all tests pass. *Depends on E3.*

**E5.** Run `bazel test --test_arg=-test.bench=. //pkgs/v1/queue/store/memory/tests:go_default_test` and same for SQLite to run benchmarks. *Depends on D1, D2, E4.*

**E6.** Update `docs/rfc-001-opensqs-server.md` — mark Phase 4 items as complete. *Depends on all above.*

---

## Relevant Files

### New Files
- `pkgs/v1/queue/dlq/move_task.go` — MoveTask struct, MoveTaskManager, task lifecycle, rate-limited message moving goroutine
- `pkgs/v1/queue/dlq/tests/BUILD.bazel` — test target (auto-generated by gazelle)
- `pkgs/v1/queue/dlq/tests/move_task_test.go` — unit tests for MoveTaskManager
- `apps/go/server/metrics/metrics.go` — Prometheus metric definitions and Collector struct
- `apps/go/server/metrics/server.go` — metrics HTTP server (/metrics endpoint)
- `apps/go/server/metrics/BUILD.bazel` — library target (auto-generated)
- `apps/go/server/metrics/tests/metrics_test.go` — metrics tests
- `apps/go/server/metrics/tests/BUILD.bazel` — test target (auto-generated)
- `pkgs/v1/queue/store/memory/tests/bench_test.go` — memory store benchmarks
- `pkgs/v1/queue/store/sqlite/tests/bench_test.go` — SQLite store benchmarks
- `apps/go/server/handlers/tests/move_task_test.go` — handler-level move task tests
- `apps/go/server/handlers/tests/bench_test.go` — handler pipeline benchmarks
- `docs/benchmarks.md` — baseline benchmark results

### Modified Files
- `apps/go/server/handlers/handler.go` — add `MoveTaskManager` field to `Handler`, add 3 switch cases, add 4 getters to `Request` interface, add `MoveTaskResult`/`MoveTasks` fields to `Response`
- `apps/go/server/handlers/actions.go` — implement `handleStartMessageMoveTask`, `handleCancelMessageMoveTask`, `handleListMessageMoveTasks`, add metrics increment calls
- `apps/go/server/handlers/adapter.go` — add `GetSourceArn()`, `GetDestinationArn()`, `GetTaskHandle()`, `GetMaxNumberOfMessagesPerSecond()` to both `QueryRequestAdapter` and `JSONRequestAdapter`, add 6 marshal cases (3 XML + 3 JSON)
- `apps/go/server/protocol/query.go` — add `GetSourceArn()`, `GetDestinationArn()`, `GetTaskHandle()`, `GetMaxNumberOfMessagesPerSecond()` methods to `QueryRequest`
- `apps/go/server/protocol/json.go` — add same 4 getters to `JSONRequest`
- `apps/go/server/protocol/marshal.go` — add 6 response structs (3 XML + 3 JSON) for move task actions, add `MoveTaskResult` struct
- `apps/go/server/main.go` — wire `MoveTaskManager`, wire metrics server, add `manager.Shutdown(ctx)` to shutdown sequence
- `apps/go/server/config.go` — add `MetricsConfig` struct, add `Metrics` field to `ServerConfig`
- `apps/go/server/config.yaml` — add `metrics:` section
- `apps/go/server/BUILD.bazel` — add metrics dep (auto-generated by gazelle)
- `pkgs/v1/queue/manager.go` — add `Shutdown(ctx)`, add `sync.WaitGroup` for in-flight tracking
- `pkgs/v1/queue/dlq/BUILD.bazel` — add move_task.go src (auto-generated by gazelle)
- `go.mod` — add `github.com/prometheus/client_golang`
- `docs/rfc-001-opensqs-server.md` — mark Phase 4 complete

### Key Reference Patterns
- **Handler dispatch**: `Handler.HandleRequest()` switch in `handler.go` (line ~155) — add cases following existing pattern
- **Protocol parsing**: `QueryRequest.GetQueueURL()` in `query.go` (line ~87) — pattern for new getters using `r.Params.Get()`
- **JSON parsing**: `JSONRequest.GetQueueURL()` in `json.go` (line ~87) — pattern for new getters using `r.getString()`
- **Response marshalling**: `marshalXMLResponse()` in `adapter.go` (line ~212) — switch on action, construct response struct, call `protocol.MarshalXMLResponse()`
- **DLQ redrive**: `QueueManager.redriveMessage()` in `manager.go` (line ~175) — pattern for moving messages between queues
- **Store lifecycle**: `MemoryStore.Close()` stops `time.AfterFunc` timers; `SQLiteStore.Close()` sets `closed = true` — `Shutdown()` will call `Close()` on all stores
- **Test helpers**: `newTestHandler()` in `handlers/tests/handlers_test.go` — pattern for creating test handler with in-memory store; `newTestManager()` in `pkgs/v1/queue/tests/manager_test.go` — pattern for creating test QueueManager
- **Health server**: `health/server.go` — pattern for Start/Stop HTTP server lifecycle
- **Config struct**: `ServerConfig` in `config.go` — pattern for adding new config sections

---

## Verification

1. **Build**: `bazel build //apps/go/server:opensqs-server` — server binary compiles with all new code
2. **Unit tests (handlers)**: `bazel test //apps/go/server/handlers/tests:go_default_test` — all existing + new move task tests pass
3. **Unit tests (dlq)**: `bazel test //pkgs/v1/queue/dlq/tests:go_default_test` — move task manager tests pass
4. **Unit tests (metrics)**: `bazel test //apps/go/server/metrics/tests:go_default_test` — metrics registration and counter tests pass
5. **Unit tests (manager)**: `bazel test //pkgs/v1/queue/tests:go_default_test` — shutdown tests pass
6. **Unit tests (all)**: `bazel test //apps/go/server/... //pkgs/v1/queue/...` — entire test suite passes
7. **Benchmarks**: `bazel test --test_arg=-test.bench=. --test_arg=-test.benchtime=1s //pkgs/v1/queue/store/memory/tests:go_default_test` — benchmarks run and produce results
8. **Manual move task test**: Start server, create DLQ + source queue with RedrivePolicy, send messages, trigger redrive, then call `StartMessageMoveTask` via AWS CLI to move messages back from DLQ to source
9. **Manual metrics test**: Start server with metrics enabled, `curl http://localhost:9326/metrics` — verify Prometheus-format output with queue counters
10. **Manual graceful shutdown test**: Start server, send messages, trigger SIGTERM, verify logs show graceful shutdown with store closures
11. **Protocol compatibility**: Test `StartMessageMoveTask` with both AWS CLI (Query Protocol / XML) and a JSON-protocol client to verify both protocols work

---

## Decisions

- **MoveTaskManager lives in `pkgs/v1/queue/dlq/`** — follows RFC structure, co-located with `redrive.go`. The manager needs a reference to `QueueManager` to look up source/destination queues by ARN.
- **Metrics use `prometheus/client_golang`** — standard Go Prometheus library. Added as a direct dependency in `go.mod`.
- **Metrics server on port 9326** — follows the pattern of SQS (9324), UI (9325), Health (8001). Configurable via `metrics.port`.
- **Graceful shutdown extends `QueueManager`** — add `Shutdown(ctx)` method rather than modifying `Store` interface. Stores already have `Close()`. The `Shutdown()` method coordinates closing all stores and waiting for in-flight ops.
- **Benchmarks in `tests/` subfolders** — follows monorepo convention. Separate `bench_test.go` files alongside existing `*_test.go` files.
- **Rate limiting for move tasks** — use `time.Ticker` with configurable interval. If `MaxNumberOfMessagesPerSecond` is 0 or not set, no rate limiting (move as fast as possible).
- **Move task goroutine lifecycle** — `StartTask` launches a goroutine that runs until all messages are moved or the task is cancelled. The goroutine checks a `cancelled chan struct{}` on each iteration.
- **Scope: Queue persistence config excluded** — the RFC mentions "Queue persistence config — Persist queue metadata to file" but this is lower priority and not included in this plan. SQLite already persists messages; queue metadata persistence can be a follow-up.

---

## Further Considerations

1. **Move task destination queue attributes** — When moving messages to a destination queue, should we preserve original message attributes, dedup IDs, and group IDs? **Recommendation: Yes, preserve all message metadata.** The `redriveMessage` method already resets `ReceiptHandle`, `IsVisible`, and `ApproximateReceiveCount` — move tasks should do the same reset but preserve `MessageAttributes`, `MessageDeduplicationID`, `MessageGroupID`, and `SequenceNumber`.

2. **Metrics for move tasks** — Should move task progress (messages moved, tasks active) be exposed as Prometheus metrics? **Recommendation: Yes** — add `move_task_messages_moved_total` counter and `move_task_active` gauge. This is included in B2.

3. **Shutdown timeout configurability** — The current shutdown timeout is hardcoded to 10 seconds. Should it be configurable? **Recommendation: Not for Phase 4** — 10 seconds is sufficient for local/dev use. Can be made configurable in Phase 5 if production deployments need longer drain times.
