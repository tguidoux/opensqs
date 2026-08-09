# OpenSQS PR Review Report — Open-Source Readiness

**Date:** August 2026  
**Scope:** Full PR (148 files, 24,623 lines)  
**Reviewer:** Automated comprehensive review (7 parallel reviews covering all code)  

---

## Executive Summary

The codebase demonstrates solid architectural foundations with clear separation of concerns, good use of interfaces, and reasonable test coverage. However, **before open-sourcing**, several critical issues must be addressed:

- **5 critical security vulnerabilities** (XSS, CSRF, SQL injection, plaintext secrets)
- **8 critical correctness bugs** (data loss, deadlocks, data races, broken batch operations)
- **Numerous high-severity issues** (ignored errors, missing validation, resource leaks)

**Recommendation:** Fix all Critical and High issues before merging. Medium and Low issues should be tracked as follow-up work.

### Issue Count by Severity

| Severity | Count |
|----------|-------|
| **Critical** | 13 |
| **High** | 36 |
| **Medium** | 54 |
| **Low** | 48 |
| **Total** | **151** |

---

## Table of Contents

1. [Critical Issues](#1-critical-issues)
2. [High Severity Issues](#2-high-severity-issues)
3. [Medium Severity Issues](#3-medium-severity-issues)
4. [Low Severity Issues](#4-low-severity-issues)
5. [Positive Observations](#5-positive-observations)
6. [Action Items Checklist](#6-action-items-checklist)

---

## 1. Critical Issues

### 1.1 XSS via `innerHTML` in Web UI JavaScript

**File:** `apps/go/server/ui/static/app.js`  
**Severity:** Critical (Security)

The auto-refresh JavaScript injects server data (queue names, message bodies, URLs) directly into `innerHTML` without any escaping. An attacker who sends a message with body `<img src=x onerror=alert(document.cookie)>` will execute arbitrary JavaScript in every visitor's browser.

**Fix:** Use `textContent` or implement an `escapeHtml()` helper for all dynamic content.

---

### 1.2 No CSRF Protection on State-Changing POST Endpoints

**File:** `apps/go/server/ui/server.go`, `apps/go/server/ui/handlers.go`  
**Severity:** Critical (Security)

All state-changing operations (create/delete/purge queue, send/delete message) are simple POST forms with no CSRF token, no `SameSite` cookie enforcement, and no `Origin`/`Referer` checking.

**Fix:** Implement CSRF tokens (e.g., `gorilla/csrf`) or at minimum check the `Origin` header.

---

### 1.3 SQL Injection via Table Name Interpolation

**File:** `pkgs/v1/queue/store/sqlite/sqlite.go`  
**Severity:** Critical (Security)

Every SQL query uses `fmt.Sprintf` to interpolate the table name. The `tableName()` sanitization uses a denylist approach with a byte-level iteration bug that can split multi-byte UTF-8 characters.

**Fix:** Use a strict allowlist validation (`^[a-zA-Z0-9_-]+$`) at the factory level, or use a fixed table name with a `queue_name` column (which already exists) and parameterize in WHERE clauses.

---

### 1.4 Default `serverSecret` in Helm Chart — Receipt Forgery

**File:** `deploy/helm/values.yaml`  
**Severity:** Critical (Security)

`serverSecret: "change-me-in-production"` is a hardcoded default. If a user installs the chart without overriding this, receipt handles are signed with a publicly known key, allowing receipt forgery.

**Fix:** Remove the default value (set to `""`), and add a `fail` template if both `serverSecret` and `existingSecret` are empty.

---

### 1.5 Server Secret Stored in Plaintext ConfigMap

**File:** `deploy/helm/templates/configmap.yaml`, `deploy/helm/templates/_helpers.tpl`  
**Severity:** Critical (Security)

The `opensqs.serverSecret` helper outputs the raw secret value directly into a ConfigMap, visible to anyone with `kubectl get cm`.

**Fix:** Use a Kubernetes `Secret` instead of a ConfigMap for the server secret.

---

### 1.6 Message ID Generation Not Unique Under Concurrency

**File:** `apps/go/server/handlers/actions.go` (lines 825-827)  
**Severity:** Critical (Correctness)

```go
func generateMessageID() string {
    return fmt.Sprintf("%x", time.Now().UnixNano())
}
```

Two messages sent in the same nanosecond get the same ID, causing message corruption, deduplication failures, and receipt handle collisions.

**Fix:** Use `github.com/google/uuid`:
```go
func generateMessageID() string {
    return uuid.NewString()
}
```

---

### 1.7 Request ID Always Returns Zero UUID

**File:** `apps/go/server/handlers/actions.go` (lines 829-831)  
**Severity:** Critical (Observability)

```go
func newRequestID() string {
    return types.EmptyRequestID // "00000000-0000-0000-0000-000000000000"
}
```

Every response returns the same zero UUID, making request tracing and debugging impossible.

**Fix:** Generate a real UUID for each request.

---

### 1.8 Batch Operations Broken via Query Protocol

**File:** `apps/go/server/protocol/query.go`, `apps/go/server/handlers/adapter.go`  
**Severity:** Critical (Correctness)

`QueryBatchEntry` is missing `ReceiptHandle` and `VisibilityTimeout` fields. This means `DeleteMessageBatch` and `ChangeMessageVisibilityBatch` via Query Protocol will always fail — every entry has an empty `ReceiptHandle`.

**Fix:** Add `ReceiptHandle` and `VisibilityTimeout` fields to `QueryBatchEntry`, parse them correctly, and map them in the adapter.

---

### 1.9 Message Loss in DLQ Move Task

**File:** `pkgs/v1/queue/dlq/move_task.go` (lines 226-237)  
**Severity:** Critical (Data Loss)

`runTask` deletes from the source queue **before** sending to the destination. If `SendMessage` to the destination fails, the message is permanently lost.

**Fix:** Send to destination first, then delete from source:
```go
if err := destQueue.Store().SendMessage(ctx, msg, 0); err != nil {
    continue // message will become visible again
}
sourceQueue.Store().DeleteMessage(ctx, msg.ReceiptHandle)
```

---

### 1.10 Deadlock Risk in Redrive Callback

**File:** `pkgs/v1/queue/manager.go` (lines 180-194), `pkgs/v1/queue/store/memory/memory.go`  
**Severity:** Critical (Deadlock)

`redriveMessage` is called as a `RedriveFunc` callback from inside the store's `visibilityTimer` goroutine while the store's own mutex is held. The callback then acquires `qm.mu.RLock()` and the DLQ store's mutex, creating a lock-ordering hazard.

**Fix:** Release the store mutex before calling `redriveFunc`, or dispatch the callback asynchronously (`go qm.redriveMessage(...)`).

---

### 1.11 Badger Store Missing DLQ Redrive

**File:** `pkgs/v1/queue/store/badger/badger.go`  
**Severity:** Critical (Correctness)

The Badger store has `maxReceiveCount` and `redriveFunc` fields but **no `redriveIfNeededLocked` method** and it's never called in `ReceiveMessages`. DLQ redrive is completely broken — messages will never be redrived to the DLQ.

**Fix:** Implement `redriveIfNeededLocked` for the Badger store, similar to the SQLite implementation.

---

### 1.12 Badger Store FIFO Violation

**File:** `pkgs/v1/queue/store/badger/badger.go` (lines 289-296)  
**Severity:** Critical (Correctness)

The FIFO in-flight check only tracks groups seen in the current scan, not groups with in-flight messages in the database. The Badger implementation will deliver multiple messages from the same group concurrently, violating FIFO guarantees.

**Fix:** During the scan, check if `sm.ReceiptHandle != ""` and `sm.VisibleAt > nowMilli` — if so, mark the group as in-flight and skip other messages in that group.

---

### 1.13 ConfigMap Duplicate `server:` Key

**File:** `deploy/helm/templates/configmap.yaml`  
**Severity:** Critical (Bug)

The `server:` key appears twice in the generated YAML. When TLS is enabled, `server.host` and `server.port` are silently overwritten by the second `server:` block.

**Fix:** Move the TLS block inside the first `server:` block, or restructure to avoid duplicate keys.

---

## 2. High Severity Issues

### Security

| # | File | Issue |
|---|------|-------|
| H1 | `apps/go/server/ui/handlers.go` | JSON injection via `RedrivePolicy` construction — `dlqArn` interpolated into JSON string without escaping |
| H2 | `apps/go/server/ui/server.go` | No security headers (CSP, X-Frame-Options, X-Content-Type-Options, HSTS) |
| H3 | `apps/go/server/config.yaml` | Hardcoded secret `"dev-secret-key-change-in-production"` in config file that will be open-sourced |
| H4 | `apps/go/server/config.go` | `Validate()` is a no-op — no validation of required fields, port ranges, storage types |
| H5 | `deploy/helm/values.prod.yaml` | Both `serverSecret` and `existingSecret` are empty — server starts with forgeable empty secret |
| H6 | `deploy/helm/templates/ingress.yaml` | Supports removed K8s API versions (`extensions/v1beta1`, `networking.k8s.io/v1beta1`) — dead code |
| H7 | `tools/rules/golang/defs.bzl` | Uses `@ubuntu_base` but README claims "distroless containers" — documentation/security inconsistency |
| H8 | Private IPs in docs | `192.168.1.119` and `192.168.1.153` hardcoded in `docs/shared-packages.md` — leaks internal network topology |

### Correctness & Data Integrity

| # | File | Issue |
|---|------|-------|
| H9 | `pkgs/v1/queue/manager.go` | `CreateQueue` silently ignores redrive policy parse errors — queue created without DLQ config |
| H10 | `pkgs/v1/queue/dlq/move_task.go` | `CancelTask` race condition — `MoveTaskStatusCancelling` immediately overwritten to `Cancelled`; `close(task.Cancelled)` can panic on double-call |
| H11 | `pkgs/v1/queue/dlq/move_task.go` | `MovedMessages` data race — `ListTasks`/`GetTask` return `*MoveTask` pointer, callers can read without lock |
| H12 | `pkgs/v1/queue/store/memory/memory.go` | Timer race condition — `time.AfterFunc` callback can access message after deletion, causing double-processing |
| H13 | `pkgs/v1/queue/store/memory/memory.go` | `ReceiveMessages` returns shared message pointer — caller modifications corrupt store state |
| H14 | `pkgs/v1/queue/store/sqlite/sqlite.go` | `tableName()` sanitization bug — byte-level iteration splits multi-byte UTF-8 characters |
| H15 | `pkgs/v1/queue/store/sqlite/sqlite.go` | N+1 query in `ReceiveMessages` for FIFO group checking — separate `SELECT COUNT(*)` per candidate |
| H16 | `pkgs/v1/queue/store/sqlite/sqlite.go` | `rows.Close()` not deferred in `ReceiveMessages` — resource leak on error paths |
| H17 | `pkgs/v1/queue/store/sqlite/sqlite.go` | `QueryRow().Scan()` errors silently ignored in count methods and FIFO in-flight check |
| H18 | `pkgs/v1/queue/store/sqlite/sqlite.go` | `redriveIfNeededLocked` not atomic — redrive + delete are separate operations without transaction |
| H19 | `pkgs/v1/queue/store/badger/badger.go` | `DeleteMessage`/`ChangeMessageVisibility` use separate View + Update transactions — TOCTOU race |
| H20 | `pkgs/v1/queue/store/badger/badger.go` | `ApproximateNumberOfMessages*` methods ignore `db.View` error — count silently returns 0 |
| H21 | `pkgs/v1/queue/store/store.go` | `StoreFactory` returns `Store` with no error — factory must panic or return nil on failure |
| H22 | `pkgs/v1/queue/store/store.go` | Global mutable `Now` variable — not thread-safe, causes test flakiness with `t.Parallel()` |
| H23 | `pkgs/v1/queue/queue.go` | `Tags()` returns internal map — callers can mutate directly, bypassing validation/locking |
| H24 | `pkgs/v1/queue/attributes.go` | `SetAttribute` doesn't validate numeric ranges — `VisibilityTimeout` of 999999 silently accepted |
| H25 | `pkgs/v1/queue/limits.go` | `LimitsMode` accepted but never used — `RelaxedMode` never actually relaxes limits |
| H26 | `pkgs/v1/queue/types/types.go` | `QueueAttributes` type name collision — map type in `types` and struct in `attributes.go` |
| H27 | `apps/go/server/handlers/actions.go` | `handleReceiveMessage` ignores `AttributeNames` and `MessageAttributeNames` from request |
| H28 | `apps/go/server/handlers/actions.go` | `handleSetQueueAttributes` — only `FifoQueue` checked for immutability |
| H29 | `apps/go/server/handlers/actions.go` | `handleTagQueue` — no tag validation or limits enforcement (AWS: max 50 tags, key/value length limits) |
| H30 | `apps/go/server/handlers/adapter.go` | `formatInt64` — hand-rolled integer-to-string with `MinInt64` overflow bug |
| H31 | `apps/go/server/handlers/adapter.go` | `DetectProtocol` — Content-Type with charset not handled, JSON requests misidentified |
| H32 | `apps/go/server/middleware/rate_limiter.go` | Unbounded `limiters` map — memory leak, grows without cleanup |
| H33 | `apps/go/server/main.go` | `log.Fatalf` in store factory closures — `os.Exit` skips all deferred cleanup |
| H34 | `apps/go/server/main.go` | `log.Fatalf` in server goroutine — `os.Exit` skips graceful shutdown |
| H35 | `apps/go/server/main.go` | No `IdleTimeout` or `ReadHeaderTimeout` on HTTP servers — Slowloris vulnerability |
| H36 | `tools/release/main.go` | `getLatestTag()` called after tag creation — release notes will be empty |

---

## 3. Medium Severity Issues

### Architecture & Design

| # | File | Issue |
|---|------|-------|
| M1 | `pkgs/v1/queue/manager.go` | `Shutdown` ignores context deadline — `ctx` accepted but never checked |
| M2 | `pkgs/v1/queue/manager.go` | `Shutdown` holds write lock during potentially slow `Close()` calls |
| M3 | `pkgs/v1/queue/manager.go` | `DeleteQueue` doesn't check `Close()` error |
| M4 | `pkgs/v1/queue/manager.go` | `PurgeQueue` uses `context.Background()` instead of accepting a context |
| M5 | `pkgs/v1/queue/manager.go` | `attributesMatch` doesn't compare `RedrivePolicy` and other attributes |
| M6 | `pkgs/v1/queue/manager.go` | `extractQueueNameFromURL` doesn't handle trailing slashes or query strings |
| M7 | `pkgs/v1/queue/attributes.go` | `QueueAttributes` struct has no mutex — data race on concurrent `SetAttribute`/`GetAttribute` |
| M8 | `pkgs/v1/queue/dlq/redrive.go` | `ParseRedrivePolicy` error handling convoluted — returns wrong error in fallback path |
| M9 | `pkgs/v1/queue/dlq/redrive.go` | No validation for empty `DeadLetterTargetArn` in `ParseRedrivePolicy` |
| M10 | `pkgs/v1/queue/store/store.go` | `Store` interface count methods don't accept `context.Context` |
| M11 | `pkgs/v1/queue/store/memory/memory.go` | `Purge` doesn't reset `sequenceCounter` — AWS resets sequence numbers on purge |
| M12 | `pkgs/v1/queue/store/sqlite/sqlite.go` | `initSchema` doesn't create index for `message_group_id` — full table scan for FIFO queries |
| M13 | `pkgs/v1/queue/store/sqlite/sqlite.go` | `ReceiveMessages` holds mutex during all DB I/O — kills throughput |
| M14 | `pkgs/v1/queue/store/badger/badger.go` | Full table scan for every count query — O(N) per call |
| M15 | `pkgs/v1/queue/store/badger/badger.go` | `Purge` uses View + Update — non-atomic, new messages can survive purge |
| M16 | `pkgs/v1/queue/store/badger/badger.go` | `keyPrefix` doesn't sanitize queue name — prefix collision possible |
| M17 | All store backends | Duplicated code: `generateReceiptHandle`, `generateNonce`, `computeContentBasedDedupID`, `cleanExpiredDedupEntries` |
| M18 | All store backends | No input validation on `maxMessages`, `visibilityTimeout`, `waitTimeSeconds` |

### Server & Handlers

| # | File | Issue |
|---|------|-------|
| M19 | `apps/go/server/handlers/actions.go` | `var errors []BatchError` shadows stdlib `errors` package |
| M20 | `apps/go/server/handlers/adapter.go` | Default case in marshalers returns `DeleteQueueResponse` — masks bugs |
| M21 | `apps/go/server/protocol/query.go` | Magic number `20` for attribute loop limit — should be named constant |
| M22 | `apps/go/server/protocol/query.go` | `parseBatchEntries` assumes contiguous indices — non-contiguous entries silently dropped |
| M23 | `apps/go/server/protocol/query.go` | `VisibilityTimeout` case in `parseBatchEntries` is a no-op — dead code |
| M24 | `apps/go/server/handlers/actions.go` | `handleSendMessageBatch` — sequential `SendMessage` calls, no parallelism |
| M25 | `apps/go/server/handlers/actions.go` | `handleListDeadLetterSourceQueues` — full table scan, no pagination |
| M26 | `apps/go/server/request_handler.go` | No request body size limit — DoS vulnerability |
| M27 | `apps/go/server/middleware/request_logger.go` | Request ID fallback not hex-encoded properly |
| M28 | `apps/go/server/middleware/request_logger.go` | Request ID not propagated to context |
| M29 | `apps/go/server/middleware/response_writer.go` | `Hijack()` not implemented — breaks WebSocket/streaming |
| M30 | `apps/go/server/tls/tls.go` | No cipher suite restrictions — allows weak ciphers |
| M31 | `apps/go/server/config.go` | `LogConfig.Level` defined but never applied to logger |
| M32 | `apps/go/server/main.go` | `os.Setenv` mutates process-wide environment — race condition |
| M33 | `apps/go/server/main.go` | Magic numbers for ports and timeouts |
| M34 | `apps/go/server/main.go` | Inconsistent error handling: `log.Fatalf`, `panic()`, returned errors |

### UI

| # | File | Issue |
|---|------|-------|
| M35 | `apps/go/server/ui/handlers.go` | Silent error swallowing on `ReceiveMessages` — errors discarded |
| M36 | `apps/go/server/ui/handlers.go` | No input validation on numeric form fields |
| M37 | `apps/go/server/ui/handlers.go` | `init()` panics on template parse failure |
| M38 | `apps/go/server/ui/handlers.go` | No HTTP method enforcement on routes |
| M39 | `apps/go/server/ui/handlers.go` | Hardcoded 1s visibility timeout for "peeking" — disrupts real consumers |
| M40 | `apps/go/server/ui/handlers.go` | `context.Background()` used instead of request context |
| M41 | `apps/go/server/ui/server.go` | `fs.Sub` error ignored |
| M42 | `apps/go/server/ui/server.go` | No `IdleTimeout` on HTTP server |

### Helm & Deployment

| # | File | Issue |
|---|------|-------|
| M43 | `deploy/helm/values.yaml` | `replicaCount: 1` with `storageType: "memory"` — no validation or warning for data loss |
| M44 | `deploy/helm/values.yaml` | `accountId: "000000000000"` — inconsistent with README's `123456789012` |
| M45 | `deploy/helm/values.yaml` | Health probes use port 8001 but health server only starts in non-local environments |
| M46 | `deploy/helm/values.yaml` | Autoscaling with `ReadWriteOnce` PVC — new pods stuck in `ContainerCreating` |
| M47 | `deploy/helm/templates/deployment.yaml` | No `checksum/config` annotation — pods don't restart on ConfigMap change |
| M48 | `deploy/helm/templates/deployment.yaml` | No `terminationGracePeriodSeconds` — server gets SIGKILLed during shutdown |
| M49 | `deploy/helm/templates/deployment.yaml` | Missing `seccompProfile: RuntimeDefault` |
| M50 | `deploy/helm/templates/service.yaml` | Health port exposed in Service — should be kubelet-only |
| M51 | `deploy/helm/templates/NOTES.txt` | ClusterIP service shows `http://:9324` — empty hostname |
| M52 | `deploy/helm/templates/pdb.yaml` | Both `minAvailable` and `maxUnavailable` can be set — invalid PDB |
| M53 | `deploy/helm/templates/configmap.yaml` | No namespace in metadata |
| M54 | `deploy/helm/values.prod.yaml` | PDB with `minAvailable: 1` and `replicaCount: 1` blocks all voluntary evictions |

### Documentation

| # | File | Issue |
|---|------|-------|
| M55 | Multiple docs | Inconsistent action counts: 20, 23, and 27 mentioned in different places |
| M56 | `docs/configuration.md` | `log.type` documented but `LogConfig` struct only has `Level` |
| M57 | `docs/queue-library.md` | Quick Start examples don't compile — missing `ctx`, wrong method names |
| M58 | `docs/shared-packages.md` | BadgerStore missing from sub-packages table |

---

## 4. Low Severity Issues

### Code Quality

| # | Issue |
|---|-------|
| L1 | `var _ = fmt.Sprintf` — unused import workaround in multiple files |
| L2 | `var _ = types.SQSVersion` — same pattern in `protocol/json.go` and `protocol/query.go` |
| L3 | Duplicate `extractQueueNameFromURL` function in `handlers/handler.go` and `queue/manager.go` |
| L4 | Duplicate `newRequestID`/`NewRequestID` functions in `handlers/actions.go` and `protocol/marshal.go` |
| L5 | `KmsMasterKeyId` should be `KmsMasterKeyID` (Go initialism convention) |
| L6 | Missing `Min*` constants (only `Max*` and `Default*` exist) |
| L7 | `MaxMessageBodySize` and `MaxMaximumMessageSize` are both 262144 — redundant |
| L8 | `MessageCounts` struct defined but never used |
| L9 | `QueueURL` duplicates `Queue.URL()` logic |
| L10 | `URL()` hardcodes `http://` — no HTTPS support |
| L11 | `generateTaskHandle` uses `time.Now().UnixNano()` — not collision-safe |
| L12 | `MoveTask.Cancelled` channel is exported — external callers could close it |
| L13 | `ConcreteSQSError` uses exported fields — bypasses factory functions |
| L14 | No sentinel errors — callers must inspect `Code()` string |
| L15 | `fmt.Sprintf` used for ARN/URL construction everywhere — no shared helper |
| L16 | Inconsistent error wrapping (`%w` vs `%s`) |

### Missing Documentation

| # | Issue |
|---|-------|
| L17 | No package-level documentation (`// Package ...`) on any package |
| L18 | `Store` interface methods don't document error types |
| L19 | No "Contributing" section in README |
| L20 | No "License" section with actual license text in README |
| L21 | No badges (CI, Go version, license) in README |
| L22 | No `helm install` instructions in README |
| L23 | `docs/benchmarks.md` says "Go 1.25.5" — likely incorrect version |
| L24 | ASCII art diagrams misaligned in `architecture.md` and `protocol.md` |
| L25 | `docs/configuration.md` documents `idleTimeout` but server doesn't set it |

### Test Quality

| # | Issue |
|---|-------|
| L26 | `string(rune('0'+i))` for message IDs — breaks for i ≥ 10 |
| L27 | `generateID()` in SQLite tests — `string(rune(time.Now().UnixNano()))` produces invalid Unicode |
| L28 | Extensive use of `time.Sleep` for async operations — flaky on CI |
| L29 | `TestShutdown_WithDeadline` — 1 nanosecond context deadline, extremely flaky |
| L30 | `TestServer_GracefulShutdown` — hardcoded port, flaky startup check |
| L31 | `TestShutdown_WithSQLiteStore` — actually tests memory store, misleading name |
| L32 | No resource cleanup (`t.Cleanup`) in `manager_test.go` and `handler_test.go` |
| L33 | `TestConcurrentAccess` — swallows errors, no post-condition assertions |
| L34 | `TestIntegration_ReceiveFromNonExistentQueue` — no assertions, always passes |
| L35 | `types_test.go` — tautological tests (constants equal themselves) |
| L36 | `TestMarshalResponse_ErrorResponse` — doesn't test marshalling |
| L37 | `BenchmarkConcurrentSendReceive` — not measuring what it claims |
| L38 | Inconsistent test naming conventions across files |
| L39 | Missing `t.Helper()` in test helpers |
| L40 | `TestPurgeQueue` — only tests purging an empty queue |

### UI & Frontend

| # | Issue |
|---|-------|
| L41 | `app.js` not wrapped in IIFE — pollutes global scope |
| L42 | `doRefresh` doesn't handle non-OK HTTP responses |
| L43 | Auto-refresh starts unconditionally on all pages |
| L44 | CSS `display: block` on `<table>` — breaks semantics/accessibility |
| L45 | Missing `aria-label` on icon-only buttons |
| L46 | `buildAttrPairs` ignores `q` parameter — dead code |
| L47 | `handleQueueRoutes` — fragile manual path parsing |
| L48 | Playground examples use hardcoded secret key |
| L49 | Playground examples inconsistently ignore errors
| L50 | `sqs_phase2_example` claims batch operations but sends individually |

### Helm & Bazel

| # | Issue |
|---|-------|
| L51 | Chart.yaml missing `icon` and `annotations` for Helm Hub |
| L52 | No `startupProbe` in deployment template |
| L53 | No `extraEnvFrom` support in deployment |
| L54 | No `networkPolicy` template |
| L55 | No `priorityClassName` configurable |
| L56 | `serviceaccount.yaml` — no `automountServiceAccountToken: false` |
| L57 | Release tool — no `--draft` or `--pre-release` flag support |
| L58 | Release tool — no test file |
| L59 | Release tool — ANSI colors unconditionally (not TTY-aware) |
| L60 | Bazel `opensqs_go_image` — typo "bunding" → "bundling" |
| L61 | Bazel `platform_transition_filegroup` — selects on host CPU, not target platform |

---

## 5. Positive Observations

The codebase has several strengths worth acknowledging:

1. **Clean architecture** — Good separation between queue library, storage backends, server, and protocol layers
2. **Interface-based design** — `Store` interface allows pluggable backends (memory, SQLite, Badger)
3. **Protocol support** — Both Query (AWS SDK) and JSON protocol implementations
4. **Comprehensive feature set** — FIFO queues, DLQ, message move tasks, visibility timeouts, deduplication
5. **Good test coverage** — 24 test files covering most functionality
6. **Structured logging** — Consistent use of the `logger` package
7. **Metrics** — Prometheus integration with relevant queue metrics
8. **Helm chart** — Production-ready deployment with PVC, HPA, PDB, ingress, TLS support
9. **Bazel build system** — Custom rules for Go binaries, images, and OCI push
10. **Release tooling** — Go-based release tool with dry-run support
11. **Web UI** — Functional dashboard with auto-refresh, dark mode, and responsive design
12. **Documentation** — Comprehensive docs covering architecture, API reference, configuration, and benchmarks

---

## 6. Action Items Checklist

### Must Fix Before Open-Sourcing (Critical)

- [ ] **C1.1** Fix XSS in `app.js` — use `textContent` or `escapeHtml()`
- [ ] **C1.2** Add CSRF protection to all POST endpoints
- [ ] **C1.3** Fix SQL injection in SQLite store — strict allowlist validation
- [ ] **C1.4** Remove default `serverSecret` from Helm chart, add `fail` template
- [ ] **C1.5** Move server secret from ConfigMap to Kubernetes Secret
- [ ] **C1.6** Use UUID for message ID generation
- [ ] **C1.7** Use UUID for request ID generation
- [ ] **C1.8** Fix `QueryBatchEntry` — add `ReceiptHandle` and `VisibilityTimeout` fields
- [ ] **C1.9** Fix message loss in `runTask` — send to destination before deleting from source
- [ ] **C1.10** Fix deadlock in redrive callback — release mutex or dispatch async
- [ ] **C1.11** Implement `redriveIfNeededLocked` in Badger store
- [ ] **C1.12** Fix FIFO violation in Badger store
- [ ] **C1.13** Fix duplicate `server:` key in ConfigMap template

### Should Fix Before Open-Sourcing (High)

- [ ] **H1-H8** Security: JSON injection, security headers, hardcoded secret, Validate(), ingress cleanup, distroless vs ubuntu, private IPs
- [ ] **H9-H12** Correctness: Silent redrive errors, CancelTask race, MovedMessages race, timer race
- [ ] **H13-H22** Data integrity: Shared pointers, tableName bug, N+1 queries, resource leaks, ignored errors, StoreFactory, global Now
- [ ] **H23-H26** API design: Tags mutation leak, SetAttribute validation, LimitsMode unused, type collision
- [ ] **H27-H31** Handlers: Ignored attribute filters, immutability checks, tag validation, formatInt64, protocol detection
- [ ] **H32-H35** Infrastructure: Unbounded limiters map, log.Fatalf in closures/goroutines, missing timeouts
- [ ] **H36** Release tool: getLatestTag called after tag creation

### Track as Follow-Up (Medium/Low)

- 54 Medium issues and 61 Low issues documented above
- Prioritize: package documentation, error wrapping consistency, test flakiness fixes, code deduplication

---

*This report was generated by a comprehensive automated review of all 148 files in the PR, covering core queue library, server handlers/protocol, infrastructure, storage backends, UI, tests, deployment, and documentation.*
