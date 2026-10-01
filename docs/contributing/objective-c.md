# Objective-C guidelines

Style and memory rules for the native bridge code under
`internal/adapter/platform/darwin/` and the C under `internal/adapter/platform/linux/`.
Read it before adding or changing a `.m`, `.h` or `.c` file. `just fmt` applies
the formatting and `just lint` runs clang-tidy on the `.m` files.

## File organization

### CGO and Go files

Native bridge implementations belong in platform bridge files, not in Go CGO comment blocks:

- **macOS**: `.m` / `.h` under `internal/adapter/platform/darwin/`
- **Linux**: `.c` / `.h` under `internal/adapter/platform/linux/` (Wayland protocol stubs stay in `wlr_protocol/`)

Go files may use a minimal CGO preamble. It holds only `#include` headers, `#cgo` flags, `#include <stdlib.h>` when using `C.CString`/`C.free`, and `extern` declarations for `//export` callbacks. Packages that call bridge symbols from another directory should blank-import `internal/adapter/platform/linux` or `darwin`, so the linker pulls in the compiled native objects once. `wlr_protocol` follows the same pattern.

Bridge `.c` / `.m` files must `#include` their matching header and must **not** re-declare structs or typedefs already defined in that header. Duplicate definitions cause `conflicting types` errors when CGO includes the same header.

### Header files (.h)

- Minimal public interface
- Use `@class` forward declarations when possible
- Group related declarations with `#pragma mark`

```objc
#import <Foundation/Foundation.h>

@class NSWindow;
@class NSColor;

typedef void *OverlayWindow;

OverlayWindow NeruCreateOverlayWindow(void);
void NeruDestroyOverlayWindow(OverlayWindow window);
void NeruShowOverlayWindow(OverlayWindow window);
void NeruHideOverlayWindow(OverlayWindow window);
```

### Implementation files (.m)

Standard structure:

1. File header comment
2. Imports
3. `#pragma mark` sections
4. Interface declarations (private)
5. Implementation
6. C interface functions

```objc
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

OverlayWindow NeruCreateOverlayWindow(void) {
    // Implementation
}
```

## Naming conventions

### C bridge exports (Go-callable)

Every function declared in a `.h` file and called from Go via CGO must use a **`Neru` prefix** (PascalCase after the prefix) to avoid symbol collisions with system libraries and to mark the public bridge functions. Do not add unprefixed CGO exports.

```objc
OverlayWindow NeruCreateOverlayWindow(void);
void NeruShowOverlayWindow(OverlayWindow window);
EventTap NeruCreateEventTap(EventTapCallback callback, void *userData);
int NeruCheckAccessibilityPermissions(void);
int NeruRegisterHotkey(int keyCode, int modifiers, int hotkeyId, HotkeyCallback callback, void *userData);
```

Objective-C methods, private `static` helpers, and symbols not exported through bridge headers keep Apple's usual camelCase without the prefix.

### Objective-C methods

- Use descriptive names with clear intent
- Follow Apple's naming conventions
- Start with lowercase letter, use camelCase

```objc
- (void)showWindow;
- (void)hideWindow;
- (void)updateHints:(NSArray *)hints;
- (NSColor *)colorFromHex:(NSString *)hexString;
```

## Property attributes

- `strong` for object ownership
- `weak` for delegates and to avoid retain cycles
- `assign` for primitive types
- `copy` for NSString and blocks

```objc
@property(nonatomic, strong) NSWindow *window;
@property(nonatomic, weak) id<Delegate> delegate;
@property(nonatomic, assign) CGFloat opacity;
@property(nonatomic, copy) NSString *title;
```

## Memory management

All Objective-C in this repo compiles with **ARC** (`-fobjc-arc`, set in
`internal/adapter/platform/darwin/cgo_flags.go`). Never write `retain`,
`release`, or `autorelease`, because under ARC they are compile errors. You
manage two boundaries by hand, because ARC cannot see across them:

- the CGO boundary, where Go holds a `void *`
- Core Foundation objects (`AX*`, `CG*`, `CF*`)

### Handing an ObjC object to Go and back

Go keeps native objects alive through an opaque `void *`. ARC does not know Go
holds a reference, so transfer ownership explicitly at the boundary:

```objc
// Create: transfer ownership OUT of ARC — the Go side now owns +1.
OverlayWindow NeruCreateOverlayWindow(void) {
    OverlayWindowController *controller = [[OverlayWindowController alloc] init];
    return (__bridge_retained void *)controller;  // ARC will not release this
}

// Destroy: transfer ownership back INTO ARC, which releases it.
void NeruDestroyOverlayWindow(OverlayWindow window) {
    OverlayWindowController *controller = CFBridgingRelease(window);
    [controller.window close];
    // controller released by ARC at end of scope
}

// Borrow: use the object without touching its refcount.
void NeruShowOverlayWindow(OverlayWindow window) {
    OverlayWindowController *controller = (__bridge OverlayWindowController *)window;
    [controller showWindow:nil];
}
```

Real examples: `overlay_darwin.m` (`NeruCreateOverlayWindow` /
`NeruDestroyOverlayWindow`, and the resize path that `CFRelease`s the old
controller before storing a `__bridge_retained` replacement).

### Core Foundation objects (AX*, CG*, CF*)

CF types are **not managed by ARC**. Follow the Create/Copy rule: anything
returned by a function named `Create` or `Copy` (including
`AXUIElementCopyAttributeValue`) is +1 and you must `CFRelease` it exactly
once.

**The ownership rule at the CGO boundary:** every `AXUIElementRef` (or other CF
ref) returned to Go through a `Neru*` function is **+1 retained, and the Go
caller owns it**. On the Go side that means calling `Element.Release()` (or
`ReleaseAll`) when done, including on every element a tree traversal enqueues
but abandons. Leaked AX elements have been a recurring bug in this repo. When
you write a traversal, release every ref you receive.

### Mach ports

`CFMachPortRef` wraps a kernel resource, and `CFRelease` alone leaks the port.
Invalidate first:

```objc
CFMachPortInvalidate(tap->eventTap);  // releases the kernel port
CFRelease(tap->eventTap);             // releases the CF wrapper
```

See `eventtap_darwin.m` for both call sites and the rationale.

### Autorelease pools

Long-running or Go-called code paths that allocate ObjC objects should wrap
their work in `@autoreleasepool { ... }`, because threads Go creates have no
ambient pool. Drawing paths and traversal loops in `overlay_darwin.m` and the
`accessibility_*` files show the pattern.

## Comments

Use HeaderDoc-style comments:

```objc
/// Initialize with frame
/// @param frame View frame
/// @return Initialized instance
- (instancetype)initWithFrame:(NSRect)frame;
```

Inline comments:

```objc
// Clear background
[[NSColor clearColor] setFill];
NSRectFill(dirtyRect);

// Pre-size for typical hint count
_hints = [NSMutableArray arrayWithCapacity:100];
```

## Code organization

Use `#pragma mark` to organize code:

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

## Threading

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

- Use `dispatch_sync` when you need the result immediately
- Use `dispatch_async` for UI updates and non-blocking operations

## See also

- [darwin/AGENTS.md](../../internal/adapter/platform/darwin/AGENTS.md), the short form of the contract this guide explains in full
