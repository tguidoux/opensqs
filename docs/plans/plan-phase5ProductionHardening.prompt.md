# Plan: Phase 5 — Production Hardening

## TL;DR

Phase 5 hardens OpenSQS for production deployments. Six areas: (1) TLS/HTTPS support for all four HTTP servers, (2) HTTP middleware for structured request logging, (3) per-queue and global rate limiting, (4) BadgerDB as an alternative storage backend, (5) a Helm chart for Kubernetes deployment, and (6) an integration test suite using `aws-sdk-go-v2`. Multi-arch Docker is already implemented via `tools/platforms/transition.bzl` and needs only verification. The implementation introduces a middleware chain in `main.go` (currently bare `http.NewServeMux`), adds TLS config to `ServerHTTPConfig`, extends `SQSConfig` with BadgerDB path, and creates a new `pkgs/v1/queue/store/badger/` package implementing the existing `Store` interface.

---

## Steps

### Phase A: HTTP Middleware Infrastructure

**A1.** Create `apps/go/server/middleware/middleware.go` — define a `Middleware` type (`func(http.Handler) http.Handler`) and a `Chain` function that composes middlewares. Foundation for request logging (Phase B) and rate limiting (Phase C). *No dependency — can start immediately.*

**A2.** Create `apps/go/server/middleware/response_writer.go` — define a `statusResponseWriter` struct that wraps `http.ResponseWriter` and captures the HTTP status code and response size. Used by the request logging middleware. *Depends on A1.*

**A3.** Create `apps/go/server/middleware/BUILD.bazel` — `opensqs_go_library` with `//visibility:public`, deps on `//pkgs/v1/logger:go_default_library`. *Depends on A1, A2.*

**A4.** Run `bazel run //:gazelle` to auto-generate BUILD entries. *Depends on A1-A3.*

### Phase B: Structured Request Logging

**B1.** Create `apps/go/server/middleware/request_logger.go` — implement `RequestLogger(log logger.LoggerInterface) Middleware`. Generates a request ID (UUID via `crypto/rand`), records start time, wraps ResponseWriter with `statusResponseWriter`, calls `next.ServeHTTP`, logs method/path/status/duration/requestID/remoteAddr using structured logger. *Depends on A1, A2.*

**B2.** Add `RequestLogging` config to `apps/go/server/config.go` — `RequestLoggingConfig` struct with `Enabled bool` field. Add `RequestLogging RequestLoggingConfig` to `ServerConfig`. *Parallel with B1.*

**B3.** Wire request logging middleware in `apps/go/server/main.go` — wrap the SQS mux with `middleware.Chain(middleware.RequestLogger(log), mux)` when `cfg.RequestLogging.Enabled`. Apply to UI server too. *Depends on A1, B1, B2.*

**B4.** Add `requestLogging` section to `apps/go/server/config.yaml` with `enabled: false`. *Depends on B2.*

**B5.** Write tests in `apps/go/server/middleware/tests/request_logger_test.go` — test middleware logs requests, captures correct status codes, generates request IDs. Use `httptest.NewRecorder` and a test handler. *Depends on B1.*

### Phase C: Rate Limiting

**C1.** Add `golang.org/x/time/rate` to `go.mod`, run `bazel run //:go.clean` and `bazel mod tidy`. *No code dependency — can start immediately.*

**C2.** Create `apps/go/server/middleware/rate_limiter.go` — implement two middlewares:
  - `GlobalRateLimiter(rps float64, burst int) Middleware` — single `rate.Limiter` for all requests.
  - `PerQueueRateLimiter(rps float64, burst int) Middleware` — extracts queue name from URL path (`/{accountId}/{queueName}`), maintains `sync.Map` of per-queue `rate.Limiter` instances. Returns `429 Too Many Requests` with SQS error XML/JSON when rate exceeded. *Depends on A1.*

**C3.** Add `RateLimit` config to `apps/go/server/config.go` — `RateLimitConfig` struct with `Enabled bool`, `RequestsPerSecond float64`, `Burst int`, `PerQueue bool` fields. Add `RateLimit RateLimitConfig` to `ServerConfig`. *Parallel with C2.*

**C4.** Wire rate limiting in `apps/go/server/main.go` — when `cfg.RateLimit.Enabled`, wrap the SQS mux with the appropriate rate limiter middleware (global or per-queue based on config). *Depends on C2, C3.*

**C5.** Add `rateLimit` section to `apps/go/server/config.yaml` with `enabled: false`, `requestsPerSecond: 1000`, `burst: 100`, `perQueue: false`. *Depends on C3.*

**C6.** Write tests in `apps/go/server/middleware/tests/rate_limiter_test.go` — test global rate limiter returns 429 after burst exhausted, per-queue limiter isolates queues, requests succeed under limit. *Depends on C2.*

### Phase D: TLS/HTTPS Support

**D1.** Add TLS config fields to `apps/go/server/config.go` — extend `ServerHTTPConfig` with `TLS TLSConfig` field. Define `TLSConfig` struct: `Enabled bool`, `CertFile string`, `KeyFile string`. Add TLS fields to `UIConfig`, `MetricsConfig`, and `HealthConfig` as well. *No dependency — can start immediately.*

**D2.** Create `apps/go/server/tls/tls.go` — helper `LoadTLSConfig(certFile, keyFile string) (*tls.Config, error)` that loads cert + key, validates them, returns `*tls.Config`. Returns `nil, nil` when both paths empty (TLS disabled). *Depends on D1.*

**D3.** Create `apps/go/server/tls/BUILD.bazel` — `opensqs_go_library` with stdlib deps only. *Depends on D2.*

**D4.** Update `apps/go/server/main.go` — for each server (SQS, UI, metrics, health): if TLS enabled, load `tls.Config`, set on `http.Server.TLSConfig`, call `ListenAndServeTLS(certFile, keyFile)` instead of `ListenAndServe()`. Add helper `startServer(srv *http.Server, tlsCfg *tls.Config, certFile, keyFile string) error`. *Depends on D1, D2.*

**D5.** Update `apps/go/server/ui/server.go` — add `TLSConfig` field to `Server` struct, use in `Start()` method. Update `NewServer` to accept optional TLS config. *Depends on D1, D2.*

**D6.** Update `apps/go/server/metrics/server.go` — same pattern as D5. *Depends on D1, D2.*

**D7.** Update `apps/go/server/health/server.go` — same pattern. *Depends on D1, D2.*

**D8.** Add TLS config sections to `apps/go/server/config.yaml` — commented-out examples for each server. *Depends on D1.*

**D9.** Write tests in `apps/go/server/tls/tests/tls_test.go` — test `LoadTLSConfig` with valid self-signed certs (generate in test), test error cases (missing file, invalid cert). *Depends on D2.*

### Phase E: BadgerDB Storage Backend

**E1.** Add `github.com/dgraph-io/badger/v4` to `go.mod`, run `bazel run //:go.clean` and `bazel mod tidy`. *No code dependency — can start immediately.*

**E2.** Create `pkgs/v1/queue/store/badger/badger.go` — implement `BadgerStore` struct implementing the `store.Store` interface (9 methods). Follow the `SQLiteStore` pattern: lazy visibility timeout evaluation (no goroutines), FIFO support with dedup cache, sequence counter, DLQ redrive support. Key = message ID, value = JSON-serialized message with metadata. Single shared `*badger.DB` instance across queues. *Depends on E1.*

**E3.** Create `pkgs/v1/queue/store/badger/BUILD.bazel` — `opensqs_go_library` with `//visibility:public`, deps on `@com_github_dgraph_io_badger_v4//`, `//pkgs/v1/queue/store:go_default_library`, `//pkgs/v1/queue/types:go_default_library`. *Depends on E2.*

**E4.** Add `BadgerPath` field to `SQSConfig` in `apps/go/server/config.go`. Update `StorageType` comment to include "badger". *No dependency — can start immediately.*

**E5.** Wire BadgerDB in `apps/go/server/main.go` — add third branch in store factory: `if cfg.SQS.StorageType == "badger" && cfg.SQS.BadgerPath != ""`. Open single BadgerDB instance, pass to `NewBadgerStore`. Add `defer db.Close()`. *Depends on E2, E4.*

**E6.** Add `badgerPath` to `apps/go/server/config.yaml` (commented out). *Depends on E4.*

**E7.** Write tests in `pkgs/v1/queue/store/badger/tests/badger_test.go` — test all Store interface methods: send/receive/delete, visibility timeout expiry, change visibility, purge, approximate counts, FIFO ordering + dedup, DLQ redrive. Follow `sqlite/tests/sqlite_test.go` pattern. *Depends on E2.*

**E8.** Write benchmarks in `pkgs/v1/queue/store/badger/tests/bench_test.go` — `BenchmarkSendMessage`, `BenchmarkReceiveMessage`, `BenchmarkDeleteMessage` using `100x` iteration pattern. *Depends on E2.*

**E9.** Run `bazel run //:gazelle` to generate BUILD files. *Depends on E2, E3, E7, E8.*

### Phase F: Helm Chart

**F1.** Create `deploy/helm/Chart.yaml` — chart metadata: `apiVersion: v2`, `name: opensqs`, `version: 0.1.0`, `appVersion: "0.1.0"`. *No dependency — can start immediately.*

**F2.** Create `deploy/helm/values.yaml` — default values: image repository/tag, service ports (9324, 9325, 9326, 8001), storage type, resources, probes, TLS settings, config overrides. *No dependency — can start immediately.*

**F3.** Create `deploy/helm/templates/_helpers.tpl` — template helpers: `opensqs.fullname`, `opensqs.labels`, `opensqs.selectorLabels`. *No dependency — can start immediately.*

**F4.** Create `deploy/helm/templates/deployment.yaml` — K8s Deployment with: container ports for all 4 servers, liveness/readiness probes on port 8001 `/health`, volume mounts for config and data, env vars for config path, resource limits. *Depends on F1, F2, F3.*

**F5.** Create `deploy/helm/templates/service.yaml` — K8s Service exposing port 9324 (SQS API) and optionally 9325 (UI). *Depends on F1, F2.*

**F6.** Create `deploy/helm/templates/configmap.yaml` — ConfigMap containing `config.yaml` content derived from values.yaml. *Depends on F2.*

**F7.** Create `deploy/helm/templates/pvc.yaml` — PersistentVolumeClaim for data storage (when storageType is sqlite or badger). Conditional on values. *Depends on F2.*

**F8.** Create `deploy/helm/templates/NOTES.txt` — post-install notes with connection instructions. *Depends on F4, F5.*

**F9.** Create `deploy/helm/templates/ingress.yaml` — optional Ingress for SQS API and UI. Conditional on values. *Depends on F2.*

### Phase G: Integration Test Suite

**G1.** Add `github.com/aws/aws-sdk-go-v2` and `github.com/aws/aws-sdk-go-v2/service/sqs` to `go.mod`, run `bazel run //:go.clean` and `bazel mod tidy`. *No code dependency — can start immediately.*

**G2.** Create `apps/go/server/integration/test_helper.go` — `startTestServer(t)` function that boots a full OpenSQS server on a random port, returns the server URL and a cleanup function. `newSQSClient(t, endpoint string)` creates an `aws-sdk-go-v2` SQS client pointed at the test server. *Depends on G1.*

**G3.** Create `apps/go/server/integration/queue_test.go` — test `CreateQueue`, `ListQueues`, `GetQueueUrl`, `DeleteQueue`, `PurgeQueue` via the AWS SDK. Verify response shapes match real SQS. *Depends on G2.*

**G4.** Create `apps/go/server/integration/message_test.go` — test `SendMessage`, `ReceiveMessage`, `DeleteMessage`, `ChangeMessageVisibility` via the AWS SDK. Test visibility timeout expiry, long polling, message attributes. *Depends on G2.*

**G5.** Create `apps/go/server/integration/batch_test.go` — test `SendMessageBatch`, `DeleteMessageBatch`, `ChangeMessageVisibilityBatch` via the AWS SDK. *Depends on G2.*

**G6.** Create `apps/go/server/integration/fifo_test.go` — test FIFO queue semantics: message group ordering, deduplication, exactly-once delivery. *Depends on G2.*

**G7.** Create `apps/go/server/integration/dlq_test.go` — test DLQ redrive: messages exceed maxReceiveCount, appear in DLQ, `StartMessageMoveTask` moves them back. *Depends on G2.*

**G8.** Create `apps/go/server/integration/BUILD.bazel` — `opensqs_go_test` target. *Depends on G2-G7.*

**G9.** Run `bazel run //:gazelle` to generate BUILD files. *Depends on G8.*

### Phase H: Verification

**H1.** Run `bazel run //:gazelle` — generates BUILD.bazel for all new packages. *Depends on all above.*

**H2.** Run `bazel run //:bazel.clean` — formats all Bazel files. *Depends on H1.*

**H3.** Run `bazel build //apps/go/server:opensqs-server` — verify server builds with all changes. *Depends on H1, H2.*

**H4.** Run `bazel test //...` — verify all tests pass (existing + new). *Depends on H3.*

**H5.** Verify multi-arch Docker image: `bazel build //apps/go/server:opensqs_server_image` — confirm both `linux_arm64` and `linux_amd64` are produced. Already implemented via `tools/platforms/transition.bzl`, needs verification only. *Depends on H3.*

**H6.** Update `docs/configuration.md` with new config sections (TLS, rate limiting, request logging, BadgerDB). *Depends on D1, C3, B2, E4.*

---

## Relevant Files

### New Files
- `apps/go/server/middleware/middleware.go` — Middleware type + Chain function
- `apps/go/server/middleware/response_writer.go` — status-capturing ResponseWriter
- `apps/go/server/middleware/request_logger.go` — structured request logging middleware
- `apps/go/server/middleware/rate_limiter.go` — global + per-queue rate limiting middleware
- `apps/go/server/middleware/tests/request_logger_test.go` — request logger tests
- `apps/go/server/middleware/tests/rate_limiter_test.go` — rate limiter tests
- `apps/go/server/tls/tls.go` — TLS config loader helper
- `apps/go/server/tls/tests/tls_test.go` — TLS loader tests
- `pkgs/v1/queue/store/badger/badger.go` — BadgerDB Store implementation
- `pkgs/v1/queue/store/badger/tests/badger_test.go` — BadgerDB store tests
- `pkgs/v1/queue/store/badger/tests/bench_test.go` — BadgerDB benchmarks
- `deploy/helm/Chart.yaml` — Helm chart metadata
- `deploy/helm/values.yaml` — Helm default values
- `deploy/helm/templates/_helpers.tpl` — Helm template helpers
- `deploy/helm/templates/deployment.yaml` — K8s Deployment
- `deploy/helm/templates/service.yaml` — K8s Service
- `deploy/helm/templates/configmap.yaml` — K8s ConfigMap
- `deploy/helm/templates/pvc.yaml` — K8s PersistentVolumeClaim
- `deploy/helm/templates/ingress.yaml` — K8s Ingress (optional)
- `deploy/helm/templates/NOTES.txt` — Post-install notes
- `apps/go/server/integration/test_helper.go` — Integration test server bootstrap
- `apps/go/server/integration/queue_test.go` — Queue management integration tests
- `apps/go/server/integration/message_test.go` — Message operations integration tests
- `apps/go/server/integration/batch_test.go` — Batch operations integration tests
- `apps/go/server/integration/fifo_test.go` — FIFO semantics integration tests
- `apps/go/server/integration/dlq_test.go` — DLQ redrive integration tests

### Modified Files
- `apps/go/server/config.go` — add `TLSConfig`, `RequestLoggingConfig`, `RateLimitConfig`, `BadgerPath` fields
- `apps/go/server/config.yaml` — add TLS, requestLogging, rateLimit, badgerPath sections
- `apps/go/server/main.go` — wire middleware chain, TLS support, BadgerDB store factory branch
- `apps/go/server/ui/server.go` — add TLS support to `Server` struct and `Start()` method
- `apps/go/server/metrics/server.go` — add TLS support to `Server` struct and `Start()` method
- `apps/go/server/health/server.go` — add TLS support to `Server` struct and `Start()` method
- `go.mod` — add `golang.org/x/time`, `github.com/dgraph-io/badger/v4`, `github.com/aws/aws-sdk-go-v2`
- `MODULE.bazel` — add `use_repo` entries for new deps
- `docs/configuration.md` — document new config sections

### Reference Files (patterns to follow)
- `pkgs/v1/queue/store/store.go` — `Store` interface (9 methods) that BadgerDB must implement
- `pkgs/v1/queue/store/sqlite/sqlite.go` — `SQLiteStore` pattern to copy for `BadgerStore` (lazy visibility, FIFO, dedup, DLQ)
- `pkgs/v1/queue/store/memory/memory.go` — `MemoryStore` pattern (timer-based visibility, FIFO groups)
- `apps/go/server/health/server.go` — Server lifecycle pattern (Start/Stop) to extend with TLS
- `apps/go/server/request_handler.go` — `handleSQSRequest` function where middleware wraps
- `tools/platforms/transition.bzl` — `multi_arch` function (already handles amd64+arm64)
- `tools/rules/golang/defs.bzl` — `opensqs_go_image` macro (already multi-arch)

---

## Verification

1. `bazel run //:gazelle` — generates BUILD.bazel for all new packages
2. `bazel run //:bazel.clean` — formats all Bazel files
3. `bazel build //apps/go/server:opensqs-server` — server builds with all changes
4. `bazel test //apps/go/server/middleware/tests:go_default_test` — middleware tests pass
5. `bazel test //apps/go/server/tls/tests:go_default_test` — TLS tests pass
6. `bazel test //pkgs/v1/queue/store/badger/tests:go_default_test` — BadgerDB store tests pass
7. `bazel test //apps/go/server/integration:go_default_test` — integration tests pass against embedded server
8. `bazel test //...` — all existing + new tests pass
9. `bazel build //apps/go/server:opensqs_server_image` — multi-arch image builds (verify both platforms)
10. `helm template deploy/helm/` — Helm chart renders without errors
11. `helm lint deploy/helm/` — Helm chart passes linting
12. Manual: start server with TLS enabled, verify `curl -k https://localhost:9324` works
13. Manual: start server with rate limiting enabled, verify 429 responses after burst
14. Manual: start server with request logging, verify structured log output

---

## Decisions

- **Middleware approach**: Introduce a simple `Middleware` type + `Chain` function rather than adopting a router framework (chi, gorilla). The codebase uses stdlib `net/http` throughout — keep it consistent.
- **TLS scope**: TLS applies to all four HTTP servers (SQS, UI, metrics, health) independently. Each has its own `TLSConfig` in the config. This allows TLS on the SQS API while keeping health checks plain HTTP (common in K8s).
- **Rate limiting strategy**: Use `golang.org/x/time/rate` (token bucket). Global limiter wraps the entire mux; per-queue limiter extracts queue name from URL path. Return SQS-formatted error (not plain 429) for SDK compatibility.
- **BadgerDB design**: Follow the `SQLiteStore` pattern (lazy visibility timeout evaluation, no goroutines). Single shared `*badger.DB` instance across queues (like SQLite's shared `*sql.DB`). Key-prefix per queue.
- **Integration tests**: Use `aws-sdk-go-v2` (Go SDK) only — Python/Node.js/Java tests are out of scope for this monorepo (no build system support for those languages). Go integration tests provide wire-level compatibility verification.
- **Helm chart**: Self-contained in `deploy/helm/` — no external chart dependencies. ConfigMap-based configuration (not env vars) to match the existing `config.yaml` pattern.
- **Multi-arch Docker**: Already implemented via `tools/platforms/transition.bzl` — Phase 5 only verifies it works, no new code needed.
- **Request logging**: Off by default (configurable). Uses the existing `LoggerInterface` for structured output. Request IDs are UUIDs generated via `crypto/rand`.

## Further Considerations

1. **CORS middleware**: Should we add CORS headers for the UI server? The UI is same-origin currently, but if served behind a different domain, CORS would be needed. Recommendation: add a simple CORS middleware as part of Phase A, enabled via config. Option A: Add now / Option B: Defer to when needed.
2. **BadgerDB vs SQLite performance**: Should we run comparative benchmarks? Recommendation: Yes, extend `docs/benchmarks.md` with BadgerDB results alongside memory and SQLite. Option A: Benchmark in Phase E / Option B: Separate effort.
3. **Helm chart testing**: Should we add `helm unittest` or `ct` (chart testing) to CI? Recommendation: Add `helm lint` to CI now, defer `helm unittest` to future. Option A: Lint only / Option B: Full chart tests.
