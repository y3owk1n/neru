//
//  eventtap.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "eventtap.h"
#import "keymap.h"
#import <Carbon/Carbon.h>
#import <os/lock.h>

#pragma mark - Type Definitions

typedef struct {
	CFMachPortRef eventTap;                          ///< Event tap reference
	CFRunLoopSourceRef runLoopSource;                ///< Run loop source
	EventTapCallback callback;                       ///< Callback function
	void *userData;                                  ///< User data pointer
	NSDictionary *__strong hotkeyLookup;             ///< Immutable hotkey lookup table: @(lookupKey) -> @YES
	NSArray<NSString *> *__strong hotkeyStrings;     ///< Raw hotkey strings for rebuild on layout change
	uint64_t hotkeyGeneration;                       ///< Generation counter for TOCTOU protection
	os_unfair_lock hotkeyLock;                       ///< Lightweight lock for hotkey lookup/strings/generation
	dispatch_block_t __strong pendingEnableBlock;    ///< Pending enable block (inner delayed block)
	dispatch_block_t __strong pendingAddSourceBlock; ///< Pending add source block
} EventTapContext;

/// Global event tap context for layout-change rebuild (single instance expected)
static EventTapContext *gEventTapContext = nil;

static inline NSUInteger hotkeyLookupKey(CGKeyCode keyCode, CGEventFlags flags) {
	uint8_t modifiers = 0;
	if (flags & kCGEventFlagMaskCommand)
		modifiers |= 1 << 0;
	if (flags & kCGEventFlagMaskShift)
		modifiers |= 1 << 1;
	if (flags & kCGEventFlagMaskAlternate)
		modifiers |= 1 << 2;
	if (flags & kCGEventFlagMaskControl)
		modifiers |= 1 << 3;
	return (keyCode << 4) | modifiers;
}

#pragma mark - Helper Functions

// Forward declaration for use in buildHotkeyLookupFromStrings
static BOOL parseHotkeyString(NSString *hotkeyString, CGKeyCode *outKeyCode, uint8_t *outModifiers);

/// Build a lookup dictionary from an array of hotkey strings.
/// Each valid hotkey is parsed and its packed (keyCode, modifiers) key is mapped to @YES.
static NSDictionary *buildHotkeyLookupFromStrings(NSArray<NSString *> *strings) {
	NSMutableDictionary *lookup = [[NSMutableDictionary alloc] initWithCapacity:strings.count];
	for (NSString *hotkeyString in strings) {
		CGKeyCode keyCode;
		uint8_t modifiers;
		if (parseHotkeyString(hotkeyString, &keyCode, &modifiers)) {
			NSUInteger lookupKey = hotkeyLookupKey(keyCode, (modifiers & (1 << 0) ? kCGEventFlagMaskCommand : 0) |
			                                                    (modifiers & (1 << 1) ? kCGEventFlagMaskShift : 0) |
			                                                    (modifiers & (1 << 2) ? kCGEventFlagMaskAlternate : 0) |
			                                                    (modifiers & (1 << 3) ? kCGEventFlagMaskControl : 0));
			lookup[@(lookupKey)] = @YES;
		}
	}
	return [lookup copy]; // return truly immutable NSDictionary
}
/// Rebuild the hotkey lookup table from stored hotkey strings.
/// Called after keyboard layout changes to re-resolve key names to keycodes.
/// Must be called on the main thread (same as handleKeyboardLayoutChanged).
static void rebuildEventTapHotkeyLookup(void) {
	EventTapContext *context = gEventTapContext;
	if (!context)
		return;

	NSArray<NSString *> *strings = nil;
	uint64_t snapshotGeneration = 0;

	os_unfair_lock_lock(&context->hotkeyLock);
	strings = context->hotkeyStrings;
	snapshotGeneration = context->hotkeyGeneration;
	os_unfair_lock_unlock(&context->hotkeyLock);

	if (!strings || strings.count == 0)
		return;

	// Build the new lookup outside the lock (expensive work)
	NSDictionary *newLookup = buildHotkeyLookupFromStrings(strings);

	// Swap the lookup table under the lock only if generation hasn't changed
	// (i.e., setEventTapHotkeys hasn't been called in the meantime).
	// Save old pointer so ARC releases it outside the lock.
	NSDictionary *oldLookup = nil;
	os_unfair_lock_lock(&context->hotkeyLock);
	if (context->hotkeyGeneration == snapshotGeneration) {
		oldLookup = context->hotkeyLookup;
		context->hotkeyLookup = newLookup;
	}
	os_unfair_lock_unlock(&context->hotkeyLock);
	oldLookup = nil; // ARC releases old dictionary here, outside the lock
}

static BOOL parseHotkeyString(NSString *hotkeyString, CGKeyCode *outKeyCode, uint8_t *outModifiers) {
	if (!hotkeyString || [hotkeyString length] == 0) {
		return NO;
	}

	*outKeyCode = 0xFFFF;
	*outModifiers = 0;

	@autoreleasepool {
		NSArray *parts = [hotkeyString componentsSeparatedByString:@"+"];
		NSString *mainKey = nil;
		BOOL needsCmd = NO, needsShift = NO, needsAlt = NO, needsCtrl = NO;

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

		CGKeyCode keyCode = keyNameToCode(mainKey);
		if (keyCode == 0xFFFF)
			return NO;

		uint8_t modifiers = 0;
		if (needsCmd)
			modifiers |= 1 << 0;
		if (needsShift)
			modifiers |= 1 << 1;
		if (needsAlt)
			modifiers |= 1 << 2;
		if (needsCtrl)
			modifiers |= 1 << 3;

		*outKeyCode = keyCode;
		*outModifiers = modifiers;
		return YES;
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
		// macOS disables the event tap if the callback takes too long.
		// Re-enable it automatically so key events keep flowing.
		if (type == kCGEventTapDisabledByTimeout) {
			CGEventTapEnable(context->eventTap, true);
			return event;
		}

		if (type == kCGEventKeyDown) {
			CGKeyCode keyCode = (CGKeyCode)CGEventGetIntegerValueField(event, kCGKeyboardEventKeycode);
			CGEventFlags flags = CGEventGetFlags(event);

			// Thread-safe hotkey check (O(1) lookup)
			// Uses os_unfair_lock for minimal latency on the event tap thread.
			// The lock protects reading the immutable hotkeyLookup pointer; the
			// dictionary itself is never mutated — writers swap in a new instance.
			BOOL isHotkey = NO;
			NSUInteger lookupKey = hotkeyLookupKey(keyCode, flags);
			os_unfair_lock_lock(&context->hotkeyLock);
			isHotkey = [context->hotkeyLookup[@(lookupKey)] boolValue];
			os_unfair_lock_unlock(&context->hotkeyLock);

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
						context->callback([fullKey UTF8String], context->userData);
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
						context->callback([fullKey UTF8String], context->userData);
					}
					return NULL;
				}
			}

			// Special handling for delete/backspace key (Shift+Delete handled in Shift-only block)
			if (keyCode == kKeyCodeDelete) {
				if (context->callback) {
					context->callback("delete", context->userData);
				}
				return NULL;
			}

			// Special handling for escape key (Shift+Escape handled in Shift-only block)
			if (keyCode == kKeyCodeEscape) {
				if (context->callback) {
					context->callback("escape", context->userData);
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
	EventTapContext *context = (EventTapContext *)calloc(1, sizeof(EventTapContext));
	if (!context)
		return NULL;

	context->callback = callback;
	context->userData = userData;

	// Initialize hotkey state with lightweight lock (no dispatch queue overhead)
	context->hotkeyLookup = [[NSDictionary alloc] init];
	context->hotkeyStrings = nil;
	context->hotkeyLock = OS_UNFAIR_LOCK_INIT;

	// Store global reference for layout-change rebuild.
	gEventTapContext = context;

	// Register for keyboard layout change notifications so the hotkey
	// lookup table is rebuilt when key names map to different keycodes.
	setKeymapLayoutChangeCallback(rebuildEventTapHotkeyLookup);

	context->pendingEnableBlock = nil;
	context->pendingAddSourceBlock = nil;

	// Set up event tap
	CGEventMask eventMask = (1 << kCGEventKeyDown);
	context->eventTap = CGEventTapCreate(kCGSessionEventTap, kCGHeadInsertEventTap, kCGEventTapOptionDefault, eventMask,
	                                     eventTapCallback, context);

	if (!context->eventTap) {
		gEventTapContext = nil;
		setKeymapLayoutChangeCallback(NULL);
		context->hotkeyLookup = nil;
		context->hotkeyStrings = nil;
		free(context);
		return NULL;
	}

	context->runLoopSource = CFMachPortCreateRunLoopSource(kCFAllocatorDefault, context->eventTap, 0);

	// Add to main run loop once during creation to avoid re-entry on enable
	if ([NSThread isMainThread]) {
		CFRunLoopAddSource(CFRunLoopGetMain(), context->runLoopSource, kCFRunLoopCommonModes);
	} else {
		__block dispatch_block_t block;
		block = dispatch_block_create(0, ^{
			// Guard against execution after cancellation (e.g., if destroyEventTap
			// cancelled this block but it was already dequeued for execution).
			if (dispatch_block_testcancel(block)) {
				block = nil; // Break retain cycle
				return;
			}

			CFRunLoopAddSource(CFRunLoopGetMain(), context->runLoopSource, kCFRunLoopCommonModes);
			context->pendingAddSourceBlock = nil;
			block = nil; // Break retain cycle
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
		NSMutableArray<NSString *> *newStrings = [NSMutableArray arrayWithCapacity:count];

		for (int i = 0; i < count; i++) {
			if (hotkeys[i] && strlen(hotkeys[i]) > 0) {
				[newStrings addObject:[NSString stringWithUTF8String:hotkeys[i]]];
			}
		}

		NSDictionary *newLookup = buildHotkeyLookupFromStrings(newStrings);

		NSArray<NSString *> *copiedStrings = [newStrings copy];

		// Thread-safe replacement using os_unfair_lock.
		// Save old pointers so ARC releases them outside the lock —
		// deallocation of a large dictionary must not block the event tap callback.
		NSDictionary *oldLookup;
		NSArray *oldStrings;
		os_unfair_lock_lock(&context->hotkeyLock);
		oldStrings = context->hotkeyStrings;
		oldLookup = context->hotkeyLookup;
		context->hotkeyStrings = copiedStrings;
		context->hotkeyGeneration++; // invalidate any in-flight rebuild
		context->hotkeyLookup = newLookup;
		os_unfair_lock_unlock(&context->hotkeyLock);
		// ARC releases old objects here, outside the lock
		oldLookup = nil;
		oldStrings = nil;
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
		__block dispatch_block_t innerBlock;
		innerBlock = dispatch_block_create(0, ^{
			// Guard against execution after cancellation
			if (dispatch_block_testcancel(innerBlock)) {
				innerBlock = nil; // Break retain cycle
				return;
			}

			CGEventTapEnable(context->eventTap, true);
			context->pendingEnableBlock = nil;
			innerBlock = nil; // Break retain cycle
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

		// Clear global reference before freeing
		if (gEventTapContext == context) {
			gEventTapContext = nil;
			setKeymapLayoutChangeCallback(NULL);
		}

		// Synchronize with any in-flight event tap callback that may be
		// holding hotkeyLock on the Mach port thread.  Acquiring the lock
		// here acts as a barrier: once we hold it, we know the callback
		// has left its critical section.  We then nil out the ARC fields
		// under the lock so the callback cannot retain a dangling pointer,
		// and release the lock before freeing the struct.
		NSDictionary *oldLookup;
		NSArray *oldStrings;
		os_unfair_lock_lock(&context->hotkeyLock);
		oldLookup = context->hotkeyLookup;
		oldStrings = context->hotkeyStrings;
		context->hotkeyLookup = nil;
		context->hotkeyStrings = nil;
		os_unfair_lock_unlock(&context->hotkeyLock);
		// ARC releases old objects outside the lock
		oldLookup = nil;
		oldStrings = nil;

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
