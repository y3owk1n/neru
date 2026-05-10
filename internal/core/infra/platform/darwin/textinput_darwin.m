#import "textinput.h"

#import <Cocoa/Cocoa.h>

@interface NeruTextInputPanel : NSPanel
@end

@implementation NeruTextInputPanel

- (BOOL)canBecomeKeyWindow {
	return YES;
}

- (BOOL)canBecomeMainWindow {
	return YES;
}

@end

@interface NeruTextInputView : NSTextView
@property(nonatomic, assign) TextInputQueryCallback queryCallback;
@property(nonatomic, assign) TextInputControlCallback confirmCallback;
@property(nonatomic, assign) TextInputControlCallback cancelCallback;
@property(nonatomic, assign) void *userData;
@end

@implementation NeruTextInputView

- (void)notifyQueryChanged {
	if (!self.queryCallback) {
		return;
	}

	NSString *str = self.string ?: @"";
	const char *cstr = [str UTF8String];
	if (!cstr) {
		cstr = "";
	}
	self.queryCallback(cstr, self.userData);
}

- (void)didChangeText {
	[super didChangeText];
	[self notifyQueryChanged];
}

- (void)setMarkedText:(id)string selectedRange:(NSRange)selectedRange replacementRange:(NSRange)replacementRange {
	[super setMarkedText:string selectedRange:selectedRange replacementRange:replacementRange];
	[self notifyQueryChanged];
}

- (void)insertText:(id)string replacementRange:(NSRange)replacementRange {
	[super insertText:string replacementRange:replacementRange];
	[self notifyQueryChanged];
}

- (void)keyDown:(NSEvent *)event {
	unsigned short keyCode = event.keyCode;

	if (keyCode == 53) {
		if ([self hasMarkedText]) {
			[super keyDown:event];
			return;
		}

		if (self.cancelCallback) {
			self.cancelCallback(self.userData);
		}
		return;
	}

	if (keyCode == 36 || keyCode == 76) {
		if ([self hasMarkedText]) {
			[super keyDown:event];
			return;
		}

		if (self.confirmCallback) {
			self.confirmCallback(self.userData);
		}
		return;
	}

	[super keyDown:event];
}

@end

static NeruTextInputPanel *gPanel = nil;
static NeruTextInputView *gTextView = nil;
static NSRunningApplication *gPreviousApp = nil;

static NSScreen *activeScreen(void) {
	NSPoint mouseLoc = [NSEvent mouseLocation];
	for (NSScreen *screen in [NSScreen screens]) {
		if (NSPointInRect(mouseLoc, screen.frame)) {
			return screen;
		}
	}
	return [NSScreen mainScreen];
}

static void startOnMainThread(
    TextInputQueryCallback queryCallback, TextInputControlCallback confirmCallback,
    TextInputControlCallback cancelCallback, int x, int y, int width, int height, void *userData) {
	NSScreen *screen = activeScreen();
	NSRect screenFrame = screen ? screen.frame : NSMakeRect(0, 0, 0, 0);
	CGFloat finalWidth = width > 0 ? (CGFloat)width : 16.0;
	CGFloat finalHeight = height > 0 ? (CGFloat)height : 16.0;
	CGFloat originX = screenFrame.origin.x + (CGFloat)x;
	CGFloat originY = screenFrame.origin.y + (screenFrame.size.height - (CGFloat)y - finalHeight);
	NSRect rect = NSMakeRect(originX, originY, finalWidth, finalHeight);

	if (gPanel) {
		// Callbacks are always the same Go bridge function pointers across sessions;
		// refresh them here in case that assumption ever changes.
		gTextView.queryCallback = queryCallback;
		gTextView.confirmCallback = confirmCallback;
		gTextView.cancelCallback = cancelCallback;
		gTextView.userData = userData;
		[gTextView setString:@""];
		[gTextView notifyQueryChanged];
		[gPanel setFrame:rect display:NO];
		[gTextView setFrame:NSMakeRect(0, 0, finalWidth, finalHeight)];
		[gPanel orderFront:nil];
		[gPanel makeKeyWindow];
		return;
	}

	gPreviousApp = [NSWorkspace sharedWorkspace].frontmostApplication;

#if defined(MAC_OS_X_VERSION_14_0) && MAC_OS_X_VERSION_MAX_ALLOWED >= MAC_OS_X_VERSION_14_0
	if (@available(macOS 14.0, *)) {
		[NSApp activate];
	} else {
		[NSApp activateIgnoringOtherApps:YES];
	}
#else
	[NSApp activateIgnoringOtherApps:YES];
#endif

	NeruTextInputPanel *panel = [[NeruTextInputPanel alloc] initWithContentRect:rect
	                                                                  styleMask:NSWindowStyleMaskBorderless
	                                                                    backing:NSBackingStoreBuffered
	                                                                      defer:NO];

	[panel setReleasedWhenClosed:NO];
	[panel setOpaque:NO];
	[panel setBackgroundColor:[NSColor clearColor]];
	[panel setHasShadow:NO];
	[panel setAlphaValue:0.01];
	[panel setLevel:NSScreenSaverWindowLevel];
	[panel setIgnoresMouseEvents:YES];
	[panel setHidesOnDeactivate:NO];
	[panel setCollectionBehavior:NSWindowCollectionBehaviorCanJoinAllSpaces | NSWindowCollectionBehaviorStationary |
	                             NSWindowCollectionBehaviorIgnoresCycle |
	                             NSWindowCollectionBehaviorFullScreenAuxiliary];

	NeruTextInputView *textView = [[NeruTextInputView alloc] initWithFrame:NSMakeRect(0, 0, finalWidth, finalHeight)];
	[textView setEditable:YES];
	[textView setSelectable:YES];
	[textView setRichText:NO];
	[textView setImportsGraphics:NO];
	[textView setAllowsUndo:NO];
	[textView setAutomaticQuoteSubstitutionEnabled:NO];
	[textView setAutomaticDashSubstitutionEnabled:NO];
	[textView setAutomaticTextReplacementEnabled:NO];
	[textView setAutomaticSpellingCorrectionEnabled:NO];
	[textView setContinuousSpellCheckingEnabled:NO];
	[textView setGrammarCheckingEnabled:NO];
	[textView setTextContainerInset:NSMakeSize(0, 0)];

	textView.queryCallback = queryCallback;
	textView.confirmCallback = confirmCallback;
	textView.cancelCallback = cancelCallback;
	textView.userData = userData;

	[panel setContentView:textView];
	[panel makeKeyAndOrderFront:nil];
	[panel makeFirstResponder:textView];

	gPanel = panel;
	gTextView = textView;
}

static void stopOnMainThread(void) {
	if (!gPanel) {
		return;
	}

	if ([gTextView hasMarkedText]) {
		[gTextView.inputContext discardMarkedText];
	}
	[gTextView setString:@""];
	[gTextView notifyQueryChanged];

	[gPanel orderOut:nil];
	[gPanel close];

	gPanel = nil;
	gTextView = nil;

	if (gPreviousApp && !gPreviousApp.terminated) {
#if defined(MAC_OS_X_VERSION_14_0) && MAC_OS_X_VERSION_MAX_ALLOWED >= MAC_OS_X_VERSION_14_0
		if (@available(macOS 14.0, *)) {
			[gPreviousApp activate];
		} else {
			[gPreviousApp activateWithOptions:NSApplicationActivateIgnoringOtherApps];
		}
#else
		[gPreviousApp activateWithOptions:NSApplicationActivateIgnoringOtherApps];
#endif
	}
	gPreviousApp = nil;
}

int NeruStartHintSearchTextInput(
    TextInputQueryCallback queryCallback, TextInputControlCallback confirmCallback,
    TextInputControlCallback cancelCallback, int x, int y, int width, int height, void *userData) {
	__block int started = 0;

	void (^work)(void) = ^{
		startOnMainThread(queryCallback, confirmCallback, cancelCallback, x, y, width, height, userData);
		started = 1;
	};

	if ([NSThread isMainThread]) {
		work();
	} else {
		dispatch_sync(dispatch_get_main_queue(), work);
	}

	return started;
}

void NeruStopHintSearchTextInput(void) {
	void (^work)(void) = ^{
		stopOnMainThread();
	};

	if ([NSThread isMainThread]) {
		work();
	} else {
		dispatch_sync(dispatch_get_main_queue(), work);
	}
}
