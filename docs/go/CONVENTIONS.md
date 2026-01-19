# Go Conventions

## Package Organization

### Package Names

- Use short, lowercase, single-word names when possible
- Avoid underscores, hyphens, or mixed caps

```go
package hints
package grid
package config
```

### Package Documentation

Every package should have a `doc.go` file with package-level documentation:

```go
// Package hints provides hint generation and management for the Neru application.
package hints
```

## File Structure

1. Package declaration
2. Imports (organized by `goimports`)
3. Constants
4. Type definitions
5. Constructor functions
6. Methods (grouped by receiver type)
7. Helper functions

## Imports

Organized by `goimports` into three groups:

1. Standard library
2. External packages
3. Internal packages

Use aliases for packages with common names:

```go
import (
  "context"

  "github.com/y3owk1n/neru/internal/core/domain"
  "go.uber.org/zap"
)
```

## Naming

- Packages: lowercase, short, descriptive
- Variables: camelCase local, PascalCase exported
- Constants: PascalCase exported, camelCase unexported
- Receiver names: consistent single-letter (e.g., `a` for `App`, `c` for `Config`)

## Function Parameters

- `context.Context` first parameter (always if present)
- Required parameters
- Optional parameters (or use functional options pattern)

```go
func (s *Service) Process(ctx context.Context, id string, opts ...Option) error
```

## Return Values

- Return errors as the last value
- Use named return values sparingly

```go
func (s *Service) Get(id string) (*Item, error) {
  item, err := s.fetch(id)
  if err != nil {
    return nil, err
  }
  return item, nil
}
```

## Error Handling

Use the `derrors` package for structured errors:

```go
import derrors "github.com/y3owk1n/neru/internal/core/errors"

// Create new error
return derrors.New(derrors.CodeInvalidConfig, "config validation failed")

// Wrap existing error
return derrors.Wrap(err, derrors.CodeIPCFailed, "failed to start IPC server")
```

## Context

- Always accept `context.Context` as first parameter for cancellable operations
- Pass context through the call stack
- Don't store context in structs

```go
func (s *Service) Process(ctx context.Context, data []byte) error {
  select {
  case <-ctx.Done():
    return ctx.Err()
  default:
    return s.doProcess(ctx, data)
  }
}
```

## Concurrency

### Mutex Usage

- Use `sync.RWMutex` for read-heavy workloads
- Use `sync.Mutex` for write-heavy or simple cases
- Always defer unlock immediately after lock

```go
func (s *Service) Get(id string) (*Item, error) {
  s.mu.RLock()
  defer s.mu.RUnlock()
  return s.cache[id], nil
}

func (s *Service) Set(id string, item *Item) {
  s.mu.Lock()
  defer s.mu.Unlock()
  s.cache[id] = item
}
```

## Comments

- Comment public APIs and exported symbols
- Use complete sentences with proper punctuation
- Explain _why_ for non-obvious code, not _what_

```go
// Pre-allocate slice capacity to avoid reallocations during hint generation.
// Typical hint count is 50-200 elements.
hints := make([]Hint, 0, 100)
```

## Performance

### Pre-allocation

```go
hints := make([]Hint, 0, expectedCount)
cache := make(map[string]*Item, expectedSize)
```

### String Building

```go
var b strings.Builder
b.WriteString("prefix")
b.WriteString(value)
b.WriteString("suffix")
return b.String()
```

## See Also

- [Testing Patterns](../testing/TESTING_PATTERNS.md)
- [Objective-C Guidelines](./OBJECTIVE_C.md)
