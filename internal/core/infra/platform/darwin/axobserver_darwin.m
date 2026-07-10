//
//  axobserver_darwin.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "axobserver.h"

#import <ApplicationServices/ApplicationServices.h>
#import <Foundation/Foundation.h>
#include <pthread.h>
#include <stdatomic.h>
#include <stdlib.h>
#include <string.h>

#pragma mark - Go bridge

// Forward one notification to Go. Declared as a //export in axobserver.go. Runs
// on the observer run-loop thread, so it must do O(1) work only — no AX calls,
// no blocking. notif is the notification name, for debug logging.
extern void handleAXObserverNotification(int pid, const char *notif);

#pragma mark - Handle

// One armed observer, owned by the Go layer through the opaque handle. notifs
// records the notification names that registered successfully, so a live disarm
// unregisters exactly those. The kAX* names are process-owned constants and are
// not retained.
typedef struct {
	AXObserverRef observer;
	AXUIElementRef appElement;
	CFStringRef notifs[32];
	int notifCount;
} NeruObserverHandle;

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
// are attached, so the thread stays alive between arms and exits only on an
// explicit CFRunLoopStop.
static void neruKeepAlivePerform(void *info) { (void)info; }

// Fires once, when the run loop is actually entered, so NeruObserverStartThread
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

void NeruObserverStartThread(void) {
	if (gRunning) {
		return;
	}

	gReady = dispatch_semaphore_create(0);
	gRunning = 1;

	if (pthread_create(&gThread, NULL, neruThreadMain, NULL) != 0) {
		// The thread never starts, so gReady would never be signaled. Leave the
		// subsystem stopped so NeruObserverArm fails cleanly instead of blocking
		// forever on the wait below.
		gRunning = 0;
		gReady = NULL;

		return;
	}

	dispatch_semaphore_wait(gReady, DISPATCH_TIME_FOREVER);
	gReady = NULL;
}

void NeruObserverStopThread(void) {
	if (!gRunning) {
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

	int pid = (int)(intptr_t)refcon;

	char nameBuf[128];
	const char *name = "";
	if (notification != NULL && CFStringGetCString(notification, nameBuf, sizeof(nameBuf), kCFStringEncodingUTF8)) {
		name = nameBuf;
	}

	handleAXObserverNotification(pid, name);
}

#pragma mark - Notification mapping

typedef struct {
	uint32_t bit;
	CFStringRef name;
} NeruNotifEntry;

// Map each mask bit to its notification name. AXLoadComplete has no public kAX*
// constant, so its name is the literal string web areas post on page load.
static const NeruNotifEntry *neruNotifTable(int *count) {
	static NeruNotifEntry table[32];
	table[0] = (NeruNotifEntry){NERU_AXNOTIF_CREATED, kAXCreatedNotification};
	table[1] = (NeruNotifEntry){NERU_AXNOTIF_UI_DESTROYED, kAXUIElementDestroyedNotification};
	table[2] = (NeruNotifEntry){NERU_AXNOTIF_LAYOUT_CHANGED, kAXLayoutChangedNotification};
	table[3] = (NeruNotifEntry){NERU_AXNOTIF_WINDOW_CREATED, kAXWindowCreatedNotification};
	table[4] = (NeruNotifEntry){NERU_AXNOTIF_WINDOW_MOVED, kAXWindowMovedNotification};
	table[5] = (NeruNotifEntry){NERU_AXNOTIF_WINDOW_RESIZED, kAXWindowResizedNotification};
	table[6] = (NeruNotifEntry){NERU_AXNOTIF_LOAD_COMPLETE, CFSTR("AXLoadComplete")};
	table[7] = (NeruNotifEntry){NERU_AXNOTIF_MENU_OPENED, kAXMenuOpenedNotification};
	table[8] = (NeruNotifEntry){NERU_AXNOTIF_MENU_CLOSED, kAXMenuClosedNotification};
	table[9] = (NeruNotifEntry){NERU_AXNOTIF_FOCUSED_UI_CHANGED, kAXFocusedUIElementChangedNotification};
	table[10] = (NeruNotifEntry){NERU_AXNOTIF_VALUE_CHANGED, kAXValueChangedNotification};
	// Web-content signals. Chromium and Firefox post these string notifications
	// (no public kAX* constant for the first three) when a page's content changes.
	table[11] = (NeruNotifEntry){NERU_AXNOTIF_LIVE_REGION_CHANGED, CFSTR("AXLiveRegionChanged")};
	table[12] = (NeruNotifEntry){NERU_AXNOTIF_LIVE_REGION_CREATED, CFSTR("AXLiveRegionCreated")};
	table[13] = (NeruNotifEntry){NERU_AXNOTIF_EXPANDED_CHANGED, CFSTR("AXExpandedChanged")};
	table[14] = (NeruNotifEntry){NERU_AXNOTIF_ROW_EXPANDED, kAXRowExpandedNotification};
	table[15] = (NeruNotifEntry){NERU_AXNOTIF_ROW_COLLAPSED, kAXRowCollapsedNotification};
	table[16] = (NeruNotifEntry){NERU_AXNOTIF_ELEMENT_BUSY_CHANGED, kAXElementBusyChangedNotification};
	*count = 17;

	return table;
}

#pragma mark - Arm / disarm (run on the run-loop thread)

static NeruObserverHandle *neruArmOnLoop(int pid, uint32_t mask, float messagingTimeout) {
	AXUIElementRef appEl = AXUIElementCreateApplication((pid_t)pid);
	if (appEl == NULL) {
		return NULL;
	}
	atomic_fetch_add(&gLiveAppElements, 1);

	// Bound this app's synchronous AX calls (the registrations below) so a wedged
	// app cannot hang the observer thread. Scoped to this element, so unrelated
	// accessibility work keeps the default timeout.
	if (messagingTimeout > 0) {
		AXUIElementSetMessagingTimeout(appEl, messagingTimeout);
	}

	AXObserverRef observer = NULL;
	AXError createErr = AXObserverCreateWithInfoCallback((pid_t)pid, neruObserverCallback, &observer);
	if (createErr != kAXErrorSuccess || observer == NULL) {
		CFRelease(appEl);
		atomic_fetch_sub(&gLiveAppElements, 1);

		return NULL;
	}
	atomic_fetch_add(&gLiveObservers, 1);

	NeruObserverHandle *handle = calloc(1, sizeof(NeruObserverHandle));
	if (handle == NULL) {
		CFRelease(observer);
		atomic_fetch_sub(&gLiveObservers, 1);
		CFRelease(appEl);
		atomic_fetch_sub(&gLiveAppElements, 1);

		return NULL;
	}
	handle->observer = observer;
	handle->appElement = appEl;

	void *refcon = (void *)(intptr_t)pid;

	int tableCount = 0;
	const NeruNotifEntry *table = neruNotifTable(&tableCount);

	int armed = 0;
	int fatal = 0;
	for (int i = 0; i < tableCount; i++) {
		if ((mask & table[i].bit) == 0) {
			continue;
		}

		AXError addErr = AXObserverAddNotification(observer, appEl, table[i].name, refcon);
		if (addErr == kAXErrorSuccess || addErr == kAXErrorNotificationAlreadyRegistered) {
			handle->notifs[handle->notifCount++] = table[i].name;
			armed++;
		} else if (addErr == kAXErrorInvalidUIElement || addErr == kAXErrorCannotComplete) {
			// The app is gone or wedged. Abort the whole handle rather than
			// registering a partial, unreliable set.
			fatal = 1;
			break;
		}
		// kAXErrorNotificationUnsupported and other soft failures: an app that
		// does not emit this notification is still worth observing for the rest.
	}

	if (fatal || armed == 0) {
		CFRelease(observer);
		atomic_fetch_sub(&gLiveObservers, 1);
		CFRelease(appEl);
		atomic_fetch_sub(&gLiveAppElements, 1);
		free(handle);

		return NULL;
	}

	CFRunLoopAddSource(gRunLoop, AXObserverGetRunLoopSource(observer), kCFRunLoopDefaultMode);

	return handle;
}

void *NeruObserverArm(int pid, uint32_t mask, float messagingTimeout) {
	if (pid <= 0 || mask == 0 || !gRunning || gRunLoop == NULL) {
		return NULL;
	}

	__block NeruObserverHandle *result = NULL;
	neruRunOnLoop(^{
		result = neruArmOnLoop(pid, mask, messagingTimeout);
	});

	return result;
}

static void neruDisarmOnLoop(NeruObserverHandle *handle, int live) {
	CFRunLoopSourceRef src = AXObserverGetRunLoopSource(handle->observer);
	if (gRunLoop != NULL && src != NULL) {
		CFRunLoopRemoveSource(gRunLoop, src, kCFRunLoopDefaultMode);
	}

	// For a live app, unregister exactly what we registered. For a process that
	// has exited (live == 0) skip the IPC — the CFRelease below drops the
	// registration and reaching a gone process only wastes a messaging timeout.
	if (live) {
		for (int i = 0; i < handle->notifCount; i++) {
			AXObserverRemoveNotification(handle->observer, handle->appElement, handle->notifs[i]);
		}
	}

	CFRelease(handle->observer);
	atomic_fetch_sub(&gLiveObservers, 1);
	CFRelease(handle->appElement);
	atomic_fetch_sub(&gLiveAppElements, 1);
	free(handle);
}

void NeruObserverDisarm(void *handle, int live) {
	if (handle == NULL) {
		return;
	}

	neruRunOnLoop(^{
		neruDisarmOnLoop((NeruObserverHandle *)handle, live);
	});
}
