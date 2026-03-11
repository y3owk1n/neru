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
	CFMachPortRef eventTap;                                   ///< Event tap reference
	CFRunLoopSourceRef runLoopSource;                         ///< Run loop source
	EventTapCallback callback;                                ///< Callback function
	EventTapPassthroughCallback passthroughCallback;          ///< Called when a modifier shortcut passes through
	void *userData;                                           ///< User data pointer
	NSDictionary *__strong hotkeyLookup;                      ///< Immutable hotkey lookup table: @(lookupKey) -> @YES
	NSArray<NSString *> *__strong hotkeyStrings;              ///< Raw hotkey strings for rebuild on layout change
	uint64_t hotkeyGeneration;                                ///< Generation counter for TOCTOU protection
	os_unfair_lock hotkeyLock;                                ///< Lightweight lock for hotkey lookup/strings/generation
	NSDictionary *__strong interceptedModifierLookup;         ///< Modifier shortcuts Neru still consumes
	NSArray<NSString *> *__strong interceptedModifierStrings; ///< Raw modifier shortcut strings
	uint64_t interceptedModifierGeneration;                   ///< Generation counter for layout rebuild
	os_unfair_lock interceptedModifierLock;                 ///< Lock for intercepted modifier lookup/strings/generation
	NSDictionary *__strong modifierBlacklistLookup;         ///< Blacklisted modifier shortcuts
	NSArray<NSString *> *__strong modifierBlacklistStrings; ///< Raw blacklist strings
	uint64_t modifierBlacklistGeneration;                   ///< Generation counter for layout rebuild
	BOOL passthroughUnboundedModifiers;                     ///< Whether unbound modifier shortcuts reach macOS
	os_unfair_lock modifierPassthroughLock;                 ///< Lock for modifier passthrough config
	dispatch_block_t __strong pendingAddSourceBlock;        ///< Pending add source block
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

// Forward declaration for use in buildKeyLookupFromStrings
static BOOL parseHotkeyString(NSString *hotkeyString, CGKeyCode *outKeyCode, uint8_t *outModifiers);

/// Build a lookup dictionary from an array of key strings.
/// Each valid key is parsed and its packed (keyCode, modifiers) key is mapped to @YES.
static NSDictionary *buildKeyLookupFromStrings(NSArray<NSString *> *strings) {
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

/// Rebuild the lookup tables that depend on keyboard layout translation.
/// Called after keyboard layout changes to re-resolve key names to keycodes.
/// Must be called on the main thread (same as handleKeyboardLayoutChanged).
static void rebuildEventTapLookups(void) {
	EventTapContext *context = gEventTapContext;
	if (!context)
		return;

	NSArray<NSString *> *strings = nil;
	uint64_t snapshotGeneration = 0;

	os_unfair_lock_lock(&context->hotkeyLock);
	strings = context->hotkeyStrings;
	snapshotGeneration = context->hotkeyGeneration;
	os_unfair_lock_unlock(&context->hotkeyLock);

	if (strings && strings.count > 0) {
		// Build the new lookup outside the lock (expensive work)
		NSDictionary *newLookup = buildKeyLookupFromStrings(strings);

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

	NSArray<NSString *> *modifierStrings = nil;
	uint64_t modifierGeneration = 0;

	os_unfair_lock_lock(&context->interceptedModifierLock);
	modifierStrings = context->interceptedModifierStrings;
	modifierGeneration = context->interceptedModifierGeneration;
	os_unfair_lock_unlock(&context->interceptedModifierLock);

	if (modifierStrings && modifierStrings.count > 0) {
		NSDictionary *newModifierLookup = buildKeyLookupFromStrings(modifierStrings);
		NSDictionary *oldModifierLookup = nil;
		os_unfair_lock_lock(&context->interceptedModifierLock);
		if (context->interceptedModifierGeneration == modifierGeneration) {
			oldModifierLookup = context->interceptedModifierLookup;
			context->interceptedModifierLookup = newModifierLookup;
		}
		os_unfair_lock_unlock(&context->interceptedModifierLock);
		oldModifierLookup = nil;
	}

	NSArray<NSString *> *blacklistStrings = nil;
	uint64_t blacklistGeneration = 0;

	os_unfair_lock_lock(&context->modifierPassthroughLock);
	blacklistStrings = context->modifierBlacklistStrings;
	blacklistGeneration = context->modifierBlacklistGeneration;
	os_unfair_lock_unlock(&context->modifierPassthroughLock);

	if (!blacklistStrings || blacklistStrings.count == 0)
		return;

	NSDictionary *newBlacklistLookup = buildKeyLookupFromStrings(blacklistStrings);
	NSDictionary *oldBlacklistLookup = nil;
	os_unfair_lock_lock(&context->modifierPassthroughLock);
	if (context->modifierBlacklistGeneration == blacklistGeneration) {
		oldBlacklistLookup = context->modifierBlacklistLookup;
		context->modifierBlacklistLookup = newBlacklistLookup;
	}
	os_unfair_lock_unlock(&context->modifierPassthroughLock);
	oldBlacklistLookup = nil;
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

static NSString *specialKeyName(CGKeyCode keyCode) {
	switch (keyCode) {
	case kKeyCodeSpace:
		return @"Space";
	case kKeyCodeReturn:
		return @"Return";
	case kKeyCodeEscape:
		return @"Escape";
	case kKeyCodeTab:
		return @"Tab";
	case kKeyCodeDelete:
		return @"Delete";
	case kKeyCodeUp:
		return @"Up";
	case kKeyCodeDown:
		return @"Down";
	case kKeyCodeLeft:
		return @"Left";
	case kKeyCodeRight:
		return @"Right";
	case kKeyCodePageUp:
		return @"PageUp";
	case kKeyCodePageDown:
		return @"PageDown";
	case kKeyCodeHome:
		return @"Home";
	case kKeyCodeEnd:
		return @"End";
	case kKeyCodeF1:
		return @"F1";
	case kKeyCodeF2:
		return @"F2";
	case kKeyCodeF3:
		return @"F3";
	case kKeyCodeF4:
		return @"F4";
	case kKeyCodeF5:
		return @"F5";
	case kKeyCodeF6:
		return @"F6";
	case kKeyCodeF7:
		return @"F7";
	case kKeyCodeF8:
		return @"F8";
	case kKeyCodeF9:
		return @"F9";
	case kKeyCodeF10:
		return @"F10";
	case kKeyCodeF11:
		return @"F11";
	case kKeyCodeF12:
		return @"F12";
	case kKeyCodeF13:
		return @"F13";
	case kKeyCodeF14:
		return @"F14";
	case kKeyCodeF15:
		return @"F15";
	case kKeyCodeF16:
		return @"F16";
	case kKeyCodeF17:
		return @"F17";
	case kKeyCodeF18:
		return @"F18";
	case kKeyCodeF19:
		return @"F19";
	case kKeyCodeF20:
		return @"F20";
	default:
		return nil;
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
			// The lock only protects snapshotting the immutable hotkeyLookup
			// pointer; the dictionary lookup runs outside the critical section
			// since the dictionary is never mutated — writers swap in a new instance.
			NSUInteger lookupKey = hotkeyLookupKey(keyCode, flags);
			NSDictionary *lookup;
			os_unfair_lock_lock(&context->hotkeyLock);
			lookup = context->hotkeyLookup;
			os_unfair_lock_unlock(&context->hotkeyLock);
			BOOL isHotkey = [lookup[@(lookupKey)] boolValue];

			// If this is a registered hotkey, let it pass through
			if (isHotkey) {
				return event;
			}

			// Check for modifiers (Shift alone is handled separately; Shift+Cmd/Alt/Ctrl is included in string)
			BOOL hasCmd = (flags & kCGEventFlagMaskCommand) != 0;
			BOOL hasShift = (flags & kCGEventFlagMaskShift) != 0;
			BOOL hasAlt = (flags & kCGEventFlagMaskAlternate) != 0;
			BOOL hasCtrl = (flags & kCGEventFlagMaskControl) != 0;

			if (hasCmd || hasAlt || hasCtrl) {
				BOOL passthroughEnabled = NO;
				NSDictionary *blacklistLookup = nil;
				EventTapPassthroughCallback ptCallback = NULL;
				os_unfair_lock_lock(&context->modifierPassthroughLock);
				passthroughEnabled = context->passthroughUnboundedModifiers;
				blacklistLookup = context->modifierBlacklistLookup;
				ptCallback = context->passthroughCallback;
				os_unfair_lock_unlock(&context->modifierPassthroughLock);

				NSDictionary *interceptedLookup = nil;
				os_unfair_lock_lock(&context->interceptedModifierLock);
				interceptedLookup = context->interceptedModifierLookup;
				os_unfair_lock_unlock(&context->interceptedModifierLock);

				BOOL isIntercepted = interceptedLookup != nil && [interceptedLookup[@(lookupKey)] boolValue];
				BOOL isBlacklisted = blacklistLookup != nil && [blacklistLookup[@(lookupKey)] boolValue];

				if (passthroughEnabled && !isIntercepted && !isBlacklisted) {
					// Notify Go that a modifier shortcut was passed through so
					// the active mode can decide whether to refresh (e.g., hints
					// mode re-collects AX elements after Cmd+Tab).
					if (ptCallback) {
						ptCallback(context->userData);
					}

					return event;
				}

				NSString *keyName = keyCodeToName(keyCode);
				if (!keyName) {
					keyName = specialKeyName(keyCode);
				}

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
				if (!keyName) {
					keyName = specialKeyName(keyCode);
				}

				if (keyName) {
					NSMutableString *fullKey = [NSMutableString stringWithString:@"Shift+"];
					[fullKey appendString:keyName];

					if (context->callback) {
						context->callback([fullKey UTF8String], context->userData);
					}
					return NULL;
				}
			}

			NSString *namedKey = specialKeyName(keyCode);
			if (namedKey && context->callback) {
				context->callback([namedKey UTF8String], context->userData);
				return NULL;
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
	context->interceptedModifierLookup = [[NSDictionary alloc] init];
	context->interceptedModifierStrings = nil;
	context->interceptedModifierLock = OS_UNFAIR_LOCK_INIT;
	context->modifierBlacklistLookup = [[NSDictionary alloc] init];
	context->modifierBlacklistStrings = nil;
	context->modifierPassthroughLock = OS_UNFAIR_LOCK_INIT;
	context->passthroughUnboundedModifiers = NO;

	// Store global reference for layout-change rebuild.
	gEventTapContext = context;

	// Register for keyboard layout change notifications so all key lookups are
	// rebuilt when key names map to different keycodes.
	setKeymapLayoutChangeCallback(rebuildEventTapLookups);

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
		context->interceptedModifierLookup = nil;
		context->interceptedModifierStrings = nil;
		context->modifierBlacklistLookup = nil;
		context->modifierBlacklistStrings = nil;
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

		NSDictionary *newLookup = buildKeyLookupFromStrings(newStrings);

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

/// Set modifier passthrough behavior for unbound Cmd/Ctrl/Alt shortcuts.
/// @param tap Event tap handle
/// @param enabled Non-zero to enable passthrough
/// @param blacklistKeys Array of blacklisted modifier shortcuts
/// @param count Number of blacklisted keys
void setEventTapModifierPassthrough(EventTap tap, int enabled, const char **blacklistKeys, int count) {
	if (!tap)
		return;
	EventTapContext *context = (EventTapContext *)tap;

	@autoreleasepool {
		NSMutableArray<NSString *> *newStrings = [NSMutableArray arrayWithCapacity:count];

		for (int i = 0; i < count; i++) {
			if (blacklistKeys[i] && strlen(blacklistKeys[i]) > 0) {
				[newStrings addObject:[NSString stringWithUTF8String:blacklistKeys[i]]];
			}
		}

		NSDictionary *newLookup = buildKeyLookupFromStrings(newStrings);
		NSArray<NSString *> *copiedStrings = [newStrings copy];

		NSDictionary *oldLookup;
		NSArray *oldStrings;
		os_unfair_lock_lock(&context->modifierPassthroughLock);
		oldStrings = context->modifierBlacklistStrings;
		oldLookup = context->modifierBlacklistLookup;
		context->modifierBlacklistStrings = copiedStrings;
		context->modifierBlacklistGeneration++;
		context->modifierBlacklistLookup = newLookup;
		context->passthroughUnboundedModifiers = enabled != 0;
		os_unfair_lock_unlock(&context->modifierPassthroughLock);
		oldLookup = nil;
		oldStrings = nil;
	}
}

/// Set modifier shortcuts that the active mode still wants Neru to consume.
/// @param tap Event tap handle
/// @param keys Array of key strings
/// @param count Number of keys
void setEventTapInterceptedModifierKeys(EventTap tap, const char **keys, int count) {
	if (!tap)
		return;
	EventTapContext *context = (EventTapContext *)tap;

	@autoreleasepool {
		NSMutableArray<NSString *> *newStrings = [NSMutableArray arrayWithCapacity:count];

		for (int i = 0; i < count; i++) {
			if (keys[i] && strlen(keys[i]) > 0) {
				[newStrings addObject:[NSString stringWithUTF8String:keys[i]]];
			}
		}

		NSDictionary *newLookup = buildKeyLookupFromStrings(newStrings);
		NSArray<NSString *> *copiedStrings = [newStrings copy];

		NSDictionary *oldLookup;
		NSArray *oldStrings;
		os_unfair_lock_lock(&context->interceptedModifierLock);
		oldStrings = context->interceptedModifierStrings;
		oldLookup = context->interceptedModifierLookup;
		context->interceptedModifierStrings = copiedStrings;
		context->interceptedModifierGeneration++;
		context->interceptedModifierLookup = newLookup;
		os_unfair_lock_unlock(&context->interceptedModifierLock);
		oldLookup = nil;
		oldStrings = nil;
	}
}

/// Set callback invoked when a modifier shortcut passes through to macOS.
/// @param tap Event tap handle
/// @param callback Passthrough callback function (may be NULL to clear)
void setEventTapPassthroughCallback(EventTap tap, EventTapPassthroughCallback callback) {
	if (!tap)
		return;
	EventTapContext *context = (EventTapContext *)tap;
	os_unfair_lock_lock(&context->modifierPassthroughLock);
	context->passthroughCallback = callback;
	os_unfair_lock_unlock(&context->modifierPassthroughLock);
}

/// Enable event tap
/// @param tap Event tap handle
void enableEventTap(EventTap tap) {
	if (!tap)
		return;

	EventTapContext *context = (EventTapContext *)tap;

	dispatch_async(dispatch_get_main_queue(), ^{
		// Enable immediately — the event tap callback already lets registered
		// hotkeys pass through (isHotkey check), so there is no need to delay.
		// Removing the previous 150ms dispatch_after eliminates a dead window
		// where key events are silently dropped right after mode activation.
		CGEventTapEnable(context->eventTap, true);
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

		NSDictionary *oldInterceptedLookup;
		NSArray *oldInterceptedStrings;
		os_unfair_lock_lock(&context->interceptedModifierLock);
		oldInterceptedLookup = context->interceptedModifierLookup;
		oldInterceptedStrings = context->interceptedModifierStrings;
		context->interceptedModifierLookup = nil;
		context->interceptedModifierStrings = nil;
		os_unfair_lock_unlock(&context->interceptedModifierLock);
		oldInterceptedLookup = nil;
		oldInterceptedStrings = nil;

		NSDictionary *oldBlacklistLookup;
		NSArray *oldBlacklistStrings;
		os_unfair_lock_lock(&context->modifierPassthroughLock);
		oldBlacklistLookup = context->modifierBlacklistLookup;
		oldBlacklistStrings = context->modifierBlacklistStrings;
		context->modifierBlacklistLookup = nil;
		context->modifierBlacklistStrings = nil;
		context->passthroughUnboundedModifiers = NO;
		context->passthroughCallback = NULL;
		os_unfair_lock_unlock(&context->modifierPassthroughLock);
		oldBlacklistLookup = nil;
		oldBlacklistStrings = nil;

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
