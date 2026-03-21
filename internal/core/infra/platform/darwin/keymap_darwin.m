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

/// overall keymaps (layout-independent and layout-dependent)
/// rebuilt on the fly when the keyboard layout changes
static NSDictionary<NSString *, NSNumber *> *gKeyNameToCodeMap = nil;
static NSDictionary<NSNumber *, NSString *> *gKeyCodeToNameMap = nil;
static TISInputSourceRef gCurrentInputSource = nil;
static const UCKeyboardLayout *gCurrentKeyboardLayout = nil;

/// user-configured input source ID (nil/empty means auto-detect fallback order)
static NSString *gConfiguredInputSourceID = nil;
/// resolved and cached reference input source used for key translation
static TISInputSourceRef gReferenceInputSource = nil;
/// true when using current-layout fallback because no stable latin layout was found
static BOOL gUsesCurrentLayoutFallback = NO;
/// whether the configured layout ID was resolved successfully (or no explicit ID was set)
static BOOL gConfiguredInputSourceResolved = YES;

/// cached keycode-to-char maps for common modifier combinations (shift, caps, shift+caps)
/// to avoid UCKeyTranslate calls during key event handling
static NSDictionary<NSNumber *, NSString *> *gKeyCodeToCharUnshifted = nil;
static NSDictionary<NSNumber *, NSString *> *gKeyCodeToCharShifted = nil;
static NSDictionary<NSNumber *, NSString *> *gKeyCodeToCharCaps = nil;
static NSDictionary<NSNumber *, NSString *> *gKeyCodeToCharShiftedCaps = nil;

/// debounce timer for keyboard layout change notifications
static dispatch_block_t gLayoutChangeDebounceBlock = nil;

/// optional callback invoked after layout maps are rebuilt
static _Atomic(KeymapLayoutChangeCallback) gLayoutChangeCallback = NULL;

#pragma mark - UCKeyTranslate Helper

/// Translate a virtual keycode to a character string using the given keyboard layout.
/// Uses UCKeyTranslate to respect the active layout while bypassing input methods.
/// @param keyboardLayout UCKeyboardLayout to translate against
/// @param keyCode Virtual key code (0-127)
/// @param modifierState Carbon-style modifier state ((EventRecord.modifiers >> 8) & 0xFF)
/// @return Character string, or nil if translation fails
static NSString *translateKeyCodeViaLayout(
    const UCKeyboardLayout *keyboardLayout, CGKeyCode keyCode, UInt32 modifierState) {
	if (!keyboardLayout) {
		return nil;
	}

	UInt32 deadKeyState = 0;
	UniChar chars[4];
	UniCharCount actualLength = 0;

	OSStatus status = UCKeyTranslate(
	    keyboardLayout, keyCode, kUCKeyActionDown, modifierState, LMGetKbdType(), kUCKeyTranslateNoDeadKeysBit,
	    &deadKeyState, sizeof(chars) / sizeof(chars[0]), &actualLength, chars);

	if (status != noErr || actualLength == 0) {
		return nil;
	}

	return [NSString stringWithCharacters:chars length:actualLength];
}

#pragma mark - Input Source Resolution

static BOOL inputSourceHasUnicodeLayoutData(TISInputSourceRef inputSource) {
	if (!inputSource) {
		return NO;
	}

	CFDataRef layoutData = (CFDataRef)TISGetInputSourceProperty(inputSource, kTISPropertyUnicodeKeyLayoutData);
	return layoutData && CFDataGetLength(layoutData) > 0;
}

static TISInputSourceRef copyKeyboardLayoutInputSourceByID(NSString *inputSourceID) {
	if (!inputSourceID || inputSourceID.length == 0) {
		return nil;
	}

	NSDictionary *filter = @{(__bridge NSString *)kTISPropertyInputSourceID : inputSourceID};
	CFArrayRef inputSourceList = TISCreateInputSourceList((__bridge CFDictionaryRef)filter, false);
	if (!inputSourceList) {
		return nil;
	}

	TISInputSourceRef matched = nil;
	CFIndex sourceCount = CFArrayGetCount(inputSourceList);
	for (CFIndex i = 0; i < sourceCount; i++) {
		TISInputSourceRef candidate = (TISInputSourceRef)CFArrayGetValueAtIndex(inputSourceList, i);
		if (inputSourceHasUnicodeLayoutData(candidate)) {
			CFRetain(candidate);
			matched = candidate;
			break;
		}
	}

	CFRelease(inputSourceList);
	return matched;
}

static BOOL inputSourceSupportsEnglish(TISInputSourceRef inputSource) {
	if (!inputSource) {
		return NO;
	}

	CFArrayRef languages = (CFArrayRef)TISGetInputSourceProperty(inputSource, kTISPropertyInputSourceLanguages);
	if (!languages) {
		return NO;
	}

	CFIndex languageCount = CFArrayGetCount(languages);
	for (CFIndex i = 0; i < languageCount; i++) {
		CFTypeRef languageValue = CFArrayGetValueAtIndex(languages, i);
		if (!languageValue || CFGetTypeID(languageValue) != CFStringGetTypeID()) {
			continue;
		}

		NSString *language = (__bridge NSString *)languageValue;
		if ([language caseInsensitiveCompare:@"en"] == NSOrderedSame || [language hasPrefix:@"en-"] ||
		    [language hasPrefix:@"en_"]) {
			return YES;
		}
	}

	return NO;
}

static TISInputSourceRef copyFirstEnglishKeyboardLayoutInputSource(void) {
	NSDictionary *filter =
	    @{(__bridge NSString *)kTISPropertyInputSourceType : (__bridge NSString *)kTISTypeKeyboardLayout};
	CFArrayRef inputSourceList = TISCreateInputSourceList((__bridge CFDictionaryRef)filter, false);
	if (!inputSourceList) {
		return nil;
	}

	TISInputSourceRef matched = nil;
	CFIndex sourceCount = CFArrayGetCount(inputSourceList);
	for (CFIndex i = 0; i < sourceCount; i++) {
		TISInputSourceRef candidate = (TISInputSourceRef)CFArrayGetValueAtIndex(inputSourceList, i);
		if (inputSourceHasUnicodeLayoutData(candidate) && inputSourceSupportsEnglish(candidate)) {
			CFRetain(candidate);
			matched = candidate;
			break;
		}
	}

	CFRelease(inputSourceList);
	return matched;
}

static TISInputSourceRef copyCurrentKeyboardLayoutInputSourceWithData(void) {
	TISInputSourceRef inputSource = TISCopyCurrentKeyboardLayoutInputSource();
	if (!inputSource) {
		return nil;
	}

	if (!inputSourceHasUnicodeLayoutData(inputSource)) {
		CFRelease(inputSource);
		return nil;
	}

	return inputSource;
}

static void clearResolvedReferenceInputSourceLocked(void) {
	if (gReferenceInputSource) {
		CFRelease(gReferenceInputSource);
		gReferenceInputSource = nil;
	}

	gUsesCurrentLayoutFallback = NO;
	gConfiguredInputSourceResolved = YES;
}

static TISInputSourceRef copyResolvedReferenceInputSource(
    BOOL *configuredLayoutResolvedOut, BOOL *usesCurrentFallbackOut) {
	// Check cache under lock
	[gKeymapLock lock];
	TISInputSourceRef cachedInputSource = gReferenceInputSource;
	NSString *configuredID = [gConfiguredInputSourceID copy];
	BOOL cachedConfiguredResolved = gConfiguredInputSourceResolved;
	BOOL cachedUsesCurrentFallback = gUsesCurrentLayoutFallback;
	if (cachedInputSource) {
		CFRetain(cachedInputSource);
	}
	[gKeymapLock unlock];

	if (cachedInputSource) {
		if (configuredLayoutResolvedOut) {
			*configuredLayoutResolvedOut = cachedConfiguredResolved;
		}
		if (usesCurrentFallbackOut) {
			*usesCurrentFallbackOut = cachedUsesCurrentFallback;
		}
		return cachedInputSource;
	}

	// Resolution order:
	// 1. User-configured input source ID
	// 2. Preferred stable Latin layouts (ABC, US)
	// 3. First English keyboard layout
	// 4. Current layout (fallback — unstable across layout switches)
	NSArray<NSString *> *preferredIDs = @[
		@"com.apple.keylayout.ABC",
		@"com.apple.keylayout.US",
	];

	TISInputSourceRef resolvedInputSource = nil;
	BOOL usesCurrentFallback = NO;
	BOOL configuredResolved = YES;

	if (configuredID.length > 0) {
		resolvedInputSource = copyKeyboardLayoutInputSourceByID(configuredID);
		if (!resolvedInputSource) {
			configuredResolved = NO;
		}
	}

	if (!resolvedInputSource) {
		for (NSString *candidateID in preferredIDs) {
			resolvedInputSource = copyKeyboardLayoutInputSourceByID(candidateID);
			if (resolvedInputSource) {
				break;
			}
		}
	}

	if (!resolvedInputSource) {
		resolvedInputSource = copyFirstEnglishKeyboardLayoutInputSource();
	}

	if (!resolvedInputSource) {
		resolvedInputSource = copyCurrentKeyboardLayoutInputSourceWithData();
		if (resolvedInputSource) {
			usesCurrentFallback = YES;
		}
	}

	// Store the resolved source under lock if it wasn't set concurrently
	[gKeymapLock lock];
	if (!gReferenceInputSource && resolvedInputSource) {
		gReferenceInputSource = resolvedInputSource;
		gUsesCurrentLayoutFallback = usesCurrentFallback;
		gConfiguredInputSourceResolved = configuredResolved;
		CFRetain(gReferenceInputSource);
		cachedInputSource = gReferenceInputSource;
	} else if (gReferenceInputSource) {
		cachedInputSource = gReferenceInputSource;
		CFRetain(cachedInputSource);
		if (resolvedInputSource) {
			CFRelease(resolvedInputSource);
		}
	} else {
		gConfiguredInputSourceResolved = configuredResolved;
		cachedInputSource = nil;
	}

	BOOL finalConfiguredResolved = gConfiguredInputSourceResolved;
	BOOL finalUsesCurrentFallback = gUsesCurrentLayoutFallback;
	[gKeymapLock unlock];

	if (configuredLayoutResolvedOut) {
		*configuredLayoutResolvedOut = finalConfiguredResolved;
	}
	if (usesCurrentFallbackOut) {
		*usesCurrentFallbackOut = finalUsesCurrentFallback;
	}

	return cachedInputSource;
}

#pragma mark - QWERTY Fallback

/// Hardcoded US QWERTY keycode-to-character mapping used as fallback
/// when UCKeyTranslate fails (e.g. missing layout data for CJK IMEs).
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

/// Hardcoded QWERTY keycode-to-name mapping used as fallback.
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

/// Build QWERTY-only fallback char maps using the hardcoded tables.
/// Used when layout data is unavailable (e.g. CJK IME without underlying layout).
static void buildQWERTYCharMaps(
    NSMutableDictionary<NSNumber *, NSString *> *unshifted, NSMutableDictionary<NSNumber *, NSString *> *shifted,
    NSMutableDictionary<NSNumber *, NSString *> *caps, NSMutableDictionary<NSNumber *, NSString *> *shiftedCaps) {
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

/// Populate name/code maps with QWERTY fallback entries.
/// Ensures special keys and basic key lookups work even without layout data.
static void buildQWERTYNameMaps(
    NSMutableDictionary<NSString *, NSNumber *> *nameToCode, NSMutableDictionary<NSNumber *, NSString *> *codeToName) {
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

/// Build special key maps which are layout-independent.
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
		@"F13" : @(kKeyCodeF13),
		@"F14" : @(kKeyCodeF14),
		@"F15" : @(kKeyCodeF15),
		@"F16" : @(kKeyCodeF16),
		@"F17" : @(kKeyCodeF17),
		@"F18" : @(kKeyCodeF18),
		@"F19" : @(kKeyCodeF19),
		@"F20" : @(kKeyCodeF20),
	} copy];

	NSMutableDictionary<NSNumber *, NSString *> *codeToName =
	    [NSMutableDictionary dictionaryWithCapacity:gSpecialNameToCodeMap.count];
	[gSpecialNameToCodeMap enumerateKeysAndObjectsUsingBlock:^(NSString *name, NSNumber *code, BOOL *stop) {
		if (!codeToName[code]) {
			codeToName[code] = name;
		}
	}];

	// Canonicalize duplicate names (Enter -> Return, Backspace -> Delete)
	codeToName[@(kKeyCodeReturn)] = @"Return";
	codeToName[@(kKeyCodeDelete)] = @"Delete";

	gSpecialCodeToNameMap = [codeToName copy];
}

/// Build layout-aware (layout-dependent) key maps.
/// Scans keycodes via UCKeyTranslate and merges with the layout-independent maps.
/// Locks gKeymapLock during the final map swap.
static void buildLayoutMaps(void) {
	@autoreleasepool {
		NSMutableDictionary<NSString *, NSNumber *> *nameToCode =
		    [NSMutableDictionary dictionaryWithDictionary:gSpecialNameToCodeMap];
		NSMutableDictionary<NSNumber *, NSString *> *codeToName =
		    [NSMutableDictionary dictionaryWithDictionary:gSpecialCodeToNameMap];

		// Pre-build modifier-variant char maps to avoid UCKeyTranslate calls at event time
		NSMutableDictionary<NSNumber *, NSString *> *unshifted = [NSMutableDictionary dictionary];
		NSMutableDictionary<NSNumber *, NSString *> *shifted = [NSMutableDictionary dictionary];
		NSMutableDictionary<NSNumber *, NSString *> *caps = [NSMutableDictionary dictionary];
		NSMutableDictionary<NSNumber *, NSString *> *shiftedCaps = [NSMutableDictionary dictionary];

		UInt32 shiftMod = (shiftKey >> 8) & 0xFF;
		UInt32 capsMod = (alphaLock >> 8) & 0xFF;
		UInt32 shiftCapsMod = shiftMod | capsMod;

		// Resolve the reference input source once and reuse it until config reload.
		// This keeps key interpretation stable across active layout switches.
		TISInputSourceRef inputSource = copyResolvedReferenceInputSource(NULL, NULL);

		if (!inputSource) {
			// No usable layout source — populate with QWERTY fallback so special
			// keys and basic key lookups still work.
			buildQWERTYNameMaps(nameToCode, codeToName);
			buildQWERTYCharMaps(unshifted, shifted, caps, shiftedCaps);

			[gKeymapLock lock];
			gKeyNameToCodeMap = [nameToCode copy];
			gKeyCodeToNameMap = [codeToName copy];
			gKeyCodeToCharUnshifted = [unshifted copy];
			gKeyCodeToCharShifted = [shifted copy];
			gKeyCodeToCharCaps = [caps copy];
			gKeyCodeToCharShiftedCaps = [shiftedCaps copy];
			[gKeymapLock unlock];
			return;
		}

		CFDataRef layoutData = (CFDataRef)TISGetInputSourceProperty(inputSource, kTISPropertyUnicodeKeyLayoutData);

		if (!layoutData) {
			// Layout source exists but has no uchr data — populate with QWERTY fallback.
			// Keep previous gCurrentInputSource/gCurrentKeyboardLayout if we had one
			// so live UCKeyTranslate fallback remains valid.
			CFRelease(inputSource);
			buildQWERTYNameMaps(nameToCode, codeToName);
			buildQWERTYCharMaps(unshifted, shifted, caps, shiftedCaps);

			[gKeymapLock lock];
			gKeyNameToCodeMap = [nameToCode copy];
			gKeyCodeToNameMap = [codeToName copy];
			gKeyCodeToCharUnshifted = [unshifted copy];
			gKeyCodeToCharShifted = [shifted copy];
			gKeyCodeToCharCaps = [caps copy];
			gKeyCodeToCharShiftedCaps = [shiftedCaps copy];
			[gKeymapLock unlock];
			return;
		}

		const UCKeyboardLayout *keyboardLayout = (const UCKeyboardLayout *)CFDataGetBytePtr(layoutData);

		// Scan printable keycodes and translate via current keyboard layout
		for (CGKeyCode keyCode = 0; keyCode <= kKeyCodeMaxPrintable; keyCode++) {
			// Skip keycodes already covered by the special key maps
			if (gSpecialCodeToNameMap[@(keyCode)]) {
				continue;
			}

			NSNumber *key = @(keyCode);

			NSString *ch = translateKeyCodeViaLayout(keyboardLayout, keyCode, 0);
			if (!ch || ch.length == 0) {
				// UCKeyTranslate failed — use QWERTY fallback
				ch = keyCodeToNameQWERTY(keyCode);
				if (!ch)
					continue;
			}

			NSString *upper = ch.uppercaseString;
			codeToName[key] = upper;

			// Add to name -> code mapping if not already present.
			// The first keycode wins if multiple keycodes produce the same character.
			if (!nameToCode[upper]) {
				nameToCode[upper] = key;
			}

			// Build char maps, falling back to QWERTY for each modifier variant
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

		// Atomically swap in the new combined maps
		NSDictionary *newNameToCode = [nameToCode copy];
		NSDictionary *newCodeToName = [codeToName copy];

		[gKeymapLock lock];
		if (gCurrentInputSource) {
			CFRelease(gCurrentInputSource);
		}
		// Take ownership of input source so the keyboard layout pointer
		// remains valid for the lifetime of gCurrentInputSource.
		gCurrentInputSource = inputSource;
		gCurrentKeyboardLayout = (const UCKeyboardLayout *)CFDataGetBytePtr(layoutData);

		gKeyNameToCodeMap = newNameToCode;
		gKeyCodeToNameMap = newCodeToName;
		gKeyCodeToCharUnshifted = [unshifted copy];
		gKeyCodeToCharShifted = [shifted copy];
		gKeyCodeToCharCaps = [caps copy];
		gKeyCodeToCharShiftedCaps = [shiftedCaps copy];
		[gKeymapLock unlock];
	}
}

#pragma mark - Layout Change Notification

/// Triggered by the system when the keyboard layout changes.
/// Note: This callback is invoked on the main thread by CFNotificationCenterGetDistributedCenter,
/// so unsynchronized access to gLayoutChangeDebounceBlock is safe. The debounced block is also
/// dispatched to the main queue, ensuring all access is serialized on the main thread.
static void handleKeyboardLayoutChanged(
    CFNotificationCenterRef center, void *observer, CFNotificationName name, const void *object,
    CFDictionaryRef userInfo) {
	[gKeymapLock lock];
	BOOL shouldRebuild = gUsesCurrentLayoutFallback;
	[gKeymapLock unlock];

	// With a resolved reference layout (configured or auto-detected), active
	// layout switches should not affect key interpretation.
	if (!shouldRebuild) {
		return;
	}

	// Debounce: cancel any pending rebuild and schedule a fresh one
	if (gLayoutChangeDebounceBlock) {
		dispatch_block_cancel(gLayoutChangeDebounceBlock);
	}

	gLayoutChangeDebounceBlock = dispatch_block_create(0, ^{
		[gKeymapLock lock];
		// Re-resolve when current-layout fallback is active so layout switches
		// pick up the new selected source.
		clearResolvedReferenceInputSourceLocked();
		[gKeymapLock unlock];

		buildLayoutMaps();

		KeymapLayoutChangeCallback cb = atomic_load(&gLayoutChangeCallback);
		if (cb)
			cb();

		gLayoutChangeDebounceBlock = nil;
	});

	dispatch_after(
	    dispatch_time(DISPATCH_TIME_NOW, (int64_t)(150 * NSEC_PER_MSEC)), dispatch_get_main_queue(),
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
	CFNotificationCenterAddObserver(
	    CFNotificationCenterGetDistributedCenter(), NULL, handleKeyboardLayoutChanged,
	    kTISNotifySelectedKeyboardInputSourceChanged, NULL, CFNotificationSuspensionBehaviorDeliverImmediately);
}

static void initializeKeyMaps(void) {
	static dispatch_once_t onceToken;
	dispatch_once(&onceToken, ^{
		gKeymapLock = [[NSLock alloc] init];

		initializeSpecialKeyMaps();

		// Register observer early, before any layout maps are built
		registerLayoutChangeObserver();
	});
}

/// Ensure layout maps are built (must be called after initializeKeyMaps).
/// Blocks until layout maps are available.
static void ensureLayoutMapsInitialized(void) {
	if (atomic_load_explicit(&gLayoutMapsInitialized, memory_order_acquire)) {
		return;
	}

	// TIS APIs must be called on the main thread.
	// Use dispatch_once to ensure single initialization, dispatching OUTSIDE of it.
	static dispatch_once_t layoutOnceToken;
	dispatch_once(&layoutOnceToken, ^{
		if ([NSThread isMainThread]) {
			buildLayoutMaps();
			atomic_store_explicit(&gLayoutMapsInitialized, true, memory_order_release);
		} else {
			// Dispatch to main thread to build layout maps
			dispatch_async(dispatch_get_main_queue(), ^{
				// Guard against double execution if the main thread already ran it
				if (!atomic_load_explicit(&gLayoutMapsInitialized, memory_order_acquire)) {
					buildLayoutMaps();
					atomic_store_explicit(&gLayoutMapsInitialized, true, memory_order_release);
				}
			});
		}
	});

	// If we're on the main thread and not yet initialized, run directly
	// to avoid deadlock (the async block is queued behind us).
	if (!atomic_load_explicit(&gLayoutMapsInitialized, memory_order_acquire) && [NSThread isMainThread]) {
		buildLayoutMaps();
		atomic_store_explicit(&gLayoutMapsInitialized, true, memory_order_release);
		return;
	}

	// Poll with timeout for all waiters.
	// Allows multiple concurrent background threads to proceed once
	// initialization is complete, rather than timing out individually.
	dispatch_time_t timeout = dispatch_time(DISPATCH_TIME_NOW, 5 * NSEC_PER_SEC);
	while (!atomic_load_explicit(&gLayoutMapsInitialized, memory_order_acquire)) {
		if (dispatch_time(DISPATCH_TIME_NOW, 0) >= timeout) {
			break;  // Timeout reached
		}
		[NSThread sleepForTimeInterval:0.001];  // 1ms sleep to avoid busy-wait
	}
}

#pragma mark - Public Functions

NSDictionary<NSString *, NSNumber *> *keyNameToCodeMap(void) {
	initializeKeyMaps();
	ensureLayoutMapsInitialized();

	[gKeymapLock lock];
	NSDictionary *map = gKeyNameToCodeMap;
	[gKeymapLock unlock];

	return map;
}

NSDictionary<NSNumber *, NSString *> *keyCodeToNameMap(void) {
	initializeKeyMaps();
	ensureLayoutMapsInitialized();

	[gKeymapLock lock];
	NSDictionary *map = gKeyCodeToNameMap;
	[gKeymapLock unlock];

	return map;
}

CGKeyCode keyNameToCode(NSString *keyName) {
	if (!keyName || keyName.length == 0) {
		return 0xFFFF;
	}

	initializeKeyMaps();
	ensureLayoutMapsInitialized();

	[gKeymapLock lock];
	NSDictionary *map = gKeyNameToCodeMap;
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

	[gKeymapLock lock];
	NSDictionary *map = gKeyCodeToNameMap;
	[gKeymapLock unlock];

	return map[@(keyCode)];
}

NSString *keyCodeToCharacter(CGKeyCode keyCode, CGEventFlags flags) {
	initializeKeyMaps();
	ensureLayoutMapsInitialized();

	// Layout-independent special keys
	switch (keyCode) {
	case kKeyCodeSpace:
		return @" ";
	case kKeyCodeReturn:
		return @"\r";
	case kKeyCodeTab:
		return @"\t";

	// Numpad keys are layout-independent
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

	// Attempt layout-aware translation via cached modifier maps
	BOOL hasShift = (flags & kCGEventFlagMaskShift) != 0;
	BOOL hasCaps = (flags & kCGEventFlagMaskAlphaShift) != 0;

	// Try cached map lookup first (no UCKeyTranslate calls for the four common modifier states)
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
	const UCKeyboardLayout *layout = gCurrentKeyboardLayout;
	TISInputSourceRef localSource = gCurrentInputSource;
	if (localSource)
		CFRetain(localSource);  // keep backing data alive
	[gKeymapLock unlock];

	if (map) {
		NSString *result = map[@(keyCode)];
		if (result) {
			if (localSource)
				CFRelease(localSource);
			return result;
		}
	}

	// Live translation for modifier states not covered by the cached maps
	if (layout) {
		// Build Carbon-style modifier state for UCKeyTranslate
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

	// Last resort: hardcoded US QWERTY fallback
	return keyCodeToCharacterQWERTY(keyCode, flags);
}

void refreshKeyboardLayoutMaps(void) {
	void (^cancelAndRebuild)(void) = ^{
		// Cancel any pending debounce rebuild before running synchronously
		if (gLayoutChangeDebounceBlock) {
			dispatch_block_cancel(gLayoutChangeDebounceBlock);
			gLayoutChangeDebounceBlock = nil;
		}

		initializeKeyMaps();

		// Clear cached reference so buildLayoutMaps re-resolves from scratch.
		// Without this the rebuild would be a no-op when a stable reference is locked.
		[gKeymapLock lock];
		clearResolvedReferenceInputSourceLocked();
		[gKeymapLock unlock];

		buildLayoutMaps();

		KeymapLayoutChangeCallback cb = atomic_load(&gLayoutChangeCallback);
		if (cb)
			cb();
	};

	if ([NSThread isMainThread]) {
		cancelAndRebuild();
	} else {
		dispatch_async(dispatch_get_main_queue(), cancelAndRebuild);
	}
}

int setReferenceKeyboardLayout(const char *inputSourceID) {
	// Normalise input — treat empty string as nil (auto-detect)
	NSString *trimmedInputSourceID = nil;
	if (inputSourceID) {
		trimmedInputSourceID =
		    [@(inputSourceID) stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceAndNewlineCharacterSet]];
		if (trimmedInputSourceID.length == 0) {
			trimmedInputSourceID = nil;
		}
	}

	__block BOOL configuredResolved = YES;

	void (^applyReferenceLayout)(void) = ^{
		initializeKeyMaps();

		[gKeymapLock lock];
		gConfiguredInputSourceID = [trimmedInputSourceID copy];
		clearResolvedReferenceInputSourceLocked();
		[gKeymapLock unlock];

		buildLayoutMaps();
		atomic_store_explicit(&gLayoutMapsInitialized, true, memory_order_release);

		[gKeymapLock lock];
		configuredResolved = gConfiguredInputSourceResolved;
		[gKeymapLock unlock];

		KeymapLayoutChangeCallback cb = atomic_load(&gLayoutChangeCallback);
		if (cb) {
			cb();
		}
	};

	if ([NSThread isMainThread]) {
		applyReferenceLayout();
	} else {
		// Dispatch to main thread and wait, with a once-guard to prevent double execution
		NSLock *applyLock = [[NSLock alloc] init];
		__block BOOL didApply = NO;

		void (^applyReferenceLayoutOnce)(void) = ^{
			[applyLock lock];
			BOOL shouldApply = !didApply;
			if (shouldApply) {
				didApply = YES;
			}
			[applyLock unlock];

			if (shouldApply) {
				applyReferenceLayout();
			}
		};

		dispatch_semaphore_t completed = dispatch_semaphore_create(0);
		dispatch_async(dispatch_get_main_queue(), ^{
			applyReferenceLayoutOnce();
			dispatch_semaphore_signal(completed);
		});

		dispatch_time_t timeout = dispatch_time(DISPATCH_TIME_NOW, (int64_t)(500 * NSEC_PER_MSEC));
		if (dispatch_semaphore_wait(completed, timeout) != 0) {
			// In tests/CLI runs the main queue may not be pumping — run directly
			applyReferenceLayoutOnce();
		}

		// Re-read the authoritative value under lock to avoid a data race:
		// the __block configuredResolved variable may be concurrently written
		// by the main queue thread and read here without synchronization.
		[gKeymapLock lock];
		configuredResolved = gConfiguredInputSourceResolved;
		[gKeymapLock unlock];
	}

	return configuredResolved ? 1 : 0;
}

void setKeymapLayoutChangeCallback(KeymapLayoutChangeCallback callback) {
	atomic_store(&gLayoutChangeCallback, callback);
}
