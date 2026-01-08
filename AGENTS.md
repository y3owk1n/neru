# AGENTS.md - Neru Development Guide

This document provides guidelines for AI agents working on the Neru codebase.

## Project Overview

Neru is a keyboard-driven navigation tool for macOS built with Go and Objective-C. The project uses:
- **Go 1.25.2** for main application logic
- **Objective-C** for macOS bridge layer (systray, accessibility APIs)
- **TOML** for configuration files
- **just** for build automation
- **golangci-lint** for linting

## Build Commands

```bash
# Development build
just build

# Release build with optimizations
just release

# Build specific version
just build-version VERSION_OVERRIDE=x.y.z

# Cross-platform CI builds
just release-ci VERSION_OVERRIDE=x.y.z

# Bundle as macOS app
just bundle

# Clean build artifacts
just clean
```

## Testing Commands

```bash
# Run all tests (unit + integration)
just test

# Run unit tests only
just test-unit

# Run integration tests only
just test-integration

# Run single test file
go test -v ./internal/config/...

# Run single test function
go test -v -run TestFunctionName ./internal/config

# Run tests with race detection
just test-race

# Run tests with coverage
just test-coverage

# View coverage as HTML
just test-coverage-html

# Run benchmarks
just bench
```

## Linting & Formatting

```bash
# Format code (Go + Objective-C)
just fmt

# Check formatting only
just fmt-check

# Lint code
just lint

# Vet code
just vet

# Download and tidy dependencies
just deps

# Verify dependencies
just verify
```

## Code Style Guidelines

### Imports

- Use `gci` or `goimports` for import formatting
- Group imports: stdlib, then third-party, then internal
- Use alias for error package: `derrors "github.com/y3owk1n/neru/internal/core/errors"`
- Example:
  ```go
  import (
      "context"
      "fmt"

      "github.com/spf13/cobra"
      "go.uber.org/zap"

      "github.com/y3owk1n/neru/internal/cli"
  )
  ```

### Naming Conventions

- **Packages**: lowercase, short, descriptive
- **Variables**: camelCase for local, PascalCase for exported
- **Constants**: PascalCase for exported, camelCase for unexported
- **Error variables**: name as `err` or descriptive names ending in `Err` (e.g., `parseErr`, `validationErr`)
- **Receiver names**: use consistent single-letter names (e.g., `a` for `App`, `c` for `Config`)
- **Interfaces**: name after what they do (e.g., `Lifecycle`, `Service`)

### Error Handling

- Use `derrors` package for structured errors with error codes
- Wrap errors with `derrors.Wrap(err, code, "message")`
- Create new errors with `derrors.New(code, "message")`
- Error codes defined in `internal/core/errors`
- Log errors with `zap.Error()` before returning
- Error messages should be lowercase and not end with punctuation

### Context Usage

- `context.Context` should be the first parameter (except for test functions)
- Use `context.Background()` for top-level initialization
- Pass context through call chains for cancellation
- Check for context cancellation in long-running operations

### Logging

- Use `go.uber.org/zap` for structured logging
- Use `zap.NamedError()` for wrapping errors
- Log at appropriate levels: `Debug`, `Info`, `Warn`, `Error`
- Include relevant fields using `zap.String()`, `zap.Int()`, etc.

### Structs and Types

- Use struct tags for serialization: `json:"fieldName" toml:"field_name"`
- Prefer small, focused structs
- Use embedding for composition where appropriate
- Document exported fields and types

### Functions

- Keep functions focused on a single responsibility
- Max ~120 statements per function (enforced by golangci-lint)
- Return errors as last value
- Use named return values when helpful for readability

### Comments

- Use `//` for comments (not `/* */`)
- Comment public APIs, exported types, and functions
- Use sentence case for comment text
- Prefix comments on receiver methods with receiver name: `// ActivateMode activates...`

### Testing

- Use table-driven tests where appropriate
- Name test files: `*_test.go`
- Use `-tags=integration` for integration tests
- Use `-tags=unit` for unit tests (in coverage commands)
- Mock external dependencies in `mocks_test.go`

### Objective-C Code (internal/core/infra/bridge/)

- Follow `.clang-format` configuration
- Use `--style=file` for formatting
- Header files in `.h`, implementation in `.m`

### Configuration Files

- TOML format for user configuration
- Follow patterns in `internal/config/config.go`
- Use struct tags for field mapping
- Validate configuration in `Validate()` methods

## Key Files

| Path | Purpose |
|------|---------|
| `cmd/neru/main.go` | Application entry point |
| `internal/app/app.go` | Core application logic |
| `internal/cli/` | CLI commands and IPC |
| `internal/config/` | Configuration loading/validation |
| `internal/core/infra/bridge/` | Objective-C macOS bridge |
| `justfile` | Build automation |
| `.golangci.yml` | Linter configuration |
| `.clang-format` | Objective-C formatting |
