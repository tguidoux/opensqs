# Server Architecture

This document describes the internal architecture of the OpenSQS server.

## Overview

```
┌──────────────────────────────────────────────────────────┐
│                      OpenSQS Server                      │
│                                                           │
│  ┌───────────────┐  ┌───────────────┐  ┌──────────────┐ │
│  │  SQS REST API  │  │  Web UI       │  │  Metrics     │ │
│  │  (Port 9324)   │  │  (Port 9325)  │  │  (Port 9326) │ │
│  └───────┬────────┘  └───────┬───────┘  └──────────────┘ │
│          │                   │                           │
│  ┌───────┴───────────────────┴────────────────┐          │
│  │           Middleware Layer                  │          │
│  │  RequestLogger · RateLimiter · Chain        │          │
│  └───────┬─────────────────────────────────────┘          │
│          │                                                │
│  ┌───────┴──────────────────────────────────────┐         │
│  │              Request Handler Layer            │         │
│  │  Protocol Detection → Parse → Dispatch       │         │
│  └───────┬──────────────────────────────────────┘         │
│          │                                                │
│  ┌───────┴──────────────────────────────────────┐         │
│  │              Action Handler Layer             │         │
│  │  CreateQueue · SendMessage · ReceiveMessage   │         │
│  └───────┬──────────────────────────────────────┘         │
│          │                                                │
│  ┌───────┴──────────────────────────────────────┐         │
│  │              Queue Engine (pkgs/v1/queue)     │         │
│  │  QueueManager → Queue → Store                 │         │
│  │  MoveTaskManager (DLQ message migration)      │         │
│  └───────┬──────────────────────────────────────┘         │
│          │                                                │
│  ┌───────┴──────────────────────────────────────┐         │
│  │              Storage Layer (pluggable)        │         │
│  │  MemoryStore · SQLiteStore · BadgerStore      │         │
│  └──────────────────────────────────────────────┘         │
│                                                           │
│  ┌──────────────────────────────────────────────┐         │
│  │  Health Check (Port 8001) · TLS · Shared Libs │         │
│  └──────────────────────────────────────────────┘         │
└──────────────────────────────────────────────────────────┘
```

## Store Factory

The server creates a `StoreFactory` function at startup based on the `storageType` config field. This factory is called for each queue to create the appropriate `Store` implementation, passing FIFO/DLQ configuration via `StoreConfig`:

- **`memory`** (default) — Creates `MemoryStore` instances with `time.AfterFunc` visibility timers
- **`sqlite`** — Creates `SQLiteStore` instances with lazy visibility timeout evaluation (no goroutines), backed by a shared SQLite database file with WAL mode enabled
- **`badger`** — Creates `BadgerStore` instances with lazy visibility timeout evaluation, backed by BadgerDB v4 with iterator-based scanning

## File Layout

```
apps/go/server/
├── main.go               # Entry point: config, logger, queue manager, HTTP servers, shutdown
├── config.go             # ServerConfig struct and sub-configs
├── config.yaml           # Local development configuration
├── request_handler.go    # HTTP handler: protocol detection, parsing, dispatch, response
├── error_response.go     # SQS error response writer (XML/JSON)
├── startup_queues.go     # Converts config attributes to QueueAttributes for startup queues
├── BUILD.bazel           # Bazel build rules (opensqs_go_library, binary, image)
├── handlers/             # Action dispatcher and protocol adapters
│   ├── handler.go        # Handler struct, Request interface, Response struct, dispatch
│   ├── actions.go        # Action handler implementations
│   └── adapter.go        # Query/JSON request adapters, protocol detection, response marshalling
├── protocol/             # Wire protocol parsers and response marshallers
│   ├── query.go          # AWS Query Protocol parser (form-urlencoded)
│   ├── json.go           # AWS JSON Protocol 1.0 parser
│   ├── marshal.go        # XML/JSON response types and marshallers
│   └── errors.go         # SQSErrorResponse (XML/JSON error formatting)
├── middleware/           # HTTP middleware
│   ├── middleware.go      # Middleware type, Chain function
│   ├── request_logger.go # RequestLogger middleware (structured per-request logging)
│   └── rate_limiter.go    # RateLimiter middleware (token bucket, global or per-queue)
├── tls/                  # TLS configuration
│   └── tls.go            # TLS config loader (cert/key files, min TLS 1.2)
├── metrics/              # Prometheus metrics server
│   ├── server.go         # Metrics HTTP server, /metrics endpoint
│   └── metrics.go        # Metric definitions (counters, gauges, histograms)
├── ui/                   # Web UI server (optional, enabled by config)
│   ├── server.go         # HTTP server, route registration, start/stop
│   ├── handlers.go       # Page handlers (index, queue detail, create), JSON API, helpers
│   ├── templates/        # HTML templates (layout, index, queue, create_queue)
│   ├── static/           # CSS and JavaScript assets
│   └── tests/            # Handler tests
├── health/               # Health check HTTP server
│   └── server.go         # GET /health → 200 OK
└── integration/          # Integration tests (15 tests covering all major features)
    └── integration_test.go
```

## Startup Flow

```
main()
  │
  ├── 1. Load config (CONFIG_PATH env var → config.yaml → /apps/go/server/config.yaml)
  ├── 2. Initialize logger (UncontextualLogger, name="opensqs-server")
  ├── 3. Create Limits (strict or relaxed based on config)
├── 4. Create QueueManager (nodeAddress, accountID, region, serverSecret, storeFactory)
  ├── 5. Create startup queues from config (if any)
  ├── 6. Create Handler (manager + limits)
  ├── 7. Register HTTP handler on "/"
  ├── 8. Start health check server (non-local only, configurable port)
  ├── 9. Start Web UI server (if enabled in config, default port 9325)
  ├── 10. Start HTTP server in goroutine
  └── 11. Wait for SIGINT/SIGTERM → graceful shutdown (10s timeout)
```

## Request Lifecycle

```
HTTP Request
  │
  ├── handleSQSRequest()
  │     ├── DetectProtocol()           → QueryProtocol or JSONProtocol
  │     ├── Parse request body         → QueryRequest or JSONRequest
  │     ├── Wrap in adapter            → QueryRequestAdapter or JSONRequestAdapter
  │     ├── Handler.HandleRequest()    → dispatch to action handler
  │     │     ├── resolveQueue()       → lookup queue by URL
  │     │     ├── Validate parameters  → limits checks
  │     │     └── Call Queue/Store     → business logic
  │     ├── MarshalResponse()          → XML or JSON bytes
  │     └── Write HTTP response        → 200 OK + content-type
  │
  └── (on error) writeErrorResponse()
        ├── Type-assert to SQSError
        ├── Create SQSErrorResponse
        ├── Serialize to XML or JSON
        └── Write with appropriate HTTP status code
```

## Key Design Decisions

### Single HTTP Endpoint

All SQS API actions are served from a single `/` endpoint. The action is determined from the request body (`Action` parameter in Query Protocol, `X-Amz-Target` header in JSON Protocol), not from the URL path.

### Dual Protocol Support

The server transparently handles both AWS wire protocols:

| Protocol | Detection | Request Format | Response Format |
|----------|-----------|----------------|-----------------|
| Query Protocol | `Content-Type: application/x-www-form-urlencoded` or no `X-Amz-Target` | Form-urlencoded body or query string | XML |
| JSON Protocol | `X-Amz-Target` header or `Content-Type: application/x-amz-json-1.0` | JSON body | JSON |

Both protocols are dispatched to the same action handlers via the `Request` interface, ensuring identical behavior regardless of client SDK.

### Pluggable Storage

The `store.Store` interface decouples queue logic from persistence. Three implementations are available:

- **MemoryStore** — In-memory storage with `time.AfterFunc` visibility timers. Default for local development.
- **SQLiteStore** — Persistent storage backed by SQLite (pure-Go `modernc.org/sqlite` driver, no CGO). Uses lazy visibility timeout evaluation (no goroutines). Suitable for production use.
- **BadgerStore** — Persistent storage backed by BadgerDB v4 (`dgraph-io/badger/v4`). Uses lazy visibility timeout evaluation with iterator-based scanning and prefix filtering. Suitable for production use.

The server selects the storage backend at startup based on the `storageType` config field. A `StoreFactory` function creates the appropriate store for each queue, passing FIFO/DLQ configuration via `StoreConfig`.

### Signed Receipt Handles

Receipt handles are HMAC-SHA256 signed using the server's secret key. They encode:
- Queue name
- Message ID
- Receive timestamp
- Random nonce (8 bytes)

This prevents forgery and allows the server to validate that a receipt handle was issued by this server instance.

### Graceful Shutdown

On `SIGINT`/`SIGTERM`, the server:
1. Stops accepting new HTTP requests
2. Waits up to 10 seconds for in-flight requests to complete
3. Stops the health check server (if running)
4. Stops the Web UI server (if running)
5. Stops the metrics server (if running)

In-flight messages with active visibility timers continue their timeout behavior in the background.

### Middleware Layer

HTTP middleware follows the standard Go `func(http.Handler) http.Handler` pattern. The `Chain` function composes multiple middlewares in order. Available middlewares:

- **RequestLogger** — Logs each request with request ID, method, path, status code, bytes written, duration, remote address, and user agent. Generates a unique request ID for correlation.
- **RateLimiter** — Token bucket rate limiting using `golang.org/x/time/rate`. Supports global or per-queue limiting. Returns `429 Too Many Requests` with `Retry-After: 1` header when rate exceeded.

### TLS Support

Each HTTP server (SQS API, UI, metrics, health) can be individually configured for TLS. The TLS config loader (`tls/tls.go`) reads certificate and key files, enforces minimum TLS 1.2, and returns a `*tls.Config`. When TLS is disabled, the server uses plain HTTP.

### Prometheus Metrics

When enabled, the metrics server exposes Prometheus-format metrics at `/metrics` on port 9326 (configurable). Metrics include:

- Message counters (sent, received, deleted) per queue
- Queue size gauges (available, in-flight, delayed) per queue
- API request counters and latency histograms per action and protocol
- Move task counters (messages moved, active tasks)

### Message Move Tasks

The `MoveTaskManager` (in `pkgs/v1/queue/dlq/move_task.go`) handles background migration of messages from a DLQ to a source or custom destination queue. Tasks run in goroutines with optional rate limiting. The manager supports:

- **Auto-discovery** — If `DestinationArn` is omitted, finds a queue whose `RedrivePolicy` points to the source.
- **Rate limiting** — `MaxNumberOfMessagesPerSecond` controls the move rate.
- **Cancellation** — Tasks can be cancelled mid-flight.
- **Status tracking** — `RUNNING` → `COMPLETED`/`CANCELLED`/`FAILED`.

### Helm Chart Deployment

OpenSQS includes a production-ready Helm chart in `deploy/helm/` for Kubernetes deployment:

- **Deployment** — Configurable replicas, image, resources, probes
- **Service** — ClusterIP/NodePort/LoadBalancer
- **ConfigMap** — Server configuration
- **PersistentVolumeClaim** — For SQLite/BadgerDB persistent storage
- **Ingress** — Optional ingress with TLS
- **HorizontalPodAutoscaler** — CPU/memory-based autoscaling
- **PodDisruptionBudget** — Availability guarantees

### Web UI Server

The Web UI is an optional HTTP server that provides a browser-based dashboard for managing queues and messages. It runs on a separate port (default `9325`) and shares the same `QueueManager` instance as the SQS API server.

**Routes:**

| Path | Method | Description |
|------|--------|-------------|
| `/` | GET | Queue list page |
| `/queues/new` | GET | Create queue form |
| `/queues/create` | POST | Create queue (redirects) |
| `/queues/{name}` | GET | Queue detail page (attributes, tags, messages) |
| `/queues/{name}/delete` | POST | Delete queue (redirects) |
| `/queues/{name}/purge` | POST | Purge queue messages (redirects) |
| `/queues/{name}/messages` | POST | Send message to queue (redirects) |
| `/queues/{name}/messages/{handle}/delete` | POST | Delete specific message (redirects) |
| `/api/queues` | GET | Queue list as JSON (for auto-refresh) |
| `/api/queues/{name}` | GET | Queue summary as JSON |
| `/api/queues/{name}/messages` | GET | Messages as JSON (for auto-refresh) |
| `/static/` | GET | CSS and JavaScript assets |

**Template System:**

HTML templates use Go's `html/template` with a layout + content block pattern. Each page template is parsed into its own `*template.Template` instance (layout + page) to prevent `{{define "content"}}` conflicts across pages.

**Static Assets:**

CSS (`style.css`) and JavaScript (`app.js`) are embedded via `//go:embed` and served at `/static/`. The JavaScript handles theme toggling (dark/light) and auto-refresh polling for queue/message tables.
