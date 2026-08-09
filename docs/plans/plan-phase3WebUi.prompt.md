# Plan: Phase 3 — Web UI

Build an embedded web dashboard for OpenSQS, following the RFC Phase 3 spec. Server-side rendered HTML + minimal vanilla JS, embedded in the binary via `go:embed`, served on a separate port (9325) using the same pattern as the health check server.

## TL;DR

Create `apps/go/server/ui/` package with HTML templates, CSS, and JS embedded via `go:embed`. The UI calls the QueueManager API directly (not via HTTP SQS protocol) for simplicity. A `ui.Server` struct mirrors `health.Server` — started in `main.go` in a goroutine, stopped via defer on graceful shutdown. Config adds `ui:` section with `enabled` and `port` fields.

---

## Steps

### Phase 3A: Config + Server Skeleton

1. **Add `UIConfig` to `apps/go/server/config.go`**
   - New struct: `UIConfig` with `Enabled bool` (yaml:`enabled`) and `Port int` (yaml:`port`)
   - Add `UI UIConfig` field to `ServerConfig` struct (yaml:`ui`)
   - Default: enabled=true, port=9325

2. **Add `ui:` section to `apps/go/server/config.yaml`**
   - `ui: { enabled: true, port: 9325 }`

3. **Create `apps/go/server/ui/` package** — *parallel with 3B*
   - `server.go` — `Server` struct wrapping `*http.Server`, `NewServer(port, manager, log)`, `Start()`, `Stop(ctx)` — mirrors `health/server.go` pattern exactly
   - `handlers.go` — HTTP handler functions for each UI route
   - `BUILD.bazel` — `opensqs_go_library` with deps on queue, queue/types, logger

4. **Wire UI server into `apps/go/server/main.go`**
   - Import `ui` package
   - After creating `handler` and before starting HTTP server, create `uiServer` if `cfg.UI.Enabled`
   - Start in goroutine, stop via defer with 5s context timeout (same as health server)
   - Log startup: `"starting UI server on :%d"`

5. **Add `ui` dep to `apps/go/server/BUILD.bazel`**
   - Add `"//apps/go/server/ui:go_default_library"` to deps list

6. **Run `bazel run //:gazelle`** to generate BUILD files

### Phase 3B: HTML Templates + Static Assets

7. **Create `apps/go/server/ui/templates/` directory** — *parallel with 3A*
   - `layout.html` — Base template with nav, theme toggle, auto-refresh toggle, content block
   - `index.html` — Queue list view: table of queues with name, type (standard/FIFO), message counts (available/in-flight/delayed), URL, actions (view, purge, delete)
   - `queue.html` — Queue detail view: attributes table, tags, DLQ info, send message form, receive messages panel, purge/delete actions
   - `create_queue.html` — Create queue form: name, FIFO toggle, content-based dedup toggle, visibility timeout, delay seconds, redrive policy (DLQ ARN + maxReceiveCount)

8. **Create `apps/go/server/ui/static/` directory**
   - `style.css` — Dark/light theme via CSS variables, responsive table layout, minimal styling (no framework)
   - `app.js` — Auto-refresh polling (fetch + DOM update), form submission via fetch, purge/delete confirmation dialogs, theme toggle with localStorage

9. **Create `apps/go/server/ui/handlers.go`** — *depends on 3A.3, 3B.7, 3B.8*
   - `//go:embed templates/*.html` and `//go:embed static/*` directives
   - Route registration on `http.ServeMux`:
     - `GET /` → queue list (renders `index.html` with `qm.ListQueues("")` data)
     - `GET /queues/{name}` → queue detail (renders `queue.html` with queue attrs, tags, counts)
     - `GET /queues/new` → create queue form
     - `POST /queues` → create queue (parse form, call `qm.CreateQueue`)
     - `POST /queues/{name}/delete` → delete queue (`qm.DeleteQueue`)
     - `POST /queues/{name}/purge` → purge queue (`qm.PurgeQueue`)
     - `POST /queues/{name}/messages` → send message (`q.Store().SendMessage`)
     - `POST /queues/{name}/messages/{receiptHandle}/delete` → delete message
     - `GET /api/queues` → JSON endpoint for auto-refresh (returns queue list as JSON)
     - `GET /api/queues/{name}/messages` → JSON endpoint for message list refresh
   - Use `html/template` for SSR rendering with auto-escaping
   - Use `context.Background()` for store calls (UI doesn't have request-scoped cancellation needs)

### Phase 3C: UI Features Implementation

10. **Queue list view** — *depends on 3B.9*
    - Table columns: Name, Type (Standard/FIFO badge), Available, In-Flight, Delayed, URL, Actions
    - Actions: View (link), Purge (POST + confirm), Delete (POST + confirm)
    - "Create Queue" button at top
    - Auto-refresh: `GET /api/queues` every 5s (configurable via toggle), update table via JS

11. **Queue detail view** — *depends on 3B.9*
    - Attributes section: all queue attributes in a key-value table (visibility timeout, max message size, retention, delay, wait time, FIFO, dedup, ARN, redrive policy)
    - Tags section: display tags, add/remove tag form
    - Send message form: body textarea, delay seconds, message group ID (FIFO only), dedup ID (FIFO only)
    - Receive messages panel: "Receive" button → calls `q.Store().ReceiveMessages(ctx, 10, 30, 0)`, displays messages with body, ID, receipt handle, receive count, sent timestamp
    - Per-message actions: Delete (POST + confirm), Change Visibility (input + POST)
    - Auto-refresh message list via `GET /api/queues/{name}/messages`

12. **Create queue form** — *depends on 3B.9*
    - Fields: Queue Name, FIFO toggle (adds `.fifo` suffix hint), ContentBasedDeduplication toggle, VisibilityTimeout, DelaySeconds, ReceiveMessageWaitTimeSeconds
    - Optional RedrivePolicy: DLQ ARN text input + MaxReceiveCount number input
    - On submit: build `QueueAttributes` from form values, call `qm.CreateQueue(name, attrs)`
    - Redirect to queue detail on success, show error on failure

13. **Auto-refresh + theme toggle** — *depends on 3B.8*
    - JS: `setInterval` with configurable interval (2s/5s/10s/off)
    - Toggle button in nav bar
    - Theme: CSS variables, `data-theme` attribute on `<html>`, persisted in localStorage
    - Auto-refresh indicator (pulsing dot when active)

### Phase 3D: Tests + Verification

14. **Create `apps/go/server/ui/tests/` directory**
    - `BUILD.bazel` with `opensqs_go_test`
    - `handlers_test.go` — Test UI handlers using `httptest.NewServer`:
      - Test queue list renders with empty queues
      - Test queue list renders with populated queues
      - Test queue detail renders with attributes
      - Test create queue form submission
      - Test send/receive/delete message flow via UI
      - Test purge queue via UI
    - Use `memory.NewMemoryStore` factory + `queue.NewQueueManager` for test setup (same pattern as handler tests)

15. **Run `bazel run //:gazelle`** to generate BUILD files for tests

16. **Build + test verification**
    - `bazel build //apps/go/server/ui:go_default_library`
    - `bazel test //apps/go/server/ui/tests:go_default_test`
    - `bazel build //apps/go/server:opensqs-server`
    - Run server: `bazel run //apps/go/server:opensqs-server`
    - Open `http://localhost:9325` in browser, verify:
      - Queue list shows startup queues (orders, notifications, dead-letter.fifo)
      - Create queue form works
      - Queue detail shows attributes + send/receive
      - Purge/delete with confirmation
      - Auto-refresh toggle works
      - Theme toggle works

17. **Update documentation**
    - `docs/README.md` — Add UI section to features, quick start for UI
    - `docs/architecture.md` — Add UI server to architecture diagram, file layout, startup flow
    - `docs/configuration.md` — Add `ui:` config section

---

## Relevant files

- `apps/go/server/health/server.go` — **Pattern to follow** for `ui/server.go` — `Server` struct, `NewServer()`, `Start()`, `Stop(ctx)`
- `apps/go/server/main.go` — **Modify** — wire UI server startup/shutdown (after handler creation, alongside health server block)
- `apps/go/server/config.go` — **Modify** — add `UIConfig` struct + `UI` field on `ServerConfig`
- `apps/go/server/config.yaml` — **Modify** — add `ui:` section
- `apps/go/server/BUILD.bazel` — **Modify** — add `ui` dep
- `apps/go/server/ui/server.go` — **Create** — UI HTTP server (mirrors health server)
- `apps/go/server/ui/handlers.go` — **Create** — Route handlers + `go:embed` directives
- `apps/go/server/ui/templates/layout.html` — **Create** — Base layout
- `apps/go/server/ui/templates/index.html` — **Create** — Queue list
- `apps/go/server/ui/templates/queue.html` — **Create** — Queue detail
- `apps/go/server/ui/templates/create_queue.html` — **Create** — Create form
- `apps/go/server/ui/static/style.css` — **Create** — Themed styling
- `apps/go/server/ui/static/app.js` — **Create** — Auto-refresh, forms, theme toggle
- `apps/go/server/ui/BUILD.bazel` — **Create** — `opensqs_go_library`
- `apps/go/server/ui/tests/handlers_test.go` — **Create** — Handler tests
- `apps/go/server/ui/tests/BUILD.bazel` — **Create** — `opensqs_go_test`
- `pkgs/v1/queue/manager.go` — **Reference** — `ListQueues()`, `CreateQueue()`, `DeleteQueue()`, `PurgeQueue()`, `LookupQueue()`
- `pkgs/v1/queue/queue.go` — **Reference** — `Name()`, `Attributes()`, `Store()`, `Tags()`, `IsFifo()`, `URL()`, `ARN()`, `ApproximateNumberOfMessages()`
- `pkgs/v1/queue/store/store.go` — **Reference** — `Store` interface for `SendMessage`, `ReceiveMessages`, `DeleteMessage`, `ChangeMessageVisibility`, `Purge`
- `pkgs/v1/queue/types/types.go` — **Reference** — `Message` struct, `MessageAttribute` type

---

## Verification

1. `bazel run //:gazelle` — generates BUILD files for new `ui/` package and tests
2. `bazel build //apps/go/server/ui:go_default_library` — UI package compiles
3. `bazel test //apps/go/server/ui/tests:go_default_test` — all UI handler tests pass
4. `bazel build //apps/go/server:opensqs-server` — full server binary builds with UI
5. `bazel run //apps/go/server:opensqs-server` — server starts, log shows "starting UI server on :9325"
6. Manual browser test at `http://localhost:9325`:
   - Queue list shows startup queues (orders, notifications, dead-letter.fifo)
   - Click queue → detail view with attributes, tags, send/receive
   - Create new queue via form → appears in list
   - Send message via form → received in message panel
   - Delete message → removed from panel
   - Purge queue → confirmation → messages cleared
   - Delete queue → confirmation → removed from list
   - Auto-refresh toggle → table updates when messages change
   - Theme toggle → dark/light switch persists across reload

---

## Decisions

- **Separate port (9325)** for UI, not mounted on the SQS server port (9324) — avoids interfering with SQS protocol detection on the main port, mirrors health server pattern
- **UI calls QueueManager API directly** (not via HTTP SQS protocol) — simpler, no need to construct SQS-formatted requests, avoids protocol parsing overhead
- **Server-side rendered HTML** with `html/template` — per RFC decision, no SPA framework
- **Vanilla JS** for interactivity (auto-refresh, form submission, theme toggle) — no dependencies, keeps binary small
- **`go:embed`** for templates and static assets — single binary, no external file dependencies
- **UI enabled by default in local environment** — can be disabled via config for headless/production deployments
- **No authentication on UI** — matches SQS API behavior (credentials accepted but not validated), per RFC §7.4

## Scope boundaries

- **Included**: Queue list, queue detail, create queue, send/receive/delete messages, purge, delete, auto-refresh, dark/light theme
- **Excluded**: Message move tasks UI (Phase 4), metrics dashboard (Phase 4), TLS support (Phase 5), real-time WebSocket updates (future), message attribute editing in send form (can be added later)
