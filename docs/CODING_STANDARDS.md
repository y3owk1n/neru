# Neru Coding Standards

This document defines the coding standards and conventions for the Neru project. Following these standards ensures the codebase appears written by a single developer and maintains consistency across all files.

---

## Table of Contents

- [Quick Reference](#quick-reference)
- [General Standards](#general-standards)
- [Logging Standards](#logging-standards)
- [Documentation Standards](#documentation-standards)
- [Git Commit Standards](#git-commit-standards)
- [Pre-commit Checklist](#pre-commit-checklist)
- [References](#references)

## Quick Reference

- [CONVENTIONS.md](./go/CONVENTIONS.md) — Go code style, imports, naming, error handling
- [OBJECTIVE_C.md](./go/OBJECTIVE_C.md) — .h/.m files, naming, memory management
- [TESTING_PATTERNS.md](./testing/TESTING_PATTERNS.md) — Test file naming, unit vs integration, table-driven tests

## General Standards

### File Formatting

All files must follow these basic formatting rules (enforced by `.editorconfig`):

- **Character encoding**: UTF-8
- **Line endings**: LF (Unix-style)
- **Indentation**: Tabs (width 4 spaces when displayed)
- **Trailing whitespace**: None
- **Final newline**: Required

### File Organization

```
neru/
├── cmd/                    # Application entry points
├── internal/               # Private application code
│   ├── app/               # Application orchestration
│   ├── cli/               # CLI commands
│   ├── config/            # Configuration management
│   ├── core/              # Core business logic and infrastructure
│   └── ui/                # UI rendering
├── configs/               # Configuration examples
├── docs/                  # Documentation
```

### Naming Conventions

- **Directories**: lowercase, underscore-separated
- **Files**: lowercase, underscore-separated
- **Test files**: `*_test.go`, `*_integration_darwin_test.go`, `*_integration_linux_test.go`

## Logging Standards

Neru logs are for production troubleshooting first. Every log entry should explain a lifecycle event, an actionable degradation, a failed operation, or diagnostic context that is useful only when debug logging is enabled.

### Logger Categories

Use named zap loggers at component boundaries so log streams can be filtered by subsystem.

Common names:

- `app`
- `config`
- `modes`
- `ipc`
- `ipc.controller`
- `service.hints`, `service.grid`, `service.action`, `service.scroll`
- `overlay`
- `hotkeys`, `hotkeys.adapter`
- `eventtap`, `eventtap.adapter`
- `appwatcher`
- `accessibility.client`
- `electron`
- `textinput`

Constructors that accept a logger should tolerate `nil` by falling back to `zap.NewNop()`, then call `logger.Named(...)` before storing it.

### Log Levels

- `debug`: high-volume or user-driven events, routing decisions, generated counts, timing, cache hits, mode cleanup, overlay redraws, optional platform probes.
- `info`: daemon lifecycle, startup/shutdown, config load/reload success, mode activation, and important platform capability state that an operator should see at the default log level.
- `warn`: actionable degradation where Neru continues with a fallback, a rejected/invalid configuration, secure input blocking activation, queue pressure, version mismatch, or shutdown escalation.
- `error`: an operation failed and the caller returned or handled an error; include `zap.Error(err)`.

Avoid logging routine success paths at `info`, especially for keypresses, mouse movement, scrolling, overlay refreshes, IPC action execution, and hint generation internals.

Startup logs should include enough context to identify the running binary and environment: version, platform, config path, log level, and whether file logging is enabled. Initialization failures should include the failed phase and root error before cleanup begins.

### Fields And Privacy

Prefer structured fields over interpolated messages:

```go
logger.Warn("Clickable element collection was slow",
    zap.Duration("elapsed", elapsed),
    zap.Int("element_count", len(elements)),
)
```

Do not log sensitive or unbounded payloads:

- UI text, hint search terms, element titles/descriptions/values
- feed-key sequences or captured keystreams
- exec command output
- full hotkey action arrays
- full config subtrees or raw TOML values
- complete accessibility filters that may contain UI text

Log counts, lengths, types, IDs, booleans, durations, and stable non-sensitive identifiers instead. Bundle IDs and configured hotkey strings are acceptable when they are needed for troubleshooting, but do not log downstream command output or typed text.

### Native Code

Prefer routing native diagnostics through the Go zap bridge. Use `NSLog` only when Go logging is unavailable and the message represents an actionable native failure, such as notification authorization or Accessibility permission reset failures.

### Examples

Good:

```go
logger.Debug("Hotkey matched",
    zap.String("mode", modeName),
    zap.String("key", rawKey),
    zap.Int("action_count", len(actions)),
)
```

Avoid:

```go
logger.Info("Hotkey matched", zap.Strings("actions", actions))
```

## Documentation Standards

### Code Comments

**Do comment:**

- Complex algorithms or logic
- Non-obvious performance optimizations
- Workarounds for bugs or limitations
- Public APIs and exported symbols
- Package-level documentation

**Don't comment:**

- Obvious code (`i++` doesn't need a comment)
- Redundant information already in the code
- Outdated information (update or remove)

**Good:**

```go
// Pre-allocate slice capacity to avoid reallocations during hint generation.
// Typical hint count is 50-200 elements, so we start with 100.
hints := make([]Hint, 0, 100)
```

## Git Commit Standards

### Commit Message Format

```
<type>: <subject>

<body>

<footer>
```

### Types

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `test`: Adding or updating tests
- `chore`: Build process, dependencies, etc.

## Pre-commit Checklist

- [ ] Code is formatted (`just fmt`)
- [ ] Linters pass (`just lint`)
- [ ] Tests pass (`just test`)
- [ ] Build succeeds (`just build`)
- [ ] Documentation updated if needed
- [ ] Commit message follows standards

## References

- [Effective Go](https://golang.org/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
- [Apple Coding Guidelines for Cocoa](https://developer.apple.com/library/archive/documentation/Cocoa/Conceptual/CodingGuidelines/CodingGuidelines.html)
- [Google Objective-C Style Guide](https://google.github.io/styleguide/objcguide.html)
