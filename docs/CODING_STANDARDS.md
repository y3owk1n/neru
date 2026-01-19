# Neru Coding Standards

This document defines the coding standards and conventions for the Neru project. Following these standards ensures the codebase appears written by a single developer and maintains consistency across all files.

## Quick Reference

- [Go Conventions](./go/CONVENTIONS.md) - Go code style, imports, naming, error handling
- [Objective-C Guidelines](./go/OBJECTIVE_C.md) - .h/.m files, naming, memory management
- [Testing Patterns](./testing/TESTING_PATTERNS.md) - Test file naming, unit vs integration, table-driven tests

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
└── scripts/               # Build and utility scripts
```

### Naming Conventions

- **Directories**: lowercase, underscore-separated
- **Files**: lowercase, underscore-separated
- **Test files**: `*_test.go`, `*_bench_test.go`, `*_integration_test.go`

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
