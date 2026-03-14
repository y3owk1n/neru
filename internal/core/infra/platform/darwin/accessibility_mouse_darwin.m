//
//  accessibility_mouse.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "accessibility.h"
#import "accessibility_constants.h"
#import "accessibility_visibility.h"
#import <Cocoa/Cocoa.h>
#include <sys/time.h>

#pragma mark - Mouse Functions

/// Move mouse cursor to position with specified event type
/// @param position Target position
/// @param eventType CGEvent type (kCGEventMouseMoved or kCGEventLeftMouseDragged)
void moveMouseWithType(CGPoint position, CGEventType eventType) {
	CGEventRef move = CGEventCreateMouseEvent(NULL, eventType, position, kCGMouseButtonLeft);
	if (move) {
		CGEventSetFlags(move, 0);
		CGEventPost(kCGHIDEventTap, move);
		CFRelease(move);
		CFRunLoopRunInMode(kCFRunLoopDefaultMode, kNeruMouseMoveDelay, false);
	}
}

/// Post a single mouse move event (non-blocking, for async animation)
/// @param position Target position
/// @param eventType CGEvent type (kCGEventMouseMoved or kCGEventLeftMouseDragged)
void postMouseMoveEvent(CGPoint position, CGEventType eventType) {
	CGEventRef move = CGEventCreateMouseEvent(NULL, eventType, position, kCGMouseButtonLeft);
	if (move) {
		CGEventSetFlags(move, 0);
		CGEventPost(kCGHIDEventTap, move);
		CFRelease(move);
	}
}

#pragma mark - Mouse Action Functions

/// Release the left button without moving
/// @return 1 on success, 0 on failure
int performLeftMouseUpAtCursor(void) {
	CGEventRef currentEvent = CGEventCreate(NULL);
	if (!currentEvent)
		return 0;

	CGPoint currentPos = CGEventGetLocation(currentEvent);
	CFRelease(currentEvent);

	CGEventRef up = CGEventCreateMouseEvent(NULL, kCGEventLeftMouseUp, currentPos, kCGMouseButtonLeft);
	if (!up)
		return 0;

	// Clear all modifier flags to ensure clean mouse up
	CGEventSetFlags(up, 0);
	CGEventPost(kCGHIDEventTap, up);
	CFRelease(up);

	CFRunLoopRunInMode(kCFRunLoopDefaultMode, kNeruMouseClickProcessingDelay, false);
	return 1;
}

/// Generic click at position
/// @param pos Target position
/// @param downEvent Mouse down event type
/// @param upEvent Mouse up event type
/// @param button Mouse button
/// @param restoreCursor Whether to restore cursor position after click
/// @param flags CGEventFlags for modifier keys (0 for none)
/// @return 1 on success, 0 on failure
static int performClickAtPosition(CGPoint pos, CGEventType downEvent, CGEventType upEvent, CGMouseButton button,
                                  bool restoreCursor, CGEventFlags flags) {
	CGPoint originalPosition = CGPointZero;
	if (restoreCursor) {
		CGEventRef currentEvent = CGEventCreate(NULL);
		if (currentEvent) {
			originalPosition = CGEventGetLocation(currentEvent);
			CFRelease(currentEvent);
		}
	}

	moveMouseWithType(pos, kCGEventMouseMoved);

	CGEventRef down = CGEventCreateMouseEvent(NULL, downEvent, pos, button);
	CGEventRef up = CGEventCreateMouseEvent(NULL, upEvent, pos, button);
	if (!down || !up) {
		if (down)
			CFRelease(down);
		if (up)
			CFRelease(up);
		if (restoreCursor)
			moveMouseWithType(originalPosition, kCGEventMouseMoved);
		return 0;
	}
	// Set modifier flags (0 = no modifiers for clean click)
	CGEventSetFlags(down, flags);
	CGEventSetFlags(up, flags);

	// Post mouse down, allow the system to process it, then post mouse up.
	CGEventPost(kCGHIDEventTap, down);
	// Give the event loop a short moment to register the down event before sending up.
	CFRunLoopRunInMode(kCFRunLoopDefaultMode, kNeruMouseClickDownUpDelay, false);

	CGEventPost(kCGHIDEventTap, up);
	CFRelease(down);
	CFRelease(up);

	// Allow a small amount of time for the click to be processed by the system
	// before restoring the cursor to avoid clicks landing in-transit.
	CFRunLoopRunInMode(kCFRunLoopDefaultMode, kNeruMouseClickProcessingDelay, false);

	if (restoreCursor)
		moveMouseWithType(originalPosition, kCGEventMouseMoved);
	return 1;
}

/// State tracking for click detection
static struct {
	CGPoint lastPosition;         ///< Last click position
	struct timeval lastClickTime; ///< Last click time
	int clickCount;               ///< Current click count
} clickState = {0};

/// Get current time in milliseconds
/// @return Current time in milliseconds
static long long getCurrentTimeMs(void) {
	struct timeval tv;
	gettimeofday(&tv, NULL);
	return (long long)tv.tv_sec * 1000 + tv.tv_usec / 1000;
}

/// Perform left click at position
/// @param position Target position
/// @param restoreCursor Whether to restore cursor position after click
/// @return 1 on success, 0 on failure
int performLeftClickAtPosition(CGPoint position, bool restoreCursor, CGEventFlags flags) {
	CGPoint originalPosition = CGPointZero;
	if (restoreCursor) {
		CGEventRef currentEvent = CGEventCreate(NULL);
		if (currentEvent) {
			originalPosition = CGEventGetLocation(currentEvent);
			CFRelease(currentEvent);
		}
	}

	// Get current time
	long long currentTime = getCurrentTimeMs();
	long long lastTime = (long long)clickState.lastClickTime.tv_sec * 1000 + clickState.lastClickTime.tv_usec / 1000;
	long long timeDiff = currentTime - lastTime;

	// Check if this is a multi-click (within double-click interval and at same position)
	// macOS typically uses 500ms as the double-click interval
	double distance =
	    sqrt(pow(position.x - clickState.lastPosition.x, 2) + pow(position.y - clickState.lastPosition.y, 2));

	if (timeDiff < kNeruDoubleClickIntervalMs && distance < kNeruDoubleClickDistancePoints) {
		// Same location, quick succession - increment click count
		clickState.clickCount++;
	} else {
		// New click sequence
		clickState.clickCount = 1;
	}

	// Update state
	clickState.lastPosition = position;
	gettimeofday(&clickState.lastClickTime, NULL);

	moveMouseWithType(position, kCGEventMouseMoved);

	CGEventRef down = CGEventCreateMouseEvent(NULL, kCGEventLeftMouseDown, position, kCGMouseButtonLeft);
	CGEventRef up = CGEventCreateMouseEvent(NULL, kCGEventLeftMouseUp, position, kCGMouseButtonLeft);

	if (!down || !up) {
		if (down)
			CFRelease(down);
		if (up)
			CFRelease(up);
		if (restoreCursor)
			moveMouseWithType(originalPosition, kCGEventMouseMoved);
		return 0;
	}

	// Set modifier flags (0 = no modifiers for clean click)
	CGEventSetFlags(down, flags);
	CGEventSetFlags(up, flags);

	// Set the click count
	CGEventSetIntegerValueField(down, kCGMouseEventClickState, clickState.clickCount);
	CGEventSetIntegerValueField(up, kCGMouseEventClickState, clickState.clickCount);

	// Post mouse down and allow a short moment before posting mouse up to ensure
	// the system attributes the down/up pair to the target location.
	CGEventPost(kCGHIDEventTap, down);
	CFRunLoopRunInMode(kCFRunLoopDefaultMode, kNeruMouseClickDownUpDelay, false);

	CGEventPost(kCGHIDEventTap, up);
	CFRelease(down);
	CFRelease(up);

	// Wait briefly to let the OS process the click before potentially moving the cursor back.
	CFRunLoopRunInMode(kCFRunLoopDefaultMode, kNeruMouseClickProcessingDelay, false);

	if (restoreCursor)
		moveMouseWithType(originalPosition, kCGEventMouseMoved);
	return 1;
}

/// Perform right click at position
/// @param position Target position
/// @param restoreCursor Whether to restore cursor position after click
/// @return 1 on success, 0 on failure
int performRightClickAtPosition(CGPoint position, bool restoreCursor, CGEventFlags flags) {
	return performClickAtPosition(position, kCGEventRightMouseDown, kCGEventRightMouseUp, kCGMouseButtonRight,
	                              restoreCursor, flags);
}

/// Perform middle click at position
/// @param position Target position
/// @param restoreCursor Whether to restore cursor position after click
/// @return 1 on success, 0 on failure
int performMiddleClickAtPosition(CGPoint position, bool restoreCursor, CGEventFlags flags) {
	return performClickAtPosition(position, kCGEventOtherMouseDown, kCGEventOtherMouseUp, kCGMouseButtonCenter,
	                              restoreCursor, flags);
}

/// Perform left mouse down at position
/// @param position Target position
/// @return 1 on success, 0 on failure
int performLeftMouseDownAtPosition(CGPoint position, CGEventFlags flags) {
	moveMouseWithType(position, kCGEventMouseMoved);
	CFRunLoopRunInMode(kCFRunLoopDefaultMode, kNeruMouseClickProcessingDelay, false);
	CGEventRef down = CGEventCreateMouseEvent(NULL, kCGEventLeftMouseDown, position, kCGMouseButtonLeft);
	if (!down)
		return 0;
	// Set modifier flags (0 = no modifiers for clean mouse down)
	CGEventSetFlags(down, flags);
	CGEventPost(kCGHIDEventTap, down);
	CFRelease(down);
	CFRunLoopRunInMode(kCFRunLoopDefaultMode, kNeruMouseClickProcessingDelay, false);
	return 1;
}

/// Perform left mouse up at position
/// @param position Target position
/// @return 1 on success, 0 on failure
int performLeftMouseUpAtPosition(CGPoint position, CGEventFlags flags) {
	moveMouseWithType(position, kCGEventMouseMoved);
	CFRunLoopRunInMode(kCFRunLoopDefaultMode, kNeruMouseClickProcessingDelay, false);
	CGEventRef up = CGEventCreateMouseEvent(NULL, kCGEventLeftMouseUp, position, kCGMouseButtonLeft);
	if (!up)
		return 0;
	// Set modifier flags (0 = no modifiers for clean mouse up)
	CGEventSetFlags(up, flags);
	CGEventPost(kCGHIDEventTap, up);
	CFRelease(up);
	CFRunLoopRunInMode(kCFRunLoopDefaultMode, kNeruMouseClickProcessingDelay, false);
	return 1;
}
