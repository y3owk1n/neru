//
//  axobserver.h
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//
//  Push-based accessibility change notifications. A dedicated CFRunLoop thread
//  services AXObserver callbacks for a set of application processes. The Go
//  layer owns lifecycle: it starts the thread, arms an observer per process, and
//  holds the returned handle. Arm, disarm, and release are all marshalled onto
//  the run-loop thread, so they are serialized against the observer's own
//  callbacks (the run loop services one thing at a time). That is what makes it
//  safe to release an AXObserver — a callback for it can never be running
//  concurrently — and keeps a synchronous AX call that hangs on the observer
//  thread, never on a caller's lock.
//

#ifndef AXOBSERVER_H
#define AXOBSERVER_H

#include <stdint.h>

#pragma mark - Notification mask bits

// Bit flags selecting which AX notifications an observer registers on an
// application element. Descendant elements' notifications bubble up to an
// observer registered on the application element. Kept in sync with the mirrored
// Go constants in axobserver.go.
//
// The first block is structural (an element or window appeared, moved, or
// vanished, a page loaded, a menu toggled, focus moved); the last block covers
// browser web content, where Chromium and Firefox post no plain "created"
// notification and instead signal a live region update, an expand or collapse,
// or a busy flag clearing. VALUE_CHANGED fires on every value update (a clock, a
// progress bar), so callers may leave it off to avoid noise.
#define NERU_AXNOTIF_CREATED (1u << 0)                // kAXCreatedNotification
#define NERU_AXNOTIF_UI_DESTROYED (1u << 1)           // kAXUIElementDestroyedNotification
#define NERU_AXNOTIF_LAYOUT_CHANGED (1u << 2)         // kAXLayoutChangedNotification
#define NERU_AXNOTIF_WINDOW_CREATED (1u << 3)         // kAXWindowCreatedNotification
#define NERU_AXNOTIF_WINDOW_MOVED (1u << 4)           // kAXWindowMovedNotification
#define NERU_AXNOTIF_WINDOW_RESIZED (1u << 5)         // kAXWindowResizedNotification
#define NERU_AXNOTIF_LOAD_COMPLETE (1u << 6)          // AXLoadComplete (no public constant)
#define NERU_AXNOTIF_MENU_OPENED (1u << 7)            // kAXMenuOpenedNotification
#define NERU_AXNOTIF_MENU_CLOSED (1u << 8)            // kAXMenuClosedNotification
#define NERU_AXNOTIF_FOCUSED_UI_CHANGED (1u << 9)     // kAXFocusedUIElementChangedNotification
#define NERU_AXNOTIF_VALUE_CHANGED (1u << 10)         // kAXValueChangedNotification (opt-in, noisy)
#define NERU_AXNOTIF_LIVE_REGION_CHANGED (1u << 11)   // AXLiveRegionChanged (no public constant)
#define NERU_AXNOTIF_LIVE_REGION_CREATED (1u << 12)   // AXLiveRegionCreated (no public constant)
#define NERU_AXNOTIF_EXPANDED_CHANGED (1u << 13)      // AXExpandedChanged (no public constant)
#define NERU_AXNOTIF_ROW_EXPANDED (1u << 14)          // kAXRowExpandedNotification
#define NERU_AXNOTIF_ROW_COLLAPSED (1u << 15)         // kAXRowCollapsedNotification
#define NERU_AXNOTIF_ELEMENT_BUSY_CHANGED (1u << 16)  // kAXElementBusyChangedNotification

#pragma mark - Observer run-loop thread

// Start the observer run-loop thread if it is not already running. Blocks until
// the run loop is live so a subsequent NeruObserverArm has a loop to attach to.
// Idempotent.
void NeruObserverStartThread(void);

// Stop the observer run-loop thread and join it. Idempotent; safe when the
// thread is not running.
void NeruObserverStopThread(void);

// Report whether the observer run-loop thread is running. Test hook.
int NeruObserverThreadRunning(void);

#pragma mark - Arm / disarm

// Create an AXObserver for pid, register the notifications selected by mask on
// the application element, and attach its run-loop source. Returns an opaque
// handle, or NULL if the application element could not be created or no
// notification could be registered. The run-loop thread must be running (call
// NeruObserverStartThread first). pid is carried in the callback refcon.
//
// messagingTimeout (seconds, ignored when <= 0) bounds the synchronous AX calls
// this observer makes to the target app, so a wedged app cannot hang the observer
// thread indefinitely. It is set on this app's element only, never process-wide,
// so unrelated accessibility work keeps the default timeout.
void *NeruObserverArm(int pid, uint32_t mask, float messagingTimeout);

// Remove the observer's source, unregister its notifications (only when live is
// non-zero, to skip IPC to a process that has already exited), release the
// observer and application element, and free the handle. Safe with a NULL handle.
void NeruObserverDisarm(void *handle, int live);

#pragma mark - Leak instrumentation (test hooks)

// Live count of created-minus-released AXObserver refs. Zero at idle; a non-zero
// idle value is a leak.
long NeruObserverLiveObserverCount(void);

// Live count of created-minus-released application AXUIElement refs. Zero at
// idle; a non-zero idle value is a leak.
long NeruObserverLiveAppElementCount(void);

#endif /* AXOBSERVER_H */
