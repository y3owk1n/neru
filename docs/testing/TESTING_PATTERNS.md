# Testing Patterns

## Test File Naming

- Unit tests: `*_test.go` (no build tag required)
- macOS integration tests: `*_integration_darwin_test.go` (tagged `//go:build integration && darwin`)
- Linux integration tests: `*_integration_linux_test.go` (tagged `//go:build integration && linux`)
- Examples: `*_example_test.go`

## Test Function Naming

```go
func TestService_Method(t *testing.T)
func TestService_Method_EdgeCase(t *testing.T)
func ExampleService_Method()
```

## Test Types

| Type        | Command                 | Purpose                                                                        |
| ----------- | ----------------------- | ------------------------------------------------------------------------------ |
| Unit        | `just test`             | Business logic, algorithms, validation with mocks                              |
| Integration | `just test-integration` | Real platform APIs, file system, IPC (tagged `//go:build integration && <os>`) |

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
// In internal/core/ports/mocks/
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

### Test Command Usage

- `just test`: Runs all unit tests (platform-agnostic).
- `just test-integration`: Runs integration tests for the **current OS**.
- `just test-all`: Runs both unit and integration tests.
