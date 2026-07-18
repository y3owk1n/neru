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

// Create an AXObserver for pid, register the fixed set of notifications on the
// application element (defined in axobserver_darwin.m), and attach its run-loop
// source. Returns an opaque handle, or NULL if the application element could
// not be created or no notification could be registered. The run-loop thread
// must be running (call NeruObserverStartThread first). pid is carried in the
// callback refcon.
//
// messagingTimeout (seconds, ignored when <= 0) bounds the synchronous AX calls
// this observer makes to the target app, so a wedged app cannot hang the observer
// thread indefinitely. It is set on this app's element only, never process-wide,
// so unrelated accessibility work keeps the default timeout.
void *NeruObserverArm(int pid, float messagingTimeout);

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
