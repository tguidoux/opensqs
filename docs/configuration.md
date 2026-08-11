# Configuration

This document describes the OpenSQS server configuration system.

## Configuration Loading

Configuration is loaded from a YAML file. The path is determined by:

1. `CONFIG_PATH` environment variable (if set)
2. `config.yaml` in the working directory (default)

The config file is bundled into the Docker image at `/apps/go/server/config.yaml`.

## Configuration Structure

### Top-Level

```yaml
server:
  # HTTP server settings
sqs:
  # SQS engine settings
log:
  # Logging settings
health:
  # Health check settings
ui:
  # Web UI settings
environment: local
queues:
  # Queues to create at startup
```

### Server (`server`)

```yaml
server:
  host: "0.0.0.0"         # HTTP listen address
  port: 9324              # HTTP listen port
  readTimeout: 30s        # HTTP read timeout
  writeTimeout: 30s      # HTTP write timeout
  idleTimeout: 120s       # HTTP idle timeout
  tls:                    # TLS configuration
    enabled: false
    certFile: ""
    keyFile: ""
```

> **Note:** `readTimeout`, `writeTimeout`, and `idleTimeout` are hardcoded in `main.go` (30s/30s/120s). Only `host`, `port`, and `tls` come from config.

#### TLS Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable TLS for the SQS API server |
| `certFile` | string | `""` | Path to TLS certificate file |
| `keyFile` | string | `""` | Path to TLS private key file |

When TLS is enabled, the server uses HTTPS. Minimum TLS version is 1.2. Each server (SQS API, UI, metrics, health) has its own independent TLS configuration.

### SQS (`sqs`)

```yaml
sqs:
  nodeAddress: localhost:9324
  accountId: "123456789012"
  region: us-east-1
  storageType: memory     # "memory", "sqlite", or "badger"
  sqlitePath: /data/opensqs.db  # SQLite DB path (used when storageType is "sqlite")
  badgerPath: /data/badger      # BadgerDB path (used when storageType is "badger")
  strictLimits: true      # true enforces SQS limits, false allows overrides
  serverSecret: my-secret-key  # HMAC signing key for receipt handles
```

| Field | Description | Default |
|-------|-------------|--------|
| `nodeAddress` | Host:port used in queue URLs | — |
| `accountId` | AWS account ID used in queue URLs and ARNs | — |
| `region` | AWS region used in queue ARNs | — |
| `storageType` | Storage backend: `memory`, `sqlite`, or `badger` | `memory` |
| `sqlitePath` | SQLite database file path (when `storageType` is `sqlite`) | — |
| `badgerPath` | BadgerDB directory path (when `storageType` is `badger`) | — |
| `strictLimits` | `true` enforces SQS limits, `false` allows overrides | `true` |
| `serverSecret` | Secret key for signing receipt handles | — |

### Log (`log`)

```yaml
log:
  level: info             # debug, info, warn, error
  type: uncontextual      # "uncontextual" or "contextual"
```

### Health (`health`)

```yaml
health:
  port: 8001              # Health check server port
```

The health check server only starts in non-local environments. If `port` is 0 or omitted, it defaults to `8001`.

### Web UI (`ui`)

```yaml
ui:
  enabled: true           # Enable or disable the Web UI server
  port: 9325              # Web UI HTTP port
  tls:                    # TLS configuration
    enabled: false
    certFile: ""
    keyFile: ""
```

| Field | Description | Default |
|-------|-------------|--------|
| `enabled` | If `true`, starts the Web UI server | `true` |
| `port` | HTTP port for the Web UI server | `9325` |
| `tls` | TLS configuration (same structure as server.tls) | disabled |

When enabled, the Web UI provides a browser-based dashboard at `http://localhost:{port}` for browsing queues, viewing messages, sending/purging, and creating queues. It shares the same `QueueManager` as the SQS API server.

### Metrics (`metrics`)

```yaml
metrics:
  enabled: false          # Enable or disable the Prometheus metrics server
  port: 9326              # Metrics HTTP port
  tls:                    # TLS configuration
    enabled: false
    certFile: ""
    keyFile: ""
```

| Field | Description | Default |
|-------|-------------|--------|
| `enabled` | If `true`, starts the Prometheus metrics server | `false` |
| `port` | HTTP port for the metrics server | `9326` |
| `tls` | TLS configuration (same structure as server.tls) | disabled |

When enabled, Prometheus-format metrics are exposed at `http://localhost:{port}/metrics`. See the [API Reference](api-reference.md#prometheus-metrics) for the full list of exposed metrics.

### Rate Limiting (`rateLimit`)

```yaml
rateLimit:
  enabled: false          # Enable or disable rate limiting
  requestsPerSecond: 1000 # Token bucket refill rate
  burst: 100              # Token bucket burst capacity
  perQueue: false         # If true, rate limit per-queue; if false, global
```

| Field | Description | Default |
|-------|-------------|--------|
| `enabled` | If `true`, enables rate limiting middleware | `false` |
| `requestsPerSecond` | Token bucket refill rate (requests/sec) | `1000` |
| `burst` | Maximum burst size | `100` |
| `perQueue` | `true` for per-queue limiting, `false` for global | `false` |

When rate limited, the server returns `429 Too Many Requests` with a `Retry-After: 1` header. Uses `golang.org/x/time/rate` token bucket algorithm.

### Request Logging (`requestLogging`)

```yaml
requestLogging:
  enabled: false          # Enable or disable request logging middleware
```

| Field | Description | Default |
|-------|-------------|--------|
| `enabled` | If `true`, logs each HTTP request with request ID, method, path, status, duration, etc. | `false` |

### Environment

```yaml
environment: local        # local, staging, prod
```

| Value | Description |
|-------|-------------|
| `local` | Local development. Health check disabled. No AWS region. |
| `staging` | Staging environment. Health check enabled. Region: `us-east-1`. |
| `prod` | Production environment. Health check enabled. Region: `us-east-1`. |

### Startup Queues (`queues`)

Queues can be created automatically at server startup. The `queues` section also controls auto-create behavior:

```yaml
queues:
  autoCreate: false
  startup:
    - name: orders
      attributes:
        visibilityTimeout: 60
        receiveMessageWaitTimeSeconds: 5
    - name: notifications
      # Uses default attributes
    - name: dead-letter.fifo
      attributes:
        fifoQueue: true
        contentBasedDeduplication: true
        visibilityTimeout: 120
```

#### QueuesConfig

| Field | Type | Description |
|-------|------|-------------|
| `autoCreate` | bool | If `true`, automatically create queues on first access |
| `startup` | []StartupQueue | List of queues to create at startup |

#### StartupQueue

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Queue name. Must end with `.fifo` for FIFO queues. |
| `attributes` | No | Queue attributes (see below) |

#### StartupQueueAttributes

All fields are optional pointers. Only non-nil fields override the defaults.

| Field | Type | Default |
|-------|------|---------|
| `visibilityTimeout` | int | 30 |
| `maximumMessageSize` | int | 262,144 |
| `messageRetentionPeriod` | int | 345,600 |
| `delaySeconds` | int | 0 |
| `receiveMessageWaitTimeSeconds` | int | 0 |
| `fifoQueue` | bool | false |
| `contentBasedDeduplication` | bool | false |


## Complete Example

```yaml
server:
  port: 9324
  readTimeout: 30s
  writeTimeout: 30s
  idleTimeout: 120s
  tls:
    enabled: false
    certFile: ""
    keyFile: ""

sqs:
  nodeAddress: localhost:9324
  accountId: "123456789012"
  region: us-east-1
  storageType: memory
  # sqlitePath: "/data/opensqs.db"  # Uncomment and set storageType to "sqlite" for SQLite persistence
  # badgerPath: "/data/badger"      # Uncomment and set storageType to "badger" for BadgerDB persistence
  strictLimits: true
  serverSecret: my-secret-key

log:
  level: info
  type: uncontextual

health:
  port: 8001

ui:
  enabled: true
  port: 9325
  tls:
    enabled: false
    certFile: ""
    keyFile: ""

metrics:
  enabled: false
  port: 9326
  tls:
    enabled: false
    certFile: ""
    keyFile: ""

rateLimit:
  enabled: false
  requestsPerSecond: 1000
  burst: 100
  perQueue: false

requestLogging:
  enabled: false

environment: local

queues:
  autoCreate: false
  startup:
    - name: orders
      attributes:
        visibilityTimeout: 60
        receiveMessageWaitTimeSeconds: 5
    - name: notifications
    - name: dead-letter.fifo
      attributes:
        fifoQueue: true
        contentBasedDeduplication: true
        visibilityTimeout: 120
```

## Config Package API

The configuration system is built on `pkgs/v1/config`:

```go
// ConfigI interface — implemented by config structs
type ConfigI[ConfigType any] interface {
    Validate() error
    WithValidation() ConfigType
}

// Load config from environment
cfg := config.NewConfigFromEnv[ServerConfig]()  // Uses CONFIG_PATH env var

// Load config from specific path
cfg := config.NewConfig[ServerConfig]("/path/to/config.yaml")

// Access config with validation
serverConfig := cfg.WithValidation()
```

### Environment Variable

| Variable | Default | Description |
|----------|---------|-------------|
| `CONFIG_PATH` | `config.yaml` | Path to the YAML config file |
