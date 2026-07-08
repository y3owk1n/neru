# Contributing to Neru

Thanks for your interest in contributing! Neru is a small project with an approachable codebase. We welcome contributions of all kinds — code, docs, bug reports, config examples, or ideas.

---

## Table of Contents

- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Build Commands](#build-commands)
- [Testing](#testing)
- [Coding Standards](#coding-standards)
- [Commit Messages](#commit-messages)
- [Pull Requests](#pull-requests)
- [Good First Contributions](#good-first-contributions)
- [Reporting Bugs](#reporting-bugs)
- [Feature Requests](#feature-requests)

---

## Getting Started

1. **Search existing issues** — check if someone is already working on the same thing.
2. **Open an issue first** for non-trivial changes — align on approach before writing code.
3. **Small, focused PRs** are preferred over large, sweeping changes.

---

## Development Setup

### Prerequisites

- **Go 1.26.4+** — [Install Go](https://golang.org/dl/)
- **Xcode Command Line Tools** — `xcode-select --install`
- **Just** — `brew install just`
- **golangci-lint** — `brew install golangci-lint`

### Quick Start

```bash
git clone https://github.com/y3owk1n/neru.git
cd neru

# Option A: Devbox (recommended)
curl -fsSL https://get.jetify.com/devbox | bash
devbox shell

# Option B: Manual
brew install go just golangci-lint

# Build and test
just build
just test
just lint
```

Devbox automatically manages Go, gopls, gofumpt, golines, golangci-lint, just, and clang-tools.

---

## Making Changes

1. **Create a branch** from `main`: `git checkout -b feat/my-feature`
2. **Make changes** following the [Coding Standards](#coding-standards)
3. **Add or update tests** for new functionality
4. **Run pre-commit checklist:**

```bash
just fmt            # Format code (goimports + gofumpt)
just lint           # Run linters (golangci-lint)
just test           # Run unit tests
just build          # Verify build
```

5. **Commit** using [conventional commits](#commit-messages)
6. **Push** and open a pull request

### Where to Add New Code

| What                  | Where                                                                      |
| :-------------------- | :------------------------------------------------------------------------- |
| Configuration options | `internal/config/config.go` + `config_<os>.go`                             |
| Navigation mode       | `internal/core/domain/` → `internal/app/services/` → `internal/app/modes/` |
| Action                | `internal/core/domain/action/` → `internal/app/services/action_service.go` |
| UI component          | `internal/app/components/` + `internal/ui/` + platform-specific rendering  |
| CLI command           | `internal/cli/` + register in `internal/cli/root.go`                       |
| Platform adapter      | `internal/core/infra/platform/<os>/`                                       |

---

## Build Commands

| Task              | Command              |
| :---------------- | :------------------- |
| Development build | `just build`         |
| macOS binary      | `just build-darwin`  |
| Linux binary      | `just build-linux`   |
| Windows binary    | `just build-windows` |
| Release build     | `just release`       |
| macOS .app bundle | `just bundle`        |
| Clean artifacts   | `just clean`         |

Manual build (advanced):

```bash
go build -o bin/neru ./cmd/neru
```

With version info: `VERSION=$(git describe --tags --always --dirty) go build -ldflags="-s -w -X github.com/y3owk1n/neru/internal/cli.Version=$VERSION" -o bin/neru ./cmd/neru`

---

## Testing

### Test Types

| Type        | Command                                | Purpose                                | File Pattern                 |
| :---------- | :------------------------------------- | :------------------------------------- | :--------------------------- |
| Unit        | `just test`                            | Business logic, algorithms, validation | `*_test.go`                  |
| Integration | `just test-integration`                | Real platform APIs, file system, IPC   | `*_integration_<os>_test.go` |
| All         | `just test-all`                        | Unit + integration                     | —                            |
| Race        | `just test-race`                       | All tests with race detector           | —                            |
| Foundation  | `just test-foundation`                 | Platform-safe core tests               | —                            |
| Specific    | `go test ./internal/core/domain/hint/` | Focused testing                        | —                            |

### Guidelines

- All new code requires tests
- Use **table-driven tests** where possible
- Unit tests should be fast with no system dependencies — use mocks
- Integration tests use real platform APIs and are tagged `//go:build integration && <os>`

### Test Structure

```go
func TestService_Process(t *testing.T) {
    service := NewService(zap.NewNop(), DefaultConfig())
    result, err := service.Process(context.Background(), "test-data")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result == nil {
        t.Fatal("expected non-nil result")
    }
}
```

### Table-Driven Tests

```go
func TestValidate(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {"valid input", "valid", false},
        {"empty input", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := Validate(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Test File Naming

```
package_test.go                        # Unit tests (no build tag)
package_integration_darwin_test.go     # macOS integration (//go:build integration && darwin)
package_integration_linux_test.go      # Linux integration (//go:build integration && linux)
package_integration_windows_test.go    # Windows integration (//go:build integration && windows)
```

### Cross-Platform Testing

Mock port interfaces for unit tests:

```go
// In internal/core/ports/mocks/
type MockAccessibilityPort struct {
    // ...
}
```

---

## Coding Standards

### Quick Reference

| Topic             | Document                                         |
| :---------------- | :----------------------------------------------- |
| Go code style     | [Go Conventions](docs/go/CONVENTIONS.md)         |
| Objective-C style | [Objective-C Guidelines](docs/go/OBJECTIVE_C.md) |

### General

- **Encoding:** UTF-8, **Line endings:** LF, **Indentation:** Tabs (width 4 when displayed)
- **Trailing whitespace:** None, **Final newline:** Required

### Logging Standards

Use named zap loggers at component boundaries. Common names: `app`, `config`, `modes`, `ipc`, `service.hints`, `overlay`, `hotkeys`, `eventtap`, `accessibility.client`.

| Level   | Usage                                                                    |
| :------ | :----------------------------------------------------------------------- |
| `debug` | High-volume events, routing, counts, timing, cache hits, overlay redraws |
| `info`  | Daemon lifecycle, startup/shutdown, config load, mode activation         |
| `warn`  | Actionable degradation, rejected config, secure input blocking           |
| `error` | Operation failed; include `zap.Error(err)`                               |

**Do not log:** UI text, hint search terms, feed-key sequences, exec output, full config subtrees.
**Do log:** Counts, lengths, types, IDs, booleans, durations, bundle IDs when needed.

```go
// ✅ Good
logger.Debug("Hotkey matched",
    zap.String("mode", modeName),
    zap.String("key", rawKey),
    zap.Int("action_count", len(actions)),
)
```

### Code Comments

**Do comment:** Complex algorithms, non-obvious optimizations, workarounds, public APIs, package docs.
**Don't comment:** Obvious code (`i++`), redundant information, outdated info.

### Debugging

```toml
[logging]
log_level = "debug"
```

```bash
tail -f ~/Library/Logs/neru/app.log   # macOS
tail -f ~/.local/state/neru/log/app.log  # Linux
dlv debug ./cmd/neru                   # Delve debugger
```

### Release Process

Releases are automated via [Release Please](https://github.com/googleapis/release-please). Versioning follows semantic versioning (`vMAJOR.MINOR.PATCH`). Merge the release-please PR to `main` to trigger a release. Homebrew is updated separately in [y3owk1n/homebrew-tap](https://github.com/y3owk1n/homebrew-tap).

---

## Commit Messages

We use [Conventional Commits](https://www.conventionalcommits.org/) for automated releases.

**Format:** `<type>(<optional scope>): <subject>`

| Type       | When to Use                            |
| :--------- | :------------------------------------- |
| `feat`     | New feature                            |
| `fix`      | Bug fix                                |
| `docs`     | Documentation only                     |
| `style`    | Formatting, no logic change            |
| `refactor` | Code restructuring, no behavior change |
| `perf`     | Performance improvement                |
| `test`     | Adding or updating tests               |
| `chore`    | Build, CI, dependencies, tooling       |

**Examples:**

```
feat(grid): add recursive subdivision mode
fix(hints): correct overlay positioning on multi-monitor setups
docs: update configuration reference for scroll mode
```

---

## Pull Requests

- **Title** — conventional commit format
- **Description** — what changed and why. Include screenshots for UI changes.
- **Keep focused** — one logical change per PR
- **Link issues** — `Closes #123`
- All CI checks (lint, test, build) must pass before merge

---

## Good First Contributions

- 🐛 Bug fixes — check [open issues](https://github.com/y3owk1n/neru/issues)
- 📝 Documentation improvements
- 📦 Config examples for common setups
- 🎥 Demo videos or GIFs
- ⚡ Performance improvements
- 🧪 Additional test coverage

---

## Reporting Bugs

Open a [GitHub Issue](https://github.com/y3owk1n/neru/issues/new) with:

1. **macOS version** and `neru --version`
2. **Steps to reproduce** — minimal and specific
3. **Expected vs actual behavior**
4. **Logs** — run with `log_level = "debug"` and attach relevant lines from `~/Library/Logs/neru/app.log`
5. **Screenshots** if visual

---

## Feature Requests

Open a [GitHub Issue](https://github.com/y3owk1n/neru/issues/new) or [Discussion](https://github.com/y3owk1n/neru/discussions) describing what you'd like to see, why it would be useful, and optionally how you envision it working.

---

## Resources

- [System Architecture](docs/ARCHITECTURE.md)
- [Cross-Platform Contribution Guide](docs/CROSS_PLATFORM.md)
- [Go Conventions](docs/go/CONVENTIONS.md)
- [Objective-C Guidelines](docs/go/OBJECTIVE_C.md)
- [Go Documentation](https://golang.org/doc/)
- [Cobra CLI](https://github.com/spf13/cobra)
- [Just Command Runner](https://github.com/casey/just)

---

Thank you for helping make Neru better!
