# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Open-source repository preparation: LICENSE, CONTRIBUTING, CODE_OF_CONDUCT, SECURITY policy
- GitHub Issue templates (bug report, feature request)
- GitHub Pull Request template
- Dependabot configuration for dependency updates
- CODEOWNERS file for automated review requests
- `.editorconfig` for consistent editor formatting
- `.golangci.yml` for Go linting configuration
- Release workflow with tag-triggered builds and container image publishing

## [v0.0.7] - 2025-01-15

### Added
- Helm chart for Kubernetes deployment (v0.0.7)
- Multi-arch container images (arm64/amd64) with distroless base
- Health check server on port 8001 for non-local environments
- Graceful shutdown with in-flight message handling
- Prometheus metrics endpoint
- Web UI for queue management (local environment)

### Changed
- Improved request handler pipeline with middleware support
- Enhanced configuration validation

## [v0.0.6] - 2024-12-01

### Added
- FIFO queue support with message group deduplication
- Dead-letter queue (DLQ) support
- Content-based deduplication
- Visibility timeout management

### Changed
- Refactored queue store interface for pluggable backends
- Improved HMAC receipt handle signing

## [v0.0.5] - 2024-11-01

### Added
- JSON Protocol 1.0 support alongside Query Protocol
- Queue attribute management (get/set attributes)
- Message purge operation
- Rate limiting with `golang.org/x/time`

### Changed
- Migrated to custom `opensqs_go_*` Bazel rules
- Enhanced structured logging throughout

## [v0.0.4] - 2024-10-01

### Added
- SQS-compatible API with Query Protocol (XML, form-urlencoded)
- In-memory message store with HMAC-signed receipt handles
- Queue manager with lifecycle management (create, delete, list)
- Standard queue operations: send, receive, delete, change visibility
- Configurable message size, queue depth, and rate limits
- Configuration loading from YAML with schema validation
- Environment enum (LOCAL, STAGING, PROD)
- Structured logging package

## [v0.0.1] - 2024-09-01

### Added
- Initial project structure with Bazel build system
- RFC-001: OpenSQS Server design specification
- Core shared packages: config, environment, logger
