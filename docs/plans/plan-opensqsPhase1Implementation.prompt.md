# Plan: OpenSQS Phase 1 Implementation

Build the MVP SQS-compatible server: queue management, message send/receive/delete, visibility timeout, in-memory store, both wire protocols (Query/XML + JSON), error responses, health check, Docker image.

## Steps

### Phase 0: Fix Missing Bazel Infrastructure
1. Create `tools/defs.bzl` with `REGISTRY` constant (`"registry.opensqs.io"`)
2. Create `tools/platforms/BUILD.bazel` with `linux_arm64` and `linux_amd64` platform targets
3. Create `tools/platforms/transition.bzl` with `multi_arch` function (wraps `oci_image_index` with platform transitions)
4. Verify `bazel build //tools/...` passes

### Phase 1: Core Types & Constants (`pkgs/v1/queue/types/`)
5. Create `pkgs/v1/queue/types/constants.go` — Action name constants, attribute name constants, protocol content-types, default SQS version (`2012-11-05`), empty request ID
6. Create `pkgs/v1/queue/types/types.go` — `Message` struct, `MessageAttribute` (String/Number/Binary), `QueueAttributes` map type, `ReceiptHandle` struct, `SQSError` interface
7. Create `pkgs/v1/queue/types/BUILD.bazel` — `opensqs_go_library`, `//visibility:public`
8. Run `bazel run //:gazelle` to verify BUILD file generation
9. Create `pkgs/v1/queue/types/tests/types_test.go` + `BUILD.bazel` — Test Message struct serialization, attribute types
10. Run `bazel test //pkgs/v1/queue/types/tests:go_default_test`

### Phase 2: Queue Attributes & Limits (`pkgs/v1/queue/`)
11. Create `pkgs/v1/queue/attributes.go` — `QueueAttributes` struct with all SQS attributes, defaults, validation, `GetAttribute`/`SetAttribute` methods
12. Create `pkgs/v1/queue/limits.go` — `Limits` struct with strict/relaxed modes, `VerifyMessageSize`, `VerifyBatchSize`, `VerifyVisibilityTimeout`, etc.
13. Create `pkgs/v1/queue/errors.go` — `SQSError` type with `Code`, `HTTPStatusCode`, `ErrorType`, `Message`; factory functions (`InvalidAction`, `QueueDoesNotExist`, `InvalidParameterValue`, etc.)
14. Create `pkgs/v1/queue/BUILD.bazel`
15. Run `bazel run //:gazelle`
16. Create `pkgs/v1/queue/tests/` — Test attribute defaults, limit enforcement, error types
17. Run `bazel test //pkgs/v1/queue/tests:go_default_test`

### Phase 3: In-Memory Message Store (`pkgs/v1/queue/store/`)
18. Create `pkgs/v1/queue/store/store.go` — `Store` interface: `SendMessage`, `ReceiveMessages`, `DeleteMessage`, `ChangeVisibility`, `ApproximateCounts`, `Purge`
19. Create `pkgs/v1/queue/store/memory/memory.go` — In-memory implementation using `sync.Mutex`, slice of messages, visibility timeout tracking via goroutine + timer, receipt handle generation (base64-encoded JSON with queue name + message ID + timestamp + random)
20. Create `pkgs/v1/queue/store/memory/BUILD.bazel`
21. Run `bazel run //:gazelle`
22. Create `pkgs/v1/queue/store/memory/tests/` — Test send/receive/delete lifecycle, visibility timeout expiry, receipt handle validation, concurrent access
23. Run `bazel test //pkgs/v1/queue/store/memory/tests:go_default_test`

### Phase 4: Queue Manager (`pkgs/v1/queue/`)
24. Create `pkgs/v1/queue/queue.go` — `Queue` struct: name, attributes, tags, message store reference, `IsFifo()`, `URL()` method
25. Create `pkgs/v1/queue/manager.go` — `QueueManager` struct: `CreateQueue`, `DeleteQueue`, `LookupQueue`, `ListQueues`, `PurgeQueue`; uses `sync.RWMutex` for concurrent access; generates queue URLs from configured node address + account ID
26. Update `pkgs/v1/queue/BUILD.bazel` to include new files
27. Run `bazel run //:gazelle`
28. Create `pkgs/v1/queue/tests/manager_test.go` — Test create/delete/lookup/list, duplicate queue detection, URL generation
29. Run `bazel test //pkgs/v1/queue/tests:go_default_test`

### Phase 5: Protocol Layer (`apps/go/server/protocol/`)
30. Create `apps/go/server/protocol/errors.go` — `SQSErrorResponse` struct, `ToXML()` and `ToJSON()` methods, error code → AWS error type mapping
31. Create `apps/go/server/protocol/query.go` — Parse `application/x-www-form-urlencoded` requests: extract `Action` parameter, parse flat params (including batch entry indexing like `SendMessageBatchRequestEntry.1.Id`), parse attributes
32. Create `apps/go/server/protocol/json.go` — Parse `application/x-amz-json-1.0` requests: extract action from `X-Amz-Target` header, parse JSON body into typed structs
33. Create `apps/go/server/protocol/marshal.go` — Response marshalling: `MarshalXMLResponse()` and `MarshalJSONResponse()` for each action's response shape
34. Create `apps/go/server/protocol/BUILD.bazel`
35. Run `bazel run //:gazelle`
36. Create `apps/go/server/protocol/tests/` — Test both protocol parsers with sample requests, verify response XML/JSON structure
37. Run `bazel test //apps/go/server/protocol/tests:go_default_test`

### Phase 6: SQS Action Handlers (`apps/go/server/handlers/`)
38. Create `apps/go/server/handlers/handler.go` — Central dispatcher: `HandleRequest(ctx, payload, protocol)`, routes to action handler based on `Action` param, shared queue URL extraction logic
39. Create `apps/go/server/handlers/create_queue.go` — `CreateQueue` handler: validate name, parse attributes, call `QueueManager.CreateQueue`, return queue URL
40. Create `apps/go/server/handlers/delete_queue.go` — `DeleteQueue` handler
41. Create `apps/go/server/handlers/get_queue_url.go` — `GetQueueUrl` handler: parse queue name, return URL
42. Create `apps/go/server/handlers/list_queues.go` — `ListQueues` handler: optional prefix filter, return list of URLs
43. Create `apps/go/server/handlers/send_message.go` — `SendMessage` handler: validate body, parse attributes, generate message ID + MD5, call store
44. Create `apps/go/server/handlers/receive_message.go` — `ReceiveMessage` handler: parse max messages + wait time + visibility timeout, call store, format response with attributes
45. Create `apps/go/server/handlers/delete_message.go` — `DeleteMessage` handler: validate receipt handle, call store
46. Create `apps/go/server/handlers/get_queue_attributes.go` — `GetQueueAttributes` handler: return requested attributes (or all)
47. Create `apps/go/server/handlers/set_queue_attributes.go` — `SetQueueAttributes` handler
48. Create `apps/go/server/handlers/purge_queue.go` — `PurgeQueue` handler
49. Create `apps/go/server/handlers/BUILD.bazel`
50. Run `bazel run //:gazelle`
51. Create `apps/go/server/handlers/tests/` — Test each handler with both protocols, verify response shapes match AWS SQS
52. Run `bazel test //apps/go/server/handlers/tests:go_default_test`

### Phase 7: Server Bootstrap (`apps/go/server/`)
53. Create `apps/go/server/config.go` — `ServerConfig` struct implementing `config.ConfigI[ServerConfig]`: server host/port, node address, AWS region/account-id, storage type, logging level
54. Create `apps/go/server/config.yaml` — Local dev config: port 9324, memory storage, strict limits
55. Create `apps/go/server/main.go` — Entry point: load config via `config.NewConfigFromEnv`, init logger, create `QueueManager`, start HTTP server with chi router, route all paths to protocol dispatcher, start health check server on port 8001, graceful shutdown on SIGTERM
56. Create `apps/go/server/health/server.go` — Simple `/health` → 200 OK HTTP server on port 8001
57. Create `apps/go/server/BUILD.bazel` — `opensqs_go_library` + `opensqs_go_binary` with `auto_load_config = True` + `data = ["config.yaml"]`
58. Run `bazel run //:gazelle`
59. Build: `bazel build //apps/go/server:server`
60. Run locally: `bazel run //apps/go/server:server` and test with `aws --endpoint-url http://localhost:9324 sqs create-queue --queue-name test`

### Phase 8: Integration Testing with AWS CLI
61. Start server locally
62. Test CreateQueue: `aws --endpoint-url http://localhost:9324 sqs create-queue --queue-name test-queue`
63. Test ListQueues: `aws --endpoint-url http://localhost:9324 sqs list-queues`
64. Test GetQueueUrl: `aws --endpoint-url http://localhost:9324 sqs get-queue-url --queue-name test-queue`
65. Test SendMessage: `aws --endpoint-url http://localhost:9324 sqs send-message --queue-url $URL --message-body "hello"`
66. Test ReceiveMessage: `aws --endpoint-url http://localhost:9324 sqs receive-message --queue-url $URL`
67. Test DeleteMessage: `aws --endpoint-url http://localhost:9324 sqs delete-message --queue-url $URL --receipt-handle $HANDLE`
68. Test GetQueueAttributes: `aws --endpoint-url http://localhost:9324 sqs get-queue-attributes --queue-url $URL --attribute-names All`
69. Test SetQueueAttributes: `aws --endpoint-url http://localhost:9324 sqs set-queue-attributes --queue-url $URL --attributes VisibilityTimeout=60`
70. Test PurgeQueue: `aws --endpoint-url http://localhost:9324 sqs purge-queue --queue-url $URL`
71. Test DeleteQueue: `aws --endpoint-url http://localhost:9324 sqs delete-queue --queue-url $URL`
72. Test error cases: missing queue, invalid params, etc.

### Phase 9: Docker Image
73. Add `opensqs_go_image` target to `apps/go/server/BUILD.bazel`
74. Build image: `bazel build //apps/go/server:server_image`
75. Load into Docker: `bazel run //apps/go/server:server_image_base_img_load_docker`
76. Test: `docker run -p 9324:9324 -p 8001:8001 registry.opensqs.io/server:latest`
77. Repeat AWS CLI tests against containerized server

## Relevant files
- `tools/defs.bzl` — Create with REGISTRY constant (referenced by `tools/rules/golang/defs.bzl` but missing)
- `tools/platforms/BUILD.bazel` — Create with linux_arm64/linux_amd64 platform targets (referenced but missing)
- `tools/platforms/transition.bzl` — Create with `multi_arch` function (referenced but missing)
- `pkgs/v1/queue/types/constants.go` — Action names, attribute names, content-types, SQS version
- `pkgs/v1/queue/types/types.go` — Message, MessageAttribute, QueueAttributes, ReceiptHandle, SQSError types
- `pkgs/v1/queue/attributes.go` — Queue attribute definitions, defaults, validation
- `pkgs/v1/queue/limits.go` — SQS limit enforcement (strict/relaxed)
- `pkgs/v1/queue/errors.go` — SQSError type with Code/HTTPStatus/Type, factory functions
- `pkgs/v1/queue/queue.go` — Queue struct with name, attributes, tags, store
- `pkgs/v1/queue/manager.go` — QueueManager: create/delete/lookup/list/purge
- `pkgs/v1/queue/store/store.go` — Store interface
- `pkgs/v1/queue/store/memory/memory.go` — In-memory store with visibility timeout tracking
- `apps/go/server/protocol/query.go` — AWS Query Protocol parser (form-urlencoded)
- `apps/go/server/protocol/json.go` — AWS JSON Protocol 1.0 parser
- `apps/go/server/protocol/marshal.go` — XML/JSON response marshalling
- `apps/go/server/protocol/errors.go` — SQS error response formatting
- `apps/go/server/handlers/handler.go` — Central request dispatcher
- `apps/go/server/handlers/create_queue.go` — CreateQueue action handler
- `apps/go/server/handlers/send_message.go` — SendMessage action handler
- `apps/go/server/handlers/receive_message.go` — ReceiveMessage action handler
- `apps/go/server/handlers/delete_message.go` — DeleteMessage action handler
- `apps/go/server/config.go` — ServerConfig struct implementing config.ConfigI
- `apps/go/server/config.yaml` — Local dev config
- `apps/go/server/main.go` — Server entry point with config/logger/HTTP/health
- `apps/go/server/health/server.go` — Health check server on port 8001
- `apps/go/server/BUILD.bazel` — opensqs_go_library + opensqs_go_binary + opensqs_go_image
- `pkgs/v1/config/config.go` — Existing config pattern to follow (config.NewConfigFromEnv)
- `pkgs/v1/logger/factory.go` — Existing logger pattern to follow (logger.New)
- `pkgs/v1/environment/env.go` — Existing env enum
- `apps/go/playground/cmd_hello_world/BUILD.bazel` — Existing binary BUILD pattern to follow

## Verification
1. `bazel build //tools/...` — Bazel infrastructure compiles
2. `bazel test //pkgs/v1/queue/types/tests:go_default_test` — Types pass
3. `bazel test //pkgs/v1/queue/tests:go_default_test` — Attributes/limits/errors pass
4. `bazel test //pkgs/v1/queue/store/memory/tests:go_default_test` — Store passes
5. `bazel test //apps/go/server/protocol/tests:go_default_test` — Protocol parsing passes
6. `bazel test //apps/go/server/handlers/tests:go_default_test` — Handlers pass
7. `bazel build //apps/go/server:server` — Server binary builds
8. `bazel run //apps/go/server:server` then AWS CLI smoke test — Server runs and responds to SQS API calls
9. `aws --endpoint-url http://localhost:9324 sqs create-queue --queue-name test` — Returns valid QueueUrl
10. Full send/receive/delete cycle via AWS CLI — Messages flow correctly
11. `bazel build //apps/go/server:server_image` — Docker image builds
12. `docker run` + AWS CLI test — Containerized server works

## Decisions
- **Phase 1 scope only**: In-memory store, no FIFO, no DLQ, no batch, no tags, no permissions, no message move tasks. These are Phase 2 per RFC.
- **Both protocols from day 1**: Query (XML) and JSON 1.0 must both work — AWS CLI uses Query, aws-sdk-go-v2 uses JSON.
- **No new Go deps for Phase 1**: Use stdlib `net/http` + `encoding/xml` + `encoding/json`. Huma v2 / chi router deferred — the SQS protocol dispatch is custom enough that raw `net/http` is simpler for MVP. Can migrate to Huma later.
- **Receipt handles**: base64-encoded JSON containing queue name, message ID, receive timestamp, random nonce. Signed with server secret (HMAC-SHA256).
- **Message IDs**: UUID v4 (using `crypto/rand`).
- **MD5 checksums**: Compute MD5 of message body and message attributes (matching AWS SQS behavior).
- **Fix missing Bazel infra first**: `tools/defs.bzl` and `tools/platforms/` are referenced but don't exist — must create before any image build works.

## Further Considerations
1. **Router choice**: Use raw `net/http` for Phase 1 (simpler, no deps) vs Huma v2 + chi (RFC says Huma). Recommendation: raw net/http for MVP, migrate when adding UI in Phase 3.
2. **Visibility timeout implementation**: Background goroutine per queue with ticker vs per-message timer. Recommendation: per-message timer channel for precision, cleaned up on delete.
3. **Long polling implementation**: `select` with `time.After` channel vs condition variable. Recommendation: channel-based with per-queue notify channel.
