//
//  axobserver.h
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//
//  Push-based accessibility change notifications for a single watched
//  application. A dedicated CFRunLoop thread services the observer's callbacks;
//  this layer owns the observer, the application element, and the thread
//  lifecycle. Watch, unwatch, and every AX create/register/release are
//  marshalled onto the run-loop thread, so they are serialized against the
//  observer's own callbacks (the run loop services one thing at a time). That
//  is what makes it safe to release an AXObserver — a callback for it can never
//  be running concurrently — and keeps a synchronous AX call that hangs on the
//  observer thread, never on a caller's lock.
//

#ifndef AXOBSERVER_H
#define AXOBSERVER_H

#pragma mark - Watch / unwatch

// Watch pid: start the run-loop thread if needed, create an AXObserver for pid,
// register the fixed notification set (defined in axobserver_darwin.m) on the
// application element, and make it the watched process, tearing down whatever
// was watched before. Watching the pid already watched is a success no-op.
// Returns non-zero on success. On failure nothing is watched afterward and the
// thread is stopped, so a later watch retries from a clean state.
//
// messagingTimeout (seconds, ignored when <= 0) bounds the synchronous AX calls
// this observer makes to the target app, so a wedged app cannot hang the
// observer thread indefinitely. It is set on this app's element only, never
// process-wide, so unrelated accessibility work keeps the default timeout.
int NeruObserverWatch(int pid, float messagingTimeout);

// Stop watching: tear down the watched observer, if any, and stop the run-loop
// thread. Safe to call when nothing is watched.
void NeruObserverUnwatch(void);

#pragma mark - Test hooks

// Report whether the observer run-loop thread is running.
int NeruObserverThreadRunning(void);

// Live count of created-minus-released AXObserver refs. Zero at idle; a
// non-zero idle value is a leak.
long NeruObserverLiveObserverCount(void);

// Live count of created-minus-released application AXUIElement refs. Zero at
// idle; a non-zero idle value is a leak.
long NeruObserverLiveAppElementCount(void);

#endif /* AXOBSERVER_H */
