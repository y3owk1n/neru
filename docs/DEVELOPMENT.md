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

| Task               | Command                   | Description                           |
| ------------------ | ------------------------- | ------------------------------------- |
| Build              | `just build`              | Compile the application               |
| Test (Unit)        | `just test`               | Run unit tests                        |
| Test (Race)        | `just test-race`          | Run tests with race detection         |
| Test (Integration) | `just test-integration`   | Run integration tests with macOS APIs |
| Test (Full Suite)  | `just test-full-suite`    | Complete test pipeline                |
| Coverage           | `just test-coverage`      | Generate coverage report              |
| Lint               | `just lint`               | Run linters                           |
| Format             | `just fmt`                | Format code                           |
| Run                | `just run`                | Build and run the application         |
| Clean              | `just clean`              | Remove build artifacts                |
| Quality Check      | `just test-quality-check` | Analyze test suite metrics            |

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

Neru has a comprehensive test suite with multiple testing strategies to ensure code quality and reliability. The test suite includes unit tests, integration tests, fuzz tests, performance benchmarks, and chaos testing.

### Test Commands Overview

| Command                       | Purpose                               | When to Use                    |
| ----------------------------- | ------------------------------------- | ------------------------------ |
| `just test`                   | Run all unit tests                    | Basic testing, CI/CD           |
| `just test-race`              | Run tests with race detection         | Concurrent code validation     |
| `just test-integration`       | Run integration tests                 | Real system API testing        |
| `just test-coverage`          | Generate coverage report              | Code coverage analysis         |
| `just test-coverage-summary`  | Show coverage percentage              | Quick coverage check           |
| `just test-coverage-detailed` | Detailed coverage analysis            | Deep coverage inspection       |
| `just test-flaky-check`       | Check for flaky tests                 | Test reliability validation    |
| `just test-quality-check`     | Test suite quality metrics            | Test suite health              |
| `just test-full-suite`        | Complete test pipeline                | Pre-release validation         |
| `just test-race-integration`  | Integration tests with race detection | Concurrent integration testing |


### Running Tests

#### Basic Unit Tests

```bash
# Run all unit tests
just test

# Run tests with verbose output
go test -v ./...

# Run tests for specific package
go test ./internal/domain/grid

# Run specific test function
go test -run TestGrid_Initialization ./internal/domain/grid
```

#### Race Detection Tests

```bash
# Run tests with race detection (detects data races)
just test-race

# Run with race detection and verbose output
go test -race -v ./...
```

Race detection is crucial for concurrent code. Use this when:

- Working with goroutines, channels, or shared state
- Modifying concurrent data structures
- Adding new threading logic

#### Integration Tests

```bash
# Run integration tests (requires macOS Accessibility permissions)
just test-integration

# Run integration tests with race detection
just test-race-integration
```

Integration tests interact with real macOS APIs and require:

- Accessibility permissions enabled
- System UI elements available for testing
- May be slower and less predictable than unit tests

**Note:** Integration tests are tagged with `//go:build integration` and only run when explicitly requested.

#### Coverage Analysis

```bash
# Generate coverage report
just test-coverage

# Show coverage percentage only
just test-coverage-summary

# Detailed coverage analysis by package and function
just test-coverage-detailed

# Generate HTML coverage report
just test-coverage-html
```

Coverage commands help you:

- Identify untested code paths
- Track coverage improvements over time
- Ensure critical code is well-tested
- Meet coverage requirements for contributions

#### Test Quality and Reliability

```bash
# Check for flaky tests (run tests multiple times)
just test-flaky-check

# Analyze test suite quality metrics
just test-quality-check
```

Use these commands to:

- Detect unreliable tests that sometimes fail
- Monitor test suite health and growth
- Ensure test suite quality standards

#### Complete Test Pipeline

```bash
# Full validation with detailed analysis (recommended for all PRs and releases)
just test-full-suite
```

**What it includes:**
- Unit tests with race detection
- Integration tests (macOS APIs)
- Detailed coverage analysis (by function, package, uncovered functions)
- Test suite quality metrics (file counts, benchmark counts, etc.)

The full test suite includes:

1. Unit tests
2. Race detection tests
3. Integration tests
4. Coverage analysis
5. Quality metrics

### Advanced Testing Features

#### Fuzz Testing

Neru includes fuzz tests for automated edge case discovery:

```go
// Example fuzz test in ipc_test.go
func FuzzCommandJSON(f *testing.F) {
    // Add seed inputs
    f.Add(`{"action":"test","params":{}}`)

    f.Fuzz(func(t *testing.T, input string) {
        var cmd ipc.Command
        err := json.Unmarshal([]byte(input), &cmd)
        // Test marshaling/unmarshaling robustness
        if err == nil {
            data, err := json.Marshal(cmd)
            // ... additional validation
        }
    })
}
```

#### Chaos Testing

Chaos tests inject failures to ensure system resilience:

```go
func TestIPCAdapter_ChaosTesting(t *testing.T) {
    // Test with malformed inputs, network failures, etc.
    // Ensures graceful error handling
}
```

#### Concurrent Testing

Comprehensive concurrent testing with multiple goroutines:

```go
func TestIPCAdapter_ConcurrentOperations(t *testing.T) {
    // Tests thread safety with 10 goroutines × 5 calls each
    // Validates concurrent access patterns
}
```

#### Property-Based Testing

Grid generation validation across multiple input combinations:

```go
func TestGrid_PropertyBased(t *testing.T) {
    // Tests grid generation with various character sets and screen sizes
    // Ensures consistent behavior across different inputs
}
```

### Testing During Development

#### Watch Mode

```bash
# Watch for file changes and run tests (requires entr)
find . -name "*.go" | entr -r just test

# Watch specific package
find internal/domain/grid -name "*.go" | entr -r go test ./internal/domain/grid
```

#### Quick Iteration

```bash
# Build and test cycle
just build && just test && ./bin/neru launch --config test-config.toml

# Fast feedback loop
just test && just lint
```

#### Debugging Tests

```bash
# Run test with verbose output
go test -v -run TestSpecificFunction ./internal/package

# Run test with CPU profiling
go test -cpuprofile cpu.prof ./internal/package

# Run test with memory profiling
go test -memprofile mem.prof ./internal/package

# Analyze profiles
go tool pprof cpu.prof
```

### Test Organization

#### Test File Naming

- Unit tests: `*_test.go` (e.g., `grid_test.go`)
- Integration tests: `*_integration_test.go` (e.g., `accessibility_integration_test.go`)
- Benchmark tests: `*_test.go` with `func Benchmark*`

#### Test Tags

```go
// Integration tests (only run with -tags=integration)
//go:build integration

package adapter

// Fuzz tests (Go 1.18+)
//go:build go1.18
```

**Note:** All integration test files (`*_integration_test.go`) now include the `//go:build integration` tag to properly separate integration tests from unit tests.

#### Test Helpers

Common test utilities are available in test files:

- `assert` functions for common assertions
- `testdata` directories for test fixtures
- Mock implementations for external dependencies
- Helper functions for test setup/teardown

### Performance Testing

#### Benchmarks

```bash
# Run all benchmarks
just bench

# Run specific benchmark
go test -bench=BenchmarkGridGeneration ./internal/domain/grid

# Run benchmarks with memory allocation info
go test -bench=. -benchmem ./...

# Compare benchmark results
go test -bench=. -count=3 ./... | tee benchmark.txt
```

#### Profiling

```bash
# Profile test execution
go test -cpuprofile cpu.prof -memprofile mem.prof ./internal/package

# Profile running application
NERU_PPROF=:6060 ./bin/neru launch
# Then visit http://localhost:6060/debug/pprof/
```

### Code Quality Tools

#### Linting

```bash
# Run all linters
just lint

# Auto-fix linting issues
golangci-lint run --fix

# Run specific linter
golangci-lint run --enable=goimports
```

#### Formatting

```bash
# Format Go code
just fmt

# Check formatting without changes
golangci-lint run --enable=gofmt --enable=goimports --disable-all
```

### Testing Best Practices

#### When to Write Tests

- **Always test new features** - Every PR should include tests
- **Test edge cases** - Empty inputs, nil values, boundary conditions
- **Test error conditions** - Ensure proper error handling
- **Test concurrent code** - Use race detection and concurrent tests
- **Test performance** - Add benchmarks for critical paths

#### Test Structure

```go
func TestFunctionName(t *testing.T) {
    // Arrange
    setup := createTestSetup()

    // Act
    result, err := functionUnderTest(input)

    // Assert
    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }
    if result != expected {
        t.Errorf("got %v, want %v", result, expected)
    }
}
```

#### Table-Driven Tests

```go
func TestParseHotkey(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    Hotkey
        wantErr bool
    }{
        {"simple", "Cmd+Space", Hotkey{Mod: Cmd, Key: Space}, false},
        {"invalid", "Invalid", Hotkey{}, true},
        // Add more test cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParseHotkey(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("ParseHotkey() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("ParseHotkey() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

#### Mocking Dependencies

```go
func TestService_DoSomething(t *testing.T) {
    // Create mock dependencies
    mockPort := &mocks.MockDependencyPort{}
    mockPort.On("Method", mock.Anything).Return(expectedResult, nil)

    // Create service with mocks
    service := NewService(mockPort, logger)

    // Test the service
    result, err := service.DoSomething(input)

    // Assert expectations
    mockPort.AssertExpectations(t)
}
```

### Continuous Integration

Tests run automatically on:

- Every push to main branch
- Every pull request
- Before releases

**CI Pipeline:**

The CI runs multiple parallel jobs to validate different aspects of the codebase:

1. **`lint`**: Code style and best practices validation
2. **`formatting`**: Code formatting compliance check
3. **`vet`**: Go static analysis for suspicious constructs
4. **`build`**: Application compilation verification
5. **`test`**: Unit tests (excludes integration tests)
6. **`test-race`**: Race detection for concurrent code
7. **`test-integration`**: macOS API integration tests

**Test Separation:**
- **Unit Tests** (`just test`): Fast, isolated tests without external dependencies
- **Integration Tests** (`just test-integration`): Real macOS API validation with `//go:build integration` tags

**Note:** Integration tests require macOS Accessibility permissions. In CI, permissions are reset using `tccutil` but GitHub Actions macOS runners have restricted TCC access. Integration tests typically fail in CI due to permission restrictions - this is expected and doesn't indicate code problems. Always run integration tests locally with proper accessibility permissions enabled in System Settings > Security & Privacy > Privacy > Accessibility.

### Troubleshooting Tests

#### Common Issues

**Tests fail intermittently:**

- Use `just test-flaky-check` to identify flaky tests
- Check for race conditions with `just test-race`
- Ensure proper test isolation

**Integration tests fail:**

- Ensure Accessibility permissions are granted
- Check system UI state
- Run tests in isolation

**Coverage not updating:**

- Ensure test files are in the same package
- Check build tags match
- Verify test functions are named correctly

**Performance regressions:**

- Run benchmarks before/after changes
- Use profiling to identify bottlenecks
- Check for memory leaks

### Contributing Test Improvements

When adding tests:

1. **Follow existing patterns** - Match the style of existing tests
2. **Add meaningful names** - Test names should describe what they verify
3. **Include edge cases** - Test boundary conditions and error paths
4. **Document complex tests** - Add comments for non-obvious test logic
5. **Update this documentation** - Add new test commands or patterns here

### Test Coverage Goals

- **Unit Tests**: Aim for 80%+ coverage on testable code
- **Integration Tests**: Cover critical user journeys
- **Performance Tests**: Benchmark critical code paths
- **Fuzz Tests**: Add for complex input parsing
- **Chaos Tests**: Ensure graceful failure handling

**Note:** Due to CGO dependencies, some infrastructure code cannot be unit tested. Integration tests provide validation for these components.

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
    # Basic testing
    just test && just lint

    # For significant changes, run full suite
    just test-full-suite
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
- [ ] Unit tests pass (`just test`)
- [ ] Race detection tests pass (`just test-race`)
- [ ] Build succeeds (`just build`)
- [ ] Documentation updated if needed
- [ ] Comments are clear and accurate
- [ ] Follows patterns in [CODING_STANDARDS.md](CODING_STANDARDS.md)

**For significant changes:**

- [ ] Integration tests pass (`just test-integration`)
- [ ] Full test suite passes (`just test-full-suite`)
- [ ] Coverage maintained or improved (`just test-coverage-summary`)

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
