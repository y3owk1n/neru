# Objective-C Guidelines

## File Organization

### Header Files (.h)

- Minimal public interface
- Use `@class` forward declarations when possible
- Group related declarations with `#pragma mark`

```objc
#import <Foundation/Foundation.h>

@class NSWindow;
@class NSColor;

typedef void *OverlayWindow;

OverlayWindow createOverlayWindow(void);
void NeruDestroyOverlayWindow(OverlayWindow window);
void NeruShowOverlayWindow(OverlayWindow window);
void NeruHideOverlayWindow(OverlayWindow window);
```

### Implementation Files (.m)

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

OverlayWindow createOverlayWindow(void) {
    // Implementation
}
```

## Naming Conventions

- Use descriptive names with clear intent
- Follow Apple's naming conventions
- Start with lowercase letter, use camelCase

```objc
- (void)showWindow;
- (void)hideWindow;
- (void)updateHints:(NSArray *)hints;
- (NSColor *)colorFromHex:(NSString *)hexString;
```

## Property Attributes

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

## Memory Management

For C interface objects:

- Use `retain`/`release`
- Always balance `retain` with `release`
- Use `autorelease` for return values

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

## Code Organization

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

## See Also

- [Go Conventions](./CONVENTIONS.md)
- [Testing Patterns](../testing/TESTING_PATTERNS.md)
