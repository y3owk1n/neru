//
//  axobserver_darwin.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "axobserver.h"

#import <ApplicationServices/ApplicationServices.h>
#import <Foundation/Foundation.h>
#include <errno.h>
#include <pthread.h>
#include <signal.h>
#include <stdatomic.h>
#include <stdlib.h>
#include <string.h>

#pragma mark - Go bridge

// Forward one notification to Go. Declared as a //export in axobserver.go. Runs
// on the observer run-loop thread, so it must do O(1) work only — no AX calls,
// no blocking. notif is the notification name, for debug logging.
extern void handleAXObserverNotification(const char *notif);

#pragma mark - The watched application

// The one watched process. This layer watches a single application at a time:
// NeruObserverWatch installs a new observer here and tears down whatever was
// watched before, NeruObserverUnwatch empties it. All reads and writes happen
// on the run-loop thread (marshalled through neruRunOnLoop), except the
// occupancy checks in NeruObserverWatch/NeruObserverUnwatch, which run on the
// caller thread strictly after the marshalled block completed.
typedef struct {
	AXObserverRef observer;
	AXUIElementRef appElement;
	int pid;
} NeruWatchedApp;

static NeruWatchedApp gWatchedApp;

#pragma mark - Leak counters

static _Atomic long gLiveObservers = 0;
static _Atomic long gLiveAppElements = 0;

long NeruObserverLiveObserverCount(void) { return atomic_load(&gLiveObservers); }

long NeruObserverLiveAppElementCount(void) { return atomic_load(&gLiveAppElements); }

#pragma mark - Run-loop thread

static pthread_t gThread;
static CFRunLoopRef gRunLoop = NULL;                // retained while the thread runs
static CFRunLoopSourceRef gKeepAlive = NULL;        // keeps the loop from exiting when idle
static CFRunLoopObserverRef gEntryObserver = NULL;  // signals gReady on loop entry
static dispatch_semaphore_t gReady = NULL;
static int gRunning = 0;

// Run block on the run-loop thread and wait for it to finish. Every AX
// create/register/release and run-loop source mutation goes through here, so
// they are serialized against the observer callbacks. If the loop is not up yet,
// or the caller already is the run-loop thread, the block runs inline.
static void neruRunOnLoop(void (^block)(void)) {
	if (gRunLoop == NULL || CFRunLoopGetCurrent() == gRunLoop) {
		block();

		return;
	}

	dispatch_semaphore_t done = dispatch_semaphore_create(0);
	CFRunLoopPerformBlock(gRunLoop, kCFRunLoopDefaultMode, ^{
		block();
		dispatch_semaphore_signal(done);
	});
	CFRunLoopWakeUp(gRunLoop);
	dispatch_semaphore_wait(done, DISPATCH_TIME_FOREVER);
}

// A no-op source that keeps CFRunLoopRun blocked even when no observer sources
// are attached, so the thread stays alive between watches and exits only on an
// explicit CFRunLoopStop.
static void neruKeepAlivePerform(void *info) { (void)info; }

// Fires once, when the run loop is actually entered, so the thread start
// returns only after the loop is genuinely running.
static void neruLoopEntered(CFRunLoopObserverRef observer, CFRunLoopActivity activity, void *info) {
	(void)observer;
	(void)activity;
	(void)info;
	dispatch_semaphore_signal(gReady);
}

static void *neruThreadMain(void *arg) {
	(void)arg;

	@autoreleasepool {
		pthread_setname_np("com.neru.axobserver");

		CFRunLoopRef rl = CFRunLoopGetCurrent();

		CFRunLoopSourceContext ctx;
		memset(&ctx, 0, sizeof(ctx));
		ctx.perform = neruKeepAlivePerform;
		gKeepAlive = CFRunLoopSourceCreate(kCFAllocatorDefault, 0, &ctx);
		CFRunLoopAddSource(rl, gKeepAlive, kCFRunLoopDefaultMode);

		// Publish the run loop (retained) so other threads can add/remove sources
		// and stop it; CFRunLoopGetCurrent returns a non-owned reference.
		gRunLoop = (CFRunLoopRef)CFRetain(rl);

		gEntryObserver = CFRunLoopObserverCreate(kCFAllocatorDefault, kCFRunLoopEntry, false, 0, neruLoopEntered, NULL);
		CFRunLoopAddObserver(rl, gEntryObserver, kCFRunLoopDefaultMode);

		CFRunLoopRun();

		CFRunLoopRemoveObserver(rl, gEntryObserver, kCFRunLoopDefaultMode);
		CFRelease(gEntryObserver);
		gEntryObserver = NULL;
		CFRunLoopRemoveSource(rl, gKeepAlive, kCFRunLoopDefaultMode);
		CFRelease(gKeepAlive);
		gKeepAlive = NULL;
	}

	return NULL;
}

// Start the run-loop thread if it is not already running, blocking until the
// loop is live so a subsequent watch has a loop to attach to.
static void neruStartThreadIfNeeded(void) {
	if (gRunning) {
		return;
	}

	gReady = dispatch_semaphore_create(0);
	gRunning = 1;

	if (pthread_create(&gThread, NULL, neruThreadMain, NULL) != 0) {
		// The thread never starts, so gReady would never be signaled. Leave the
		// subsystem stopped so NeruObserverWatch fails cleanly instead of blocking
		// forever on the wait below.
		gRunning = 0;
		gReady = NULL;

		return;
	}

	dispatch_semaphore_wait(gReady, DISPATCH_TIME_FOREVER);
	gReady = NULL;
}

// Stop and join the run-loop thread when nothing is watched. Runs on the caller
// thread, never inside a neruRunOnLoop block: stopping joins the run-loop
// thread, and a join issued from that thread would deadlock against itself.
static void neruStopThreadIfIdle(void) {
	if (!gRunning || gWatchedApp.observer != NULL) {
		return;
	}

	CFRunLoopStop(gRunLoop);
	CFRunLoopWakeUp(gRunLoop);
	pthread_join(gThread, NULL);

	CFRelease(gRunLoop);
	gRunLoop = NULL;
	gRunning = 0;
}

int NeruObserverThreadRunning(void) { return gRunning; }

#pragma mark - Callback (runs on the run-loop thread)

static void neruObserverCallback(
    AXObserverRef observer, AXUIElementRef element, CFStringRef notification, CFDictionaryRef info, void *refcon) {
	(void)observer;
	(void)element;
	(void)info;
	(void)refcon;

	char nameBuf[128];
	const char *name = "";
	if (notification != NULL && CFStringGetCString(notification, nameBuf, sizeof(nameBuf), kCFStringEncodingUTF8)) {
		name = nameBuf;
	}

	handleAXObserverNotification(name);
}

#pragma mark - Watched notifications

// The notifications every observer registers on the application element;
// descendant elements' notifications bubble up to it. The set covers the
// structural changes that mean the UI actually changed (an element or window
// appeared, moved, or vanished; a page finished loading; a menu opened or
// closed; focus moved), plus the signals browsers post for web content, where
// Chromium and Firefox emit no plain "created" notification (a live region
// updating or being created, a disclosure or row expanding or collapsing, a
// busy flag clearing). Value-change notifications such as AXValueChanged must
// stay out of this list: they fire on every value update (a ticking clock, a
// progress bar) and would wake the observer continuously.
//
// The names are written as their literal string values so the array can be a
// compile-time constant (the SDK's kAX* symbols are runtime-initialized externs
// a static initializer cannot reference). The standard names are the values of
// the kAX*Notification constants Apple defines in AXNotificationConstants.h:
// https://developer.apple.com/documentation/applicationservices/axnotificationconstants_h/miscellaneous_defines
// AXLoadComplete, AXLiveRegionChanged, AXLiveRegionCreated, and AXExpandedChanged
// have no public constant; they are the strings browser engines post.
static const CFStringRef gNotificationNames[] = {
    CFSTR("AXCreated"),           CFSTR("AXUIElementDestroyed"),
    CFSTR("AXLayoutChanged"),     CFSTR("AXWindowCreated"),
    CFSTR("AXWindowMoved"),       CFSTR("AXWindowResized"),
    CFSTR("AXLoadComplete"),      CFSTR("AXMenuOpened"),
    CFSTR("AXMenuClosed"),        CFSTR("AXFocusedUIElementChanged"),
    CFSTR("AXLiveRegionChanged"), CFSTR("AXLiveRegionCreated"),
    CFSTR("AXExpandedChanged"),   CFSTR("AXRowExpanded"),
    CFSTR("AXRowCollapsed"),      CFSTR("AXElementBusyChanged"),
};

static const int gNotificationNameCount = (int)(sizeof(gNotificationNames) / sizeof(gNotificationNames[0]));

// Report whether pid still names a live process, so teardown can skip the
// notification-unregister IPC to a process that has already exited. kill with
// signal 0 delivers no signal at all: it only performs the existence and
// permission check, so nothing is killed or disturbed.
static int neruProcessAlive(int pid) { return kill(pid, 0) == 0 || errno != ESRCH; }

#pragma mark - Watch / unwatch (watched-app mutations run on the run-loop thread)

// Tear down one observer: remove its run-loop source, unregister the watched
// notifications when the process is still alive, and release everything. The
// unregister loop walks the same fixed name list registration offers; a name
// the app never accepted returns a harmless not-registered error. It bails on
// the first error that means the app is gone or wedged, so a beachballing app
// costs at most one messaging timeout here instead of one per name.
static void neruTeardownOnLoop(NeruWatchedApp watched) {
	CFRunLoopSourceRef src = AXObserverGetRunLoopSource(watched.observer);
	if (gRunLoop != NULL && src != NULL) {
		CFRunLoopRemoveSource(gRunLoop, src, kCFRunLoopDefaultMode);
	}

	if (neruProcessAlive(watched.pid)) {
		for (int i = 0; i < gNotificationNameCount; i++) {
			AXError removeErr =
			    AXObserverRemoveNotification(watched.observer, watched.appElement, gNotificationNames[i]);
			if (removeErr == kAXErrorInvalidUIElement || removeErr == kAXErrorCannotComplete) {
				break;
			}
		}
	}

	CFRelease(watched.observer);
	atomic_fetch_sub(&gLiveObservers, 1);
	CFRelease(watched.appElement);
	atomic_fetch_sub(&gLiveAppElements, 1);
}

// Switch the watched application to pid: build and register the new observer first, install
// it, then tear down the previously watched one, so the run loop never sits
// with zero observers mid-switch. On failure nothing is watched afterward and the
// previous observer is torn down too, so a later watch of the same pid retries
// from a clean state.
static int neruWatchOnLoop(int pid, float messagingTimeout) {
	// Watching the pid already watched is a success no-op, so a caller can
	// re-point the observer on every refresh without tearing down and rebuilding
	// the observer each time.
	if (gWatchedApp.observer != NULL && gWatchedApp.pid == pid) {
		return 1;
	}

	NeruWatchedApp prev = gWatchedApp;
	int hadPrev = gWatchedApp.observer != NULL;

	gWatchedApp.observer = NULL;
	gWatchedApp.appElement = NULL;
	gWatchedApp.pid = 0;

	AXUIElementRef appEl = AXUIElementCreateApplication((pid_t)pid);
	if (appEl == NULL) {
		if (hadPrev) {
			neruTeardownOnLoop(prev);
		}

		return 0;
	}
	atomic_fetch_add(&gLiveAppElements, 1);

	// Bound this app's synchronous AX calls (the registrations below and the
	// teardown's unregistrations) so a wedged app cannot hang the observer
	// thread. Scoped to this element, so unrelated accessibility work keeps the
	// default timeout.
	if (messagingTimeout > 0) {
		AXUIElementSetMessagingTimeout(appEl, messagingTimeout);
	}

	AXObserverRef observer = NULL;
	AXError createErr = AXObserverCreateWithInfoCallback((pid_t)pid, neruObserverCallback, &observer);
	if (createErr != kAXErrorSuccess || observer == NULL) {
		CFRelease(appEl);
		atomic_fetch_sub(&gLiveAppElements, 1);

		if (hadPrev) {
			neruTeardownOnLoop(prev);
		}

		return 0;
	}
	atomic_fetch_add(&gLiveObservers, 1);

	int registered = 0;
	int fatal = 0;
	for (int i = 0; i < gNotificationNameCount; i++) {
		AXError addErr = AXObserverAddNotification(observer, appEl, gNotificationNames[i], NULL);
		if (addErr == kAXErrorSuccess || addErr == kAXErrorNotificationAlreadyRegistered) {
			registered++;
		} else if (addErr == kAXErrorInvalidUIElement || addErr == kAXErrorCannotComplete) {
			// The app is gone or wedged. Abort the whole watch rather than
			// registering a partial, unreliable set.
			fatal = 1;
			break;
		}
		// kAXErrorNotificationUnsupported and other soft failures: an app that
		// does not emit this notification is still worth observing for the rest.
	}

	if (fatal || registered == 0) {
		CFRelease(observer);
		atomic_fetch_sub(&gLiveObservers, 1);
		CFRelease(appEl);
		atomic_fetch_sub(&gLiveAppElements, 1);

		if (hadPrev) {
			neruTeardownOnLoop(prev);
		}

		return 0;
	}

	CFRunLoopAddSource(gRunLoop, AXObserverGetRunLoopSource(observer), kCFRunLoopDefaultMode);

	gWatchedApp.observer = observer;
	gWatchedApp.appElement = appEl;
	gWatchedApp.pid = pid;

	if (hadPrev) {
		neruTeardownOnLoop(prev);
	}

	return 1;
}

int NeruObserverWatch(int pid, float messagingTimeout) {
	if (pid <= 0) {
		return 0;
	}

	neruStartThreadIfNeeded();

	if (!gRunning || gRunLoop == NULL) {
		return 0;
	}

	__block int ok = 0;
	neruRunOnLoop(^{
		ok = neruWatchOnLoop(pid, messagingTimeout);
	});

	neruStopThreadIfIdle();

	return ok;
}

void NeruObserverUnwatch(void) {
	if (!gRunning) {
		return;
	}

	neruRunOnLoop(^{
		if (gWatchedApp.observer == NULL) {
			return;
		}

		NeruWatchedApp watched = gWatchedApp;
		gWatchedApp.observer = NULL;
		gWatchedApp.appElement = NULL;
		gWatchedApp.pid = 0;

		neruTeardownOnLoop(watched);
	});

	neruStopThreadIfIdle();
}
