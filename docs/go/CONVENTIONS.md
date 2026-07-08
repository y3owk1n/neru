# Go Conventions

## Package Organization

### Package Names

- Short, lowercase, single-word names when possible
- No underscores, hyphens, or mixed caps

```go
package hints
package grid
package config
```

### Package Documentation

Every package should have a `doc.go` file:

```go
// Package hints provides hint generation and management for the Neru application.
package hints
```

---

## File Structure

1. Package declaration
2. Imports (organized by `goimports` — stdlib, external, internal)
3. Constants
4. Type definitions
5. Constructor functions
6. Methods (grouped by receiver type)
7. Helper functions

---

## Imports

Organized by `goimports` into three groups:

```go
import (
    "context"

    "github.com/y3owk1n/neru/internal/core/domain"
    "go.uber.org/zap"
)
```

---

## Naming

| Scope          | Convention                                | Example                         |
| :------------- | :---------------------------------------- | :------------------------------ |
| Variables      | camelCase local, PascalCase exported      | `localVar`, `ExportedVar`       |
| Constants      | PascalCase exported, camelCase unexported | `MaxSize`                       |
| Receiver names | Consistent single-letter                  | `a` for `App`, `c` for `Config` |
| Packages       | lowercase, short                          | `hints`, `grid`                 |

---

## Function Parameters

- `context.Context` first (always if present)
- Required parameters
- Optional parameters (or functional options)

```go
func (s *Service) Process(ctx context.Context, id string, opts ...Option) error
```

---

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

---

## Error Handling

Use the `derrors` package for structured errors:

```go
import derrors "github.com/y3owk1n/neru/internal/core/errors"

// Create new error
return derrors.New(derrors.CodeInvalidConfig, "config validation failed")

// Wrap existing error
return derrors.Wrap(err, derrors.CodeIPCFailed, "failed to start IPC server")
```

---

## Context

- Always accept `context.Context` as first parameter for cancellable operations
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

---

## Concurrency

| Pattern                                    | When to Use                 |
| :----------------------------------------- | :-------------------------- |
| `sync.RWMutex`                             | Read-heavy workloads        |
| `sync.Mutex`                               | Write-heavy or simple cases |
| Always defer unlock immediately after lock |

```go
func (s *Service) Get(id string) (*Item, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.cache[id], nil
}
```

---

## Comments

- Comment public APIs and exported symbols
- Complete sentences with proper punctuation
- Explain _why_ for non-obvious code, not _what_

```go
// Pre-allocate slice capacity to avoid reallocations during hint generation.
// Typical hint count is 50-200 elements.
hints := make([]Hint, 0, 100)
```

---

## Performance Patterns

```go
// Pre-allocate slices
hints := make([]Hint, 0, expectedCount)

// String building
var b strings.Builder
b.WriteString("prefix")
b.WriteString(value)
return b.String()
```

---

## Cross-Platform Conventions

### Build Tags

Always include a blank line after the tag:

```go
//go:build darwin

package platform
```

### Platform Isolation

- **The One Rule:** Non-darwin code must never import `internal/core/infra/platform/darwin`
- Use **Ports** in `internal/core/ports/` for platform-agnostic interfaces
- Use **Adapters** in `internal/core/infra/` for implementations

### OS-Specific File Naming

| Suffix               | Purpose              |
| :------------------- | :------------------- |
| `*_darwin.go`        | macOS-specific       |
| `*_linux.go`         | Linux-specific       |
| `*_linux_common.go`  | Shared Linux wrapper |
| `*_linux_x11.go`     | X11 backend          |
| `*_linux_wayland.go` | Wayland backend      |
| `*_windows.go`       | Windows-specific     |
| `*_other.go`         | Non-target fallback  |

### Platform Factory

`internal/core/infra/platform/factory.go` uses build-tagged `factory_<os>.go` files to return the correct `ports.SystemPort` implementation.

---

## See Also

- [Contributing Guide](../../CONTRIBUTING.md)
- [Objective-C Guidelines](OBJECTIVE_C.md)
