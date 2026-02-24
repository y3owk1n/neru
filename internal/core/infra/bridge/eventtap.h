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

#endif // EVENTTAP_H
