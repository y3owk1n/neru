# Development Guide

Contributing to Neru: build instructions, architecture overview, and contribution guidelines.

---

## Table of Contents

- [Quick Start](#quick-start)
- [Development Setup](#development-setup)
- [Building & Running](#building--running)
- [Testing](#testing)
- [Architecture Overview](#architecture-overview)
  - [Project Structure](#project-structure)
  - [Core Concepts](#core-concepts)
  - [Architectural Layers](#architectural-layers)
  - [Data Flow](#data-flow)
- [Contributing](#contributing)
  - [Development Workflow](#development-workflow)
  - [Code Standards](#code-standards)
  - [Testing Guidelines](#testing-guidelines)
  - [Documentation](#documentation)
- [Release Process](#release-process)
- [Development Tips](#development-tips)
- [Troubleshooting](#troubleshooting)
- [Resources](#resources)

---

## Quick Start

Get Neru running locally in 5 minutes:

```bash
# 1. Clone and setup
git clone https://github.com/y3owk1n/neru.git
cd neru

# 2. Install dependencies
brew install just golangci-lint

# 3. Build and run
just build
./bin/neru launch

# 4. Test it works
neru hints  # Should show hint overlays
```

**Need help?** See [Installation Guide](INSTALLATION.md) for detailed setup instructions.

---

## Development Setup

### Prerequisites

- **Go 1.25+** - [Install Go](https://golang.org/dl/)
- **Xcode Command Line Tools** - `xcode-select --install`
- **Just** - Command runner ([Install](https://github.com/casey/just))

    ```bash
    brew install just
    ```

- **golangci-lint** - Linter ([Install](https://golangci-lint.run/usage/install/))

    ```bash
    brew install golangci-lint
    ```

### Clone Repository

```bash
git clone https://github.com/y3owk1n/neru.git
cd neru
```

### Verify Setup

```bash
# Check Go version
go version  # Should be 1.25+

# Check tools
just --version
golangci-lint --version

# List available commands
just --list
```

### Development Environment

For the best development experience, we recommend:

1. **IDE Setup**: Use VS Code with Go extension or GoLand
2. **EditorConfig**: Install EditorConfig plugin for consistent formatting
3. **Pre-commit Hooks**: Set up git hooks to automate formatting and linting

```bash
# Install pre-commit hooks
cp scripts/pre-commit .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

### Common Development Tasks

| Task   | Command                 | Description                        |
| ------ | ----------------------- | ---------------------------------- |
| Build  | `just build`            | Compile the application            |
| Test   | `just test`             | Run unit tests                     |
| Test   | `just test-integration` | Run integration tests              |
| Test   | `just test-all`         | Run all tests (unit + integration) |
| Lint   | `just lint`             | Run linters                        |
| Format | `just fmt`              | Format code                        |
| Run    | `just run`              | Build and run the application      |
| Clean  | `just clean`            | Remove build artifacts             |

### Debugging

To debug Neru during development:

1. **Enable Debug Logging**:

    ```toml
    [logging]
    log_level = "debug"
    ```

2. **View Logs**:

    ```bash
    tail -f ~/Library/Logs/neru/app.log
    ```

3. **Use Delve Debugger**:

    ```bash
    dlv debug ./cmd/neru
    ```

### Profiling

Enable Go's pprof HTTP server to profile the running application:

```bash
# Start Neru with pprof server on port 6060
NERU_PPROF=:6060 ./bin/neru launch

# Access profiles in browser
open http://localhost:6060/debug/pprof/

# Or use command line tools
go tool pprof http://localhost:6060/debug/pprof/heap
```

---

## Building

Neru uses [Just](https://github.com/casey/just) as a command runner (alternative to Make).

### Quick Start

```bash
# Development build
just build

# Run
./bin/neru launch
```

### Build Commands

```bash
# Development build (auto-detects version from git tags)
just build

# Release build (optimized, stripped)
just release

# Build app bundle (macOS .app)
just bundle

# Build with custom version
just build-version v1.0.0

# Clean build artifacts
just clean
```

### Manual Build

Without Just:

```bash
# Basic build
go build -o bin/neru ./cmd/neru

# With version info
VERSION=$(git describe --tags --always --dirty)
go build \
  -ldflags="-s -w -X github.com/y3owk1n/neru/internal/cli.Version=$VERSION" \
  -o bin/neru \
  ./cmd/neru

# For release (optimized)
go build \
  -ldflags="-s -w -X github.com/y3owk1n/neru/internal/cli.Version=$VERSION" \
  -trimpath \
  -o bin/neru \
  ./cmd/neru
```

### Build Flags Explained

- `-ldflags="-s -w"` - Strip debug info and symbol table (smaller binary)
- `-trimpath` - Remove file system paths from binary
- `-X pkg.Var=value` - Set string variable at build time (version injection)

---

## Testing

Neru has a comprehensive test suite with clear separation between unit tests and integration tests. For detailed testing guidelines and standards, see [CODING_STANDARDS.md](CODING_STANDARDS.md#testing-standards).

### Test Organization

| Test Type             | Purpose                   | Command                 | Coverage                                           |
| --------------------- | ------------------------- | ----------------------- | -------------------------------------------------- |
| **Unit Tests**        | Business logic with mocks | `just test`             | 50+ tests covering algorithms, isolated components |
| **Integration Tests** | Real system interactions  | `just test-integration` | 9 tests covering macOS APIs, IPC, file operations  |

### Run Tests

```bash
# Unit tests only (fast, CI)
just test

# Integration tests only (comprehensive, local)
just test-integration

# All tests (unit + integration)
just test && just test-integration

# With race detection
just test-race

# Coverage report
just test-coverage
```

### Test Coverage Areas

#### Unit Test Coverage

- **Domain Logic**: Hint generation, grid calculations, element filtering
- **Service Logic**: Action processing, mode transitions, configuration validation
- **Adapter Interfaces**: Port implementations with mocked dependencies
- **Configuration**: TOML parsing, validation, defaults
- **CLI Logic**: Command parsing, argument validation

#### Integration Test Coverage

- **macOS Accessibility API**: Real UI element access, cursor control, mouse actions
- **macOS Event Tap API**: Real global keyboard event interception
- **macOS Hotkey API**: Real global hotkey registration/unregistration
- **Unix Socket IPC**: Real inter-process communication
- **macOS Overlay API**: Real window/overlay management
- **File System Operations**: Real config file loading/reloading
- **Component Coordination**: Real service-to-adapter interactions

### Run Linter

```bash
# Run all linters
just lint

# Auto-fix issues
golangci-lint run --fix
```

### Test During Development

```bash
# Watch mode (requires entr or similar)
find . -name "*.go" | entr -r just test

# Quick iteration with unit tests
just build && just test

# Full validation before commit
just test && just lint && just build

# Test specific package
go test ./internal/core/domain/hint/

# Test with verbose output
go test -v ./internal/app/services/

# Integration test specific component
go test -tags=integration ./internal/core/infra/accessibility/
```

---

## Architecture Overview

### What Neru Does

Neru is a keyboard-driven navigation tool for macOS that enhances productivity by allowing users to quickly navigate and interact with UI elements using keyboard shortcuts.

**Two Navigation Modes:**

- **Hints Mode**: Uses macOS Accessibility APIs to identify clickable elements and overlay hint labels
- **Grid Mode**: Divides the screen into a grid system for coordinate-based navigation

Both modes support various actions (click, scroll, etc.) and can be configured extensively through TOML configuration files.

### Key Technologies

- **Go** - Core application logic, CLI, configuration
- **CGo + Objective-C** - macOS Accessibility API integration
- **Cobra** - CLI framework
- **TOML** - Configuration format
- **Unix Sockets** - IPC communication

### Architectural Layers

Neru follows clean architecture with clear separation of concerns:

#### Domain Layer (`internal/core/domain`)

Pure business logic with no external dependencies:

- **Entities**: Core concepts (Hint, Grid, Element, Action)
- **Value Objects**: Immutable data structures
- **Business Rules**: Domain logic and validation

#### Ports Layer (`internal/core/ports`)

Interfaces defining contracts between layers:

- **AccessibilityPort**: UI element access and interaction
- **OverlayPort**: UI overlay management
- **ConfigPort**: Configuration management
- **InfrastructurePort**: System-level operations

#### Application Layer (`internal/app`)

Implements use cases and orchestrates domain entities:

- **Services**: Business logic orchestration (HintService, GridService, ActionService)
- **Components**: UI components for Hints, Grid, and Scroll modes
- **Modes**: Navigation mode logic and state management
- **Lifecycle**: Application startup, shutdown, and orchestration

#### Infrastructure Layer (`internal/core/infra`)

Concrete implementations of ports:

- **Accessibility**: macOS Accessibility API integration
- **Overlay**: UI overlay management and rendering
- **Config**: Configuration loading and parsing
- **EventTap**: Global input monitoring
- **Hotkeys**: System hotkey registration
- **IPC**: Inter-process communication
- **Bridge**: Objective-C UI components

#### Presentation Layer (`internal/ui`)

User interface rendering:

- **UI**: Overlay rendering and coordinate conversion

### Data Flow

1. **Startup**: Configuration is loaded → Dependencies are wired → Hotkeys registered → App waits for input
2. **User Interaction**: Hotkey pressed → Event tap captures → Mode activated → UI overlays displayed
3. **Processing**: User input processed → Actions determined → System APIs called → Results rendered
4. **Cleanup**: Mode exited → Overlays hidden → State reset → App returns to idle

### Core Packages

#### `internal/core/domain`

Core business logic and entities (pure Go, no external dependencies):

- **Element**: UI element representation with bounds, role, and state
- **Hint/Grid/Action**: Navigation and interaction primitives

#### `internal/core/ports`

Interface contracts between layers:

- **AccessibilityPort**: UI element access and interaction
- **OverlayPort**: UI overlay management
- **ConfigPort**: Configuration management
- **InfrastructurePort**: System-level operations

#### `internal/app`

Application orchestration and use cases:

- **Services**: Business logic orchestration (HintService, GridService, ActionService)
- **Components**: UI components for Hints, Grid, and Scroll modes
- **Modes**: Navigation mode logic and state management
- **App**: Central application state and dependencies
- **Lifecycle**: Startup, shutdown, and orchestration

#### `internal/core/infra`

Infrastructure implementations:

- **Accessibility**: macOS Accessibility API integration
- **Overlay**: UI overlay management and rendering
- **Config**: Configuration loading and parsing
- **EventTap**: Global input monitoring
- **Hotkeys**: System hotkey registration
- **IPC**: Inter-process communication
- **Bridge**: Objective-C UI components

#### `internal/ui`

Presentation layer:

- **UI**: Overlay rendering and coordinate conversion

#### `internal/cli`

Command-line interface (Cobra-based):

- Command parsing and dispatch
- Output formatting and error handling

#### `internal/config`

Configuration management:

- TOML parsing and validation
- Multi-location config loading
- Default value provision

### Where to Add New Code

**Configuration Options:**

1. Add fields to `internal/config/config.go` structs
2. Update `DefaultConfig()` with sensible defaults
3. Add validation in `Validate*()` methods
4. Update `configs/` examples and `docs/CONFIGURATION.md`

**Navigation Modes:**

1. Define domain entities in `internal/core/domain/`
2. Create service in `internal/app/services/`
3. Implement infrastructure in `internal/core/infra/`
4. Add components in `internal/app/components/`
5. Register mode in `internal/app/modes/`

**Actions:**

1. Define action in `internal/core/domain/action/`
2. Implement logic in `internal/app/services/action_service.go`
3. Add handling in `internal/app/modes/actions.go`
4. Update config and documentation

**UI Components:**

1. Create components in `internal/app/components/`
2. Implement rendering in `internal/ui/`
3. Add Objective-C in `internal/core/infra/bridge/` if needed
4. Register in `internal/app/app.go`

**CLI Commands:**

1. Create command file in `internal/cli/`
2. Register in `internal/cli/root.go`
3. Document in `docs/CLI.md`

### Dependency Injection and Wiring

Neru uses manual dependency injection for better testability and explicit dependency management:

1. **Construction**: Dependencies are explicitly passed to constructors
2. **Wiring**: `internal/app/app.go` wires all components together
3. **Testing**: Dependencies can be mocked by passing test doubles in `NewWithDeps`
4. **Ports**: Interfaces define contracts between layers

Example of dependency injection in action:

```go
// In internal/app/app.go
hintService := services.NewHintService(accAdapter, overlayAdapter, hintGen, cfg.Hints, logger)
gridService := services.NewGridService(overlayAdapter, logger)
actionService := services.NewActionService(accAdapter, overlayAdapter, cfg.Action, logger)
```

---

## Contributing

### Development Workflow

1. **Fork and clone** the repository
2. **Create a feature branch**: `git checkout -b feature/amazing-feature`
3. **Make changes** following [Coding Standards](CODING_STANDARDS.md)
4. **Add tests** for new functionality
5. **Test thoroughly**: `just test && just lint && just build`
6. **Commit conventionally**: `git commit -m "feat: description"`
7. **Push and open PR** with description and screenshots

### Before You Start

- **Read the Architecture**: Understand layered design and code placement
- **Check Existing Issues**: Search for similar work or start discussions
- **Follow Standards**: See [CODING_STANDARDS.md](CODING_STANDARDS.md)
- **Write Tests**: All new code needs appropriate test coverage
- **Update Docs**: Keep documentation current with changes

### Code Standards

**All code must follow the [Coding Standards](CODING_STANDARDS.md) document.** See [Testing Standards](CODING_STANDARDS.md#testing-standards) for test requirements.

**Pre-commit Checklist:**

- [ ] Code formatted (`just fmt`)
- [ ] Linters pass (`just lint`)
- [ ] Tests pass (`just test`)
- [ ] Build succeeds (`just build`)
- [ ] Documentation updated
- [ ] Follows [CODING_STANDARDS.md](CODING_STANDARDS.md)

**Key Requirements:**

- Use `goimports` for import organization
- Add godoc comments for exported symbols
- Use custom error package with proper wrapping
- Follow established naming patterns and receiver conventions

### Testing Guidelines

**All new code requires appropriate tests.** See [CODING_STANDARDS.md](CODING_STANDARDS.md#testing-standards) for detailed guidelines.

**Test Types:**

- **Unit Tests**: Business logic, algorithms, validation (fast, no system deps)
- **Integration Tests**: Real macOS APIs, file system, IPC (tagged `//go:build integration`)

**When to Use:**

- Unit: Business logic, config validation, component interfaces
- Integration: macOS APIs, file operations, IPC, component coordination

**Test Organization:**

```
package_test.go              # Unit tests
package_integration_test.go # Integration tests (tagged)
```

### Documentation

- **Update docs/** for significant changes
- **Add godoc comments** for exported symbols
- **Keep docs consistent** with code changes
- **Include examples** where helpful

### Commit Messages

Use clear, descriptive commit messages following conventional commits:

**Format:** `<type>(<scope>): <subject>`

**Types:**

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `test`: Adding or updating tests
- `chore`: Build process, dependencies, etc.

**Good:**

```
feat: add grid-based navigation mode

Implement grid-based navigation as an alternative to hint-based navigation.
Grid mode divides the screen into cells and allows precise cursor positioning.

Closes #123
```

**Bad:**

```
fix bug
```

---

## Release Process

Releases are being handled via [Release Please](https://github.com/googleapis/release-please) automatically.

### Version Numbering

Neru uses semantic versioning: `vMAJOR.MINOR.PATCH`

- **MAJOR** - Breaking changes
- **MINOR** - New features (backward compatible)
- **PATCH** - Bug fixes

### Creating a Release

Creating a release is just as easy as merging the release please PR, and it will build and publish the binaries on github.

> [!NOTE]
> Homebrew version bump is in a separate repo, it will be updated separately.

---

## Development Tips

### Quick Iteration

```bash
# Build and run
just build && ./bin/neru launch

# Watch for changes (requires entr)
ls **/*.go | entr -r sh -c 'just build && ./bin/neru launch'
```

### Debugging

```bash
# Enable debug logging in config
[logging]
log_level = "debug"

# Watch logs
tail -f ~/Library/Logs/neru/app.log
```

### Profiling

Enable pprof for performance analysis:

```bash
# Start with profiling
NERU_PPROF=:6060 ./bin/neru launch

# Access profiles
open http://localhost:6060/debug/pprof/

# CPU profile
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
```

### Useful Commands

```bash
# Code quality
just fmt          # Format code
just lint         # Run linters
just test         # Run tests

# Dependencies
go mod tidy       # Clean up modules
go get -u ./...   # Update dependencies
```

---

## Troubleshooting

- **Read existing code** - Well-structured codebase
- **Check issues** - Search for similar problems
- **Ask in discussions** - Open GitHub discussion for questions
- **Open draft PR** - Get early feedback on approach

## Resources

- **Go Documentation:** <https://golang.org/doc/>
- **macOS Accessibility:** <https://developer.apple.com/documentation/applicationservices/ax_ui_element_ref>
- **TOML Spec:** <https://toml.io/>
- **Cobra CLI:** <https://github.com/spf13/cobra>
- **Just Command Runner:** <https://github.com/casey/just>
