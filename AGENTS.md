# AGENTS.md

Guide for AI agents and human contributors working on the OpenSQS codebase.
Canonical build/convention details also live in `.github/copilot-instructions.md` — read it for Bazel workflows.

## Golden Rules

1. **Search before writing.** Grep for similar patterns, read existing implementations, and copy/adapt them. Never invent a pattern that already exists.
2. **Never create summary documents** of conversations, completed work, or code changes.
3. **Use Bazel** for all builds, tests, and dependency management. No `go run`, `go build`, or `go test` directly.
4. **Prefer constants** over magic strings. SQS action names, attribute names, and limits all have constants in `pkgs/v1/queue/types/constants.go`.
5. **Use AI TODOs** when dealing with multi-step tasks.
6. **Keep docs and agent instructions in sync.** When you change public APIs, add new features, or modify architecture, update `AGENTS.md`, `.github/copilot-instructions.md`, and relevant files in `docs/` in the same PR.

---

## Project Layout

```
apps/go/
├── server/                    # SQS-compatible HTTP server
│   ├── main.go                # Entry point — startup flow, wiring, graceful shutdown
│   ├── config.go              # ServerConfig struct (embeds ConfigI[ServerConfig])
│   ├── config.yaml            # Local dev configuration
│   ├── request_handler.go     # HTTP handler → protocol detection → handler dispatch
│   ├── error_response.go      # Error → SQSError → XML/JSON response
│   ├── startup_queues.go      # Pre-create queues from config at boot
│   ├── handlers/              # SQS API action handlers (dispatched by action name)
│   ├── protocol/              # Query (XML) and JSON protocol parsers + response marshallers
│   ├── middleware/            # Request logging, SigV4 auth, rate limiting
│   ├── health/                # Health check server (port 8001, non-local)
│   ├── metrics/               # Prometheus metrics collector
│   ├── tls/                   # TLS config loader
│   ├── ui/                    # Web UI for queue management
│   └── serverbase/            # Shared server abstractions
├── playground/                # Example programs using the queue library
pkgs/v1/
├── config/                   # Generic YAML config loader (ConfigI[T])
├── environment/              # Environment enum (LOCAL, STAGING, PROD)
├── id/                       # ID generation (hex IDs for request tracking)
├── logger/                   # Structured logging (contextual + uncontextual)
└── queue/                    # Queue engine — the core domain
    ├── queue.go              # Queue struct, attribute access, URL/ARN construction
    ├── manager.go            # QueueManager — lifecycle, lookup, DLQ wiring
    ├── attributes.go         # QueueAttributes — thread-safe, atomic batch updates
    ├── limits.go             # SQS limit enforcement (Strict/Relaxed modes)
    ├── errors.go             # SQS error factory functions
    ├── store/                # Store interface + memory/sqlite/badger implementations
    ├── types/                # Message, SQSError, constants — no circular deps
    └── dlq/                   # Dead-letter queue, message move tasks
tools/
├── rules/golang/             # opensqs_go_* Bazel macros
├── release/                  # Release tool (version bump, tag, image push, GH release)
├── ci/                       # CI tooling (affected target detection)
└── platforms/                # Bazel platform transitions (multi-arch)
deploy/helm/                  # Helm chart for Kubernetes deployment
```

---

## Architectural Patterns

### Dependency Injection

The codebase uses **constructor injection** and **factory closures** — no DI framework, no global state.

**Handler** receives all dependencies via `NewHandler()`:
```go
// apps/go/server/handlers/handler.go
type Handler struct {
    manager     *queue.QueueManager
    limits      *queue.Limits
    autoCreate  bool
    moveTaskMgr *dlq.MoveTaskManager
    metrics     *metrics.Collector
    log         logger.LoggerInterface
}
```

**StoreFactory** is a function type, not an interface — the `QueueManager` calls it to create per-queue stores:
```go
// pkgs/v1/queue/store/store.go
type StoreFactory func(queueName string, visibilityTimeout int, serverSecret []byte, attrs StoreConfig) (Store, error)
```

In `main.go`, the factory is a **closure that captures the DB handle**:
```go
// Pattern: closure captures DB, QueueManager is backend-agnostic
storeFactory := func(queueName string, vt int, secret []byte, cfg store.StoreConfig) (store.Store, error) {
    return memory.NewMemoryStore(queueName, vt, secret, cfg)
}
manager := queue.NewQueueManager(nodeAddress, accountID, region, serverSecret, storeFactory)
```

**When adding a new store backend**: implement the `Store` interface, create a factory function, and wire it in `main.go`'s `switch cfg.SQS.StorageType`. No changes to `QueueManager` or `Queue` needed.

### Interface-Based Design

Key interfaces and where they live:

| Interface | Location | Purpose |
|-----------|----------|---------|
| `store.Store` | `pkgs/v1/queue/store/store.go` | Message storage backend (memory, sqlite, badger) |
| `logger.LoggerInterface` | `pkgs/v1/logger/` | Structured logging |
| `handlers.Request` | `apps/go/server/handlers/` | Protocol-agnostic request access |
| `types.SQSError` | `pkgs/v1/queue/types/types.go` | SQS error with code, HTTP status, type |

**Rule**: Define interfaces in the package that **consumes** them, not the package that implements them. The `Store` interface lives in `store/` (consumer), implementations live in `store/memory/`, `store/sqlite/`, `store/badger/`.

### Adapter Pattern (Protocol Layer)

The `Request` interface decouples handler logic from wire protocol:

```
HTTP Request
  → protocol.ParseQueryRequest() / ParseJSONRequest()
    → QueryRequestAdapter / JSONRequestAdapter (implements Request)
      → Handler.HandleRequest(req Request)
```

**When adding a new SQS action**: add the action constant to `types/constants.go`, add a case to the `switch` in `Handler.HandleRequest()`, implement the handler in the appropriate `actions_*.go` file, and add response structs in `protocol/marshal.go`.

### Error Handling

Errors are **typed SQS errors**, not plain `error` values:

```go
// pkgs/v1/queue/types/types.go
type SQSError interface {
    error
    Code() string           // "InvalidParameterValue", "QueueDoesNotExist", etc.
    HTTPStatusCode() int    // 400, 403, 500
    ErrorType() string      // "Sender" or "Receiver"
    Message() string
}
```

Factory functions in `queue/errors.go` create properly configured errors:
```go
return nil, queue.NewQueueDoesNotExist(queueURL)
return nil, queue.NewInvalidParameterValue("visibilityTimeout", "must be 0-43200")
```

`error_response.go` uses `errors.As()` to extract `SQSError` from any error. If the error isn't an `SQSError`, it's wrapped in `queue.NewInternalError()` — **unknown errors never leak as 500s without context**.

**Rule**: Always return `*SQSError` from domain logic. Use the factory functions — never construct `ConcreteSQSError` directly.

### Thread Safety

- `QueueAttributes` uses `sync.RWMutex` with **atomic batch updates** — `SetAttributes()` validates against a clone first, only commits if all attributes pass.
- `Queue` uses **fine-grained locking** — separate mutexes for attributes and tags.
- `QueueManager.Shutdown()` snapshots queues under lock, releases the lock, then closes stores — **never holds a lock during slow operations**.
- `MemoryStore` deep-copies messages before returning them via `copyMessage()` — callers can't mutate internal state.
- Time is injectable via `atomic.Pointer` in the store package — `SetNowFunc()` for deterministic testing.

### Configuration

Generic config loading with type constraints:

```go
// pkgs/v1/config/config.go
type ConfigI[ConfigType any] interface {
    Validate() error
    WithValidation() ConfigType
}

// Usage in apps/go/server/config.go
type ServerConfig struct {
    config.ConfigI[ServerConfig]
    Server  ServerConfigSection `yaml:"server"`
    SQS     SQSConfig           `yaml:"sqs"`
    Log     LogConfig           `yaml:"log"`
    // ...
}

// Loading
cfg := config.NewConfigFromEnv[ServerConfig]().Config
```

**When adding config fields**: add the field to the config struct with a `yaml:"..."` tag, add validation in `Validate()`, and document it in `docs/configuration.md`.

---

## Code Quality Standards

### Structure
- **Small files, single responsibility.** Handler actions are split by domain: `actions_queue.go`, `actions_message.go`, `actions_batch.go`, etc.
- **Early returns and guard clauses.** No deep nesting.
- **Self-documenting names.** `resolveQueue()`, `attributesMatch()`, `redriveMessage()` — the code tells a story.
- **Group related code.** Types in `types/`, errors in `errors.go`, limits in `limits.go`.

### Go Conventions
- Import path prefix: `github.com/tguidoux/opensqs/`
- Use `opensqs_go_*` Bazel rules, never standard `go_*` rules
- Configuration structs embed `config.ConfigI[T]`
- Services follow: `main.go`, `config.go`, `config.yaml`, `BUILD.bazel`
- Health checks on `/health` endpoint, port 8001 for non-local environments
- `//visibility:public` for shared libraries in `pkgs/v1/`
- `//visibility:private` for internal packages

### Error Handling
- Handle errors explicitly — never ignore them
- Use `fmt.Errorf("doing X: %w", err)` for wrapping with context
- Return `*SQSError` from domain logic via factory functions
- Fail fast — don't continue with invalid state
- Add context: queue names, message IDs, request IDs in logs

### Logging
- Use `logger.LoggerInterface` — never `fmt.Println` or `log` package
- Include structured fields: `requestId`, `queueName`, `messageId`
- Never log secrets or message payloads in production
- Fluent API — all methods return `LoggerInterface` for chaining

### Security
- Never hardcode secrets — use AWS SSM Parameter Store in production
- HMAC-SHA256 signed receipt handles (server secret never logged)
- SigV4 authentication in middleware
- Distroless container images
- Principle of least privilege

---

## Testing

### Conventions
- Tests in `tests/` subfolders within each package:
  ```
  pkgs/v1/mypackage/
  ├── BUILD.bazel
  ├── mycode.go
  └── tests/
      ├── BUILD.bazel
      └── mycode_test.go
  ```
- Use `testify/assert` for assertions
- Test happy path, error cases, and edge conditions
- Tests are independent — no shared state between tests
- Descriptive test names: `TestReceiveMessage_VisibilityTimeoutExpires`

### Running Tests
```bash
bazel test //...                                          # All tests
bazel test //pkgs/v1/queue/tests:go_default_test          # Specific package
bazel test --test_output=all //pkgs/v1/queue/tests:go_default_test  # Verbose
```

After adding new test files: `bazel run //:gazelle` to generate BUILD.bazel entries.

---

## Development Workflow

```bash
bazel run //:clean        # Full workspace cleanup (format + regenerate)
bazel run //:go.clean     # Go-specific: gazelle + update repos (after adding deps)
bazel run //:bazel.clean  # Bazel file formatting (buildifier)
bazel run //:gazelle      # Regenerate BUILD files (after adding Go files/packages)
```

### Adding a Go Dependency
1. Add the import to your Go code
2. Add the dependency to `go.mod`
3. Run `bazel run //:go.clean`
4. Follow any buildozer command prompts

### Static Analysis
- **nogo** (Bazel's Go analyzer) runs automatically on every `bazel build` / `bazel test`
- Configured in `nogo_config.json` at the repo root
- No separate lint command needed

### Releasing
```bash
bazel run //tools/release:release                    # Auto-increment patch
bazel run //tools/release:release -- v1.0.0          # Specific version
bazel run //tools/release:release -- --dry-run       # Preview
```
See [CONTRIBUTING.md](CONTRIBUTING.md) for the full release process.

---

## Documentation Maintenance

Documentation is part of the codebase — it must stay accurate or it becomes misleading.

### Files to Keep in Sync

| File | What to Update |
|------|----------------|
| `AGENTS.md` | Project layout, architectural patterns, common task recipes, code quality standards |
| `.github/copilot-instructions.md` | Build system details, Bazel workflows, development commands, project structure |
| `docs/api-reference.md` | SQS API actions, queue attributes, request/response formats |
| `docs/configuration.md` | Config fields, YAML structure, environment variables |
| `docs/architecture.md` | High-level architecture, component diagrams, data flow |
| `docs/queue-library.md` | Public queue library API, store interface, usage examples |
| `docs/protocol.md` | Query/JSON protocol details, error response formats |
| `README.md` | Feature list, quickstart, badges, community links |
| `CONTRIBUTING.md` | Development setup, coding conventions, PR process, release steps |
| `CHANGELOG.md` | Notable changes per version (follows [Keep a Changelog](https://keepachangelog.com)) |

### When to Update

- **New SQS API action** → update `docs/api-reference.md`, `AGENTS.md` common tasks, `docs/protocol.md`
- **New queue attribute** → update `docs/api-reference.md`, `docs/configuration.md` (if configurable)
- **New store backend** → update `docs/queue-library.md`, `docs/architecture.md`, `AGENTS.md` common tasks
- **New config field** → update `docs/configuration.md`, `apps/go/server/config.yaml` (with default)
- **New middleware** → update `docs/architecture.md`, `AGENTS.md` common tasks
- **New package** → update `AGENTS.md` project layout, `.github/copilot-instructions.md` project structure
- **Breaking change** → update all affected docs, add entry to `CHANGELOG.md`
- **New dependency** → update `docs/queue-library.md` or `docs/architecture.md` if architecturally significant

### Rules

- **Docs live next to code.** If you add a feature, update the docs in the same PR — no "docs later."
- **AGENTS.md and copilot-instructions.md are complementary.** `AGENTS.md` covers architecture and how-to; `copilot-instructions.md` covers build system and conventions. Update both when the change crosses both domains.
- **Don't document what doesn't exist.** Remove references to deleted features, packages, or patterns.
- **Keep examples runnable.** Code snippets in docs should compile and work — stale examples are worse than no examples.
- **CHANGELOG.md is for user-facing changes only.** Internal refactors that don't affect the API or behavior don't need an entry.

---

## Common Tasks

### Add a New SQS API Action
1. Add the action constant to `pkgs/v1/queue/types/constants.go`
2. Add a case to the `switch` in `handlers/handler.go` → `HandleRequest()`
3. Implement the handler in the appropriate `handlers/actions_*.go` file
4. Add request getter methods to the `Request` interface if needed
5. Implement getters in both `QueryRequestAdapter` and `JSONRequestAdapter`
6. Add response structs in `protocol/marshal.go` (XML + JSON)
7. Add tests in `handlers/tests/`
8. Run `bazel run //:gazelle` to update BUILD files

### Add a New Queue Attribute
1. Add the constant to `pkgs/v1/queue/types/constants.go`
2. Add the field to `QueueAttributes` in `pkgs/v1/queue/attributes.go`
3. Add a case to `GetAttribute()` and `SetAttribute()` with validation
4. Add to the `validAttributes` slice
5. Add to `cloneUnlocked()` and `validateAndSet()`
6. Update `docs/api-reference.md`

### Add a New Store Backend
1. Create a new package under `pkgs/v1/queue/store/<backend>/`
2. Implement the `Store` interface (9 methods)
3. Create a factory function matching `store.StoreFactory`
4. Wire it in `apps/go/server/main.go` → `switch cfg.SQS.StorageType`
5. Add tests in `store/<backend>/tests/`
6. Run `bazel run //:gazelle`

### Add Middleware
1. Create a file in `apps/go/server/middleware/`
2. Implement `func(http.Handler) http.Handler`
3. Wire it in `main.go` via `middleware.Chain()`
4. First middleware in the chain = outermost wrapper
