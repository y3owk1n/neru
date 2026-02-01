//
//  eventtap.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "eventtap.h"
#import <Carbon/Carbon.h>

#pragma mark - Type Definitions

typedef struct {
	CFMachPortRef eventTap;              ///< Event tap reference
	CFRunLoopSourceRef runLoopSource;    ///< Run loop source
	EventTapCallback callback;           ///< Callback function
	void *userData;                      ///< User data pointer
	NSMutableArray *hotkeys;             ///< Hotkeys array
	dispatch_queue_t accessQueue;        ///< Thread-safe access queue
	dispatch_block_t pendingEnableBlock; ///< Pending enable block
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

		// Map key names to key codes (same as in hotkeys.m)
		NSDictionary *keyMap = @{
			@"Space" : @(49),
			@"Return" : @(36),
			@"Enter" : @(36),
			@"Escape" : @(53),
			@"Tab" : @(48),
			@"Delete" : @(51),
			@"Backspace" : @(51),

			// Letters
			@"A" : @(0),
			@"B" : @(11),
			@"C" : @(8),
			@"D" : @(2),
			@"E" : @(14),
			@"F" : @(3),
			@"G" : @(5),
			@"H" : @(4),
			@"I" : @(34),
			@"J" : @(38),
			@"K" : @(40),
			@"L" : @(37),
			@"M" : @(46),
			@"N" : @(45),
			@"O" : @(31),
			@"P" : @(35),
			@"Q" : @(12),
			@"R" : @(15),
			@"S" : @(1),
			@"T" : @(17),
			@"U" : @(32),
			@"V" : @(9),
			@"W" : @(13),
			@"X" : @(7),
			@"Y" : @(16),
			@"Z" : @(6),

			// Numbers
			@"0" : @(29),
			@"1" : @(18),
			@"2" : @(19),
			@"3" : @(20),
			@"4" : @(21),
			@"5" : @(23),
			@"6" : @(22),
			@"7" : @(26),
			@"8" : @(28),
			@"9" : @(25),

			// Function keys
			@"F1" : @(122),
			@"F2" : @(120),
			@"F3" : @(99),
			@"F4" : @(118),
			@"F5" : @(96),
			@"F6" : @(97),
			@"F7" : @(98),
			@"F8" : @(100),
			@"F9" : @(101),
			@"F10" : @(109),
			@"F11" : @(103),
			@"F12" : @(111),

			// Arrow keys
			@"Left" : @(123),
			@"Right" : @(124),
			@"Down" : @(125),
			@"Up" : @(126),
		};

		NSNumber *expectedKeyCode = keyMap[mainKey];
		if (!expectedKeyCode) {
			// Try uppercase version (fixed from lowercase)
			expectedKeyCode = keyMap[[mainKey uppercaseString]];
		}

		return expectedKeyCode && [expectedKeyCode intValue] == keyCode;
	}
}

/// Map keycode to character (US QWERTY layout, lowercase for letters)
/// This function bypasses input methods and provides direct keycode-to-character mapping
NSString *keyCodeToCharacter(CGKeyCode keyCode, CGEventFlags flags) {
	BOOL hasShift = (flags & kCGEventFlagMaskShift) != 0;
	BOOL hasCapsLock = (flags & kCGEventFlagMaskAlphaShift) != 0;

	// Determine if we should output uppercase
	BOOL uppercase = hasShift != hasCapsLock; // XOR: shift and capslock cancel each other for letters

	NSString *charStr = nil;

	switch (keyCode) {
	// Letters
	case 0:
		charStr = uppercase ? @"A" : @"a";
		break;
	case 1:
		charStr = uppercase ? @"S" : @"s";
		break;
	case 2:
		charStr = uppercase ? @"D" : @"d";
		break;
	case 3:
		charStr = uppercase ? @"F" : @"f";
		break;
	case 4:
		charStr = uppercase ? @"H" : @"h";
		break;
	case 5:
		charStr = uppercase ? @"G" : @"g";
		break;
	case 6:
		charStr = uppercase ? @"Z" : @"z";
		break;
	case 7:
		charStr = uppercase ? @"X" : @"x";
		break;
	case 8:
		charStr = uppercase ? @"C" : @"c";
		break;
	case 9:
		charStr = uppercase ? @"V" : @"v";
		break;
	case 11:
		charStr = uppercase ? @"B" : @"b";
		break;
	case 12:
		charStr = uppercase ? @"Q" : @"q";
		break;
	case 13:
		charStr = uppercase ? @"W" : @"w";
		break;
	case 14:
		charStr = uppercase ? @"E" : @"e";
		break;
	case 15:
		charStr = uppercase ? @"R" : @"r";
		break;
	case 16:
		charStr = uppercase ? @"Y" : @"y";
		break;
	case 17:
		charStr = uppercase ? @"T" : @"t";
		break;
	case 31:
		charStr = uppercase ? @"O" : @"o";
		break;
	case 32:
		charStr = uppercase ? @"U" : @"u";
		break;
	case 34:
		charStr = uppercase ? @"I" : @"i";
		break;
	case 35:
		charStr = uppercase ? @"P" : @"p";
		break;
	case 37:
		charStr = uppercase ? @"L" : @"l";
		break;
	case 38:
		charStr = uppercase ? @"J" : @"j";
		break;
	case 40:
		charStr = uppercase ? @"K" : @"k";
		break;
	case 45:
		charStr = uppercase ? @"N" : @"n";
		break;
	case 46:
		charStr = uppercase ? @"M" : @"m";
		break;

	// Numbers (shifted symbols)
	case 18:
		charStr = hasShift ? @"!" : @"1";
		break;
	case 19:
		charStr = hasShift ? @"@" : @"2";
		break;
	case 20:
		charStr = hasShift ? @"#" : @"3";
		break;
	case 21:
		charStr = hasShift ? @"$" : @"4";
		break;
	case 22:
		charStr = hasShift ? @"^" : @"6";
		break;
	case 23:
		charStr = hasShift ? @"%" : @"5";
		break;
	case 24:
		charStr = hasShift ? @"+" : @"=";
		break;
	case 25:
		charStr = hasShift ? @"(" : @"9";
		break;
	case 26:
		charStr = hasShift ? @"&" : @"7";
		break;
	case 27:
		charStr = hasShift ? @"_" : @"-";
		break;
	case 28:
		charStr = hasShift ? @"*" : @"8";
		break;
	case 29:
		charStr = hasShift ? @")" : @"0";
		break;

	// Symbols
	case 30:
		charStr = hasShift ? @"}" : @"]";
		break;
	case 33:
		charStr = hasShift ? @"{" : @"[";
		break;
	case 39:
		charStr = hasShift ? @"\"" : @"'";
		break;
	case 41:
		charStr = hasShift ? @":" : @";";
		break;
	case 42:
		charStr = hasShift ? @"|" : @"\\";
		break;
	case 43:
		charStr = hasShift ? @"<" : @",";
		break;
	case 44:
		charStr = hasShift ? @"?" : @"/";
		break;
	case 47:
		charStr = hasShift ? @">" : @".";
		break;

	// Space
	case 49:
		charStr = @" ";
		break;

	// Tab and Return
	case 36:
		charStr = @"\r"; // Return
		break;
	case 48:
		charStr = @"\t"; // Tab
		break;

	// Backtick/Tilde
	case 50:
		charStr = hasShift ? @"~" : @"`";
		break;

	// Numpad
	case 65:
		charStr = @".";
		break;
	case 67:
		charStr = @"*";
		break;
	case 69:
		charStr = @"+";
		break;
	case 71:
		charStr = @"\x7f"; // Clear
		break;
	case 75:
		charStr = @"/";
		break;
	case 76:
		charStr = @"\x03"; // Enter
		break;
	case 78:
		charStr = @"-";
		break;
	case 81:
		charStr = @"=";
		break;
	case 82:
		charStr = @"0";
		break;
	case 83:
		charStr = @"1";
		break;
	case 84:
		charStr = @"2";
		break;
	case 85:
		charStr = @"3";
		break;
	case 86:
		charStr = @"4";
		break;
	case 87:
		charStr = @"5";
		break;
	case 88:
		charStr = @"6";
		break;
	case 89:
		charStr = @"7";
		break;
	case 91:
		charStr = @"8";
		break;
	case 92:
		charStr = @"9";
		break;

	default:
		break;
	}

	return charStr;
}

/// Map keycode to key name (US QWERTY layout)
NSString *keyCodeToName(CGKeyCode keyCode) {
	switch (keyCode) {
	case 49:
		return @"Space";
	case 36:
		return @"Return";
	case 53:
		return @"Escape";
	case 48:
		return @"Tab";
	case 51:
		return @"Delete";
	case 116:
		return @"PageUp";
	case 121:
		return @"PageDown";
	case 115:
		return @"Home";
	case 119:
		return @"End";
	case 123:
		return @"Left";
	case 124:
		return @"Right";
	case 125:
		return @"Down";
	case 126:
		return @"Up";
	// Letters (US QWERTY layout)
	case 0:
		return @"A";
	case 1:
		return @"S";
	case 2:
		return @"D";
	case 3:
		return @"F";
	case 4:
		return @"H";
	case 5:
		return @"G";
	case 6:
		return @"Z";
	case 7:
		return @"X";
	case 8:
		return @"C";
	case 9:
		return @"V";
	case 11:
		return @"B";
	case 12:
		return @"Q";
	case 13:
		return @"W";
	case 14:
		return @"E";
	case 15:
		return @"R";
	case 16:
		return @"Y";
	case 17:
		return @"T";
	// Numbers and symbols
	case 18:
		return @"1";
	case 19:
		return @"2";
	case 20:
		return @"3";
	case 21:
		return @"4";
	case 22:
		return @"6";
	case 23:
		return @"5";
	case 24:
		return @"=";
	case 25:
		return @"9";
	case 26:
		return @"7";
	case 27:
		return @"-";
	case 28:
		return @"8";
	case 29:
		return @"0";
	case 30:
		return @"]";
	case 31:
		return @"O";
	case 32:
		return @"U";
	case 33:
		return @"[";
	case 34:
		return @"I";
	case 35:
		return @"P";
	case 37:
		return @"L";
	case 38:
		return @"J";
	case 39:
		return @"'";
	case 40:
		return @"K";
	case 41:
		return @";";
	case 42:
		return @"\\";
	case 43:
		return @",";
	case 44:
		return @"/";
	case 45:
		return @"N";
	case 46:
		return @"M";
	case 47:
		return @".";
	// Function keys
	case 122:
		return @"F1";
	case 120:
		return @"F2";
	case 99:
		return @"F3";
	case 118:
		return @"F4";
	case 96:
		return @"F5";
	case 97:
		return @"F6";
	case 98:
		return @"F7";
	case 100:
		return @"F8";
	case 101:
		return @"F9";
	case 109:
		return @"F10";
	case 103:
		return @"F11";
	case 111:
		return @"F12";
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

			// Check for modifiers (excluding Shift which is used for normal typing)
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

			// Special handling for delete/backspace key (keycode 51) without modifiers
			if (keyCode == 51) {
				if (context->callback) {
					context->callback("\x7f", context->userData);
				}
				return NULL;
			}

			// Special handling for escape key (keycode 53) without modifiers
			if (keyCode == 53) {
				if (context->callback) {
					context->callback("\x1b", context->userData);
				}
				return NULL;
			}

			// Handle arrow keys and special keys without modifiers
			switch (keyCode) {
			case 126: // Up arrow
				if (context->callback) {
					context->callback("Up", context->userData);
				}
				return NULL;
			case 125: // Down arrow
				if (context->callback) {
					context->callback("Down", context->userData);
				}
				return NULL;
			case 123: // Left arrow
				if (context->callback) {
					context->callback("Left", context->userData);
				}
				return NULL;
			case 124: // Right arrow
				if (context->callback) {
					context->callback("Right", context->userData);
				}
				return NULL;
			case 116: // PageUp
				if (context->callback) {
					context->callback("PageUp", context->userData);
				}
				return NULL;
			case 121: // PageDown
				if (context->callback) {
					context->callback("PageDown", context->userData);
				}
				return NULL;
			case 115: // Home
				if (context->callback) {
					context->callback("Home", context->userData);
				}
				return NULL;
			case 119: // End
				if (context->callback) {
					context->callback("End", context->userData);
				}
				return NULL;
			default:
				break;
			}

			// Map key code to character directly using US QWERTY layout
			// This bypasses input methods completely
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
		dispatch_async(dispatch_get_main_queue(), ^{
			CFRunLoopAddSource(CFRunLoopGetMain(), context->runLoopSource, kCFRunLoopCommonModes);
		});
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

	// Always enable asynchronously to avoid overlap with disable/destroy
	// Use a short delay to ensure prior disable completes first
	dispatch_async(dispatch_get_main_queue(), ^{
		// Cancel any existing pending block
		if (context->pendingEnableBlock) {
			dispatch_block_cancel(context->pendingEnableBlock);
			context->pendingEnableBlock = nil;
		}

		dispatch_block_t block = dispatch_block_create(0, ^{
			CGEventTapEnable(context->eventTap, true);
			context->pendingEnableBlock = nil;
		});

		context->pendingEnableBlock = block;
		dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(0.15 * NSEC_PER_SEC)), dispatch_get_main_queue(),
		               block);
	});
}

/// Disable event tap
/// @param tap Event tap handle
void disableEventTap(EventTap tap) {
	if (!tap)
		return;

	EventTapContext *context = (EventTapContext *)tap;

	// Always disable asynchronously to avoid overlap with enable/destroy
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
	};

	// Disable first (must be on main thread)
	if ([NSThread isMainThread]) {
		cleanupBlock();
	} else {
		dispatch_sync(dispatch_get_main_queue(), cleanupBlock);
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
	context->hotkeys = nil;            // ARC will handle deallocation
	context->accessQueue = nil;        // ARC will handle deallocation
	context->pendingEnableBlock = nil; // ARC will handle deallocation

	free(context);
}
