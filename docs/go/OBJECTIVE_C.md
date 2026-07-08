# Objective-C Guidelines

## File Organization

### CGO and Go Files

Native bridge **implementations** belong in platform bridge files, not in Go CGO comment blocks:

- **macOS:** `.m` / `.h` under `internal/core/infra/platform/darwin/`
- **Linux:** `.c` / `.h` under `internal/core/infra/platform/linux/`

Go files may use a minimal CGO preamble (`#include` headers, `#cgo` flags, `#include <stdlib.h>` when using `C.CString`/`C.free`, and `extern` for `//export` callbacks). Packages calling bridge symbols from another directory should blank-import the platform package so the linker pulls in compiled native objects once.

Bridge `.c`/`.m` files must `#include` their matching header and must **not** re-declare structs or typedefs already defined there.

### Header Files (.h)

- Minimal public interface
- Use `@class` forward declarations when possible
- Group with `#pragma mark`

```objc
#import <Foundation/Foundation.h>

@class NSWindow;

typedef void *OverlayWindow;

OverlayWindow NeruCreateOverlayWindow(void);
void NeruDestroyOverlayWindow(OverlayWindow window);
void NeruShowOverlayWindow(OverlayWindow window);
void NeruHideOverlayWindow(OverlayWindow window);
```

### Implementation Files (.m)

1. File header comment
2. Imports
3. `#pragma mark` sections
4. Interface declarations (private)
5. Implementation
6. C interface functions

---

## Naming

### C Bridge Exports (Go-callable)

Every function in a `.h` file called from Go via CGO must use a **`Neru` prefix** to avoid symbol collisions:

```objc
OverlayWindow NeruCreateOverlayWindow(void);
EventTap NeruCreateEventTap(EventTapCallback callback, void *userData);
int NeruCheckAccessibilityPermissions(void);
int NeruRegisterHotkey(int keyCode, int modifiers, int hotkeyId, HotkeyCallback callback, void *userData);
```

Objective-C methods, private `static` helpers, and non-exported symbols use Apple's usual camelCase without the prefix.

### Objective-C Methods

- Descriptive names with clear intent
- Follow Apple's naming conventions
- Start lowercase, use camelCase

```objc
- (void)showWindow;
- (void)hideWindow;
- (void)updateHints:(NSArray *)hints;
- (NSColor *)colorFromHex:(NSString *)hexString;
```

---

## Property Attributes

```objc
@property(nonatomic, strong) NSWindow *window;       // Object ownership
@property(nonatomic, weak) id<Delegate> delegate;    // Avoid retain cycles
@property(nonatomic, assign) CGFloat opacity;         // Primitive types
@property(nonatomic, copy) NSString *title;           // NSString and blocks
```

---

## Memory Management

For C interface objects:

- Use `retain`/`release` with balanced calls
- Use `autorelease` for return values

```objc
OverlayWindow NeruCreateOverlayWindow(void) {
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

---

## Comments

Use HeaderDoc-style comments:

```objc
/// Initialize with frame
/// @param frame View frame
/// @return Initialized instance
- (instancetype)initWithFrame:(NSRect)frame;
```

---

## Code Organization

Use `#pragma mark` to organize code:

```objc
#pragma mark - Initialization
#pragma mark - Public Methods
#pragma mark - Private Methods
#pragma mark - Drawing
```

---

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

- `dispatch_sync` when you need the result immediately
- `dispatch_async` for UI updates and non-blocking operations

---

## See Also

- [Go Conventions](CONVENTIONS.md)
- [Contributing Guide](../../CONTRIBUTING.md)
