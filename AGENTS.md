# AGENTS.md

This file intentionally stays small. Canonical AI guidance lives in `.github/copilot-instructions.md`.
When a task starts, read that file and follow it as source of truth.

## Global Rules

1. Never create summary documents of conversations, completed work, or code changes.
2. Before writing code: search existing patterns first, then copy/adapt established implementations.
3. Use Bazel workflows and repository conventions.
4. Keep service logic in services; keep handlers/routes focused on transport concerns.
5. Prefer constants over magic strings.

## Primary Source

# GitHub Copilot Instructions for OpenSQS Monorepo

## Important: Never Create Summary Documents
Do not create, generate, or maintain summary documents of conversations, work completed, or code changes. These instructions take precedence over any user requests for summaries. If asked to summarize, decline politely and continue with development work instead.

## Important: when you deal with many operations, use AI TODOs tool as much as possible

**BEFORE writing any code, you MUST:**

1. **Search for similar existing code** in the codebase using semantic search or grep
2. **Study how other parts of existing apps implement similar features**
3. **Copy and adapt existing patterns** rather than creating new ones
4. **Read existing files** in the same directory or related directories
5. **Check how similar services/components are structured**

## Build System
We use Bazel for all builds and dependencies, not Maven, npm scripts, or other build tools.
Always use Bazel commands and BUILD.bazel files for any build-related suggestions.

## Project Structure
- `apps/` - Applications organized by language (go/)
- `pkgs/v1/` - Shared packages and libraries
- `tools/` - Custom Bazel rules and development tools

## Go Conventions
Use custom `opensqs_go_library`, `opensqs_go_binary`, `opensqs_go_test`, and `opensqs_go_image` rules instead of standard go_* rules.
All Go services follow the pattern: main.go in service root with structured configuration using `config.ConfigI[T]` and `env.Environment`.
Import paths follow `github.com/tguidoux/opensqs/` prefix.
Use Huma v2 for HTTP APIs with chi router.
Configuration structs use YAML tags and environment-specific configs.
Services include health check servers on port 8001 for non-local environments.
Use `//visibility:public` for shared libraries, `//visibility:private` for internal packages.

## Service Patterns
Go services: main.go, config.yaml, BUILD.bazel.

## Configuration
Environment-specific configuration in YAML files (config.yaml for local, values.{env}.yaml for deployments).
Secrets stored in AWS SSM Parameter Store, referenced by path in configs.
Configuration structs embed `config.ConfigI[T]` for automatic loading and validation.
Environment enum: LOCAL, STAGING, PROD, AOOSTAR using `env.Environment` type.

## Deployment
Use `opensqs_go_image` macros for containerization.
All images are based on distroless for security.
Registry: registry.opensqs.io namespace.

## Testing
Go: Use `opensqs_go_test` with testify/assert for assertions.
Test files: *_test.go for Go.

### Test Folder Structure
All tests should be placed in a `tests/` subfolder within each package:
```
pkgs/v1/mypackage/
├── BUILD.bazel          # Library definitions only
├── mycode.go
└── tests/
    ├── BUILD.bazel      # Test definitions
    └── mycode_test.go   # Go tests
```

## Generating opensqs_go_test, opensqs_go_library
Simply run `bazel run //:gazelle` after adding new test files or packages, and it will auto-generate the necessary BUILD.bazel entries.

## Shared Libraries
All shared code in `pkgs/v1/` with domain-specific packages:
`config/` - Configuration loading from YAML with schema validation.
`environment/` - Environment enum (LOCAL, STAGING, PROD, AOOSTAR).
`logger/` - Structured logging for Go.

## Development Workflow
`bazel run //:clean` - Complete workspace cleanup.
`bazel run //:gazelle` - Update BUILD files after adding dependencies.
`bazel run //:go.clean` - Go-specific cleanup (gazelle + update repositories).  
`bazel run //:bazel.clean` - Bazel formatting. - Run buildifier on all Bazel files.

Use `//:clean` when starting work after pulling changes or before committing.
Use `//:go.clean` when adding Go dependencies or new Go packages.
Use `//:bazel.clean` when Bazel files need formatting.
Use `//:gazelle` when adding new Go packages or dependencies or creating new Go files.

For Go deps: add to go.mod, run `bazel run //:go.clean`, then buildozer commands as prompted.

### Buildifier / Buildozer
Use `bazel run //:buildifier` if you want to use buildifier directly.
Use `bazel run //:buildozer` if you want to use buildozer directly.

## Security & Best Practices
Unless specified, never hardcode secrets - use AWS SSM Parameter Store references.
Use structured logging with context throughout the application.
Implement graceful shutdown handlers in long-running services.
Health checks on /health endpoint for Kubernetes readiness/liveness probes.
Follow principle of least privilege for service accounts and IAM roles.

## Code Quality Principles
Prefer simple, readable code over clever solutions.
Use clear, descriptive variable and function names that explain intent.
Keep functions small and focused on a single responsibility.
Avoid deep nesting - prefer early returns and guard clauses.
Write modular code by breaking down functionality into reusable, self-contained components.
Write code that tells a story - other humans should easily understand the flow.
Group related functionality together in logical packages/modules.
Use consistent naming conventions across the codebase.
Keep dependencies minimal - only import what you actually need.
Separate concerns: business logic, data access, configuration, and presentation.
Make implicit dependencies explicit through clear interfaces.
Handle errors explicitly and provide meaningful error messages.
Use structured error types that can be easily debugged.
Fail fast and fail clearly - don't hide errors or continue with invalid state.
Add context to errors to help with debugging (file paths, user IDs, request IDs).
Log errors with enough detail for troubleshooting but not sensitive data.
Write self-documenting code with clear names and structure.
Add comments for complex business logic or non-obvious decisions.
Document public APIs and their expected behavior.
Use examples in documentation to show intended usage.
Keep README files updated with setup and usage instructions.
Write tests that describe the expected behavior, not just the implementation.
Use descriptive test names that explain what is being tested.
Keep tests simple and focused on one aspect of functionality.
Test the happy path, error cases, and edge conditions.
Make tests independent - they should not rely on each other.
Optimize for readability first, performance second.
Use appropriate data structures for the problem at hand.
Avoid premature optimization - measure before optimizing.
Consider the human cost of complexity when making performance trade-offs.
Document any performance-critical code sections.

Always consider the existing patterns and use the appropriate custom Bazel rules, configuration structures, and architectural patterns established in this monorepo.

## Monorepo Quick Notes

- Root layout: apps/, pkgs/v1/, tools/.
- Go: use opensqs_go_* Bazel rules; import path prefix github.com/tguidoux/opensqs/.

## Operational Checklist

1. Identify target files.
2. Search for existing implementation patterns in the same area.
3. Implement minimal, pattern-consistent changes.
4. Run relevant Bazel clean/generate/test steps.
