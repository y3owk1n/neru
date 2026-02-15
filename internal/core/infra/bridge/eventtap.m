//
//  eventtap.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "eventtap.h"
#import "keymap.h"
#import <Carbon/Carbon.h>

#pragma mark - Type Definitions

typedef struct {
	CFMachPortRef eventTap;                 ///< Event tap reference
	CFRunLoopSourceRef runLoopSource;       ///< Run loop source
	EventTapCallback callback;              ///< Callback function
	void *userData;                         ///< User data pointer
	NSMutableArray *hotkeys;                ///< Hotkeys array
	dispatch_queue_t accessQueue;           ///< Thread-safe access queue
	dispatch_block_t pendingEnableBlock;    ///< Pending enable block (inner delayed block)
	dispatch_block_t pendingAddSourceBlock; ///< Pending add source block
} EventTapContext;

#pragma mark - Helper Functions

/// Helper function to check if current key combination matches a hotkey
/// @param keyCode Key code
/// @param flags Event flags
/// @param hotkeyString Hotkey string
/// @return YES if matches, NO otherwise
BOOL isHotkeyMatch(CGKeyCode keyCode, CGEventFlags flags, NSString *hotkeyString) {
	if (!hotkeyString || [hotkeyString length] == 0) {
		return NO;
	}

	@autoreleasepool {
		NSArray *parts = [hotkeyString componentsSeparatedByString:@"+"];
		NSString *mainKey = nil;
		BOOL needsCmd = NO, needsShift = NO, needsAlt = NO, needsCtrl = NO;

		// Parse hotkey string
		for (NSString *part in parts) {
			NSString *trimmed = [part stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];

			if ([trimmed isEqualToString:@"Cmd"] || [trimmed isEqualToString:@"Command"]) {
				needsCmd = YES;
			} else if ([trimmed isEqualToString:@"Shift"]) {
				needsShift = YES;
			} else if ([trimmed isEqualToString:@"Alt"] || [trimmed isEqualToString:@"Option"]) {
				needsAlt = YES;
			} else if ([trimmed isEqualToString:@"Ctrl"] || [trimmed isEqualToString:@"Control"]) {
				needsCtrl = YES;
			} else {
				mainKey = trimmed;
			}
		}

		if (!mainKey)
			return NO;

		// Check modifier flags
		BOOL hasCmd = (flags & kCGEventFlagMaskCommand) != 0;
		BOOL hasShift = (flags & kCGEventFlagMaskShift) != 0;
		BOOL hasAlt = (flags & kCGEventFlagMaskAlternate) != 0;
		BOOL hasCtrl = (flags & kCGEventFlagMaskControl) != 0;

		if (needsCmd != hasCmd || needsShift != hasShift || needsAlt != hasAlt || needsCtrl != hasCtrl) {
			return NO;
		}

		// Map key names to key codes using shared keymap
		CGKeyCode expectedKeyCode = keyNameToCode(mainKey);
		return expectedKeyCode != 0xFFFF && expectedKeyCode == keyCode;
	}
}

#pragma mark - Event Tap Callback

/// Event tap callback function
/// @param proxy Event tap proxy
/// @param type Event type
/// @param event Event reference
/// @param refcon Reference context
/// @return Event reference or NULL
CGEventRef eventTapCallback(CGEventTapProxy proxy, CGEventType type, CGEventRef event, void *refcon) {
	EventTapContext *context = (EventTapContext *)refcon;
	if (!context)
		return event;

	@autoreleasepool {
		if (type == kCGEventKeyDown) {
			CGKeyCode keyCode = (CGKeyCode)CGEventGetIntegerValueField(event, kCGKeyboardEventKeycode);
			CGEventFlags flags = CGEventGetFlags(event);

			// Thread-safe hotkey check
			__block BOOL isHotkey = NO;
			dispatch_sync(context->accessQueue, ^{
				for (NSString *hotkeyString in context->hotkeys) {
					if (isHotkeyMatch(keyCode, flags, hotkeyString)) {
						isHotkey = YES;
						break;
					}
				}
			});

			// If this is a registered hotkey, let it pass through
			if (isHotkey) {
				return event;
			}

			// Check for modifiers (Shift alone is handled separately; Shift+Cmd/Alt/Ctrl is included in string)
			BOOL hasCmd = (flags & kCGEventFlagMaskCommand) != 0;
			BOOL hasShift = (flags & kCGEventFlagMaskShift) != 0;
			BOOL hasAlt = (flags & kCGEventFlagMaskAlternate) != 0;
			BOOL hasCtrl = (flags & kCGEventFlagMaskControl) != 0;

			// If there are modifiers (Cmd, Alt, Ctrl), construct a modifier key name
			if (hasCmd || hasAlt || hasCtrl) {
				NSString *keyName = keyCodeToName(keyCode);
				if (keyName) {
					NSMutableString *fullKey = [NSMutableString string];

					if (hasCmd)
						[fullKey appendString:@"Cmd+"];
					if (hasShift)
						[fullKey appendString:@"Shift+"];
					if (hasAlt)
						[fullKey appendString:@"Alt+"];
					if (hasCtrl)
						[fullKey appendString:@"Ctrl+"];

					[fullKey appendString:keyName];

					if (context->callback) {
						context->callback([fullKey UTF8String], context -> userData);
					}
					return NULL;
				}
			}

			// Handle Shift+Letter for direct action matching (before Unicode translation)
			if (hasShift && !hasCmd && !hasAlt && !hasCtrl) {
				NSString *keyName = keyCodeToName(keyCode);
				if (keyName) {
					NSMutableString *fullKey = [NSMutableString stringWithString:@"Shift+"];
					[fullKey appendString:keyName];

					if (context->callback) {
						context->callback([fullKey UTF8String], context -> userData);
					}
					return NULL;
				}
			}

			// Special handling for delete/backspace key (Shift+Delete handled in Shift-only block)
			if (keyCode == kKeyCodeDelete) {
				if (context->callback) {
					context->callback("\x7f", context->userData);
				}
				return NULL;
			}

			// Special handling for escape key (Shift+Escape handled in Shift-only block)
			if (keyCode == kKeyCodeEscape) {
				if (context->callback) {
					context->callback("\x1b", context->userData);
				}
				return NULL;
			}

			// Handle arrow keys and special keys using lookup table
			// Note: Shift+Arrow is handled in Shift-only block since keyCodeToName returns non-nil for these
			{
				static const struct {
					CGKeyCode code;
					const char *name;
				} specialKeys[] = {
				    {kKeyCodeUp, "Up"},       {kKeyCodeDown, "Down"},     {kKeyCodeLeft, "Left"},
				    {kKeyCodeRight, "Right"}, {kKeyCodePageUp, "PageUp"}, {kKeyCodePageDown, "PageDown"},
				    {kKeyCodeHome, "Home"},   {kKeyCodeEnd, "End"},
				};

				for (size_t i = 0; i < sizeof(specialKeys) / sizeof(specialKeys[0]); i++) {
					if (keyCode == specialKeys[i].code) {
						if (context->callback) {
							context->callback(specialKeys[i].name, context->userData);
						}
						return NULL;
					}
				}
			}

			// Map key code to character using current keyboard layout (with US QWERTY fallback)
			// Uses UCKeyTranslate to respect the active keyboard layout while bypassing input methods
			NSString *keyChar = keyCodeToCharacter(keyCode, flags);
			if (keyChar && context->callback) {
				const char *keyCString = [keyChar UTF8String];
				if (keyCString) {
					context->callback(keyCString, context->userData);
				}

				// Consume the event (don't pass it through)
				return NULL;
			}

			// Unknown key code, pass to system
			return event;
		}

		return event;
	}
}

#pragma mark - Event Tap Functions

/// Create event tap
/// @param callback Callback function
/// @param userData User data pointer
/// @return Event tap handle
EventTap createEventTap(EventTapCallback callback, void *userData) {
	EventTapContext *context = (EventTapContext *)malloc(sizeof(EventTapContext));
	if (!context)
		return NULL;

	context->callback = callback;
	context->userData = userData;

	// Initialize with ARC-compatible array
	context->hotkeys = [[NSMutableArray alloc] init];
	context->accessQueue = dispatch_queue_create("com.neru.eventtap", DISPATCH_QUEUE_SERIAL);
	context->pendingEnableBlock = nil;
	context->pendingAddSourceBlock = nil;

	// Set up event tap
	CGEventMask eventMask = (1 << kCGEventKeyDown);
	context->eventTap = CGEventTapCreate(kCGSessionEventTap, kCGHeadInsertEventTap, kCGEventTapOptionDefault, eventMask,
	                                     eventTapCallback, context);

	if (!context->eventTap) {
		context->hotkeys = nil;
		context->accessQueue = nil;
		free(context);
		return NULL;
	}

	context->runLoopSource = CFMachPortCreateRunLoopSource(kCFAllocatorDefault, context->eventTap, 0);

	// Add to main run loop once during creation to avoid re-entry on enable
	if ([NSThread isMainThread]) {
		CFRunLoopAddSource(CFRunLoopGetMain(), context->runLoopSource, kCFRunLoopCommonModes);
	} else {
		dispatch_block_t block = dispatch_block_create(0, ^{
			CFRunLoopAddSource(CFRunLoopGetMain(), context->runLoopSource, kCFRunLoopCommonModes);
			context->pendingAddSourceBlock = nil;
		});

		context->pendingAddSourceBlock = block;
		dispatch_async(dispatch_get_main_queue(), block);
	}

	return (EventTap)context;
}

/// Set event tap hotkeys
/// @param tap Event tap handle
/// @param hotkeys Array of hotkey strings
/// @param count Number of hotkeys
void setEventTapHotkeys(EventTap tap, const char **hotkeys, int count) {
	if (!tap)
		return;
	EventTapContext *context = (EventTapContext *)tap;

	@autoreleasepool {
		NSMutableArray *newHotkeys = [NSMutableArray arrayWithCapacity:count];

		for (int i = 0; i < count; i++) {
			if (hotkeys[i] && strlen(hotkeys[i]) > 0) {
				NSString *hotkeyString = [NSString stringWithUTF8String:hotkeys[i]];
				[newHotkeys addObject:hotkeyString];
			}
		}

		// Thread-safe replacement
		dispatch_sync(context->accessQueue, ^{
			[context->hotkeys removeAllObjects];
			[context->hotkeys addObjectsFromArray:newHotkeys];
		});
	}
}

/// Enable event tap
/// @param tap Event tap handle
void enableEventTap(EventTap tap) {
	if (!tap)
		return;

	EventTapContext *context = (EventTapContext *)tap;

	dispatch_async(dispatch_get_main_queue(), ^{
		// Cancel any existing pending inner block
		if (context->pendingEnableBlock) {
			dispatch_block_cancel(context->pendingEnableBlock);
			context->pendingEnableBlock = nil;
		}

		// Create delayed enable block
		dispatch_block_t innerBlock = dispatch_block_create(0, ^{
			CGEventTapEnable(context->eventTap, true);
			context->pendingEnableBlock = nil;
		});

		context->pendingEnableBlock = innerBlock;
		dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(0.15 * NSEC_PER_SEC)), dispatch_get_main_queue(),
		               innerBlock);
	});
}

/// Disable event tap
/// @param tap Event tap handle
void disableEventTap(EventTap tap) {
	if (!tap)
		return;

	EventTapContext *context = (EventTapContext *)tap;

	// Disable on main thread to avoid races
	dispatch_async(dispatch_get_main_queue(), ^{
		CGEventTapEnable(context->eventTap, false);
	});
}

/// Destroy event tap
/// @param tap Event tap handle
void destroyEventTap(EventTap tap) {
	if (!tap)
		return;

	EventTapContext *context = (EventTapContext *)tap;

	// Helper block for cleanup tasks that must run on main thread
	void (^cleanupBlock)(void) = ^{
		if (context->eventTap) {
			CGEventTapEnable(context->eventTap, false);
		}
		if (context->runLoopSource) {
			CFRunLoopRemoveSource(CFRunLoopGetMain(), context->runLoopSource, kCFRunLoopCommonModes);
		}
		// Cancel any pending enable block
		if (context->pendingEnableBlock) {
			dispatch_block_cancel(context->pendingEnableBlock);
			context->pendingEnableBlock = nil;
		}
		// Cancel any pending add source block
		if (context->pendingAddSourceBlock) {
			dispatch_block_cancel(context->pendingAddSourceBlock);
			context->pendingAddSourceBlock = nil;
		}

		// Clean up resources
		if (context->eventTap) {
			CFRelease(context->eventTap);
			context->eventTap = NULL;
		}

		if (context->runLoopSource) {
			CFRelease(context->runLoopSource);
			context->runLoopSource = NULL;
		}

		// Clean up hotkeys and queue
		context->hotkeys = nil;               // ARC will handle deallocation
		context->accessQueue = nil;           // ARC will handle deallocation
		context->pendingEnableBlock = nil;    // ARC will handle deallocation
		context->pendingAddSourceBlock = nil; // ARC will handle deallocation

		free(context);
	};

	// Execute cleanup on main thread
	if ([NSThread isMainThread]) {
		cleanupBlock();
	} else {
		dispatch_async(dispatch_get_main_queue(), cleanupBlock);
	}
}
