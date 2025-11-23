# Infrastructure Layer

The `internal/infra` package contains low-level, platform-specific code. This layer isolates the complexity of CGo, Objective-C bridges, and OS-level interactions.

## Components

### Accessibility (`internal/infra/accessibility`)
- **CGo Bridge**: Direct interaction with macOS `AXUIElement` APIs.
- **Tree Traversal**: Algorithms for efficiently walking the accessibility tree.
- **Memory Management**: Handles `CFRelease` for Core Foundation objects.

### Bridge (`internal/infra/bridge`)
- **Objective-C**: Contains `.m` and `.h` files for Objective-C code.
- **Screen**: APIs for getting screen bounds and resolution.
- **Mouse**: APIs for moving the cursor and simulating clicks.

### EventTap (`internal/infra/eventtap`)
- Wraps `CGEventTap` to intercept system-wide input events.

### Logger (`internal/infra/logger`)
- Provides the structured logging implementation (Zap).

## Rules
- **Isolation**: CGo code should be restricted to this layer as much as possible.
- **Safety**: Must handle all CGo memory management carefully to prevent leaks.
- **Performance**: Critical paths (like tree traversal) are optimized here.
