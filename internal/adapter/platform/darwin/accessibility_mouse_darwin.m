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
#import <os/lock.h>
#include <sys/time.h>

#pragma mark - Mouse Functions

/// Lock making "pan the zoom viewport, then post the event" indivisible.
///
/// Cursor operations are issued concurrently — each hotkey press dispatches its
/// own goroutine, repeat-while-held runs another, and IPC actions run on the IPC
/// goroutine — and those paths do not share a lock. Two interleaved operations
/// could otherwise pan for one target and then post the other, leaving the
/// cursor at a correct coordinate but outside the magnified region, where
/// nothing corrects it until the next move.
///
/// Every posted event that carries a position takes this, button events
/// included: they reposition the cursor just as a move does, so a click whose
/// button events escaped the lock could still land outside the magnified region
/// even though the move preceding it was itself atomic. Each pan/post pair being
/// individually consistent is what guarantees that whichever one happens to go
/// last leaves the cursor and the viewport agreeing.
static os_unfair_lock mouseMoveLock = OS_UNFAIR_LOCK_INIT;

/// Marks events Neru posts itself, so the mode event tap can tell its own
/// synthetic clicks apart from ones the user physically produced. Must match
/// the marker in keyfeed_darwin.m and the check in eventTapCallback.
static const int neruSyntheticMouseEventMarker = 0x1337;

/// Pan the zoom viewport to reveal a point and post an event positioned there
/// @param event Event to post; its location must already be set to position
/// @param position Position the event acts at
static void postPositionedEventLocked(CGEventRef event, CGPoint position) {
	os_unfair_lock_lock(&mouseMoveLock);

	// Tag before posting so the event tap ignores it. Every synthetic mouse
	// event Neru emits funnels through here, so this is the single point that
	// keeps `action left_click` from being mistaken for a physical click.
	CGEventSetIntegerValueField(event, kCGEventSourceUserData, neruSyntheticMouseEventMarker);

	NeruEnsureZoomViewportContainsPoint(position);
	CGEventPost(kNeruMouseEventTapLocation, event);

	os_unfair_lock_unlock(&mouseMoveLock);
}

/// Pan the zoom viewport to reveal a point and post a mouse move there
/// @param position Target position
/// @param eventType CGEvent type (kCGEventMouseMoved or a *MouseDragged type)
/// @param button Button the drag belongs to; ignored for kCGEventMouseMoved
static void postMouseMoveLocked(CGPoint position, CGEventType eventType, CGMouseButton button) {
	CGEventRef move = CGEventCreateMouseEvent(NULL, eventType, position, button);
	if (!move)
		return;

	CGEventSetFlags(move, 0);
	postPositionedEventLocked(move, position);
	CFRelease(move);
}

/// Move mouse cursor to position with specified event type and drag button
/// @param position Target position
/// @param eventType CGEvent type (kCGEventMouseMoved or a *MouseDragged type)
/// @param button Button the drag belongs to; ignored for kCGEventMouseMoved
void NeruMoveMouseWithTypeAndButton(CGPoint position, CGEventType eventType, CGMouseButton button) {
	postMouseMoveLocked(position, eventType, button);

	// Deliberately outside the lock: this spins the run loop for milliseconds,
	// and serialising that would stall every other cursor path.
	CFRunLoopRunInMode(kCFRunLoopDefaultMode, kNeruMouseMoveDelay, false);
}

/// Move mouse cursor to position with specified event type
/// @param position Target position
/// @param eventType CGEvent type (kCGEventMouseMoved or kCGEventLeftMouseDragged)
void NeruMoveMouseWithType(CGPoint position, CGEventType eventType) {
	NeruMoveMouseWithTypeAndButton(position, eventType, kCGMouseButtonLeft);
}

/// Post a single mouse move event (non-blocking, for async animation)
/// @param position Target position
/// @param eventType CGEvent type (kCGEventMouseMoved or a *MouseDragged type)
/// @param button Button the drag belongs to; ignored for kCGEventMouseMoved
void NeruPostMouseMoveEventWithButton(CGPoint position, CGEventType eventType, CGMouseButton button) {
	postMouseMoveLocked(position, eventType, button);
}

/// Post a single mouse move event (non-blocking, for async animation)
/// @param position Target position
/// @param eventType CGEvent type (kCGEventMouseMoved or kCGEventLeftMouseDragged)
void NeruPostMouseMoveEvent(CGPoint position, CGEventType eventType) {
	postMouseMoveLocked(position, eventType, kCGMouseButtonLeft);
}

#pragma mark - Mouse Action Functions

/// Release a button without moving
/// @param upEvent Mouse up event type matching button
/// @param button Mouse button to release
/// @return 1 on success, 0 on failure
int NeruPerformMouseUpAtCursor(CGEventType upEvent, CGMouseButton button) {
	CGEventRef currentEvent = CGEventCreate(NULL);
	if (!currentEvent)
		return 0;

	CGPoint currentPos = CGEventGetLocation(currentEvent);
	CFRelease(currentEvent);

	CGEventRef up = CGEventCreateMouseEvent(NULL, upEvent, currentPos, button);
	if (!up)
		return 0;

	// Clear all modifier flags to ensure clean mouse up
	CGEventSetFlags(up, 0);
	postPositionedEventLocked(up, currentPos);
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
static int performClickAtPosition(
    CGPoint pos, CGEventType downEvent, CGEventType upEvent, CGMouseButton button, bool restoreCursor,
    CGEventFlags flags) {
	// Capture original cursor position before moving
	CGPoint originalPosition = CGPointZero;
	if (restoreCursor) {
		CGEventRef currentEvent = CGEventCreate(NULL);
		if (currentEvent) {
			originalPosition = CGEventGetLocation(currentEvent);
			CFRelease(currentEvent);
		}
	}

	NeruMoveMouseWithType(pos, kCGEventMouseMoved);

	// Create down and up events
	CGEventRef down = CGEventCreateMouseEvent(NULL, downEvent, pos, button);
	CGEventRef up = CGEventCreateMouseEvent(NULL, upEvent, pos, button);
	if (!down || !up) {
		if (down)
			CFRelease(down);
		if (up)
			CFRelease(up);
		if (restoreCursor)
			NeruMoveMouseWithType(originalPosition, kCGEventMouseMoved);
		return 0;
	}

	// Set modifier flags (0 = no modifiers for clean click)
	CGEventSetFlags(down, flags);
	CGEventSetFlags(up, flags);

	// Post mouse down, allow the system to process it, then post mouse up.
	// Give the event loop a short moment to register the down event before sending up.
	postPositionedEventLocked(down, pos);
	CFRunLoopRunInMode(kCFRunLoopDefaultMode, kNeruMouseClickDownUpDelay, false);

	postPositionedEventLocked(up, pos);
	CFRelease(down);
	CFRelease(up);

	// Allow a small amount of time for the click to be processed by the system
	// before restoring the cursor to avoid clicks landing in-transit.
	CFRunLoopRunInMode(kCFRunLoopDefaultMode, kNeruMouseClickProcessingDelay, false);

	if (restoreCursor)
		NeruMoveMouseWithType(originalPosition, kCGEventMouseMoved);
	return 1;
}

/// State tracking for click detection
static struct {
	CGPoint lastPosition;          ///< Last click position
	struct timeval lastClickTime;  ///< Last click time
	int clickCount;                ///< Current click count
} clickState = {0};

/// Lock guarding clickState — NeruPerformLeftClickAtPosition may be called
/// from concurrent goroutines via the CGo bridge.
static os_unfair_lock clickStateLock = OS_UNFAIR_LOCK_INIT;

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
int NeruPerformLeftClickAtPosition(CGPoint position, bool restoreCursor, CGEventFlags flags) {
	// Capture original cursor position before moving
	CGPoint originalPosition = CGPointZero;
	if (restoreCursor) {
		CGEventRef currentEvent = CGEventCreate(NULL);
		if (currentEvent) {
			originalPosition = CGEventGetLocation(currentEvent);
			CFRelease(currentEvent);
		}
	}

	// Determine click count (single, double, triple...)
	os_unfair_lock_lock(&clickStateLock);

	long long currentTime = getCurrentTimeMs();
	long long lastTime = (long long)clickState.lastClickTime.tv_sec * 1000 + clickState.lastClickTime.tv_usec / 1000;
	long long timeDiff = currentTime - lastTime;
	double distance =
	    sqrt(pow(position.x - clickState.lastPosition.x, 2) + pow(position.y - clickState.lastPosition.y, 2));

	if (timeDiff < kNeruDoubleClickIntervalMs && distance < kNeruDoubleClickDistancePoints) {
		// Same location, quick succession — increment click count
		clickState.clickCount++;
	} else {
		// New click sequence
		clickState.clickCount = 1;
	}

	// Update click state
	clickState.lastPosition = position;
	gettimeofday(&clickState.lastClickTime, NULL);

	int clickCount = clickState.clickCount;
	os_unfair_lock_unlock(&clickStateLock);

	NeruMoveMouseWithType(position, kCGEventMouseMoved);

	// Create down and up events
	CGEventRef down = CGEventCreateMouseEvent(NULL, kCGEventLeftMouseDown, position, kCGMouseButtonLeft);
	CGEventRef up = CGEventCreateMouseEvent(NULL, kCGEventLeftMouseUp, position, kCGMouseButtonLeft);
	if (!down || !up) {
		if (down)
			CFRelease(down);
		if (up)
			CFRelease(up);
		if (restoreCursor)
			NeruMoveMouseWithType(originalPosition, kCGEventMouseMoved);
		return 0;
	}

	// Set modifier flags and click count
	CGEventSetFlags(down, flags);
	CGEventSetFlags(up, flags);

	CGEventSetIntegerValueField(down, kCGMouseEventClickState, clickCount);
	CGEventSetIntegerValueField(up, kCGMouseEventClickState, clickCount);

	// Post mouse down and allow a short moment before posting mouse up to ensure
	// the system attributes the down/up pair to the target location.
	postPositionedEventLocked(down, position);
	CFRunLoopRunInMode(kCFRunLoopDefaultMode, kNeruMouseClickDownUpDelay, false);

	postPositionedEventLocked(up, position);
	CFRelease(down);
	CFRelease(up);

	// Wait briefly to let the OS process the click before potentially moving the cursor back.
	CFRunLoopRunInMode(kCFRunLoopDefaultMode, kNeruMouseClickProcessingDelay, false);

	if (restoreCursor)
		NeruMoveMouseWithType(originalPosition, kCGEventMouseMoved);
	return 1;
}

/// Perform right click at position
/// @param position Target position
/// @param restoreCursor Whether to restore cursor position after click
/// @return 1 on success, 0 on failure
int NeruPerformRightClickAtPosition(CGPoint position, bool restoreCursor, CGEventFlags flags) {
	return performClickAtPosition(
	    position, kCGEventRightMouseDown, kCGEventRightMouseUp, kCGMouseButtonRight, restoreCursor, flags);
}

/// Perform middle click at position
/// @param position Target position
/// @param restoreCursor Whether to restore cursor position after click
/// @return 1 on success, 0 on failure
int NeruPerformMiddleClickAtPosition(CGPoint position, bool restoreCursor, CGEventFlags flags) {
	return performClickAtPosition(
	    position, kCGEventOtherMouseDown, kCGEventOtherMouseUp, kCGMouseButtonCenter, restoreCursor, flags);
}

/// Post a single button event at a position
/// @param position Target position
/// @param buttonEvent Mouse down or up event type matching button
/// @param button Mouse button the event belongs to
/// @param flags CGEventFlags for modifier keys (0 for none)
/// @return 1 on success, 0 on failure
static int performButtonEventAtPosition(
    CGPoint position, CGEventType buttonEvent, CGMouseButton button, CGEventFlags flags) {
	NeruMoveMouseWithType(position, kCGEventMouseMoved);
	CFRunLoopRunInMode(kCFRunLoopDefaultMode, kNeruMouseClickProcessingDelay, false);

	CGEventRef event = CGEventCreateMouseEvent(NULL, buttonEvent, position, button);
	if (!event)
		return 0;

	// Set modifier flags (0 = no modifiers for a clean press/release)
	CGEventSetFlags(event, flags);
	postPositionedEventLocked(event, position);
	CFRelease(event);

	CFRunLoopRunInMode(kCFRunLoopDefaultMode, kNeruMouseClickProcessingDelay, false);
	return 1;
}

/// Perform a mouse down at position
/// @param position Target position
/// @param downEvent Mouse down event type matching button
/// @param button Mouse button to press
/// @param flags CGEventFlags for modifier keys (0 for none)
/// @return 1 on success, 0 on failure
int NeruPerformMouseDownAtPosition(CGPoint position, CGEventType downEvent, CGMouseButton button, CGEventFlags flags) {
	return performButtonEventAtPosition(position, downEvent, button, flags);
}

/// Perform a mouse up at position
/// @param position Target position
/// @param upEvent Mouse up event type matching button
/// @param button Mouse button to release
/// @param flags CGEventFlags for modifier keys (0 for none)
/// @return 1 on success, 0 on failure
int NeruPerformMouseUpAtPosition(CGPoint position, CGEventType upEvent, CGMouseButton button, CGEventFlags flags) {
	return performButtonEventAtPosition(position, upEvent, button, flags);
}
