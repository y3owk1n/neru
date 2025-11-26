# Development Guide

Contributing to Neru: build instructions, architecture overview, and contribution guidelines.

---

## Table of Contents

- [Development Setup](#development-setup)
- [Building](#building)
- [Testing](#testing)
- [Architecture](#architecture)
  - [Overview](#overview)
  - [Project Structure](#project-structure)
  - [Core Packages](#core-packages)
  - [Key Technologies](#key-technologies)
  - [Architectural Layers](#architectural-layers)
  - [Data Flow](#data-flow)
- [Contributing](#contributing)
  - [Before You Start](#before-you-start)
  - [Contribution Workflow](#contribution-workflow)
  - [Code Style](#code-style)
  - [Testing Guidelines](#testing-guidelines)
  - [Documentation](#documentation)
  - [Commit Messages](#commit-messages)
- [Release Process](#release-process)
- [Development Tips](#development-tips)
- [Need Help?](#need-help)
- [Project Philosophy](#project-philosophy)
- [Resources](#resources)

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

| Task   | Command      | Description                   |
| ------ | ------------ | ----------------------------- |
| Build  | `just build` | Compile the application       |
| Test   | `just test`  | Run unit tests                |
| Lint   | `just lint`  | Run linters                   |
| Format | `just fmt`   | Format code                   |
| Run    | `just run`   | Build and run the application |
| Clean  | `just clean` | Remove build artifacts        |

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

### Run Tests

```bash
# All tests
just test

# With race detection
just test-race

# With integration tests
just test-integration

# Coverage
just test-coverage
```

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

# Quick iteration
just build && ./bin/neru launch --config test-config.toml
```

---

## Architecture

### Overview

Neru is a keyboard-driven navigation tool for macOS, designed to enhance productivity by allowing users to quickly navigate and interact with UI elements using keyboard shortcuts. The architecture is modular and follows the Ports and Adapters pattern to separate concerns and facilitate testing and maintenance.

Neru provides two primary navigation modes:

1. **Hints Mode**: Uses macOS Accessibility APIs to identify clickable elements and overlay hint labels
2. **Grid Mode**: Divides the screen into a grid system for coordinate-based navigation

Both modes support various actions (click, scroll, etc.) and can be configured extensively through TOML configuration files.

### Key Technologies

- **Go** - Core application logic, CLI, configuration
- **CGo + Objective-C** - macOS Accessibility API integration
- **Cobra** - CLI framework
- **TOML** - Configuration format
- **Unix Sockets** - IPC communication

### Architectural Layers

Neru follows a clean architecture with clear separation of concerns:

#### 1. Domain Layer (`internal/domain`)

Contains pure business logic with no external dependencies:

- **Entities**: Core concepts like Hint, Grid, Element, etc.
- **Value Objects**: Immutable data structures
- **Interfaces**: Contracts for external dependencies

#### 2. Application Layer (`internal/application`)

Implements use cases and orchestrates domain entities:

- **Services**: Business logic implementations (HintService, GridService)
- **Ports**: Interfaces defining interactions with infrastructure

#### 3. Adapter Layer (`internal/adapter`)

Implements application ports with concrete technologies:

- **Accessibility Adapter**: Bridges domain with macOS Accessibility APIs
- **Overlay Adapter**: Manages UI overlays and rendering
- **Config Adapter**: Handles configuration loading/parsing
- **Hotkey Adapter**: Manages global hotkey registration
- **IPC Adapter**: Handles inter-process communication

#### 4. Infrastructure Layer (`internal/infra`)

Low-level technical implementations:

- **Accessibility**: Direct CGo/Objective-C wrappers for macOS APIs
- **Event Tap**: System-wide input monitoring
- **Hotkeys**: Carbon API integration for global hotkeys
- **IPC**: Unix socket communication
- **Bridge**: Objective-C UI components

#### 5. Presentation/UI Layer (`internal/ui`, `internal/features`)

Handles user interface and presentation logic:

- **Features**: View models and UI adapters for specific functionalities
- **UI**: Rendering logic and overlay management

### Data Flow

1. **Startup**: Configuration is loaded → Dependencies are wired → Hotkeys registered → App waits for input
2. **User Interaction**: Hotkey pressed → Event tap captures → Mode activated → UI overlays displayed
3. **Processing**: User input processed → Actions determined → System APIs called → Results rendered
4. **Cleanup**: Mode exited → Overlays hidden → State reset → App returns to idle

### Key Packages

#### `internal/domain`

Contains the core business logic and entities. This package is pure Go and has no external dependencies.

- **Element**: Represents a UI element with bounds, role, and state.
- **Hint**: Represents a visual hint overlay.
- **Grid**: Represents the grid-based navigation system.
- **Action**: Defines types of actions (click, scroll, etc.).

#### `internal/application`

Implements the application's use cases using Ports and Adapters.

- **Ports**: Interfaces that define interactions with the outside world (`AccessibilityPort`, `OverlayPort`).
- **Services**: Orchestrate logic using domain entities and ports (`HintService`, `GridService`, `ActionService`, `ScrollService`).

#### `internal/adapter`

Concrete implementations of the application ports.

- **Accessibility**: Adapts `internal/infra/accessibility` to `AccessibilityPort`.
- **Overlay**: Adapts `internal/features` (View Models) and `internal/infra/bridge` to `OverlayPort`.
- **Config**: Adapts `internal/config` to `ConfigPort`.
- **Hotkey**: Adapts `internal/infra/hotkeys` to `HotkeyPort`.
- **IPC**: Adapts `internal/infra/ipc` to `IPCPort`.

#### `internal/infra`

Low-level infrastructure code, including CGo and OS interactions.

- **Accessibility**: Direct CGo calls to macOS Accessibility APIs.
- **EventTap**: System-wide input interception.
- **Hotkeys**: Global hotkey registration via Carbon APIs.
- **IPC**: Unix socket communication.
- **Metrics**: Prometheus/OpenTelemetry metrics.
  - Configurable via `[metrics]` section in config.
  - Can be disabled to reduce overhead.

#### `internal/features`

Contains View Models and UI-specific adapters that bridge the Domain layer with the Overlay infrastructure. This layer handles the presentation logic for Hints, Grid, and Scroll modes.

#### `internal/config`

TOML configuration parsing and validation.

**Responsibilities:**

- Load config from multiple locations
- Parse TOML into structs
- Validate configuration
- Provide defaults

#### `internal/app`

Main application orchestration layer that ties all components together:

- **App**: Main application instance containing all state and dependencies
- **Modes**: Mode-specific logic (hints, grid, scroll, action)
- **Components**: Feature components with their specific contexts
- **Lifecycle**: Application startup, shutdown, and state management

#### `internal/cli`

Cobra-based CLI commands.

**Responsibilities:**

- Parse command-line arguments
- Dispatch to appropriate handlers
- Format output
- Error messages

### Where to Add New Code

When contributing to Neru, here's where to place new functionality based on its purpose:

#### Adding New Configuration Options

1. Add fields to appropriate structs in `internal/config/config.go`
2. Update `DefaultConfig()` function with sensible defaults
3. Add validation logic in the `Validate*()` methods
4. Update all TOML configuration examples in `configs/` directory
5. Document new options in `docs/CONFIGURATION.md`
6. Ensure `configs/default-config.toml` reflects new defaults with explanation

#### Adding New Navigation Modes

1. Define domain entities in `internal/domain/`
2. Create application service in `internal/application/services/`
3. Implement adapter in `internal/adapter/`
4. Add feature components in `internal/features/`
5. Register mode in `internal/app/modes/`
6. Update CLI commands in `internal/cli/` if needed

#### Adding New Actions

1. Define action in `internal/domain/action/`
2. Implement action logic in `internal/application/services/action_service.go`
3. Add action handling in `internal/app/modes/actions.go`
4. Update configuration options in `internal/config/config.go` if needed
5. Document new actions in `docs/CONFIGURATION.md`

#### Adding New UI Components

1. Create feature components in `internal/features/`
2. Implement overlay rendering in `internal/ui/`
3. Add Objective-C components in `internal/infra/bridge/` if needed
4. Register components in `internal/app/app.go`

#### Adding New CLI Commands

1. Create new command file in `internal/cli/`
2. Register command in `internal/cli/root.go`
3. Add documentation in `docs/CLI.md`

#### Enhancing Accessibility Support

1. Modify low-level CGo wrappers in `internal/infra/accessibility/`
2. Update adapter in `internal/adapter/accessibility/`
3. Add configuration options in `internal/config/config.go`
4. Document changes in `docs/CONFIGURATION.md`

### Dependency Injection and Wiring

Neru uses manual dependency injection for better testability and explicit dependency management:

1. **Construction**: Dependencies are explicitly passed to constructors
2. **Wiring**: `internal/app/app.go` wires all components together
3. **Testing**: Dependencies can be mocked by passing test doubles in `NewWithDeps`
4. **Ports**: Interfaces define contracts between layers

Example of dependency injection in action:

```go
// In internal/app/app.go
hintService := services.NewHintService(accAdapter, overlayAdapter, hintGen, logger)
gridService := services.NewGridService(overlayAdapter, logger)
actionService := services.NewActionService(accAdapter, overlayAdapter, cfg.Action, logger)
```

---

## Contributing

### Before You Start

1. **Read the Architecture**: Understand the layered architecture and where different types of code belong
2. **Check Existing Issues**: Look for existing issues or start a discussion for major changes
3. **Follow Coding Standards**: Adhere to the guidelines in [CODING_STANDARDS.md](CODING_STANDARDS.md)
4. **Write Tests**: All new functionality should include appropriate tests
5. **Update Documentation**: Keep docs up-to-date with your changes

### Contribution Workflow

1. **Fork the repository**
2. **Create a feature branch**

    ```bash
    git checkout -b feature/amazing-feature
    ```

3. **Make your changes**
    - Write clean, documented code
    - Follow existing code style
    - Add tests for new features
    - Update documentation as needed
4. **Test thoroughly**

    ```bash
    just test && just lint
    ```

5. **Verify build**

    ```bash
    just build
    ```

6. **Commit with conventional commit**

    ```bash
    git commit -m "feat: description of what it does"
    git commit -m "fix(scope): description of what it does"
    ```

7. **Push to your branch**

    ```bash
    git push origin feature/amazing-feature
    ```

8. **Open a Pull Request**
    - Describe what the PR does
    - Reference any related issues
    - Include screenshots/demos if applicable

### Code Style

**All code must follow the [Coding Standards](CODING_STANDARDS.md) document.**

Key requirements:

- **Run formatters before committing:**

    ```bash
    just fmt
    ```

- **Ensure linting passes:**

    ```bash
    just lint
    ```

- **Follow established patterns** - Review existing code for consistency
- **Document exports** - Add godoc comments for public functions/types
- **Handle errors properly** - Use the custom error package with proper wrapping
- **Use meaningful names** - `clickableElement` not `ce`
- **Keep receiver names consistent** - Use short, consistent receiver names (e.g., `s` for Service, `a` for App)

#### Pre-commit Checklist

Before committing, ensure:

- [ ] Code is formatted (`just fmt`)
- [ ] Linters pass (`just lint`)
- [ ] Tests pass (`just test`)
- [ ] Build succeeds (`just build`)
- [ ] Documentation updated if needed
- [ ] Comments are clear and accurate
- [ ] Follows patterns in [CODING_STANDARDS.md](CODING_STANDARDS.md)

**Example:**

```go
// Good - follows coding standards
func (s *Service) GenerateHints(ctx context.Context, elements []Element) ([]Hint, error) {
    if len(elements) == 0 {
        return nil, derrors.New(derrors.CodeInvalidInput, "no elements to generate hints for")
    }
    // ...
}

// Bad - inconsistent receiver, missing context, poor error handling
func (service *Service) gen(e []Element) []Hint {
    // ...
}
```

### Testing Guidelines

- **Write tests for new features**
- **Test edge cases** - Empty inputs, nil values, boundary conditions
- **Use table-driven tests** where appropriate
- **Mock external dependencies** - Don't rely on system state
- **Test at appropriate levels** - Unit tests for domain logic, integration tests for complex flows

**Example table-driven test:**

```go
func TestParseHotkey(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    Hotkey
        wantErr bool
    }{
        {"simple", "Cmd+Space", Hotkey{Mod: Cmd, Key: Space}, false},
        {"invalid", "Cmd-Space", Hotkey{}, true},
        // ...
    }

    for _, testCase := range tests {
        t.Run(testCase.name, func(t *testing.T) {
            got, err := ParseHotkey(testCase.input)
            if (err != nil) != testCase.wantErr {
                t.Errorf("ParseHotkey() error = %v, wantErr %v", err, testCase.wantErr)
                return
            }
            if !reflect.DeepEqual(got, testCase.want) {
                t.Errorf("ParseHotkey() = %v, want %v", got, testCase.want)
            }
        })
    }
}
```

### Documentation

- **Update README.md** if changing user-facing behavior
- **Update docs/** for significant features
- **Add godoc comments** for exported functions
- **Include examples** in documentation
- **Keep docs consistent** with code changes

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
# One-liner: build and run
just build && ./bin/neru launch --config test-config.toml

# Watch for changes (requires entr)
ls **/*.go | entr -r sh -c 'just build && ./bin/neru launch'
```

### Debugging

```bash
# Enable debug logging
[logging]
log_level = "debug"

# Run verbose output
./bin/neru launch

# Watch logs in real-time
tail -f ~/Library/Logs/neru/app.log
```

### Useful Go Commands

```bash
# Format code
gofmt -w .

# Organize imports
goimports -w .

# Check for suspicious constructs
go vet ./...

# List dependencies
go list -m all

# Update dependencies
go get -u ./...
go mod tidy
```

### Profiling

#### Test Profiling

Profile specific tests to identify performance bottlenecks:

```bash
# CPU profile
go test -cpuprofile cpu.prof ./internal/hints
go tool pprof cpu.prof

# Memory profile
go test -memprofile mem.prof ./internal/hints
go tool pprof mem.prof
```

#### Runtime Profiling with NERU_PPROF

Enable Go's [pprof](https://pkg.go.dev/net/http/pprof) HTTP server to profile the running application. This is useful for debugging performance issues, memory leaks, or understanding runtime behavior.

**Enable profiling:**

```bash
# Start Neru with pprof server on port 6060
NERU_PPROF=:6060 ./bin/neru launch

# Or use a different port
NERU_PPROF=localhost:8080 ./bin/neru launch
```

**Access profiles:**

```bash
# View available profiles in browser
open http://localhost:6060/debug/pprof/

# CPU profile (30 seconds)
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# Heap profile
go tool pprof http://localhost:6060/debug/pprof/heap

# Goroutine profile
go tool pprof http://localhost:6060/debug/pprof/goroutine

# Block profile (mutex contention)
go tool pprof http://localhost:6060/debug/pprof/block
```

**Interactive analysis:**

```bash
# Start interactive pprof session
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# Inside pprof:
(pprof) top10        # Show top 10 functions by CPU time
(pprof) list FuncName # Show source code for function
(pprof) web          # Open call graph in browser (requires graphviz)
(pprof) pdf          # Generate PDF call graph
```

**Common use cases:**

- **High CPU usage**: Use CPU profile to find hot code paths
- **Memory leaks**: Use heap profile to identify memory allocations
- **Goroutine leaks**: Use goroutine profile to find stuck goroutines
- **Lock contention**: Use block profile to find mutex bottlenecks

**Example workflow:**

```bash
# 1. Start Neru with profiling
NERU_PPROF=:6060 ./bin/neru launch

# 2. Use the app normally to reproduce the issue

# 3. In another terminal, capture a profile
go tool pprof -http=:8081 http://localhost:6060/debug/pprof/heap

# 4. Browser opens with interactive flame graph and call tree
```

> [!TIP]
> Install graphviz for better visualization: `brew install graphviz`

---

## Need Help?

- **Read existing code** - The codebase is well-structured
- **Check issues** - Someone may have asked the same question
- **Ask in discussions** - Open a discussion for questions
- **Open a draft PR** - Get early feedback on your approach

---

## Project Philosophy

### Keep It Simple

Neru intentionally avoids:

- GUI settings (config files are superior)
- Feature bloat (focus on core navigation)

When adding features, ask:

1. Does this align with keyboard-driven productivity?
2. Is this the simplest way to achieve the goal?
3. Will this complicate maintenance?

### Community-Driven

Neru thrives on community contributions:

- **PRs over issues** - Code speaks louder than feature requests
- **Best-effort maintenance** - No promises of 24/7 support
- **Collective ownership** - Everyone can improve Neru

Your contributions shape Neru's future!

---

## Resources

- **Go Documentation:** <https://golang.org/doc/>
- **macOS Accessibility API:** <https://developer.apple.com/documentation/applicationservices/ax_ui_element_ref>
- **TOML Spec:** <https://toml.io/>
- **Cobra CLI Framework:** <https://github.com/spf13/cobra>
- **Just Command Runner:** <https://github.com/casey/just>
