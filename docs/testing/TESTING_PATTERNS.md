# Testing Patterns

## Test File Naming

- Unit tests: `*_test.go` (no build tag required)
- Integration tests: `*_integration_test.go` (tagged `//go:build integration`)
- Unit benchmarks: `*_bench_test.go`
- Integration benchmarks: `*_bench_integration_test.go` (tagged `//go:build integration`)
- Examples: `*_example_test.go`

## Test Function Naming

```go
func TestService_Method(t *testing.T)
func TestService_Method_EdgeCase(t *testing.T)
func BenchmarkService_Method(b *testing.B)
func ExampleService_Method()
```

## Test Types

| Type                  | Command                  | Purpose                                                             |
| --------------------- | ------------------------ | ------------------------------------------------------------------- |
| Unit                  | `just test`              | Business logic, algorithms, validation with mocks                   |
| Integration           | `just test-integration`  | Real macOS APIs, file system, IPC (tagged `//go:build integration`) |
| Unit Benchmark        | `just bench`             | Pure algorithm performance                                          |
| Integration Benchmark | `just bench-integration` | Real system performance                                             |

## When to Use Each Type

| Scenario          | Test Type   | Example                            |
| ----------------- | ----------- | ---------------------------------- |
| Business logic    | Unit        | Hint generation, grid calculations |
| Config validation | Unit        | TOML parsing, field validation     |
| macOS API calls   | Integration | Accessibility, event tap, hotkeys  |
| File operations   | Integration | Config loading, log writing        |
| IPC communication | Integration | CLI-to-daemon messaging            |

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

### Benchmarks

```go
func BenchmarkService_Process(b *testing.B) {
  service := NewService(zap.NewNop(), DefaultConfig())
  ctx := context.Background()

  b.ResetTimer()
  for i := 0; i < b.N; i++ {
    _, _ = service.Process(ctx, "test-data")
  }
}
```
