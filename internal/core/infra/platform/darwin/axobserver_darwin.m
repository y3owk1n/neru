//
//  axobserver_darwin.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "axobserver.h"

#import <ApplicationServices/ApplicationServices.h>
#import <Cocoa/Cocoa.h>

#include <dispatch/dispatch.h>
#include <pthread.h>
#include <stdatomic.h>
#include <stdlib.h>

#pragma mark - Go bridge

// Implemented in axobserver.go. Called on the observer run-loop thread; must do
// O(1) work only (no AX calls, no blocking). notif is the notification name, for
// debug logging.
extern void handleAXNotification(int pid, unsigned long long epoch, const char *notif);

#pragma mark - Handle

// One armed observer. notifs holds the notification name constants that were
// successfully registered, so a live disarm can unregister exactly those. The
// kAX*Notification values are process-owned constants and are not retained.
typedef struct {
	AXObserverRef observer;
	AXUIElementRef appElement;
	CFStringRef notifs[10];
	int notifCount;
} NeruObserverHandle;

#pragma mark - Leak counters

static _Atomic long gLiveObservers = 0;
static _Atomic long gLiveAppElements = 0;

long NeruObserverLiveObserverCount(void) {
	return atomic_load(&gLiveObservers);
}

long NeruObserverLiveAppElementCount(void) {
	return atomic_load(&gLiveAppElements);
}

#pragma mark - Run-loop thread

static pthread_t gObserverThread;
static CFRunLoopRef gObserverRunLoop = NULL;      // retained while running
static CFRunLoopSourceRef gKeepAlive = NULL;      // owned by the observer thread
static CFRunLoopObserverRef gEntryObserver = NULL;  // signals gReady on loop entry
static dispatch_semaphore_t gReady = NULL;
static int gRunning = 0;

// runOnObserverLoop runs block on the observer run-loop thread and waits for it
// to finish. All AXObserver create/register/release and run-loop source
// operations go through here, so they are serialized against the observer's own
// callbacks (the run loop dispatches one thing at a time). This is what makes it
// safe to CFRelease an AXObserver: a callback for it can never be running
// concurrently. If the loop is not up yet, the block runs inline (the caller is
// the actor goroutine, which starts the thread before arming).
static void runOnObserverLoop(void (^block)(void)) {
	if (gObserverRunLoop == NULL) {
		block();

		return;
	}

	if (CFRunLoopGetCurrent() == gObserverRunLoop) {
		block();

		return;
	}

	dispatch_semaphore_t sem = dispatch_semaphore_create(0);
	CFRunLoopPerformBlock(gObserverRunLoop, kCFRunLoopDefaultMode, ^{
		block();
		dispatch_semaphore_signal(sem);
	});
	CFRunLoopWakeUp(gObserverRunLoop);
	dispatch_semaphore_wait(sem, DISPATCH_TIME_FOREVER);
}

// No-op perform for the keep-alive source. Its only job is to keep the run loop
// from exiting when no observer sources are attached.
static void keepAlivePerform(void *info) {
	(void)info;
}

// Fires once, when the run loop is actually entered, so NeruStartObserverThread
// only returns after the loop is genuinely running.
static void observerLoopEntered(
    CFRunLoopObserverRef observer, CFRunLoopActivity activity, void *info) {
	(void)observer;
	(void)activity;
	(void)info;
	dispatch_semaphore_signal(gReady);
}

static void *observerThreadMain(void *arg) {
	(void)arg;
	@autoreleasepool {
		CFRunLoopRef rl = CFRunLoopGetCurrent();

		CFRunLoopSourceContext ctx;
		memset(&ctx, 0, sizeof(ctx));
		ctx.perform = keepAlivePerform;
		gKeepAlive = CFRunLoopSourceCreate(kCFAllocatorDefault, 0, &ctx);
		CFRunLoopAddSource(rl, gKeepAlive, kCFRunLoopDefaultMode);

		// Publish the run loop (retained) so other threads can add/remove sources
		// and stop it. CFRunLoopGetCurrent returns a non-owned reference.
		gObserverRunLoop = (CFRunLoopRef)CFRetain(rl);

		// Signal readiness from a one-shot entry observer, so the waiter is
		// released only once the loop is actually running.
		gEntryObserver = CFRunLoopObserverCreate(
		    kCFAllocatorDefault, kCFRunLoopEntry, false, 0, observerLoopEntered, NULL);
		CFRunLoopAddObserver(rl, gEntryObserver, kCFRunLoopDefaultMode);

		CFRunLoopRun();

		// Stopped via CFRunLoopStop. Tear down on this thread.
		CFRunLoopRemoveObserver(rl, gEntryObserver, kCFRunLoopDefaultMode);
		CFRelease(gEntryObserver);
		gEntryObserver = NULL;
		CFRunLoopRemoveSource(rl, gKeepAlive, kCFRunLoopDefaultMode);
		CFRelease(gKeepAlive);
		gKeepAlive = NULL;
	}

	return NULL;
}

void NeruStartObserverThread(void) {
	if (gRunning) {
		return;
	}

	gReady = dispatch_semaphore_create(0);
	gRunning = 1;

	int rc = pthread_create(&gObserverThread, NULL, observerThreadMain, NULL);
	if (rc != 0) {
		// The thread never starts, so gReady would never be signaled. Leave the
		// subsystem stopped so NeruArmObserver fails cleanly instead of the caller
		// blocking forever on the wait below.
		gRunning = 0;
		gReady = NULL;

		return;
	}

	// Wait until the run loop is live and published before returning, so the
	// first NeruArmObserver has a valid run loop to attach to.
	dispatch_semaphore_wait(gReady, DISPATCH_TIME_FOREVER);
}

void NeruStopObserverThread(void) {
	if (!gRunning) {
		return;
	}

	// CFRunLoopStop / WakeUp are thread-safe.
	CFRunLoopStop(gObserverRunLoop);
	CFRunLoopWakeUp(gObserverRunLoop);

	pthread_join(gObserverThread, NULL);

	CFRelease(gObserverRunLoop);
	gObserverRunLoop = NULL;
	gReady = NULL;
	gRunning = 0;
}

int NeruObserverThreadRunning(void) {
	return gRunning;
}

#pragma mark - Callback

static void observerCallback(
    AXObserverRef observer,
    AXUIElementRef element,
    CFStringRef notification,
    CFDictionaryRef info,
    void *refcon) {
	(void)observer;
	(void)element;
	(void)info;

	uint64_t packed = (uint64_t)(uintptr_t)refcon;
	int pid = (int)(uint32_t)(packed & 0xFFFFFFFFu);
	unsigned long long epoch = (unsigned long long)(packed >> 32);

	char nameBuf[128];
	const char *name = "";
	if (notification != NULL &&
	    CFStringGetCString(notification, nameBuf, sizeof(nameBuf), kCFStringEncodingUTF8)) {
		name = nameBuf;
	}

	handleAXNotification(pid, epoch, name);
}

#pragma mark - Arm / disarm

// Maps a mask bit to its notification name constant.
typedef struct {
	uint32_t bit;
	CFStringRef name;
} NeruNotifEntry;

static const NeruNotifEntry *notifTable(int *count) {
	static NeruNotifEntry table[10];
	table[0] = (NeruNotifEntry){NeruAXNotifLayoutChanged, kAXLayoutChangedNotification};
	table[1] = (NeruNotifEntry){NeruAXNotifCreated, kAXCreatedNotification};
	table[2] = (NeruNotifEntry){NeruAXNotifUIElementDestroyed, kAXUIElementDestroyedNotification};
	table[3] = (NeruNotifEntry){NeruAXNotifWindowCreated, kAXWindowCreatedNotification};
	table[4] = (NeruNotifEntry){NeruAXNotifWindowMoved, kAXWindowMovedNotification};
	table[5] = (NeruNotifEntry){NeruAXNotifWindowResized, kAXWindowResizedNotification};
	table[6] =
	    (NeruNotifEntry){NeruAXNotifFocusedUIElementChanged, kAXFocusedUIElementChangedNotification};
	table[7] = (NeruNotifEntry){NeruAXNotifMenuOpened, kAXMenuOpenedNotification};
	table[8] = (NeruNotifEntry){NeruAXNotifMenuClosed, kAXMenuClosedNotification};
	table[9] = (NeruNotifEntry){NeruAXNotifValueChanged, kAXValueChangedNotification};
	*count = 10;

	return table;
}

// armOnLoop runs on the observer run-loop thread (via runOnObserverLoop). It is
// serialized against the observer callbacks, so the synchronous AX registration
// calls here cannot race a callback.
static NeruObserverHandle *armOnLoop(int pid, unsigned long long epoch, uint32_t mask) {
	AXUIElementRef appEl = AXUIElementCreateApplication(pid);
	if (appEl == NULL) {
		return NULL;
	}
	atomic_fetch_add(&gLiveAppElements, 1);

	AXObserverRef obs = NULL;
	AXError createErr = AXObserverCreateWithInfoCallback(pid, observerCallback, &obs);
	if (createErr != kAXErrorSuccess || obs == NULL) {
		CFRelease(appEl);
		atomic_fetch_sub(&gLiveAppElements, 1);

		return NULL;
	}
	atomic_fetch_add(&gLiveObservers, 1);

	NeruObserverHandle *handle = calloc(1, sizeof(NeruObserverHandle));
	handle->observer = obs;
	handle->appElement = appEl;

	void *refcon = (void *)(uintptr_t)((epoch << 32) | (uint64_t)(uint32_t)pid);

	int tableCount = 0;
	const NeruNotifEntry *table = notifTable(&tableCount);

	int armed = 0;
	int fatal = 0;
	for (int i = 0; i < tableCount; i++) {
		if ((mask & table[i].bit) == 0) {
			continue;
		}

		AXError addErr = AXObserverAddNotification(obs, appEl, table[i].name, refcon);
		if (addErr == kAXErrorSuccess || addErr == kAXErrorNotificationAlreadyRegistered) {
			handle->notifs[handle->notifCount++] = table[i].name;
			armed++;
		} else if (addErr == kAXErrorInvalidUIElement || addErr == kAXErrorCannotComplete) {
			// The app is gone or wedged. Abort the whole handle rather than
			// registering a partial, unreliable set.
			fatal = 1;
			break;
		}
		// kAXErrorNotificationUnsupported and any other soft failure: skip this
		// notification and continue with the rest.
	}

	if (fatal || armed == 0) {
		CFRelease(obs);
		atomic_fetch_sub(&gLiveObservers, 1);
		CFRelease(appEl);
		atomic_fetch_sub(&gLiveAppElements, 1);
		free(handle);

		return NULL;
	}

	// Already on the run-loop thread, so the source is serviced on the next
	// iteration without an explicit wake.
	CFRunLoopSourceRef src = AXObserverGetRunLoopSource(obs);
	CFRunLoopAddSource(gObserverRunLoop, src, kCFRunLoopDefaultMode);

	return handle;
}

void *NeruArmObserver(int pid, unsigned long long epoch, uint32_t mask) {
	if (!gRunning || gObserverRunLoop == NULL) {
		return NULL;
	}

	__block NeruObserverHandle *result = NULL;
	runOnObserverLoop(^{
		result = armOnLoop(pid, epoch, mask);
	});

	return result;
}

// disarmOnLoop runs on the observer run-loop thread. Because it is serialized
// against callbacks, releasing the observer here cannot race an in-flight
// callback for it.
static void disarmOnLoop(NeruObserverHandle *h, int live) {
	CFRunLoopSourceRef src = AXObserverGetRunLoopSource(h->observer);
	if (gObserverRunLoop != NULL && src != NULL) {
		CFRunLoopRemoveSource(gObserverRunLoop, src, kCFRunLoopDefaultMode);
	}

	// For a live app, unregister exactly the notifications we registered. For a
	// dead/wedged app (live == 0) skip this to avoid a blocking IPC to a gone
	// process; CFRelease of the observer drops the registration.
	if (live) {
		for (int i = 0; i < h->notifCount; i++) {
			AXObserverRemoveNotification(h->observer, h->appElement, h->notifs[i]);
		}
	}

	CFRelease(h->observer);
	atomic_fetch_sub(&gLiveObservers, 1);
	CFRelease(h->appElement);
	atomic_fetch_sub(&gLiveAppElements, 1);
	free(h);
}

void NeruDisarmObserver(void *handle, int live) {
	if (handle == NULL) {
		return;
	}

	runOnObserverLoop(^{
		disarmOnLoop((NeruObserverHandle *)handle, live);
	});
}

#pragma mark - Messaging timeout

void NeruSetObserverMessagingTimeout(float seconds) {
	AXUIElementRef systemWide = AXUIElementCreateSystemWide();
	if (systemWide != NULL) {
		AXUIElementSetMessagingTimeout(systemWide, seconds);
		CFRelease(systemWide);
	}
}
