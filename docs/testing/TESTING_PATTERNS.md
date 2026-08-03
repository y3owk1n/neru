# Testing Patterns

Conventions for writing tests. Which recipe runs what is in
[DEVELOPMENT.md](../DEVELOPMENT.md#common-tasks); the test tiers and what they
cover are in [DEVELOPMENT.md](../DEVELOPMENT.md#testing).

## Test File Naming

- Unit tests: `*_test.go` (no build tag required)
- macOS integration tests: `*_integration_darwin_test.go` (tagged `//go:build integration && darwin`)
- Linux integration tests: `*_integration_linux_test.go` (tagged `//go:build integration && linux`)
- Windows integration tests: `*_integration_windows_test.go` (tagged `//go:build integration && windows`)
- Examples: `*_example_test.go`

Only the macOS slot is populated today — the Linux and Windows patterns are
reserved for the first tests that land there.

## Test Function Naming

```go
func TestService_Method(t *testing.T)
func TestService_Method_EdgeCase(t *testing.T)
func ExampleService_Method()
```

## When to Use Each Type

| Scenario           | Test Type   | Example                            |
| ------------------ | ----------- | ---------------------------------- |
| Business logic     | Unit        | Hint generation, grid calculations |
| Config validation  | Unit        | TOML parsing, field validation     |
| Platform API calls | Integration | Accessibility, event tap, hotkeys  |
| File operations    | Integration | Config loading, log writing        |
| IPC communication  | Integration | CLI-to-daemon messaging            |

## Test Structure

### Arrange-Act-Assert

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

## Cross-Platform Testing

### Mocking Ports

Since core services depend on `ports` interfaces, use **mocks** for unit tests.

```go
// In internal/ports/mocks/
type MockAccessibilityPort struct {
    // ...
}
```

### OS-Specific Integration Tests

Integration tests that depend on native APIs (like macOS Accessibility) must use build tags.

```go
//go:build integration && darwin

package accessibility_test

import "testing"

func TestDarwinAccessibility(t *testing.T) {
    // ...
}
```

#### macOS tests that need the main run loop

Native work such as building the keyboard layout maps or creating a `CGEventTap`
is dispatched to the main queue, which is only drained while the main run loop
runs. The daemon starts that loop in `cmd/neru`; a `go test` binary never does.
A test that reaches such code therefore only passes when the Go scheduler
happens to run it on the main OS thread — otherwise `dispatch_async` work
silently times out (an empty keymap makes every key name fail to parse) and
`dispatch_sync` deadlocks.

Pump the run loop from `TestMain` so the behaviour is deterministic:

```go
// Pin the main thread during package init so TestMain still runs on it.
func init() {
    runtime.LockOSThread()
}

func TestMain(m *testing.M) {
    os.Exit(darwin.RunMainLoopForTesting(m.Run))
}
```

`*_integration_darwin_test.go` files are exempt from the "One Rule", so they may
import `internal/adapter/platform/darwin` directly.

Integration tests only run meaningfully on their target OS — the build tag
excludes them everywhere else. See
[DEVELOPMENT.md](../DEVELOPMENT.md#common-tasks) for the recipes.
