//
//  keymap.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "keymap.h"
#include <stdatomic.h>

#pragma mark - Static Data

/// mutex lock for layout-aware keymaps
static NSLock *gKeymapLock = nil;

/// layout-independent maps for special keys built once on initialisation
static NSDictionary<NSString *, NSNumber *> *gSpecialNameToCodeMap = nil;
static NSDictionary<NSNumber *, NSString *> *gSpecialCodeToNameMap = nil;

/// overall keymaps layout independent and dependent
/// rebuild on the fly when the keyboard layout changes
static NSDictionary<NSString *, NSNumber *> *gKeyNameToCodeMap = nil;
static NSDictionary<NSNumber *, NSString *> *gKeyCodeToNameMap = nil;
static TISInputSourceRef gCurrentInputSource = nil;
static const UCKeyboardLayout *gCurrentKeyboardLayout = nil;

/// cached keycode to char maps with common modifiers like shift and caps
/// to avoid UCKeyTranslate calls during run
static NSDictionary<NSNumber *, NSString *> *gKeyCodeToCharUnshifted = nil;
static NSDictionary<NSNumber *, NSString *> *gKeyCodeToCharShifted = nil;
static NSDictionary<NSNumber *, NSString *> *gKeyCodeToCharCaps = nil;
static NSDictionary<NSNumber *, NSString *> *gKeyCodeToCharShiftedCaps = nil;

/// debounce timer for keyboard layout change notifications
static dispatch_block_t gLayoutChangeDebounceBlock = nil;

#pragma mark - UCKeyTranslate Helper

/// figure out which character string each virtual keycode maps to
/// read the physical keyboard layout via TISCopyCurrentKeyboardLayoutInputSource

/// @param keyCode Virtual key code (0-127)
/// @param modifierState Carbon-style modifier state ((EventRecord.modifiers >> 8) & 0xFF)
/// @return Character string, or nil if translation fails
static NSString *translateKeyCodeViaLayout(const UCKeyboardLayout *keyboardLayout, CGKeyCode keyCode,
                                           UInt32 modifierState) {
	if (!keyboardLayout) {
		return nil;
	}

	UInt32 deadKeyState = 0;
	UniChar chars[4];
	UniCharCount actualLength = 0;

	OSStatus status = UCKeyTranslate(keyboardLayout, keyCode, kUCKeyActionDown, modifierState, LMGetKbdType(),
	                                 kUCKeyTranslateNoDeadKeysBit, &deadKeyState, sizeof(chars) / sizeof(chars[0]),
	                                 &actualLength, chars);

	if (status != noErr || actualLength == 0) {
		return nil;
	}

	return [NSString stringWithCharacters:chars length:actualLength];
}

#pragma mark - QWERTY Fallback

/// hardcoded US QWERTY keycode-to-character mapping as fallback
/// when UCKeyTranslate fails (e.g. missing layout data)

/// @param keyCode Key code
/// @param flags Event flags (for shift/capslock detection)
/// @return Character string or nil if not found
static NSString *keyCodeToCharacterQWERTY(CGKeyCode keyCode, CGEventFlags flags) {
	BOOL hasShift = (flags & kCGEventFlagMaskShift) != 0;
	BOOL hasCapsLock = (flags & kCGEventFlagMaskAlphaShift) != 0;
	BOOL uppercase = hasShift != hasCapsLock;

	switch (keyCode) {
	// letters
	case kKeyCodeA:
		return uppercase ? @"A" : @"a";
	case kKeyCodeB:
		return uppercase ? @"B" : @"b";
	case kKeyCodeC:
		return uppercase ? @"C" : @"c";
	case kKeyCodeD:
		return uppercase ? @"D" : @"d";
	case kKeyCodeE:
		return uppercase ? @"E" : @"e";
	case kKeyCodeF:
		return uppercase ? @"F" : @"f";
	case kKeyCodeG:
		return uppercase ? @"G" : @"g";
	case kKeyCodeH:
		return uppercase ? @"H" : @"h";
	case kKeyCodeI:
		return uppercase ? @"I" : @"i";
	case kKeyCodeJ:
		return uppercase ? @"J" : @"j";
	case kKeyCodeK:
		return uppercase ? @"K" : @"k";
	case kKeyCodeL:
		return uppercase ? @"L" : @"l";
	case kKeyCodeM:
		return uppercase ? @"M" : @"m";
	case kKeyCodeN:
		return uppercase ? @"N" : @"n";
	case kKeyCodeO:
		return uppercase ? @"O" : @"o";
	case kKeyCodeP:
		return uppercase ? @"P" : @"p";
	case kKeyCodeQ:
		return uppercase ? @"Q" : @"q";
	case kKeyCodeR:
		return uppercase ? @"R" : @"r";
	case kKeyCodeS:
		return uppercase ? @"S" : @"s";
	case kKeyCodeT:
		return uppercase ? @"T" : @"t";
	case kKeyCodeU:
		return uppercase ? @"U" : @"u";
	case kKeyCodeV:
		return uppercase ? @"V" : @"v";
	case kKeyCodeW:
		return uppercase ? @"W" : @"w";
	case kKeyCodeX:
		return uppercase ? @"X" : @"x";
	case kKeyCodeY:
		return uppercase ? @"Y" : @"y";
	case kKeyCodeZ:
		return uppercase ? @"Z" : @"z";

	// numbers (shifted symbols)
	case kKeyCode0:
		return hasShift ? @")" : @"0";
	case kKeyCode1:
		return hasShift ? @"!" : @"1";
	case kKeyCode2:
		return hasShift ? @"@" : @"2";
	case kKeyCode3:
		return hasShift ? @"#" : @"3";
	case kKeyCode4:
		return hasShift ? @"$" : @"4";
	case kKeyCode5:
		return hasShift ? @"%" : @"5";
	case kKeyCode6:
		return hasShift ? @"^" : @"6";
	case kKeyCode7:
		return hasShift ? @"&" : @"7";
	case kKeyCode8:
		return hasShift ? @"*" : @"8";
	case kKeyCode9:
		return hasShift ? @"(" : @"9";

	// symbols
	case kKeyCodeRightBracket:
		return hasShift ? @"}" : @"]";
	case kKeyCodeLeftBracket:
		return hasShift ? @"{" : @"[";
	case kKeyCodeQuote:
		return hasShift ? @"\"" : @"'";
	case kKeyCodeSemicolon:
		return hasShift ? @":" : @";";
	case kKeyCodeBackslash:
		return hasShift ? @"|" : @"\\";
	case kKeyCodeComma:
		return hasShift ? @"<" : @",";
	case kKeyCodeSlash:
		return hasShift ? @"?" : @"/";
	case kKeyCodePeriod:
		return hasShift ? @">" : @".";
	case kKeyCodeEqual:
		return hasShift ? @"+" : @"=";
	case kKeyCodeMinus:
		return hasShift ? @"_" : @"-";
	case kKeyCodeBacktick:
		return hasShift ? @"~" : @"`";

	default:
		return nil;
	}
}

/// hardcoded QWERTY keycode-to-name mapping for fallback

/// @param keyCode Key code
/// @return Uppercase key name or nil
static NSString *keyCodeToNameQWERTY(CGKeyCode keyCode) {
	switch (keyCode) {
	case kKeyCodeA:
		return @"A";
	case kKeyCodeB:
		return @"B";
	case kKeyCodeC:
		return @"C";
	case kKeyCodeD:
		return @"D";
	case kKeyCodeE:
		return @"E";
	case kKeyCodeF:
		return @"F";
	case kKeyCodeG:
		return @"G";
	case kKeyCodeH:
		return @"H";
	case kKeyCodeI:
		return @"I";
	case kKeyCodeJ:
		return @"J";
	case kKeyCodeK:
		return @"K";
	case kKeyCodeL:
		return @"L";
	case kKeyCodeM:
		return @"M";
	case kKeyCodeN:
		return @"N";
	case kKeyCodeO:
		return @"O";
	case kKeyCodeP:
		return @"P";
	case kKeyCodeQ:
		return @"Q";
	case kKeyCodeR:
		return @"R";
	case kKeyCodeS:
		return @"S";
	case kKeyCodeT:
		return @"T";
	case kKeyCodeU:
		return @"U";
	case kKeyCodeV:
		return @"V";
	case kKeyCodeW:
		return @"W";
	case kKeyCodeX:
		return @"X";
	case kKeyCodeY:
		return @"Y";
	case kKeyCodeZ:
		return @"Z";
	case kKeyCode0:
		return @"0";
	case kKeyCode1:
		return @"1";
	case kKeyCode2:
		return @"2";
	case kKeyCode3:
		return @"3";
	case kKeyCode4:
		return @"4";
	case kKeyCode5:
		return @"5";
	case kKeyCode6:
		return @"6";
	case kKeyCode7:
		return @"7";
	case kKeyCode8:
		return @"8";
	case kKeyCode9:
		return @"9";
	case kKeyCodeEqual:
		return @"=";
	case kKeyCodeMinus:
		return @"-";
	case kKeyCodeRightBracket:
		return @"]";
	case kKeyCodeLeftBracket:
		return @"[";
	case kKeyCodeQuote:
		return @"'";
	case kKeyCodeSemicolon:
		return @";";
	case kKeyCodeBackslash:
		return @"\\";
	case kKeyCodeComma:
		return @",";
	case kKeyCodeSlash:
		return @"/";
	case kKeyCodePeriod:
		return @".";
	case kKeyCodeBacktick:
		return @"`";
	default:
		return nil;
	}
}

#pragma mark - Keymap Building

/// build QWERTY-only fallback char maps using the hardcoded tables
/// used when layout data is unavailable (e.g. CJK IME without underlying layout)
static void buildQWERTYCharMaps(NSMutableDictionary<NSNumber *, NSString *> *unshifted,
                                NSMutableDictionary<NSNumber *, NSString *> *shifted,
                                NSMutableDictionary<NSNumber *, NSString *> *caps,
                                NSMutableDictionary<NSNumber *, NSString *> *shiftedCaps) {
	for (CGKeyCode keyCode = 0; keyCode <= kKeyCodeMaxPrintable; keyCode++) {
		NSNumber *key = @(keyCode);

		NSString *ch = keyCodeToCharacterQWERTY(keyCode, 0);
		if (ch)
			unshifted[key] = ch;

		NSString *sh = keyCodeToCharacterQWERTY(keyCode, kCGEventFlagMaskShift);
		if (sh)
			shifted[key] = sh;

		NSString *cp = keyCodeToCharacterQWERTY(keyCode, kCGEventFlagMaskAlphaShift);
		if (cp)
			caps[key] = cp;

		NSString *sc = keyCodeToCharacterQWERTY(keyCode, kCGEventFlagMaskShift | kCGEventFlagMaskAlphaShift);
		if (sc)
			shiftedCaps[key] = sc;
	}
}

/// populate name/code maps with QWERTY fallback entries
/// ensures special keys and basic key lookups work even without layout data
static void buildQWERTYNameMaps(NSMutableDictionary<NSString *, NSNumber *> *nameToCode,
                                NSMutableDictionary<NSNumber *, NSString *> *codeToName) {
	for (CGKeyCode keyCode = 0; keyCode <= kKeyCodeMaxPrintable; keyCode++) {
		NSString *ch = keyCodeToNameQWERTY(keyCode);
		if (ch) {
			codeToName[@(keyCode)] = ch;
			if (!nameToCode[ch]) {
				nameToCode[ch] = @(keyCode);
			}
		}
	}
}

/// build special key maps which should be layout independent
static void initializeSpecialKeyMaps(void) {
	gSpecialNameToCodeMap = [@{
		// special keys
		@"Space" : @(kKeyCodeSpace),
		@"Return" : @(kKeyCodeReturn),
		@"Enter" : @(kKeyCodeReturn),
		@"Escape" : @(kKeyCodeEscape),
		@"Tab" : @(kKeyCodeTab),
		@"Delete" : @(kKeyCodeDelete),
		@"Backspace" : @(kKeyCodeDelete),

		// nav keys
		@"Left" : @(kKeyCodeLeft),
		@"Right" : @(kKeyCodeRight),
		@"Down" : @(kKeyCodeDown),
		@"Up" : @(kKeyCodeUp),
		@"PageUp" : @(kKeyCodePageUp),
		@"PageDown" : @(kKeyCodePageDown),
		@"Home" : @(kKeyCodeHome),
		@"End" : @(kKeyCodeEnd),

		// function keys
		@"F1" : @(kKeyCodeF1),
		@"F2" : @(kKeyCodeF2),
		@"F3" : @(kKeyCodeF3),
		@"F4" : @(kKeyCodeF4),
		@"F5" : @(kKeyCodeF5),
		@"F6" : @(kKeyCodeF6),
		@"F7" : @(kKeyCodeF7),
		@"F8" : @(kKeyCodeF8),
		@"F9" : @(kKeyCodeF9),
		@"F10" : @(kKeyCodeF10),
		@"F11" : @(kKeyCodeF11),
		@"F12" : @(kKeyCodeF12),
	} copy];

	NSMutableDictionary<NSNumber *, NSString *> *codeToName =
	    [NSMutableDictionary dictionaryWithCapacity:gSpecialNameToCodeMap.count];
	[gSpecialNameToCodeMap enumerateKeysAndObjectsUsingBlock:^(NSString *name, NSNumber *code, BOOL *stop) {
		if (!codeToName[code]) {
			codeToName[code] = name;
		}
	}];

	// canonicalize duplicate names
	codeToName[@(kKeyCodeReturn)] = @"Return";
	codeToName[@(kKeyCodeDelete)] = @"Delete";

	gSpecialCodeToNameMap = [codeToName copy];
}

/// layout aware/layout dependent keys
/// we scan the keycodes via UCKeyTranslate and then merge with the layout
/// independent keymaps to build the combined keymap to use

/// locks gKeymapLock during build
static void buildLayoutMaps(void) {
	@autoreleasepool {
		NSMutableDictionary<NSString *, NSNumber *> *nameToCode =
		    [NSMutableDictionary dictionaryWithDictionary:gSpecialNameToCodeMap];
		NSMutableDictionary<NSNumber *, NSString *> *codeToName =
		    [NSMutableDictionary dictionaryWithDictionary:gSpecialCodeToNameMap];

		// cache some keymaps with modifiers for speedyquick lookup
		NSMutableDictionary<NSNumber *, NSString *> *unshifted = [NSMutableDictionary dictionary];
		NSMutableDictionary<NSNumber *, NSString *> *shifted = [NSMutableDictionary dictionary];
		NSMutableDictionary<NSNumber *, NSString *> *caps = [NSMutableDictionary dictionary];
		NSMutableDictionary<NSNumber *, NSString *> *shiftedCaps = [NSMutableDictionary dictionary];

		UInt32 shiftMod = (shiftKey >> 8) & 0xFF;
		UInt32 capsMod = (alphaLock >> 8) & 0xFF;
		UInt32 shiftCapsMod = shiftMod | capsMod;

		/// try to get the current keyboard layout
		TISInputSourceRef inputSource = TISCopyCurrentKeyboardLayoutInputSource();
		if (!inputSource) {
			// no layout source available — populate with QWERTY fallback
			// so special keys and basic key lookups still work
			buildQWERTYNameMaps(nameToCode, codeToName);
			buildQWERTYCharMaps(unshifted, shifted, caps, shiftedCaps);

			[gKeymapLock lock];
			[gKeyNameToCodeMap release];
			gKeyNameToCodeMap = [nameToCode copy];
			[gKeyCodeToNameMap release];
			gKeyCodeToNameMap = [codeToName copy];
			[gKeyCodeToCharUnshifted release];
			gKeyCodeToCharUnshifted = [unshifted copy];
			[gKeyCodeToCharShifted release];
			gKeyCodeToCharShifted = [shifted copy];
			[gKeyCodeToCharCaps release];
			gKeyCodeToCharCaps = [caps copy];
			[gKeyCodeToCharShiftedCaps release];
			gKeyCodeToCharShiftedCaps = [shiftedCaps copy];
			[gKeymapLock unlock];
			return;
		}

		CFDataRef layoutData = (CFDataRef)TISGetInputSourceProperty(inputSource, kTISPropertyUnicodeKeyLayoutData);
		if (!layoutData) {
			// layout source exists but has no uchr data (e.g. some CJK IMEs)
			// populate with QWERTY fallback, keep previous layout pointer if we had one
			CFRelease(inputSource);
			buildQWERTYNameMaps(nameToCode, codeToName);
			buildQWERTYCharMaps(unshifted, shifted, caps, shiftedCaps);

			[gKeymapLock lock];
			[gKeyNameToCodeMap release];
			gKeyNameToCodeMap = [nameToCode copy];
			[gKeyCodeToNameMap release];
			gKeyCodeToNameMap = [codeToName copy];
			[gKeyCodeToCharUnshifted release];
			gKeyCodeToCharUnshifted = [unshifted copy];
			[gKeyCodeToCharShifted release];
			gKeyCodeToCharShifted = [shifted copy];
			[gKeyCodeToCharCaps release];
			gKeyCodeToCharCaps = [caps copy];
			[gKeyCodeToCharShiftedCaps release];
			gKeyCodeToCharShiftedCaps = [shiftedCaps copy];
			// don't touch gCurrentInputSource/gCurrentKeyboardLayout —
			// keep previous valid layout for live UCKeyTranslate fallback
			[gKeymapLock unlock];
			return;
		}

		const UCKeyboardLayout *keyboardLayout = (const UCKeyboardLayout *)CFDataGetBytePtr(layoutData);

		// scan printable keycodes and translate via current keyboard layout
		for (CGKeyCode keyCode = 0; keyCode <= kKeyCodeMaxPrintable; keyCode++) {
			// skip keycodes already covered by special key maps
			if (gSpecialCodeToNameMap[@(keyCode)]) {
				continue;
			}
			NSNumber *key = @(keyCode);

			NSString *ch = translateKeyCodeViaLayout(keyboardLayout, keyCode, 0);
			if (!ch || ch.length == 0) {
				// UCKeyTranslate failed use QWERTY fallback
				ch = keyCodeToNameQWERTY(keyCode);
				if (!ch)
					continue;
			}
			NSString *upper = ch.uppercaseString;
			codeToName[key] = upper;

			// add to name -> code mapping if not already present
			// (the result is that the first keycode wins if it so
			// happens that multiple keycodes produce the same character)
			if (!nameToCode[upper]) {
				nameToCode[upper] = key;
			}

			// char maps (reuse unshifted result)
			unshifted[key] = ch;

			NSString *sh = translateKeyCodeViaLayout(keyboardLayout, keyCode, shiftMod);
			if (!sh)
				sh = keyCodeToCharacterQWERTY(keyCode, kCGEventFlagMaskShift);
			if (sh)
				shifted[key] = sh;

			NSString *cp = translateKeyCodeViaLayout(keyboardLayout, keyCode, capsMod);
			if (!cp)
				cp = keyCodeToCharacterQWERTY(keyCode, kCGEventFlagMaskAlphaShift);
			if (cp)
				caps[key] = cp;

			NSString *sc = translateKeyCodeViaLayout(keyboardLayout, keyCode, shiftCapsMod);
			if (!sc)
				sc = keyCodeToCharacterQWERTY(keyCode, kCGEventFlagMaskShift | kCGEventFlagMaskAlphaShift);
			if (sc)
				shiftedCaps[key] = sc;
		}

		// atomic swap combined maps
		NSDictionary *newNameToCode = [nameToCode copy];
		NSDictionary *newCodeToName = [codeToName copy];

		[gKeymapLock lock];
		if (gCurrentInputSource) {
			CFRelease(gCurrentInputSource);
		}
		// take ownership of input source so keyboard layout is
		// valid while gCurrentInputSource lives
		gCurrentInputSource = inputSource;
		gCurrentKeyboardLayout = (const UCKeyboardLayout *)CFDataGetBytePtr(layoutData);

		// Release old dictionaries before assigning new ones (MRC)
		// Transfer ownership from local vars (already +1 from copy) to globals
		[gKeyNameToCodeMap release];
		gKeyNameToCodeMap = newNameToCode;
		[gKeyCodeToNameMap release];
		gKeyCodeToNameMap = newCodeToName;

		[gKeyCodeToCharUnshifted release];
		gKeyCodeToCharUnshifted = [unshifted copy];
		[gKeyCodeToCharShifted release];
		gKeyCodeToCharShifted = [shifted copy];
		[gKeyCodeToCharCaps release];
		gKeyCodeToCharCaps = [caps copy];
		[gKeyCodeToCharShiftedCaps release];
		gKeyCodeToCharShiftedCaps = [shiftedCaps copy];
		[gKeymapLock unlock];
	}
}

#pragma mark - Layout Change Notification

/// triggered by the system when keyboard layout changed to trigger rebuild
/// Note: This callback is invoked on the main thread by CFNotificationCenterGetDistributedCenter,
/// so unsynchronized access to gLayoutChangeDebounceBlock is safe. The debounced block is also
/// dispatched to the main queue, ensuring all access is serialized on the main thread.
static void handleKeyboardLayoutChanged(CFNotificationCenterRef center, void *observer, CFNotificationName name,
                                        const void *object, CFDictionaryRef userInfo) {
	if (gLayoutChangeDebounceBlock) {
		dispatch_block_cancel(gLayoutChangeDebounceBlock);
	}

	gLayoutChangeDebounceBlock = dispatch_block_create(0, ^{
		buildLayoutMaps();
		gLayoutChangeDebounceBlock = nil;
	});

	dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(150 * NSEC_PER_MSEC)), dispatch_get_main_queue(),
	               gLayoutChangeDebounceBlock);
	// Note: This introduces a 150ms window where keymap queries return stale data.
	// Tradeoff is acceptable since CJK input methods fire multiple notifications per keystroke,
	// and users aren't typically typing hotkeys during layout switches.
}

#pragma mark - Initialization

/// Flag for tracking layout maps initialization status (atomic for thread safety)
static atomic_bool gLayoutMapsInitialized = false;

/// Register notification observer (called once from initializeKeyMaps)
static void registerLayoutChangeObserver(void) {
	CFNotificationCenterAddObserver(CFNotificationCenterGetDistributedCenter(), NULL, handleKeyboardLayoutChanged,
	                                kTISNotifySelectedKeyboardInputSourceChanged, NULL,
	                                CFNotificationSuspensionBehaviorDeliverImmediately);
}

static void initializeKeyMaps(void) {
	static dispatch_once_t onceToken;
	dispatch_once(&onceToken, ^{
		gKeymapLock = [[NSLock alloc] init];

		initializeSpecialKeyMaps();

		// Register observer early, before any layout maps building
		registerLayoutChangeObserver();
	});
}

/// Ensure layout maps are built (must be called after initializeKeyMaps)
/// Blocks until layout maps are available
static void ensureLayoutMapsInitialized(void) {
	if (atomic_load_explicit(&gLayoutMapsInitialized, memory_order_acquire)) {
		return;
	}

	// TIS APIs must be called on main thread
	// Use dispatch_once to ensure single initialization, but dispatch OUTSIDE of it
	static dispatch_once_t layoutOnceToken;
	dispatch_once(&layoutOnceToken, ^{
		if ([NSThread isMainThread]) {
			buildLayoutMaps();
			atomic_store_explicit(&gLayoutMapsInitialized, true, memory_order_release);
		} else {
			// Dispatch to main thread to build layout maps
			dispatch_async(dispatch_get_main_queue(), ^{
				// Guard against double execution if main thread already ran it
				if (!atomic_load_explicit(&gLayoutMapsInitialized, memory_order_acquire)) {
					buildLayoutMaps();
					atomic_store_explicit(&gLayoutMapsInitialized, true, memory_order_release);
				}
			});
		}
	});

	// If we're on main thread and not initialized yet, run directly
	// to avoid deadlock (the async block is queued behind us)
	if (!atomic_load_explicit(&gLayoutMapsInitialized, memory_order_acquire) && [NSThread isMainThread]) {
		buildLayoutMaps();
		atomic_store_explicit(&gLayoutMapsInitialized, true, memory_order_release);
		return;
	}

	// Poll with timeout for all waiters (semaphore only wakes one)
	// This allows multiple concurrent background threads to all proceed
	// once initialization is complete, rather than timing out
	dispatch_time_t timeout = dispatch_time(DISPATCH_TIME_NOW, 5 * NSEC_PER_SEC);
	while (!atomic_load_explicit(&gLayoutMapsInitialized, memory_order_acquire)) {
		if (dispatch_time(DISPATCH_TIME_NOW, 0) >= timeout) {
			break; // Timeout reached
		}
		[NSThread sleepForTimeInterval:0.001]; // 1ms sleep to avoid busy-wait
	}
}

#pragma mark - Public Functions

NSDictionary<NSString *, NSNumber *> *keyNameToCodeMap(void) {
	initializeKeyMaps();
	ensureLayoutMapsInitialized();

	// lock to prevent conflicts with rebuild
	[gKeymapLock lock];
	NSDictionary *map = gKeyNameToCodeMap;
	// Retain to ensure dictionary stays valid after releasing lock
	map = [[map retain] autorelease];
	[gKeymapLock unlock];

	return map;
}

NSDictionary<NSNumber *, NSString *> *keyCodeToNameMap(void) {
	initializeKeyMaps();
	ensureLayoutMapsInitialized();

	// lock to prevent conflicts with rebuild
	[gKeymapLock lock];
	NSDictionary *map = gKeyCodeToNameMap;
	// Retain to ensure dictionary stays valid after releasing lock
	map = [[map retain] autorelease];
	[gKeymapLock unlock];

	return map;
}

CGKeyCode keyNameToCode(NSString *keyName) {
	if (!keyName || keyName.length == 0) {
		return 0xFFFF;
	}

	initializeKeyMaps();
	ensureLayoutMapsInitialized();

	// lock to prevent conflicts with rebuild
	[gKeymapLock lock];
	NSDictionary *map = gKeyNameToCodeMap;
	// Retain to ensure dictionary stays valid after releasing lock
	map = [[map retain] autorelease];
	[gKeymapLock unlock];

	NSNumber *code = map[keyName];
	if (!code) {
		// Try uppercase version
		code = map[keyName.uppercaseString];
	}

	return code ? code.unsignedShortValue : 0xFFFF;
}

NSString *keyCodeToName(CGKeyCode keyCode) {
	initializeKeyMaps();
	ensureLayoutMapsInitialized();

	// lock to prevent conflicts with rebuild
	[gKeymapLock lock];
	NSDictionary *map = gKeyCodeToNameMap;
	// Retain to ensure dictionary stays valid after releasing lock
	map = [[map retain] autorelease];
	[gKeymapLock unlock];

	return map[@(keyCode)];
}

NSString *keyCodeToCharacter(CGKeyCode keyCode, CGEventFlags flags) {
	initializeKeyMaps();
	ensureLayoutMapsInitialized();

	// for special keys layout independent hardcoded values
	switch (keyCode) {
	case kKeyCodeSpace:
		return @" ";
	case kKeyCodeReturn:
		return @"\r";
	case kKeyCodeTab:
		return @"\t";

	// numpad is layout independent
	case kKeyCodeNumpadDot:
		return @".";
	case kKeyCodeNumpadMultiply:
		return @"*";
	case kKeyCodeNumpadPlus:
		return @"+";
	case kKeyCodeNumpadClear:
		return @"\x7f";
	case kKeyCodeNumpadDivide:
		return @"/";
	case kKeyCodeNumpadEnter:
		return @"\x03";
	case kKeyCodeNumpadMinus:
		return @"-";
	case kKeyCodeNumpadEquals:
		return @"=";
	case kKeyCodeNumpad0:
		return @"0";
	case kKeyCodeNumpad1:
		return @"1";
	case kKeyCodeNumpad2:
		return @"2";
	case kKeyCodeNumpad3:
		return @"3";
	case kKeyCodeNumpad4:
		return @"4";
	case kKeyCodeNumpad5:
		return @"5";
	case kKeyCodeNumpad6:
		return @"6";
	case kKeyCodeNumpad7:
		return @"7";
	case kKeyCodeNumpad8:
		return @"8";
	case kKeyCodeNumpad9:
		return @"9";

	default:
		break;
	}

	// attempt layout-aware translation
	BOOL hasShift = (flags & kCGEventFlagMaskShift) != 0;
	BOOL hasCaps = (flags & kCGEventFlagMaskAlphaShift) != 0;

	// try cached map lookup first
	// (no modifiers, shift only, caps only, shift+caps) without any UCKeyTranslate calls
	[gKeymapLock lock];
	NSDictionary *map;
	if (hasShift && !hasCaps) {
		map = gKeyCodeToCharShifted;
	} else if (hasCaps && !hasShift) {
		map = gKeyCodeToCharCaps;
	} else if (!hasShift && !hasCaps) {
		map = gKeyCodeToCharUnshifted;
	} else {
		map = gKeyCodeToCharShiftedCaps;
	}
	// Retain to ensure dictionary stays valid after releasing lock
	map = [[map retain] autorelease];
	const UCKeyboardLayout *layout = gCurrentKeyboardLayout;
	TISInputSourceRef localSource = gCurrentInputSource;
	if (localSource)
		CFRetain(localSource); // keep backing data alive
	[gKeymapLock unlock];

	if (map) {
		NSString *result = map[@(keyCode)];
		if (result) {
			if (localSource)
				CFRelease(localSource);
			return result;
		}
	}

	// live translation based on layout if the modifier
	// state is not one which we have cached in the map
	if (layout) {
		// build Carbon-style modifier state for UCKeyTranslate
		UInt32 modifierState = 0;
		if (hasShift) {
			modifierState |= (shiftKey >> 8) & 0xFF;
		}
		if (hasCaps) {
			modifierState |= (alphaLock >> 8) & 0xFF;
		}
		NSString *result = translateKeyCodeViaLayout(layout, keyCode, modifierState);
		if (result && result.length > 0) {
			if (localSource)
				CFRelease(localSource);
			return result;
		}
	}

	if (localSource)
		CFRelease(localSource);

	// as a last resort
	// hardcoded US QWERTY fallback if layout translation fails
	return keyCodeToCharacterQWERTY(keyCode, flags);
}

void refreshKeyboardLayoutMaps(void) {
	void (^cancelAndRebuild)(void) = ^{
		if (gLayoutChangeDebounceBlock) {
			dispatch_block_cancel(gLayoutChangeDebounceBlock);
			gLayoutChangeDebounceBlock = nil;
		}
		initializeKeyMaps();
		buildLayoutMaps();
	};

	if ([NSThread isMainThread]) {
		cancelAndRebuild();
	} else {
		dispatch_async(dispatch_get_main_queue(), cancelAndRebuild);
	}
}
