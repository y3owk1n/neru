//
//  eventtap.h
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#ifndef EVENTTAP_H
#define EVENTTAP_H

#import <Foundation/Foundation.h>

#pragma mark - Modifier Key Constants

/// Modifier key flags (must match across C, ObjC, and Go)
typedef enum {
	ModifierNone = 0,
	ModifierCmd = 1 << 0,
	ModifierShift = 1 << 1,
	ModifierAlt = 1 << 2,
	ModifierCtrl = 1 << 3,
} ModifierKey;

#pragma mark - Type Definitions

/// Event tap callback type
/// @param key Pressed key
/// @param userData User data pointer
typedef void (*EventTapCallback)(const char *key, void *userData);

/// Passthrough notification callback type.
/// Invoked when a modifier shortcut passes through to macOS.
/// @param userData User data pointer
typedef void (*EventTapPassthroughCallback)(void *userData);

/// Per-hotkey tap callback type.
/// @param hotkeyID Registered hotkey identifier
/// @param eventKind 1 = pressed, 2 = released
/// @param userData User data pointer
typedef void (*HotkeyTapCallback)(int hotkeyID, int eventKind, void *userData);

/// Per-hotkey tap handle
typedef void *HotkeyTapRef;

/// Event tap handle
typedef void *EventTap;

#pragma mark - Event Tap Functions

/// Create event tap
/// @param callback Callback function
/// @param userData User data pointer
/// @return Event tap handle
EventTap NeruCreateEventTap(EventTapCallback callback, void *userData);

/// Enable event tap
/// @param tap Event tap handle
void NeruEnableEventTap(EventTap tap);

/// Disable event tap
/// @param tap Event tap handle
void NeruDisableEventTap(EventTap tap);

/// Destroy event tap
/// @param tap Event tap handle
void NeruDestroyEventTap(EventTap tap);

/// Set event tap hotkeys
/// @param tap Event tap handle
/// @param hotkeys Array of hotkey strings
/// @param count Number of hotkeys
void NeruSetEventTapHotkeys(EventTap tap, const char **hotkeys, int count);

/// Set event tap modifier passthrough behavior.
/// When enabled, modifier shortcuts that are not claimed by the active mode are
/// passed through to macOS. Blacklisted shortcuts remain consumed by Neru.
/// @param tap Event tap handle
/// @param enabled Non-zero to enable passthrough
/// @param blacklistKeys Array of blacklisted key strings (same format as hotkeys)
/// @param count Number of blacklisted keys
void NeruSetEventTapModifierPassthrough(EventTap tap, int enabled, const char **blacklistKeys, int count);

/// Set modifier shortcuts that the active mode still wants Neru to consume.
/// @param tap Event tap handle
/// @param keys Array of key strings (same format as hotkeys)
/// @param count Number of keys
void NeruSetEventTapInterceptedModifierKeys(EventTap tap, const char **keys, int count);

/// Set callback invoked when a modifier shortcut passes through to macOS.
/// The callback is delivered asynchronously after the passthrough is observed.
/// @param tap Event tap handle
/// @param callback Passthrough callback function (may be NULL to clear)
void NeruSetEventTapPassthroughCallback(EventTap tap, EventTapPassthroughCallback callback);

/// Enable or disable sticky modifier toggle detection.
/// When enabled, modifier key events (Shift, Cmd, Alt, Ctrl) are detected and
/// callback is invoked with "__modifier_<name>_down/up" strings for sticky modifier toggling.
/// @param tap Event tap handle
/// @param enabled Non-zero to enable, zero to disable
void NeruSetEventTapStickyModifierToggle(EventTap tap, int enabled);

#pragma mark - Per-Hotkey CGEventTap

/// Create a per-hotkey CGEventTap that captures a single key+modifier combo.
/// The tap is always active and
/// consumes the event when matched.
/// @param hotkeyID Opaque identifier passed back in the callback
/// @param keyCode CG keycode of the hotkey
/// @param modifiers ModifierKey bitmask
/// @param callback Callback invoked on press/release
/// @param userData User data pointer
/// @return Hotkey tap handle, or NULL on failure (e.g. no Accessibility permission)
HotkeyTapRef NeruCreateHotkeyTap(int hotkeyID, int keyCode, int modifiers, HotkeyTapCallback callback, void *userData);

/// Destroy a per-hotkey CGEventTap previously created by NeruCreateHotkeyTap.
/// @param tap Hotkey tap handle (NULL-safe)
void NeruDestroyHotkeyTap(HotkeyTapRef tap);

#pragma mark - Key String Parsing

/// Parse a key string (e.g., "Cmd+Shift+Space") into a key code and modifier bitmask.
/// @param keyString Key string to parse
/// @param keyCode Output parameter for key code
/// @param modifiers Output parameter for modifier bitmask
/// @return 1 on success, 0 on failure
int NeruParseKeyString(const char *keyString, int *keyCode, int *modifiers);

#pragma mark - Standalone Utilities

/// Post a physical modifier key-down or key-up event to macOS.
/// This is a standalone utility that does not require an event tap handle.
/// @param modifier One of "cmd", "shift", "alt", "ctrl"
/// @param isDown Non-zero for key-down, zero for key-up
void NeruPostEventTapModifierEvent(const char *modifier, int isDown);

#endif /* EVENTTAP_H */
