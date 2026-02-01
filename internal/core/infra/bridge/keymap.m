//
//  keymap.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "keymap.h"

#pragma mark - Static Data

static NSDictionary<NSString *, NSNumber *> *gKeyNameToCodeMap = nil;
static NSDictionary<NSNumber *, NSString *> *gKeyCodeToNameMap = nil;

#pragma mark - Initialization

static void initializeKeyMaps(void) {
	static dispatch_once_t onceToken;
	dispatch_once(&onceToken, ^{
		@autoreleasepool {
			// Build name -> code map
			gKeyNameToCodeMap = [@{
				// Special keys
				@"Space" : @(kKeyCodeSpace),
				@"Return" : @(kKeyCodeReturn),
				@"Enter" : @(kKeyCodeReturn),
				@"Escape" : @(kKeyCodeEscape),
				@"Tab" : @(kKeyCodeTab),
				@"Delete" : @(kKeyCodeDelete),
				@"Backspace" : @(kKeyCodeDelete),

				// Navigation keys
				@"Left" : @(kKeyCodeLeft),
				@"Right" : @(kKeyCodeRight),
				@"Down" : @(kKeyCodeDown),
				@"Up" : @(kKeyCodeUp),
				@"PageUp" : @(kKeyCodePageUp),
				@"PageDown" : @(kKeyCodePageDown),
				@"Home" : @(kKeyCodeHome),
				@"End" : @(kKeyCodeEnd),

				// Letters
				@"A" : @(kKeyCodeA),
				@"B" : @(kKeyCodeB),
				@"C" : @(kKeyCodeC),
				@"D" : @(kKeyCodeD),
				@"E" : @(kKeyCodeE),
				@"F" : @(kKeyCodeF),
				@"G" : @(kKeyCodeG),
				@"H" : @(kKeyCodeH),
				@"I" : @(kKeyCodeI),
				@"J" : @(kKeyCodeJ),
				@"K" : @(kKeyCodeK),
				@"L" : @(kKeyCodeL),
				@"M" : @(kKeyCodeM),
				@"N" : @(kKeyCodeN),
				@"O" : @(kKeyCodeO),
				@"P" : @(kKeyCodeP),
				@"Q" : @(kKeyCodeQ),
				@"R" : @(kKeyCodeR),
				@"S" : @(kKeyCodeS),
				@"T" : @(kKeyCodeT),
				@"U" : @(kKeyCodeU),
				@"V" : @(kKeyCodeV),
				@"W" : @(kKeyCodeW),
				@"X" : @(kKeyCodeX),
				@"Y" : @(kKeyCodeY),
				@"Z" : @(kKeyCodeZ),

				// Numbers
				@"0" : @(kKeyCode0),
				@"1" : @(kKeyCode1),
				@"2" : @(kKeyCode2),
				@"3" : @(kKeyCode3),
				@"4" : @(kKeyCode4),
				@"5" : @(kKeyCode5),
				@"6" : @(kKeyCode6),
				@"7" : @(kKeyCode7),
				@"8" : @(kKeyCode8),
				@"9" : @(kKeyCode9),

				// Symbols
				@"=" : @(kKeyCodeEqual),
				@"-" : @(kKeyCodeMinus),
				@"]" : @(kKeyCodeRightBracket),
				@"[" : @(kKeyCodeLeftBracket),
				@"'" : @(kKeyCodeQuote),
				@";" : @(kKeyCodeSemicolon),
				@"\\" : @(kKeyCodeBackslash),
				@"," : @(kKeyCodeComma),
				@"/" : @(kKeyCodeSlash),
				@"." : @(kKeyCodePeriod),
				@"`" : @(kKeyCodeBacktick),

				// Function keys
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

			// Build code -> name map by inverting the name -> code map
			NSMutableDictionary<NSNumber *, NSString *> *codeToName =
			    [NSMutableDictionary dictionaryWithCapacity:gKeyNameToCodeMap.count];
			[gKeyNameToCodeMap enumerateKeysAndObjectsUsingBlock:^(NSString *name, NSNumber *code, BOOL *stop) {
				// Only add if not already present (handles duplicates like Return/Enter)
				if (!codeToName[code]) {
					codeToName[code] = name;
				}
			}];

			// Canonicalize some names
			codeToName[@(kKeyCodeReturn)] = @"Return";
			codeToName[@(kKeyCodeDelete)] = @"Delete";

			gKeyCodeToNameMap = [codeToName copy];
		}
	});
}

#pragma mark - Public Functions

NSDictionary<NSString *, NSNumber *> *keyNameToCodeMap(void) {
	initializeKeyMaps();
	return gKeyNameToCodeMap;
}

NSDictionary<NSNumber *, NSString *> *keyCodeToNameMap(void) {
	initializeKeyMaps();
	return gKeyCodeToNameMap;
}

CGKeyCode keyNameToCode(NSString *keyName) {
	if (!keyName || keyName.length == 0) {
		return 0xFFFF;
	}

	initializeKeyMaps();

	NSNumber *code = gKeyNameToCodeMap[keyName];
	if (!code) {
		// Try uppercase version
		code = gKeyNameToCodeMap[keyName.uppercaseString];
	}

	return code ? code.unsignedShortValue : 0xFFFF;
}

NSString *keyCodeToName(CGKeyCode keyCode) {
	initializeKeyMaps();
	return gKeyCodeToNameMap[@(keyCode)];
}

NSString *keyCodeToCharacter(CGKeyCode keyCode, CGEventFlags flags) {
	BOOL hasShift = (flags & kCGEventFlagMaskShift) != 0;
	BOOL hasCapsLock = (flags & kCGEventFlagMaskAlphaShift) != 0;
	BOOL uppercase = hasShift != hasCapsLock;

	switch (keyCode) {
	// Letters
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

	// Numbers (shifted symbols)
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

	// Symbols
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

	// Special keys
	case kKeyCodeSpace:
		return @" ";
	case kKeyCodeReturn:
		return @"\r";
	case kKeyCodeTab:
		return @"\t";

	// Numpad
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
		return nil;
	}
}
