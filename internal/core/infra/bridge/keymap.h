//
//  keymap.h
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#ifndef KEYMAP_H
#define KEYMAP_H

#import <Carbon/Carbon.h>
#import <Foundation/Foundation.h>

#pragma mark - Key Code Constants

typedef NS_ENUM(uint16_t, KeyCode) {
	// Special keys
	kKeyCodeSpace = 49,
	kKeyCodeReturn = 36,
	kKeyCodeEscape = 53,
	kKeyCodeTab = 48,
	kKeyCodeDelete = 51,

	// Navigation keys
	kKeyCodeLeft = 123,
	kKeyCodeRight = 124,
	kKeyCodeDown = 125,
	kKeyCodeUp = 126,
	kKeyCodePageUp = 116,
	kKeyCodePageDown = 121,
	kKeyCodeHome = 115,
	kKeyCodeEnd = 119,

	// Letters
	kKeyCodeA = 0,
	kKeyCodeB = 11,
	kKeyCodeC = 8,
	kKeyCodeD = 2,
	kKeyCodeE = 14,
	kKeyCodeF = 3,
	kKeyCodeG = 5,
	kKeyCodeH = 4,
	kKeyCodeI = 34,
	kKeyCodeJ = 38,
	kKeyCodeK = 40,
	kKeyCodeL = 37,
	kKeyCodeM = 46,
	kKeyCodeN = 45,
	kKeyCodeO = 31,
	kKeyCodeP = 35,
	kKeyCodeQ = 12,
	kKeyCodeR = 15,
	kKeyCodeS = 1,
	kKeyCodeT = 17,
	kKeyCodeU = 32,
	kKeyCodeV = 9,
	kKeyCodeW = 13,
	kKeyCodeX = 7,
	kKeyCodeY = 16,
	kKeyCodeZ = 6,

	// Numbers
	kKeyCode0 = 29,
	kKeyCode1 = 18,
	kKeyCode2 = 19,
	kKeyCode3 = 20,
	kKeyCode4 = 21,
	kKeyCode5 = 23,
	kKeyCode6 = 22,
	kKeyCode7 = 26,
	kKeyCode8 = 28,
	kKeyCode9 = 25,

	// Symbols
	kKeyCodeEqual = 24,
	kKeyCodeMinus = 27,
	kKeyCodeRightBracket = 30,
	kKeyCodeLeftBracket = 33,
	kKeyCodeQuote = 39,
	kKeyCodeSemicolon = 41,
	kKeyCodeBackslash = 42,
	kKeyCodeComma = 43,
	kKeyCodeSlash = 44,
	kKeyCodePeriod = 47,
	kKeyCodeBacktick = 50,

	// Function keys
	kKeyCodeF1 = 122,
	kKeyCodeF2 = 120,
	kKeyCodeF3 = 99,
	kKeyCodeF4 = 118,
	kKeyCodeF5 = 96,
	kKeyCodeF6 = 97,
	kKeyCodeF7 = 98,
	kKeyCodeF8 = 100,
	kKeyCodeF9 = 101,
	kKeyCodeF10 = 109,
	kKeyCodeF11 = 103,
	kKeyCodeF12 = 111,

	// Numpad
	kKeyCodeNumpadDot = 65,
	kKeyCodeNumpadMultiply = 67,
	kKeyCodeNumpadPlus = 69,
	kKeyCodeNumpadClear = 71,
	kKeyCodeNumpadDivide = 75,
	kKeyCodeNumpadEnter = 76,
	kKeyCodeNumpadMinus = 78,
	kKeyCodeNumpadEquals = 81,
	kKeyCodeNumpad0 = 82,
	kKeyCodeNumpad1 = 83,
	kKeyCodeNumpad2 = 84,
	kKeyCodeNumpad3 = 85,
	kKeyCodeNumpad4 = 86,
	kKeyCodeNumpad5 = 87,
	kKeyCodeNumpad6 = 88,
	kKeyCodeNumpad7 = 89,
	kKeyCodeNumpad8 = 91,
	kKeyCodeNumpad9 = 92,
};

#pragma mark - Key Mapping Functions

/// Returns the shared key name to keycode mapping dictionary
/// Keys: "Space", "Return", "A", "1", "F1", etc.
/// Values: NSNumber containing CGKeyCode
NSDictionary<NSString *, NSNumber *> *keyNameToCodeMap(void);

/// Returns the shared keycode to name mapping dictionary
/// Keys: NSNumber containing CGKeyCode
/// Values: "Space", "Return", "A", "1", "F1", etc.
NSDictionary<NSNumber *, NSString *> *keyCodeToNameMap(void);

/// Map key name to keycode (case-insensitive)
/// @param keyName Key name like "Space", "Return", "A", "1"
/// @return Keycode or 0xFFFF if not found
CGKeyCode keyNameToCode(NSString *keyName);

/// Map keycode to key name
/// @param keyCode Key code
/// @return Key name or nil if not found
NSString *keyCodeToName(CGKeyCode keyCode);

/// Map keycode to character with shift/capslock handling
/// @param keyCode Key code
/// @param flags Event flags (for shift/capslock detection)
/// @return Character string or nil if not found
NSString *keyCodeToCharacter(CGKeyCode keyCode, CGEventFlags flags);

#endif // KEYMAP_H
