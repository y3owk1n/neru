# Objective-C guidelines

Rules for the native bridge code under `internal/adapter/platform/darwin/` and
the C under `internal/adapter/platform/linux/` that `just fmt` (clang-format)
and `just lint` (the clang analyzer through clang-tidy, on `.m` files) cannot
enforce. [darwin/AGENTS.md](../../internal/adapter/platform/darwin/AGENTS.md)
holds the short form of this contract.

## File organization

Native code lives in bridge files, not in Go CGO comment blocks:

- **macOS**: `.m` and `.h` under `internal/adapter/platform/darwin/`
- **Linux**: `.c` and `.h` under `internal/adapter/platform/linux/`, with
  Wayland protocol stubs in `wlr_protocol/`

A Go file's CGO preamble holds only `#include` lines, `#cgo` flags,
`#include <stdlib.h>` when it uses `C.CString` or `C.free`, and `extern`
declarations for `//export` callbacks. A package that calls bridge symbols from
another directory blank-imports `internal/adapter/platform/linux` or `darwin`,
so the linker pulls in the native objects once. `wlr_protocol` follows the same
pattern.

A bridge `.c` or `.m` file includes its matching header and never re-declares
a struct or typedef that header defines. A duplicate causes a
`conflicting types` error when CGO includes the same header.

Keep headers minimal, prefer `@class` forward declarations, and group code with
`#pragma mark` sections. Document functions with HeaderDoc `///` comments.

## Naming

Every function declared in a `.h` file and called from Go uses a **`Neru`
prefix**, PascalCase after it. This avoids collisions with system symbols and
marks the bridge surface:

```objc
OverlayWindow NeruCreateOverlayWindow(void);
EventTap NeruCreateEventTap(EventTapCallback callback, void *userData);
int NeruRegisterHotkey(int keyCode, int modifiers, int hotkeyId, HotkeyCallback callback, void *userData);
```

Objective-C methods, `static` helpers and symbols outside bridge headers use
Apple's camelCase without the prefix.

## Memory management

All Objective-C here compiles with ARC (`-fobjc-arc`, set in
`internal/adapter/platform/darwin/cgo_flags.go`). ARC cannot see across two
boundaries, so you manage them by hand: the CGO boundary, where Go holds a
`void *`, and Core Foundation objects (`AX*`, `CG*`, `CF*`).

### Handing an ObjC object to Go and back

ARC does not know Go holds a reference, so transfer ownership explicitly:

```objc
// Create: move ownership out of ARC. Go now owns +1.
OverlayWindow NeruCreateOverlayWindow(void) {
    OverlayWindowController *controller = [[OverlayWindowController alloc] init];
    return (__bridge_retained void *)controller;
}

// Destroy: move ownership back into ARC, which releases it at end of scope.
void NeruDestroyOverlayWindow(OverlayWindow window) {
    OverlayWindowController *controller = CFBridgingRelease(window);
    [controller.window close];
}

// Borrow: use the object without touching its refcount.
void NeruShowOverlayWindow(OverlayWindow window) {
    OverlayWindowController *controller = (__bridge OverlayWindowController *)window;
    [controller showWindow:nil];
}
```

`overlay_darwin.m` has the real pair, plus a resize path that `CFRelease`s the
old controller before storing a `__bridge_retained` replacement.

### Core Foundation refs returned to Go

Every `AXUIElementRef`, or other CF ref, returned to Go through a `Neru*`
function is **+1 retained, and the Go caller owns it**. The Go side calls
`Element.Release()` or `ReleaseAll` when done, including on every element a
tree traversal enqueues but abandons. Leaked AX elements are a recurring bug
here, so release every ref a traversal receives.

### Mach ports

`CFRelease` alone leaks the kernel port behind a `CFMachPortRef`. Invalidate it
first:

```objc
CFMachPortInvalidate(tap->eventTap);  // releases the kernel port
CFRelease(tap->eventTap);             // releases the CF wrapper
```

`eventtap_darwin.m` has both call sites.

### Autorelease pools

Threads Go creates have no autorelease pool. Wrap Go-called or long-running
code that allocates ObjC objects in `@autoreleasepool { ... }`, as the drawing
paths and traversal loops in `overlay_darwin.m` and the `accessibility_*` files
do.

## Threading

Update UI on the main thread only:

```objc
if ([NSThread isMainThread]) {
    [self.window orderFront:nil];
} else {
    dispatch_async(dispatch_get_main_queue(), ^{
        [self.window orderFront:nil];
    });
}
```

Use `dispatch_sync` when the caller needs the result, and `dispatch_async` for
UI updates and other non-blocking work.
