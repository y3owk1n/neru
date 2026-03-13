//
//  hotkeys.h
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#ifndef HOTKEYS_H
#define HOTKEYS_H

#import <Foundation/Foundation.h>

#pragma mark - Type Definitions

/// Hotkey callback type
/// @param hotkeyId Hotkey identifier
/// @param userData User data pointer
typedef void (*HotkeyCallback)(int hotkeyId, void *userData);

/// Modifier keys
typedef enum {
	ModifierNone = 0,       ///< No modifier
	ModifierCmd = 1 << 0,   ///< Command key
	ModifierShift = 1 << 1, ///< Shift key
	ModifierAlt = 1 << 2,   ///< Alt/Option key
	ModifierCtrl = 1 << 3   ///< Control key
} ModifierKey;

#pragma mark - Hotkey Functions

/// Register hotkey
/// @param keyCode Key code
/// @param modifiers Modifier keys
/// @param hotkeyId Hotkey identifier
/// @param callback Callback function
/// @param userData User data pointer
/// @return 1 on success, 0 on failure
int registerHotkey(int keyCode, int modifiers, int hotkeyId, HotkeyCallback callback, void *userData);

/// Unregister hotkey
/// @param hotkeyId Hotkey identifier
void unregisterHotkey(int hotkeyId);

/// Unregister all hotkeys
void unregisterAllHotkeys(void);

/// Parse key string
/// @param keyString Key string (e.g., "Cmd+Shift+Space")
/// @param keyCode Output parameter for key code
/// @param modifiers Output parameter for modifiers
/// @return 1 on success, 0 on failure
int parseKeyString(const char *keyString, int *keyCode, int *modifiers);

#endif // HOTKEYS_H
