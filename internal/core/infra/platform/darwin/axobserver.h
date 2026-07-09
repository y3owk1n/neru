//
//  axobserver.h
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//
//  Push-based accessibility change notifications. A dedicated, lazily started
//  CFRunLoop thread services AXObserver callbacks for a set of application
//  processes. The Go ObserverManager owns lifecycle and calls arm/disarm from
//  its actor goroutine, but the actual AXObserver create/register/release and
//  run-loop source operations are marshalled onto the run-loop thread. That
//  serializes them against the observer's own callbacks (the run loop dispatches
//  one thing at a time), so releasing an AXObserver can never race an in-flight
//  callback for it, and a synchronous AX registration call that hangs stalls only
//  the observer thread, never the caller's lock.
//

#ifndef AXOBSERVER_H
#define AXOBSERVER_H

#import <Foundation/Foundation.h>
#include <stdint.h>

#pragma mark - Notification mask bits

// Bit flags selecting which AX notifications an observer registers on an
// application element. Kept in sync with the mirrored Go constants in
// axobserver.go. Descendant elements' notifications bubble up to an observer
// registered on the application element.
enum {
	NeruAXNotifLayoutChanged = 1u << 0,
	NeruAXNotifCreated = 1u << 1,
	NeruAXNotifUIElementDestroyed = 1u << 2,
	NeruAXNotifWindowCreated = 1u << 3,
	NeruAXNotifWindowMoved = 1u << 4,
	NeruAXNotifWindowResized = 1u << 5,
	NeruAXNotifFocusedUIElementChanged = 1u << 6,
	NeruAXNotifMenuOpened = 1u << 7,
	NeruAXNotifMenuClosed = 1u << 8,
	NeruAXNotifValueChanged = 1u << 9,
};

#pragma mark - Observer run-loop thread

/// Start the dedicated observer run-loop thread if it is not already running.
/// Blocks until the run loop is live so a subsequent NeruArmObserver can attach
/// its source. Idempotent.
void NeruStartObserverThread(void);

/// Stop the observer run-loop thread and join it. Idempotent; safe when the
/// thread is not running.
void NeruStopObserverThread(void);

/// Report whether the observer run-loop thread is currently running. Test hook.
int NeruObserverThreadRunning(void);

#pragma mark - Arm / disarm

/// Create an AXObserver for pid, register the notifications selected by mask on
/// the application element, and attach its run-loop source to the observer
/// thread. refcon carries (epoch << 32 | pid) so the callback can reject stale
/// events. Returns an opaque handle, or NULL if the application element could
/// not be created or no notification could be registered. The observer thread
/// must be running (call NeruStartObserverThread first).
void *NeruArmObserver(int pid, unsigned long long epoch, uint32_t mask);

/// Remove the observer's source, unregister notifications (only when live != 0,
/// to avoid IPC to a dead process), release the observer and application
/// element, and free the handle. Safe to call with a NULL handle.
void NeruDisarmObserver(void *handle, int live);

#pragma mark - Messaging timeout

/// Set the accessibility messaging timeout on the system-wide element, bounding
/// synchronous AX calls process-wide (best effort; see
/// AXUIElementSetMessagingTimeout).
void NeruSetObserverMessagingTimeout(float seconds);

#pragma mark - Leak instrumentation (test hooks)

/// Live count of created-minus-released AXObserver refs.
long NeruObserverLiveObserverCount(void);

/// Live count of created-minus-released application AXUIElement refs.
long NeruObserverLiveAppElementCount(void);

#endif /* AXOBSERVER_H */
