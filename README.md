# OpenSQS Monorepo

## Philosophy

**Hermetic, reproducible builds.** Everything runs through Bazel to ensure consistent builds anywhere. No "works on my machine" problems. Dependencies are pinned, toolchains are hermetic, and all tools (formatters, linters) are managed through Bazel.

**Go-only by design.** Go for high-performance services and CLI tools. Shared libraries in `pkgs/v1/` provide common functionality across all services.

**Service-oriented architecture.** Each service is independently deployable with its own configuration.

## Structure

```
opensqs/
├── apps/               # Applications
│   └── go/            # Go services and playground
├── pkgs/v1/           # Shared Go libraries (config, logger, environment)
└── tools/             # Custom Bazel rules and dev tools
```

**Custom Bazel Rules:** Use `opensqs_go_*` rules instead of standard rules. These wrap standard rules with project conventions and distroless container support.

## Quick Setup

1. **Install Bazelisk** (manages Bazel versions automatically):
   ```bash
   brew install bazelisk  # macOS
   ```

2. **Clone and setup:**
   ```bash
   git clone <repo>
   cd opensqs
   bazel run //:clean  # Initialize and format everything
   ```

3. **That's it.** Bazel handles all toolchains and dependencies hermetically.

## Common Commands

```bash
# Format and update everything
bazel run //:clean

# Language-specific cleanup
bazel run //:go.clean       # Go: update BUILD files and dependencies
bazel run //:bazel.clean    # Bazel: format BUILD files with buildifier

# Run services
bazel run //apps/go/playground/cmd_hello_world:cmd_hello_world

# Build and test
bazel build //apps/go/...
bazel test //...
bazel test //pkgs/v1/config/tests:go_default_test

# Containerize
bazel run //apps/go/playground/cmd_hello_world:image_load_docker
```

## Adding Dependencies

**Go:** Add import to code, run `bazel run //:go.clean`, follow buildozer command

## Shared Libraries (`pkgs/v1/`)

| Package | Description |
|---------|-------------|
| `config/` | Configuration loading from YAML with schema validation |
| `environment/` | Environment enum (LOCAL, STAGING, PROD, AOOSTAR) |
| `logger/` | Structured logging for Go |

## Configuration

**Config:** Environment-specific YAML files (`config.yaml` for local, `values.<env>.yaml` for deployments). Secrets via AWS SSM Parameter Store.

**Environments:** LOCAL, STAGING, PROD, AOOSTAR
