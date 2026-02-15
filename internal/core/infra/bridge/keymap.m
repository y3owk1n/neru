//
//  keymap.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "keymap.h"

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

/// max virtual key code to scan when building layout maps
/// ADB keyboards use keycodes 0-127, but printable keys are below 50.
static const CGKeyCode kMaxPrintableKeyCode = 50;

#pragma mark - UCKeyTranslate Helper

/// figure out which character string each virtual keycode maps to
/// read the physical keyboard layout via TISCopyCurrentKeyboardLayoutInputSource

/// @param keyCode Virtual key code (0-127)
/// @param modifierState Carbon-style modifier state ((EventRecord.modifiers >> 8) & 0xFF)
/// @return Character string, or nil if translation fails
static NSString *translateKeyCodeViaLayout(CGKeyCode keyCode, UInt32 modifierState) {
	UInt32 deadKeyState = 0;
	UniChar chars[4];
	UniCharCount actualLength = 0;

	OSStatus status = UCKeyTranslate(gCurrentKeyboardLayout, keyCode, kUCKeyActionDown, modifierState,
	                                 LMGetKbdType(), kUCKeyTranslateNoDeadKeysBit, &deadKeyState,
	                                 sizeof(chars) / sizeof(chars[0]), &actualLength, chars);

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

	NSMutableDictionary<NSNumber *, NSString *> *codeToName = [NSMutableDictionary dictionaryWithCapacity:gSpecialNameToCodeMap.count];
	[gSpecialNameToCodeMap
	    enumerateKeysAndObjectsUsingBlock:^(NSString *name, NSNumber *code, BOOL *stop) {
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
		NSMutableDictionary<NSString *, NSNumber *> *nameToCode = [NSMutableDictionary dictionaryWithDictionary:gSpecialNameToCodeMap];
		NSMutableDictionary<NSNumber *, NSString *> *codeToName = [NSMutableDictionary dictionaryWithDictionary:gSpecialCodeToNameMap];

		/// try to get the current keyboard layout
		TISInputSourceRef inputSource = TISCopyCurrentKeyboardLayoutInputSource();
		if (!inputSource) {
			return;
		}

		CFDataRef layoutData =
			(CFDataRef)TISGetInputSourceProperty(inputSource, kTISPropertyUnicodeKeyLayoutData);
		if (!layoutData) {
			CFRelease(inputSource);
			return;
		}

		if (gCurrentInputSource) {
			CFRelease(gCurrentInputSource);
		}

		// take ownership of input source so keyboard layout is
		// valid while gCurrentInputSource lives
		gCurrentInputSource = inputSource;  
		gCurrentKeyboardLayout = (const UCKeyboardLayout *)CFDataGetBytePtr(layoutData);  

		// scan printable keycodes and translate via current keyboard layout
		for (CGKeyCode keyCode = 0; keyCode <= kMaxPrintableKeyCode; keyCode++) {
			// skip keycodes already covered by special key maps
			if (gSpecialCodeToNameMap[@(keyCode)]) {
				continue;
			}

			NSString *ch = translateKeyCodeViaLayout(keyCode, 0);
			if (!ch || ch.length == 0) {
				// UCKeyTranslate failed use QWERTY fallback
				ch = keyCodeToNameQWERTY(keyCode);
				if (!ch) {
					continue;
				}
			}

			NSString *upper = ch.uppercaseString;
			codeToName[@(keyCode)] = upper;

			// add to name -> code mapping if not already present
			// (the result is that the first keycode wins if it so 
			// happens that multiple keycodes produce the same character)
			if (!nameToCode[upper]) {
				nameToCode[upper] = @(keyCode);
			}
		}

		// atomic swap combined maps
		NSDictionary *newNameToCode = [nameToCode copy];
		NSDictionary *newCodeToName = [codeToName copy];

		[gKeymapLock lock];
		gKeyNameToCodeMap = newNameToCode;
		gKeyCodeToNameMap = newCodeToName;
		[gKeymapLock unlock];
	}
}

#pragma mark - Layout Change Notification

/// triggered by the system when keyboard layout changed to trigger rebuild
static void handleKeyboardLayoutChanged(CFNotificationCenterRef center, void *observer,
                                        CFNotificationName name, const void *object,
                                        CFDictionaryRef userInfo) {
	buildLayoutMaps();
}

#pragma mark - Initialization

static void initializeKeyMaps(void) {
	static dispatch_once_t onceToken;
	dispatch_once(&onceToken, ^{
		gKeymapLock = [[NSLock alloc] init];

		initializeSpecialKeyMaps();
		buildLayoutMaps();

		// trigger keymap rebuild on keyboard layout change
		CFNotificationCenterAddObserver(
		    CFNotificationCenterGetDistributedCenter(), NULL, handleKeyboardLayoutChanged,
		    kTISNotifySelectedKeyboardInputSourceChanged, NULL,
		    CFNotificationSuspensionBehaviorDeliverImmediately);
	});
}

#pragma mark - Public Functions

NSDictionary<NSString *, NSNumber *> *keyNameToCodeMap(void) {
	initializeKeyMaps();

	// lock to prevent conflicts with rebuild
	[gKeymapLock lock];
	NSDictionary *map = gKeyNameToCodeMap;
	[gKeymapLock unlock];

	return map;
}

NSDictionary<NSNumber *, NSString *> *keyCodeToNameMap(void) {
	initializeKeyMaps();

	// lock to prevent conflicts with rebuild
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

	// lock to prevent conflicts with rebuild
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

	// lock to prevent conflicts with rebuild
	[gKeymapLock lock];
	NSDictionary *map = gKeyCodeToNameMap;
	[gKeymapLock unlock];

	return map[@(keyCode)];
}

NSString *keyCodeToCharacter(CGKeyCode keyCode, CGEventFlags flags) {
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

	// build Carbon-style modifier state for UCKeyTranslate
	UInt32 modifierState = 0;
	if (flags & kCGEventFlagMaskShift) {
		modifierState |= (shiftKey >> 8) & 0xFF;
	}
	if (flags & kCGEventFlagMaskAlphaShift) {
		modifierState |= (alphaLock >> 8) & 0xFF;
	}

	// attempt layout-aware translation first
	NSString *result = translateKeyCodeViaLayout(keyCode, modifierState);
	if (result && result.length > 0) {
		return result;
	}

	// hardcoded US QWERTY fallback if layout translation fails
	return keyCodeToCharacterQWERTY(keyCode, flags);
}

void refreshKeyboardLayoutMaps(void) {
	initializeKeyMaps();
	buildLayoutMaps();
}
