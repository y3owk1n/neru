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

# 2. Set up development environment

## Option A: Using Devbox (Recommended)
devbox shell

See [Development Environment Options](#development-environment-options) for install details

## Option B: Manual installation
brew install just golangci-lint

See [Development Environment Options](#development-environment-options) for install details

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

### Development Environment Options

For the best development experience, choose one of the following setup methods:

#### Option A: Devbox (Recommended)

[Devbox](https://www.jetify.com/devbox) provides an isolated development environment with all required tools pre-configured.

```bash
# Install Devbox
curl -fsSL https://get.jetify.com/devbox | bash

# Option 1: Enter the development shell manually
devbox shell

# Option 2: Use direnv for automatic shell activation (recommended)
# Install direnv: brew install direnv
# Add to your shell: eval "$(direnv hook bash)" (or zsh/fish)
# The .envrc file will automatically activate devbox when you cd into the project
```

Devbox automatically installs and manages:

- Go 1.25.5
- gopls (Go language server)
- gotools, gofumpt, golines (Go formatting tools)
- golangci-lint (linter)
- just (command runner)
- clang-tools (C/C++ tools for CGo)

#### Option B: Manual Installation

Install essential tools manually using Homebrew. Note that Devbox provides additional development tools (gopls, gofumpt, golines, etc.) that can be installed separately if desired.

```bash
brew install go just golangci-lint llvm
```

**Tool descriptions:**

- `go` - Go compiler and toolchain (1.25+ required)
- `just` - Command runner for build scripts
- `golangci-lint` - Go linter and formatter
- `llvm` - LLVM tools including clang-format for C/C++/Objective-C formatting (required for CGo code)

**Optional additional tools** (install via `go install` if desired):

- `gopls`: `go install golang.org/x/tools/gopls@latest`
- `gofumpt`: `go install mvdan.cc/gofumpt@latest`
- `golines`: `go install github.com/segmentio/golines@latest`

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

| Task   | Command                  | Description                        |
| ------ | ------------------------ | ---------------------------------- |
| Build  | `just build`             | Compile the application            |
| Test   | `just test`              | Run unit and integration tests     |
| Test   | `just test-unit`         | Run unit tests                     |
| Test   | `just test-integration`  | Run integration tests              |
| Test   | `just test-race`         | Run all tests with race detection  |
| Test   | `just test-coverage`     | Run unit tests with coverage       |
| Test   | `just test-all`          | Run all tests (unit + integration) |
| Bench  | `just bench`             | Run all benchmarks                 |
| Bench  | `just bench-unit`        | Run unit benchmarks                |
| Bench  | `just bench-integration` | Run integration benchmarks         |
| Lint   | `just lint`              | Run linters                        |
| Format | `just fmt`               | Format code                        |
| Run    | `just run`               | Build and run the application      |
| Clean  | `just clean`             | Remove build artifacts             |

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

| Test Type                  | File Pattern                  | Purpose                                                    | Command                  | Coverage                                            |
| -------------------------- | ----------------------------- | ---------------------------------------------------------- | ------------------------ | --------------------------------------------------- |
| **Unit Tests**             | `*_test.go`                   | Business logic with mocks (no build tag required)          | `just test`              | 50+ tests covering algorithms, isolated components  |
| **Integration Tests**      | `*_integration_test.go`       | Real system interactions (tagged `//go:build integration`) | `just test-integration`  | 15+ tests covering macOS APIs, IPC, file operations |
| **Unit Benchmarks**        | `*_bench_test.go`             | Performance testing                                        | `just bench`             | Performance benchmarks for critical paths           |
| **Integration Benchmarks** | `*_bench_integration_test.go` | Real system performance                                    | `just bench-integration` | Performance testing with real macOS APIs            |

### Test File Naming Convention

```text
package_test.go                    # Unit tests (logic, mocks)
package_integration_test.go       # Integration tests (real system calls) //go:build integration
package_bench_test.go             # Unit benchmarks (algorithms without system calls)
package_bench_integration_test.go # Integration benchmarks (real system performance) //go:build integration
```

### Run Tests

```bash
# Unit tests only (fast, CI)
just test

# Integration tests only (comprehensive, local)
just test-integration

# All tests (unit + integration)
just test-all

# Benchmarks
just bench

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
- **Pure Logic Benchmarks**: Performance testing of algorithms without system calls

#### Integration Test Coverage

- **macOS Accessibility API**: Real UI element access, cursor control, mouse actions
- **macOS Event Tap API**: Real global keyboard event interception
- **macOS Hotkey API**: Real global hotkey registration/unregistration
- **Unix Socket IPC**: Real inter-process communication
- **macOS Overlay API**: Real window/overlay management
- **File System Operations**: Real config file loading/reloading
- **Component Coordination**: Real service-to-adapter interactions
- **System Benchmarks**: Performance testing with real macOS APIs

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

**Four Navigation Modes:**

- **Hints Mode**: Uses macOS Accessibility APIs to identify clickable elements and overlay hint labels
- **Grid Mode**: Divides the screen into a grid system for coordinate-based navigation
- **Scroll Mode**: Provides Vim-style scrolling at the cursor position
- **Quad-Grid Mode**: Recursive quadrant navigation with center preview and reset/backtrack support

All modes support various actions and can be configured extensively through TOML configuration files.

### Mode Interface Contract

Neru uses a standardized `Mode` interface to ensure consistent behavior across all navigation modes. This interface defines the contract that all mode implementations must follow.

#### Interface Definition

```go
type Mode interface {
    // Activate initializes and starts the mode with optional action parameters
    Activate(action *string)

    // HandleKey processes keyboard input during normal mode operation
    HandleKey(key string)

    // HandleActionKey processes keyboard input when in action sub-mode
    HandleActionKey(key string)

    // Exit performs cleanup and deactivates the mode
    Exit()

    // ToggleActionMode switches between normal mode and action sub-mode
    ToggleActionMode()

    // ModeType returns the domain mode type identifier
    ModeType() domain.Mode
}
```

#### Implementation Pattern

All mode implementations follow this pattern:

1. **Struct Definition**: Create a struct that holds a reference to the Handler
2. **Constructor**: Provide a `NewXXXMode(handler *Handler)` constructor
3. **Interface Methods**: Implement all required interface methods
4. **Registration**: Register the mode in `Handler.NewHandler()`

#### Method Contracts

##### `Activate(action *string)`

- **Purpose**: Initialize the mode and set it as the active mode
- **Parameters**: Optional action string for pending actions
- **Responsibilities**:
  - Call `handler.SetModeXXX()` to change app state
  - Show mode-specific overlays/UI
  - Initialize mode-specific state
  - Log mode activation

##### `HandleKey(key string)`

- **Purpose**: Process keyboard input during normal mode operation
- **Parameters**: Single key string (e.g., "a", "j", "escape")
- **Responsibilities**:
  - Route keys to appropriate handlers
  - Update mode state based on input
  - Handle mode-specific navigation logic

##### `HandleActionKey(key string)`

- **Purpose**: Process keyboard input when in action sub-mode
- **Parameters**: Single key string representing action selection
- **Responsibilities**:
  - Delegate to `handler.handleActionKey()` for action execution
  - Handle action-specific key mappings

##### `Exit()`

- **Purpose**: Clean up mode state and return to idle
- **Responsibilities**:
  - Hide overlays and UI elements
  - Reset mode-specific state
  - Call `handler.SetModeIdle()` if needed

##### `ToggleActionMode()`

- **Purpose**: Switch between normal mode and action sub-mode
- **Responsibilities**:
  - Delegate to handler's toggle method (e.g., `toggleActionModeForHints()`)
  - Handle mode-specific action mode transitions

##### `ModeType()`

- **Purpose**: Return the domain mode identifier
- **Returns**: `domain.Mode` enum value (e.g., `domain.ModeHints`)

#### Implementation Examples

##### Basic Mode Structure

```go
type ExampleMode struct {
    handler *Handler
}

func NewExampleMode(handler *Handler) *ExampleMode {
    return &ExampleMode{handler: handler}
}

func (m *ExampleMode) ModeType() domain.Mode {
    return domain.ModeExample
}

func (m *ExampleMode) Activate(action *string) {
    m.handler.SetModeExample()
    // Show example overlay
    // Initialize state
    m.handler.logger.Info("Example mode activated")
}

func (m *ExampleMode) HandleKey(key string) {
    switch key {
    case "escape":
        m.handler.SetModeIdle()
    // Handle other keys...
    }
}

func (m *ExampleMode) HandleActionKey(key string) {
    m.handler.handleActionKey(key, "Example")
}

func (m *ExampleMode) Exit() {
    // Hide overlays
    // Reset state
}

func (m *ExampleMode) ToggleActionMode() {
    m.handler.toggleActionModeForExample()
}
```

##### Registration Pattern

```go
func NewHandler(...) *Handler {
    handler := &Handler{...}
    handler.modes = map[domain.Mode]Mode{
        domain.ModeHints:  NewHintsMode(handler),
        domain.ModeGrid:   NewGridMode(handler),
        domain.ModeAction: NewActionMode(handler),
        domain.ModeScroll: NewScrollMode(handler),
        // Add new modes here
    }
    return handler
}
```

#### Best Practices

1. **Consistent Naming**: Use `XXXMode` for struct names, `NewXXXMode` for constructors
2. **Handler Reference**: Always store a reference to the Handler for accessing services
3. **State Management**: Use the Handler's state management methods
4. **Logging**: Log mode transitions and important events
5. **Error Handling**: Handle errors gracefully, don't panic
6. **Resource Cleanup**: Always clean up overlays and state in `Exit()`
7. **Action Mode Support**: Implement `ToggleActionMode()` even if not used

#### Adding New Modes

To add a new navigation mode:

1. **Define Domain Mode**: Add to `internal/core/domain/modes.go`
2. **Create Implementation**: Implement the Mode interface
3. **Add CLI Command**: Create CLI command in `internal/cli/`
4. **Update IPC**: Add handler in `internal/app/ipc_controller.go`
5. **Register Mode**: Add to Handler's mode map
6. **Add Tests**: Create unit and integration tests
7. **Update Config**: Add hotkey defaults
8. **Update Docs**: Document in CLI.md and DEVELOPMENT.md

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
- **Modes**: Navigation mode implementations following the `Mode` interface
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
- **Modes**: Navigation mode implementations following the `Mode` interface
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
5. Implement the `Mode` interface in `internal/app/modes/` (see `HintsMode`, `GridMode`, `ScrollMode` for examples)
    - See also `QuadGridMode` for recursive quadrant navigation
6. Register mode in the Handler's mode map in `internal/app/modes/handler.go`

**Actions:**

1. Define action in `internal/core/domain/action/`
2. Implement logic in `internal/app/services/action_service.go`
3. Add handling in `internal/app/modes/actions.go`
4. Update config and documentation

**UI Components:**

1. Create components in `internal/app/components/`
2. Implement rendering in `internal/ui/`
3. Add Objective-C in `internal/core/infra/bridge/` if needed
4. Register in `internal/app/component_factory.go` or `internal/app/app_initialization.go`

**CLI Commands:**

1. Create command file in `internal/cli/`
2. Register in `internal/cli/root.go`
3. Document in `docs/CLI.md`

### Dependency Injection and Wiring

Neru uses manual dependency injection for better testability and explicit dependency management:

1. **Construction**: Dependencies are explicitly passed to constructors
2. **Wiring**: `internal/app/app_initialization.go` wires all components together
3. **Testing**: Dependencies can be mocked by passing test doubles in `NewWithDeps`
4. **Ports**: Interfaces define contracts between layers

Example of dependency injection in action:

```go
// In internal/app/app_initialization.go
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
- **Benchmarks**: Performance testing (tagged based on whether they use system resources)

**When to Use:**

- **Unit Tests**: Business logic, config validation, component interfaces, pure algorithms
- **Integration Tests**: macOS APIs, file operations, IPC, component coordination
- **Benchmarks**: Performance-critical code paths (unit for pure logic, integration for system calls)

**Test Organization:**

```
package_test.go              # Unit tests (logic, mocks)
package_integration_test.go # Integration tests (real system calls)
package_bench_test.go        # Benchmarks (unit or integration based)
```

**Benchmark Classification:**

- **Unit Benchmarks** (`*_bench_test.go`): Pure algorithm performance, no system calls
- **Integration Benchmarks** (`*_bench_integration_test.go`): Real system performance, tagged `//go:build integration`

Examples:

- Domain logic benchmarks → Unit benchmarks
- File I/O benchmarks → Integration benchmarks
- IPC performance benchmarks → Integration benchmarks

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
