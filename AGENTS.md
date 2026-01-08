# AGENTS.md - Neru Development Guide

This document provides guidelines for AI agents working on the Neru codebase.

## Project Overview

Neru is a keyboard-driven navigation tool for macOS built with Go and Objective-C:
- **Go 1.25+** for application logic
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
- Use alias: `derrors "github.com/y3owk1n/neru/internal/core/errors"`

### Naming Conventions

- **Packages**: lowercase, short, descriptive
- **Variables**: camelCase local, PascalCase exported
- **Constants**: PascalCase exported, camelCase unexported
- **Receiver names**: consistent single-letter (e.g., `a` for `App`, `c` for `Config`)

### Error Handling

- Use `derrors` package for structured errors with error codes
- Wrap errors: `derrors.Wrap(err, code, "message")`
- Create new errors: `derrors.New(code, "message")`
- Log errors with `zap.Error()` before returning
- Error messages: lowercase, no trailing punctuation

### Context Usage

- `context.Context` first parameter (except test functions)
- Use `context.Background()` for top-level initialization
- Pass context through call chains for cancellation

### Logging

- Use `go.uber.org/zap` for structured logging
- Log levels: `Debug`, `Info`, `Warn`, `Error`

### Functions

- Keep functions focused on single responsibility
- Return errors as last value
- Use named return values sparingly

### Comments

- Use `//` for comments (not `/* */`)
- Comment public APIs and exported symbols
- Prefix receiver method comments with receiver name

### Testing

- Use table-driven tests where appropriate
- Name test files: `*_test.go`
- Unit tests: no build tag
- Integration tests: `//go:build integration`

### Objective-C Code

- Follow `.clang-format` configuration
- Header files in `.h`, implementation in `.m`

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
