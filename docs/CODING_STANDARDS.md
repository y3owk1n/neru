# Neru Coding Standards

This document defines the coding standards and conventions for the Neru project. Following these standards ensures the codebase appears written by a single developer and maintains consistency across all files.

## Table of Contents

- [General Standards](#general-standards)
- [Go Standards](#go-standards)
- [Objective-C Standards](#objective-c-standards)
- [Testing Standards](#testing-standards)
- [Documentation Standards](#documentation-standards)
- [Git Commit Standards](#git-commit-standards)

---

## General Standards

### File Formatting

All files must follow these basic formatting rules (enforced by `.editorconfig`):

- **Character encoding**: UTF-8
- **Line endings**: LF (Unix-style)
- **Indentation**: Tabs (width 4 spaces when displayed)
- **Trailing whitespace**: None
- **Final newline**: Required

### File Organization

```
neru/
├── cmd/                    # Application entry points
├── internal/               # Private application code
│   ├── app/               # Application orchestration
│   │   ├── components/    # UI components (overlays, etc.)
│   │   ├── modes/         # Application modes (hints, grid, scroll)
│   │   └── services/      # Business logic services
│   ├── cli/               # CLI commands
│   ├── config/            # Configuration management
│   ├── core/              # Core business logic and infrastructure
│   │   ├── domain/        # Domain models and logic
│   │   ├── errors/        # Error definitions
│   │   ├── infra/         # Infrastructure (bridge, IPC, etc.)
│   │   └── ports/         # Port interfaces
│   └── ui/                # UI rendering
├── configs/               # Configuration examples
├── docs/                  # Documentation
└── scripts/               # Build and utility scripts
```

### Naming Conventions

- **Directories**: lowercase, underscore-separated (e.g., `event_tap`, `app_watcher`)
- **Files**: lowercase, underscore-separated (e.g., `hint_service.go`, `overlay.m`)
- **Test files**: `*_test.go` for unit tests, `*_bench_test.go` for benchmarks, `*_integration_test.go` for integration tests

---

## Go Standards

### Package Organization

#### Package Names

- Use short, lowercase, single-word names when possible
- Avoid underscores, hyphens, or mixed caps
- Use meaningful names that describe the package purpose

**Good:**

```go
package hints
package grid
package config
```

**Bad:**

```go
package hintUtils
package grid_manager
package cfg
```

#### Package Documentation

Every package must have a `doc.go` file with package-level documentation:

```go
// Package hints provides hint generation and management for the Neru application.
//
// This package implements the hint-based navigation system, including hint generation,
// label assignment, and hint overlay rendering.
package hints
```

### File Structure

#### Standard File Order

1. Package declaration
2. Imports (organized by `goimports`)
3. Constants
4. Type definitions
5. Constructor functions
6. Methods (grouped by receiver type)
7. Helper functions

**Example:**

```go
package example

import (
 "context"
 "fmt"

  "github.com/y3owk1n/neru/internal/core/domain"
 "go.uber.org/zap"
)

const (
 DefaultTimeout = 5 * time.Second
)

type Service struct {
 logger *zap.Logger
 config *Config
}

func NewService(logger *zap.Logger, config *Config) *Service {
 return &Service{
  logger: logger,
  config: config,
 }
}

func (s *Service) Process(ctx context.Context) error {
 // Implementation
}
```

### Type Definitions

#### Struct Definitions

- Use named fields
- Group related fields together
- Add comments for non-obvious fields
- Exported fields first, then unexported

**Example:**

```go
// Service manages hint generation and rendering.
type Service struct {
 // Exported fields
 Logger *zap.Logger
 Config *Config

 // Unexported fields
 generator Generator
 cache     *Cache
 mu        sync.RWMutex
}
```

#### Interface Definitions

- Keep interfaces small and focused
- Name interfaces with `-er` suffix when possible
- Document the interface purpose and contract

**Example:**

```go
// Generator creates hint labels for elements.
type Generator interface {
 // Generate creates hint labels for the given count.
 Generate(count int) ([]string, error)
}
```

### Function and Method Signatures

#### Naming

- Use camelCase for unexported functions
- Use PascalCase for exported functions
- Use descriptive names (avoid abbreviations unless widely known)
- Receiver names should be consistent and short (1-2 characters)

**Example:**

```go
// Good receiver names
func (s *Service) Start() error
func (g *Grid) Cell(x, y int) *Cell
func (c *Config) Validate() error

// Bad receiver names
func (service *Service) Start() error  // Too long
func (this *Grid) Cell(x, y int)    // Don't use 'this'
func (cfg *Config) Validate() error    // Inconsistent abbreviation
```

#### Parameter Order

Standard parameter order:

1. `context.Context` (always first if present)
2. Required parameters
3. Optional parameters (or use functional options pattern)

**Example:**

```go
func (s *Service) Process(ctx context.Context, id string, opts ...Option) error
```

#### Return Values

- Return errors as the last return value
- Use named return values sparingly (only when it improves clarity)
- Prefer explicit returns over naked returns

**Example:**

```go
// Good
func (s *Service) Get(id string) (*Item, error) {
 item, err := s.fetch(id)
 if err != nil {
  return nil, err
 }
 return item, nil
}

// Avoid naked returns
func (s *Service) Get(id string) (item *Item, err error) {
 item, err = s.fetch(id)
 return  // Naked return - avoid this
}
```

### Error Handling

#### Error Creation

Use the custom error package for all errors:

```go
import derrors "github.com/y3owk1n/neru/internal/core/errors"

// Create new error
return derrors.New(derrors.CodeInvalidConfig, "config validation failed")

// Wrap existing error
return derrors.Wrap(err, derrors.CodeIPCFailed, "failed to start IPC server")

// Format error message
return derrors.Newf(derrors.CodeInvalidConfig, "invalid value %q for field %s", value, field)
```

#### Error Handling Pattern

```go
result, err := someOperation()
if err != nil {
 return derrors.Wrap(err, derrors.CodeOperationFailed, "operation description")
}
```

#### Error Variable Naming

- Always use `err` for error variables
- For multiple errors in scope, use descriptive names: `parseErr`, `validateErr`, `closeErr`

**Example:**

```go
data, parseErr := parse(input)
if parseErr != nil {
 return derrors.Wrap(parseErr, derrors.CodeParseFailed, "failed to parse input")
}

validateErr := validate(data)
if validateErr != nil {
 return derrors.Wrap(validateErr, derrors.CodeValidationFailed, "validation failed")
}
```

### Comments and Documentation

#### Exported Symbols

All exported symbols must have documentation comments:

```go
// Service manages hint generation and overlay rendering.
// It coordinates between the hint generator, accessibility adapter,
// and overlay renderer to provide hint-based navigation.
type Service struct {
 // ...
}

// NewService creates a new hint service with the provided dependencies.
func NewService(logger *zap.Logger, config *Config) *Service {
 // ...
}

// Generate creates hint labels for all clickable elements on screen.
// It returns an error if hint generation fails or no elements are found.
func (s *Service) Generate(ctx context.Context) ([]Hint, error) {
 // ...
}
```

#### Comment Style

- Use complete sentences with proper punctuation
- Start with the name of the thing being described
- Explain _why_ for non-obvious code, not _what_
- Keep comments up-to-date with code changes

**Example:**

```go
// Good: Explains why
// Pre-allocate slice capacity to avoid reallocations during hint generation.
// Typical hint count is 50-200 elements.
hints := make([]Hint, 0, 100)

// Bad: States the obvious
// Create a slice for hints
hints := make([]Hint, 0, 100)
```

### Import Organization

Imports are automatically organized by `goimports` into three groups:

1. Standard library
2. External packages
3. Internal packages

**Example:**

```go
import (
  "context"
  "fmt"

  "github.com/y3owk1n/neru/internal/core/domain"
  "go.uber.org/zap"
)
```

#### Import Aliases

- Use aliases for packages with common names: `derrors` for `internal/core/errors`
- Use descriptive aliases for adapter packages: `accessibilityAdapter`, `overlayAdapter`
- Avoid single-letter aliases except for well-known packages

### Concurrency Patterns

#### Mutex Usage

- Use `sync.RWMutex` for read-heavy workloads
- Use `sync.Mutex` for write-heavy or simple cases
- Always defer unlock immediately after lock

**Example:**

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

#### Context Usage

- Always accept `context.Context` as the first parameter for operations that may be cancelled
- Pass context through the call stack
- Don't store context in structs

**Example:**

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

### Performance Considerations

#### Slice and Map Pre-allocation

Pre-allocate slices and maps when the size is known or can be estimated:

```go
// Pre-allocate with known capacity
hints := make([]Hint, 0, expectedCount)

// Pre-allocate map
cache := make(map[string]*Item, expectedSize)
```

#### String Building

Use `strings.Builder` for efficient string concatenation:

```go
var b strings.Builder
b.WriteString("prefix")
b.WriteString(value)
b.WriteString("suffix")
return b.String()
```

---

## Objective-C Standards

### File Organization

#### Header Files (.h)

- Minimal public interface
- Use `@class` forward declarations when possible
- Group related declarations with `#pragma mark`

**Example:**

```objc
//
//  overlay.h
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import <Foundation/Foundation.h>

// Forward declarations
@class NSWindow;
@class NSColor;

// Type definitions
typedef void *OverlayWindow;

// Function declarations
OverlayWindow createOverlayWindow(void);
void NeruDestroyOverlayWindow(OverlayWindow window);
void NeruShowOverlayWindow(OverlayWindow window);
void NeruHideOverlayWindow(OverlayWindow window);
```

#### Implementation Files (.m)

Standard file structure:

1. File header comment
2. Imports
3. `#pragma mark` sections for organization
4. Interface declarations (private)
5. Implementation
6. C interface functions

**Example:**

```objc
//
//  overlay.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "overlay.h"
#import <Cocoa/Cocoa.h>

#pragma mark - Overlay View Interface

@interface OverlayView : NSView
@property(nonatomic, strong) NSMutableArray *hints;
@end

#pragma mark - Overlay View Implementation

@implementation OverlayView

- (instancetype)initWithFrame:(NSRect)frame {
    self = [super initWithFrame:frame];
    if (self) {
        _hints = [NSMutableArray arrayWithCapacity:100];
    }
    return self;
}

@end

#pragma mark - C Interface Implementation

OverlayWindow createOverlayWindow(void) {
    // Implementation
}
```

### Naming Conventions

#### Methods

- Use descriptive names with clear intent
- Follow Apple's naming conventions
- Start with lowercase letter
- Use camelCase

**Example:**

```objc
- (void)showWindow;
- (void)hideWindow;
- (void)updateHints:(NSArray *)hints;
- (NSColor *)colorFromHex:(NSString *)hexString;
```

#### Properties

- Use descriptive names
- Specify attributes explicitly
- Group related properties

**Example:**

```objc
@property(nonatomic, strong) NSWindow *window;
@property(nonatomic, strong) NSColor *backgroundColor;
@property(nonatomic, assign) CGFloat opacity;
@property(nonatomic, assign) BOOL isVisible;
```

### Memory Management

#### Property Attributes

- Use `strong` for object ownership
- Use `weak` for delegates and to avoid retain cycles
- Use `assign` for primitive types
- Use `copy` for NSString and blocks

**Example:**

```objc
@property(nonatomic, strong) NSWindow *window;
@property(nonatomic, weak) id<Delegate> delegate;
@property(nonatomic, assign) CGFloat opacity;
@property(nonatomic, copy) NSString *title;
```

#### Manual Memory Management

- Use `retain`/`release` for C interface objects
- Always balance `retain` with `release`
- Use `autorelease` for return values

**Example:**

```objc
OverlayWindow createOverlayWindow(void) {
    OverlayWindowController *controller = [[OverlayWindowController alloc] init];
    [controller retain];
    return (void *)controller;
}

void NeruDestroyOverlayWindow(OverlayWindow window) {
    OverlayWindowController *controller = (OverlayWindowController *)window;
    [controller.window close];
    [controller release];
}
```

### Comments and Documentation

#### HeaderDoc Style

Use HeaderDoc-style comments for documentation:

```objc
/// Initialize with frame
/// @param frame View frame
/// @return Initialized instance
- (instancetype)initWithFrame:(NSRect)frame;

/// Apply hint style
/// @param style Hint style
- (void)applyStyle:(HintStyle)style;

/// Create color from hex string
/// @param hexString Hex color string
/// @param defaultColor Default color
/// @return NSColor instance
- (NSColor *)colorFromHex:(NSString *)hexString defaultColor:(NSColor *)defaultColor;
```

#### Inline Comments

```objc
// Clear background
[[NSColor clearColor] setFill];
NSRectFill(dirtyRect);

// Pre-size for typical hint count
_hints = [NSMutableArray arrayWithCapacity:100];
```

### Code Organization

#### Pragma Marks

Use `#pragma mark` to organize code into logical sections:

```objc
#pragma mark - Initialization

- (instancetype)init {
    // ...
}

#pragma mark - Public Methods

- (void)show {
    // ...
}

#pragma mark - Private Methods

- (void)updateDisplay {
    // ...
}

#pragma mark - Drawing

- (void)drawRect:(NSRect)dirtyRect {
    // ...
}
```

### Threading

#### Main Thread Dispatch

Always update UI on the main thread:

```objc
if ([NSThread isMainThread]) {
    [self.window orderFront:nil];
} else {
    dispatch_async(dispatch_get_main_queue(), ^{
        [self.window orderFront:nil];
    });
}
```

#### Synchronous vs Asynchronous

- Use `dispatch_sync` when you need the result immediately
- Use `dispatch_async` for UI updates and non-blocking operations

---

## Testing Standards

### Test Organization

**File Naming:**

- Unit tests: `service_test.go`
- Integration tests: `service_integration_test.go` (tagged `//go:build integration`)
- Unit benchmarks: `service_bench_test.go`
- Integration benchmarks: `service_bench_integration_test.go` (tagged `//go:build integration`)
- Examples: `service_example_test.go`

**Function Naming:**

```go
func TestService_Method(t *testing.T)
func TestService_Method_EdgeCase(t *testing.T)
func TestService_Method_Integration(t *testing.T)  //go:build integration
func BenchmarkService_Method(b *testing.B)
func BenchmarkService_Method_Integration(b *testing.B)  //go:build integration
func ExampleService_Method()
```

### Test Types

**Unit Tests** (`just test`):

- Business logic, algorithms, validation
- Use mocks for external dependencies
- Fast execution, run on every commit

**Integration Tests** (`just test-integration`):

- Real macOS APIs, file system, IPC
- Use actual implementations
- Tagged with `//go:build integration`
- Run before releases

**Benchmarks** (`just bench`):

- **Unit Benchmarks**: Pure algorithm performance testing (no system calls)
- **Integration Benchmarks**: Real system performance testing (tagged `//go:build integration`)

### When to Use Each Type

| Scenario             | Test Type           | Example                            |
| -------------------- | ------------------- | ---------------------------------- |
| Business logic       | Unit                | Hint generation, grid calculations |
| Config validation    | Unit                | TOML parsing, field validation     |
| Component interfaces | Unit                | Port implementations with mocks    |
| Pure algorithms      | Unit Benchmark      | Sorting, filtering performance     |
| macOS API calls      | Integration         | Accessibility, event tap, hotkeys  |
| File operations      | Integration         | Config loading, log writing        |
| IPC communication    | Integration         | CLI-to-daemon messaging            |
| System performance   | Integration Benchmark | Real API call performance        |

### Test Structure

**Arrange-Act-Assert Pattern:**

```go
func TestService_Process(t *testing.T) {
  // Arrange
  service := NewService(zap.NewNop(), DefaultConfig())

  // Act
  result, err := service.Process(context.Background(), "test-data")

  // Assert
  if err != nil {
   t.Fatalf("unexpected error: %v", err)
  }
  if result == nil {
   t.Fatal("expected non-nil result")
  }
}
```

**Table-Driven Tests:**

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

**Benchmark Tests:**

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

### Unit vs Integration Tests

**Unit Tests** (`just test`):

- Test isolated business logic and algorithms
- Use mocks for external dependencies
- Fast execution, run on every commit
- Cover domain logic, service orchestration, configuration validation

**Integration Tests** (`just test-integration`):

- Test real system interactions with macOS APIs
- Use actual system resources (Accessibility, IPC, file system)
- Tagged with `//go:build integration`
- Slower execution, run before releases
- Cover infrastructure implementations, real component coordination

#### When to Use Unit Tests

- Business logic and algorithms (hint generation, grid calculations)
- Configuration validation and parsing
- Component interfaces with mocked dependencies
- Pure functions and data transformations
- Error handling logic
- Validation rules

#### When to Use Integration Tests

- macOS API interactions (Accessibility, Event Tap, Hotkeys)
- IPC communication between components
- File system operations (config loading/saving)
- Component coordination with real dependencies
- End-to-end workflows with system resources

#### Examples

**Unit Test Example:**

```go
func TestHintGenerator_Generate(t *testing.T) {
    gen := hint.NewAlphabetGenerator("abc")
    elements := []*element.Element{/* mock elements */}
    hints := gen.Generate(context.Background(), elements)
    // Assert hint generation logic
}
```

**Integration Test Example:**

```go
//go:build integration

func TestAccessibilityAdapter_GetCursorPosition(t *testing.T) {
    adapter := accessibility.NewAdapter(/* real infra */)
    pos, err := adapter.GetCursorPosition()
    // Assert real cursor position from system
}
```

### Test Structure

Use the Arrange-Act-Assert pattern:

```go
func TestService_Process(t *testing.T) {
 // Arrange
 logger := zap.NewNop()
 config := &Config{Timeout: 5 * time.Second}
 service := NewService(logger, config)

 // Act
 result, err := service.Process(context.Background(), "test-data")

 // Assert
 if err != nil {
  t.Fatalf("unexpected error: %v", err)
 }
 if result == nil {
  t.Fatal("expected non-nil result")
 }
}
```

### Table-Driven Tests

Use table-driven tests for multiple test cases:

```go
func TestValidate(t *testing.T) {
 tests := []struct {
  name    string
  input   string
  wantErr bool
 }{
  {
   name:    "valid input",
   input:   "valid",
   wantErr: false,
  },
  {
   name:    "empty input",
   input:   "",
   wantErr: true,
  },
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

### Benchmark Tests

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

---

## Documentation Standards

### README Files

- Keep the main README concise and focused
- Link to detailed documentation in `docs/`
- Include quick start examples
- Maintain up-to-date installation instructions

### Documentation Files

All documentation files in `docs/`:

- [INSTALLATION.md](file:///Users/kylewong/Dev/neru/docs/INSTALLATION.md) - Installation instructions
- [CONFIGURATION.md](file:///Users/kylewong/Dev/neru/docs/CONFIGURATION.md) - Configuration reference
- [CLI.md](file:///Users/kylewong/Dev/neru/docs/CLI.md) - CLI usage
- [DEVELOPMENT.md](file:///Users/kylewong/Dev/neru/docs/DEVELOPMENT.md) - Development guide
- [TROUBLESHOOTING.md](file:///Users/kylewong/Dev/neru/docs/TROUBLESHOOTING.md) - Common issues
- [CODING_STANDARDS.md](file:///Users/kylewong/Dev/neru/docs/CODING_STANDARDS.md) - This document

### Code Comments

#### When to Comment

**Do comment:**

- Complex algorithms or logic
- Non-obvious performance optimizations
- Workarounds for bugs or limitations
- Public APIs and exported symbols
- Package-level documentation

**Don't comment:**

- Obvious code (`i++` doesn't need a comment)
- Redundant information already in the code
- Outdated information (update or remove)

#### Comment Quality

**Good:**

```go
// Pre-allocate slice capacity to avoid reallocations during hint generation.
// Typical hint count is 50-200 elements, so we start with 100.
hints := make([]Hint, 0, 100)
```

**Bad:**

```go
// Make a slice
hints := make([]Hint, 0, 100)
```

---

## Git Commit Standards

### Commit Message Format

```
<type>: <subject>

<body>

<footer>
```

### Types

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `test`: Adding or updating tests
- `chore`: Build process, dependencies, etc.

### Examples

```
feat: add grid-based navigation mode

Implement grid-based navigation as an alternative to hint-based navigation.
Grid mode divides the screen into cells and allows precise cursor positioning.

Closes #123
```

```
fix: restore cursor position after hint selection

The cursor was not being restored to its original position after selecting
a hint when restore_cursor_position was enabled. This was caused by the
cursor state being reset too early in the hint selection flow.

Fixes #456
```

---

## Enforcement

### Automated Checks

Run before committing:

```bash
# Format all code
just fmt

# Check formatting (CI)
just fmt-check

# Run linters
just lint

# Run tests
just test

# Build
just build
```

### Pre-commit Checklist

- [ ] Code is formatted (`just fmt`)
- [ ] Linters pass (`just lint`)
- [ ] Tests pass (`just test`)
- [ ] Build succeeds (`just build`)
- [ ] Documentation updated if needed
- [ ] Comments are clear and accurate
- [ ] Commit message follows standards

### Code Review Checklist

- [ ] Follows naming conventions
- [ ] Proper error handling
- [ ] Adequate test coverage
- [ ] Documentation is complete
- [ ] No obvious performance issues
- [ ] Consistent with existing code style
- [ ] No unnecessary complexity

---

## References

- [Effective Go](https://golang.org/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
- [Apple Coding Guidelines for Cocoa](https://developer.apple.com/library/archive/documentation/Cocoa/Conceptual/CodingGuidelines/CodingGuidelines.html)
- [Google Objective-C Style Guide](https://google.github.io/styleguide/objcguide.html)
