# Shared Packages

This document describes the shared packages in `pkgs/v1/` that provide cross-cutting functionality used by the OpenSQS server and other applications.

## Package Overview

```
pkgs/v1/
├── config/       # Configuration loading from YAML with validation
├── environment/  # Environment enum (PROD, STAGING, LOCAL, AOOSTAR)
├── logger/       # Structured JSON logging
└── queue/        # Queue engine (see Queue Library docs)
    ├── queue.go          # Queue struct and methods
    ├── manager.go        # QueueManager for multi-queue management
    ├── attributes.go     # QueueAttributes and defaults
    ├── limits.go         # Limits validation (strict/relaxed)
    ├── errors.go         # SQS error types and factories
    ├── dlq/              # RedrivePolicy parsing, MoveTaskManager
    ├── types/            # Message, SQSError, constants
    └── store/
        ├── store.go      # Store interface, StoreFactory, StoreConfig
        ├── memory/       # In-memory store (FIFO, DLQ, visibility timers)
        ├── sqlite/       # SQLite persistent store
        └── badger/       # BadgerDB persistent store
```

---

## config

**Import:** `github.com/tguidoux/opensqs/pkgs/v1/config`

**Files:** `config.go`, `default.go`

### Overview

Generic configuration loading from YAML files with environment-based path resolution and validation support.

### ConfigI Interface

```go
type ConfigI[ConfigType any] interface {
    Validate() error
    WithValidation() ConfigType
}
```

Any config struct that implements `Validate()` and `WithValidation()` can be loaded with the generic config loader.

### Config Struct

```go
type Config[ConfigType any] struct {
    ConfigPath string
    Config     ConfigType
    data       []byte  // unexported raw YAML data
}
```

### Functions

#### `NewConfig[ConfigType any](configPath string) Config[ConfigType]`

Reads and parses a YAML config file. Panics on read or parse errors.

```go
cfg := config.NewConfig[MyConfig]("/path/to/config.yaml")
```

#### `NewConfigFromEnv[ConfigType ConfigI[ConfigType]](envVar ...string) *Config[ConfigType]`

Loads config from a path specified by environment variable. Falls back to `DefaultConfigPath` (`config.yaml`).

```go
// Uses CONFIG_PATH env var, falls back to "config.yaml"
cfg := config.NewConfigFromEnv[MyConfig]()

// Use custom env var
cfg := config.NewConfigFromEnv[MyConfig]("MY_CONFIG_PATH")
```

### Constants

| Constant | Value |
|----------|-------|
| `DefaultConfigPath` | `config.yaml` |
| `DefaultConfigEnvVar` | `CONFIG_PATH` |

### Default Endpoints (`default.go`)

AWS endpoint URLs for local and AOOSTAR environments:

| Constant | Value |
|----------|-------|
| `LOCAL_AWS_S3_ENDPOINT_URL` | `http://localhost:9000` |
| `LOCAL_AWS_SQS_ENDPOINT_URL` | `http://localhost:9324` |
| `LOCAL_AWS_SSM_ENDPOINT_URL` | `http://localhost:8000` |
| `AOOSTAR_AWS_S3_ENDPOINT_URL` | `http://192.168.1.119:9000` |
| `AOOSTAR_AWS_SQS_ENDPOINT_URL` | `http://192.168.1.153:9324` |
| `AOOSTAR_AWS_SSM_ENDPOINT_URL` | `http://192.168.1.153:8000` |

### Endpoint Functions

| Function | LOCAL | AOOSTAR | STAGING/PROD |
|----------|-------|---------|--------------|
| `GetRegion(env)` | `""` | `""` | `us-east-1` |
| `GetSSMRegion(env)` | `""` | `""` | `us-east-1` |
| `GetS3Region(env)` | `us-east-1` | `us-east-1` | `us-east-1` |
| `GetSQSRegion(env)` | `""` | `""` | `us-east-1` |
| `GetAWSS3EndpointURL(env)` | Local URL | AOOSTAR URL | `""` |
| `GetAWSSQSEndpointURL(env)` | Local URL | AOOSTAR URL | `""` |
| `GetAWSSSMEndpointURL(env)` | Local URL | AOOSTAR URL | `""` |

---

## environment

**Import:** `github.com/tguidoux/opensqs/pkgs/v1/environment`

**File:** `env.go`

### Overview

Simple environment enum used throughout the codebase to switch behavior between deployment targets.

### Type

```go
type Environment string
```

### Constants

| Constant | Value | Description |
|----------|-------|-------------|
| `PROD` | `"prod"` | Production environment |
| `STAGING` | `"staging"` | Staging environment |
| `LOCAL` | `"local"` | Local development |
| `AOOSTAR` | `"aoostar"` | Custom environment |

### Usage

```go
import "github.com/tguidoux/opensqs/pkgs/v1/environment"

if env == environment.LOCAL {
    // Local-only behavior
}
```

---

## logger

**Import:** `github.com/tguidoux/opensqs/pkgs/v1/logger`

**Files:** `factory.go`, `contextual_logger.go`, `uncontextual_logger.go`

### Overview

Structured JSON logging built on Go's `log/slog` package. Provides two logger types:

- **Contextual** — Requires `context.Context` for each log call (for request-scoped tracing)
- **Uncontextual** — Uses `context.Background()` internally (simpler API)

Both implement `LoggerInterface`.

### LoggerInterface

```go
type LoggerInterface interface {
    Debug(msg string, extra ...map[string]any) LoggerInterface
    Info(msg string, extra ...map[string]any) LoggerInterface
    Error(msg string, extra ...map[string]any) LoggerInterface
    Warning(msg string, extra ...map[string]any) LoggerInterface
    Fatal(msg string, extra ...map[string]any) LoggerInterface
    Debugf(format string, args ...any) LoggerInterface
    Infof(format string, args ...any) LoggerInterface
    Errorf(format string, args ...any) LoggerInterface
    Warningf(format string, args ...any) LoggerInterface
    Fatalf(format string, args ...any) LoggerInterface
    WithExtra(extra ...map[string]any) LoggerInterface
    GetWriter() *os.File
    Printf(msg string, args ...interface{})
}
```

### LoggerType

```go
type LoggerType int

const (
    UncontextualLoggerType LoggerType = 0
    ContextLoggerType      LoggerType = 1
)
```

### Factory Functions

#### `New(name string, loggerType LoggerType, level ...slog.Level) LoggerInterface`

Creates a logger by type. Both types currently return `UncontextualLogger`.

```go
log := logger.New("my-service", logger.UncontextualLoggerType, slog.LevelInfo)
```

#### `NewUncontextual(name string, level ...slog.Level) LoggerInterface`

Creates an uncontextual logger.

```go
log := logger.NewUncontextual("my-service")
```

#### `NewContextual(name string, level ...slog.Level) *Logger`

Creates a contextual logger (returns concrete `*Logger` type).

```go
log := logger.NewContextual("my-service", slog.LevelDebug)
```

### Log Level Resolution

Log level is determined by:
1. Variadic `level` argument (if provided)
2. `DEBUG` environment variable (if set)
3. Default: `slog.LevelInfo`

### Output Format

JSON on `stdout`. Example:

```json
{
  "asctime": "2025-01-15T10:30:00.000Z",
  "level": "INFO",
  "msg": "Server started",
  "logger.name": "opensqs-server",
  "extra": {"port": "9324"}
}
```

Note: The `time` field is renamed to `asctime` in the output.

### Contextual Logger (`*Logger`)

All methods take `context.Context` as first argument:

```go
log := logger.NewContextual("my-service")
log.Info(ctx, "Request received", map[string]any{"method": "POST", "path": "/"})
log.Infof(ctx, "Queue %s created", queueName)
log.Error(ctx, "Failed to process", map[string]any{"error": err.Error()})
log.Fatal(ctx, "Critical failure")  // Calls os.Exit(1)
```

#### `WithExtra(extra ...map[string]any) *Logger`

Adds extra fields to the logger. Returns the same instance (fields accumulate):

```go
log := log.WithExtra(map[string]any{"requestId": "abc-123"})
log.Info(ctx, "Processing")  // Includes requestId in output
```

#### `Printf(msg string, args ...interface{})`

Implements the `rs/cors` Logger interface for compatibility with HTTP middleware.

### Uncontextual Logger (`*UncontextualLogger`)

Same API as contextual logger but without `context.Context`:

```go
log := logger.NewUncontextual("my-service")
log.Info("Server started", map[string]any{"port": "9324"})
log.Infof("Queue %s created", queueName)
log.Error("Failed", map[string]any{"error": err.Error()})
```

#### `GetLogger() *Logger`

Returns the underlying contextual `*Logger` for cases where context is needed:

```go
unctxLog := logger.NewUncontextual("my-service")
ctxLog := unctxLog.GetLogger()
ctxLog.Info(ctx, "With context")
```

### Method Chaining

All log methods return `LoggerInterface`, enabling chaining:

```go
log.Info("Step 1").
    WithExtra(map[string]any{"step": 1}).
    Info("Step 2")
```

---

## queue

**Import:** `github.com/tguidoux/opensqs/pkgs/v1/queue`

The queue engine package provides the core SQS-compatible messaging functionality. It can be used as a standalone Go library or as the backend for the OpenSQS server.

### Sub-packages

| Package | Import Path | Description |
|---------|-------------|-------------|
| `queue` | `.../pkgs/v1/queue` | `QueueManager`, `Queue`, `QueueAttributes`, `Limits`, error types |
| `queue/types` | `.../pkgs/v1/queue/types` | `Message`, `MessageAttribute`, `SQSError` interface, constants |
| `queue/dlq` | `.../pkgs/v1/queue/dlq` | `RedrivePolicy` struct and parser |
| `queue/store` | `.../pkgs/v1/queue/store` | `Store` interface, `StoreFactory`, `StoreConfig`, `RedriveFunc` |
| `queue/store/memory` | `.../pkgs/v1/queue/store/memory` | In-memory `MemoryStore` implementation |
| `queue/store/sqlite` | `.../pkgs/v1/queue/store/sqlite` | Persistent `SQLiteStore` implementation |

### Key Types

- **`QueueManager`** — Manages multiple queues, creates stores via `StoreFactory`
- **`Queue`** — Single queue with attributes, tags, and underlying `Store`
- **`Store` interface** — Storage contract (SendMessage, ReceiveMessages, DeleteMessage, etc.)
- **`StoreFactory`** — Function type that creates a `Store` per queue
- **`StoreConfig`** — FIFO/DLQ configuration passed to stores
- **`RedrivePolicy`** — DLQ configuration parsed from JSON

### Features

- 20 SQS API actions implemented
- FIFO queues with message groups, deduplication, and sequence numbers
- Dead-letter queues with automatic redrive
- Message attributes and system attributes
- Dual protocol support (Query + JSON)
- Long polling with `WaitTimeSeconds`
- Pluggable storage (MemoryStore + SQLiteStore)
- Signed receipt handles (HMAC-SHA256)

> **See:** [Queue Library](queue-library.md) for detailed API documentation and usage examples.
