# Contributing to OpenSQS

First off, thank you for considering contributing to OpenSQS! 🎉

This document describes how to set up your development environment and submit changes.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Prerequisites](#prerequisites)
- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Build System (Bazel)](#build-system-bazel)
- [Coding Conventions](#coding-conventions)
- [Testing](#testing)
- [Submitting Changes](#submitting-changes)
- [Reporting Bugs](#reporting-bugs)
- [Feature Requests](#feature-requests)

## Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| [Go](https://go.dev/dl/) | 1.25+ | `brew install go` |
| [Bazelisk](https://github.com/bazelbuild/bazelisk) | Latest | `brew install bazelisk` |
| [Git](https://git-scm.com/) | 2.20+ | `brew install git` |

Bazelisk manages the Bazel version automatically — you don't need to install Bazel directly.

## Development Setup

```bash
# Clone your fork
git clone https://github.com/<your-username>/opensqs.git
cd opensqs

# Add upstream remote
git remote add upstream https://github.com/tguidoux/opensqs.git

# Initialize the workspace (formats Bazel files, regenerates BUILD files)
bazel run //:clean

# Verify everything builds and tests pass
bazel test //...
```

## Project Structure

```
opensqs/
├── apps/go/           # Applications (server, playground examples)
├── pkgs/v1/           # Shared libraries (config, environment, logger, queue)
├── tools/             # Custom Bazel rules and dev tooling
├── deploy/helm/       # Helm chart for Kubernetes deployment
├── docs/              # Documentation
└── .github/           # CI workflows, issue/PR templates
```

See [docs/architecture.md](docs/architecture.md) for the full architecture overview.

## Build System (Bazel)

OpenSQS uses Bazel with custom `opensqs_go_*` rules for hermetic, reproducible builds.

### Common Commands

```bash
# Full workspace cleanup (format + regenerate)
bazel run //:clean

# Go-specific: update BUILD files and dependencies
bazel run //:go.clean

# Bazel file formatting
bazel run //:bazel.clean

# Regenerate BUILD files after adding Go files or packages
bazel run //:gazelle

# Build everything
bazel build //apps/go/...

# Run all tests
bazel test //...

# Run the server
bazel run //apps/go/server:opensqs-server

# Build and load Docker image
bazel run //apps/go/server:opensqs_server_image_platform_transition_load_docker
```

### Adding Go Dependencies

1. Add the import to your Go code
2. Add the dependency to `go.mod`
3. Run `bazel run //:go.clean`
4. Follow any buildozer command prompts to update BUILD files

### Custom Bazel Rules

We use custom macros instead of standard `go_*` rules:

| Custom Rule | Replaces |
|-------------|----------|
| `opensqs_go_library` | `go_library` |
| `opensqs_go_binary` | `go_binary` |
| `opensqs_go_test` | `go_test` |
| `opensqs_go_image` | `go_image` |

After adding new Go files or packages, always run `bazel run //:gazelle` to auto-generate BUILD.bazel entries.

## Coding Conventions

### Go Style

- Follow [Effective Go](https://go.dev/doc/effective_go) and the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use `gofmt` / `goimports` for formatting (run via `bazel run //:bazel.clean`)
- Static analysis is enforced via [Bazel nogo](https://github.com/bazelbuild/rules_go/blob/master/docs/go/core.rst#nogo) — configured in `nogo_config.json` at the repo root. It runs automatically on every `bazel build` / `bazel test`, so no separate lint command is needed
- Prefer simple, readable code over clever solutions
- Keep functions small and focused on a single responsibility
- Use clear, descriptive variable and function names
- Prefer early returns and guard clauses over deep nesting

### Error Handling

- Handle errors explicitly — never ignore them
- Provide meaningful error messages with context
- Use `fmt.Errorf("doing X: %w", err)` for wrapping
- Fail fast and fail clearly — don't continue with invalid state

### Logging

- Use structured logging via the `pkgs/v1/logger/` package
- Include context (request IDs, queue names) in log entries
- Never log sensitive data (secrets, message payloads in production)

### Configuration

- Configuration structs embed `config.ConfigI[T]` for automatic loading and validation
- Use YAML tags for all config fields
- Never hardcode secrets — reference AWS SSM Parameter Store paths in production configs

### Visibility

- Use `//visibility:public` for shared libraries in `pkgs/v1/`
- Use `//visibility:private` for internal packages

## Testing

### Test Conventions

- Test files go in a `tests/` subfolder within each package:
  ```
  pkgs/v1/mypackage/
  ├── BUILD.bazel          # Library definitions only
  ├── mycode.go
  └── tests/
      ├── BUILD.bazel      # Test definitions
      └── mycode_test.go   # Go tests
  ```
- Use `testify/assert` for assertions
- Test the happy path, error cases, and edge conditions
- Make tests independent — they should not rely on each other
- Use descriptive test names that explain what is being tested

### Running Tests

```bash
# Run all tests
bazel test //...

# Run specific package tests
bazel test //pkgs/v1/queue/tests:go_default_test
bazel test //apps/go/server/handlers/tests:go_default_test

# Run with verbose output
bazel test --test_output=all //pkgs/v1/queue/tests:go_default_test
```

After adding new test files, run `bazel run //:gazelle` to generate BUILD.bazel entries.

## Submitting Changes

### Pull Request Process

1. **Fork** the repository and create a feature branch from `main`:
   ```bash
   git checkout -b my-feature
   ```
2. **Make** your changes, following the coding conventions above
3. **Test** your changes:
   ```bash
   bazel run //:clean    # Clean workspace
   bazel test //...       # Run all tests
   ```
4. **Commit** with a clear, descriptive message:
   ```bash
   git commit -m "Add support for FIFO message group deduplication"
   ```
   - Use the imperative mood ("Add" not "Added")
   - Reference issues: `Fixes #123` or `Refs #123`
5. **Push** to your fork:
   ```bash
   git push origin my-feature
   ```
6. **Open a Pull Request** against the `main` branch
   - Fill in the PR template completely
   - Link related issues
   - Ensure CI passes

### PR Checklist

Before submitting your PR, verify:

- [ ] Code follows the project's coding conventions
- [ ] Tests are added for new functionality
- [ ] All tests pass: `bazel test //...`
- [ ] BUILD.bazel files are updated: `bazel run //:gazelle`
- [ ] Bazel files are formatted: `bazel run //:bazel.clean`
- [ ] No secrets or credentials are committed
- [ ] Documentation is updated if needed
- [ ] Commit messages are clear and descriptive

### Commit Message Guidelines

- Use the imperative mood: "Add" not "Added", "Fix" not "Fixed"
- Keep the subject line under 72 characters
- Reference issues when applicable: `Fixes #123`
- For breaking changes, start with `BREAKING CHANGE:`

## Releasing

Releases are managed by the release tool at [`tools/release`](tools/release/main.go). It handles the full release flow: Helm chart version bump, git tag, OCI image build/push (multi-arch via Bazel), and GitHub Release creation.

### Creating a Release

```bash
# Auto-increment patch version (e.g., v0.1.0 → v0.1.1)
bazel run //tools/release:release

# Release a specific version
bazel run //tools/release:release -- v1.0.0

# Preview what would happen (no changes made)
bazel run //tools/release:release -- --dry-run

# Create a pre-release
bazel run //tools/release:release -- v0.9.0 --pre-release

# Skip specific steps
bazel run //tools/release:release -- v1.0.0 --skip-image
bazel run //tools/release:release -- v1.0.0 --skip-tag --skip-release
```

Releases can also be triggered via the **Release** GitHub Actions workflow (manual dispatch with inputs for version, draft, pre-release, and skip flags).

### Release Process

1. The tool updates `deploy/helm/Chart.yaml` (`version` and `appVersion`)
2. Commits the version bump
3. Creates an annotated git tag and pushes it
4. Builds and pushes the multi-arch OCI image to GHCR via `bazel run //apps/go/server:opensqs_server_image_push --stamp`
5. Creates a GitHub Release with auto-generated notes from commits since the last tag

### Updating the Changelog

After a release, update [`CHANGELOG.md`](CHANGELOG.md) with the changes for the new version, following the [Keep a Changelog](https://keepachangelog.com/) format.

## Reporting Bugs

Bugs are tracked as [GitHub Issues](https://github.com/tguidoux/opensqs/issues). Use the **Bug Report** template and include:

- A clear, descriptive title
- Steps to reproduce the issue
- Expected vs. actual behavior
- Your environment (OS, Go version, OpenSQS version)
- Relevant logs or error messages

## Feature Requests

Feature requests are also tracked as [GitHub Issues](https://github.com/tguidoux/opensqs/issues). Use the **Feature Request** template and include:

- A clear description of the proposed feature
- The use case or problem it solves
- Any alternatives you've considered
- Whether you're willing to help implement it

## Questions

For questions about using OpenSQS, please [open a discussion](https://github.com/tguidoux/opensqs/discussions) rather than an issue.

---

Thank you for contributing to OpenSQS! 🚀
