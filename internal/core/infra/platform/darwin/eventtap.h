//
//  eventtap.h
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#ifndef EVENTTAP_H
#define EVENTTAP_H

#import <Foundation/Foundation.h>

#pragma mark - Type Definitions

/// Event tap callback type
/// @param key Pressed key
/// @param userData User data pointer
typedef void (*EventTapCallback)(const char *key, void *userData);

/// Passthrough notification callback type.
/// Invoked when a modifier shortcut passes through to macOS.
/// @param userData User data pointer
typedef void (*EventTapPassthroughCallback)(void *userData);

/// Event tap handle
typedef void *EventTap;

#pragma mark - Event Tap Functions

/// Create event tap
/// @param callback Callback function
/// @param userData User data pointer
/// @return Event tap handle
EventTap createEventTap(EventTapCallback callback, void *userData);

/// Enable event tap
/// @param tap Event tap handle
void enableEventTap(EventTap tap);

/// Disable event tap
/// @param tap Event tap handle
void disableEventTap(EventTap tap);

/// Destroy event tap
/// @param tap Event tap handle
void destroyEventTap(EventTap tap);

/// Set event tap hotkeys
/// @param tap Event tap handle
/// @param hotkeys Array of hotkey strings
/// @param count Number of hotkeys
void setEventTapHotkeys(EventTap tap, const char **hotkeys, int count);

/// Set event tap modifier passthrough behavior.
/// When enabled, modifier shortcuts that are not claimed by the active mode are
/// passed through to macOS. Blacklisted shortcuts remain consumed by Neru.
/// @param tap Event tap handle
/// @param enabled Non-zero to enable passthrough
/// @param blacklistKeys Array of blacklisted key strings (same format as hotkeys)
/// @param count Number of blacklisted keys
void setEventTapModifierPassthrough(EventTap tap, int enabled, const char **blacklistKeys, int count);

/// Set modifier shortcuts that the active mode still wants Neru to consume.
/// @param tap Event tap handle
/// @param keys Array of key strings (same format as hotkeys)
/// @param count Number of keys
void setEventTapInterceptedModifierKeys(EventTap tap, const char **keys, int count);

/// Set callback invoked when a modifier shortcut passes through to macOS.
/// The callback is delivered asynchronously after the passthrough is observed.
/// @param tap Event tap handle
/// @param callback Passthrough callback function (may be NULL to clear)
void setEventTapPassthroughCallback(EventTap tap, EventTapPassthroughCallback callback);

/// Enable or disable sticky modifier toggle detection.
/// When enabled, modifier key events (Shift, Cmd, Alt, Ctrl) are detected and
/// callback is invoked with "__modifier_<name>" strings for sticky modifier toggling.
/// @param tap Event tap handle
/// @param enabled Non-zero to enable, zero to disable
void setEventTapStickyModifierToggle(EventTap tap, int enabled);
void postEventTapModifierEvent(const char* modifier, int isDown);

#endif /* EVENTTAP_H */
