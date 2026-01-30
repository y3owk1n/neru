//
//  appwatcher.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "appwatcher.h"
#import <Cocoa/Cocoa.h>

#pragma mark - External Function Declarations

extern void handleAppLaunch(const char *appName, const char *bundleID);
extern void handleAppTerminate(const char *appName, const char *bundleID);
extern void handleAppActivate(const char *appName, const char *bundleID);
extern void handleAppDeactivate(const char *appName, const char *bundleID);
extern void handleScreenParametersChanged(void);

#pragma mark - App Watcher Delegate Implementation

@interface AppWatcherDelegate : NSObject
@property(nonatomic, strong) dispatch_source_t debounceTimer;
@end

@implementation AppWatcherDelegate

/// Handle application launch notification
/// @param notification Notification object
- (void)applicationDidLaunch:(NSNotification *)notification {
	@autoreleasepool {
		NSRunningApplication *app = notification.userInfo[NSWorkspaceApplicationKey];
		if (!app)
			return;

		// Copy strings to prevent dangling pointers
		NSString *appName = app.localizedName ?: @"Unknown";
		NSString *bundleID = app.bundleIdentifier ?: @"unknown.bundle";

		char *appNameCopy = strdup([appName UTF8String]);
		char *bundleIDCopy = strdup([bundleID UTF8String]);

		// Call Go callback on main thread for consistency
		dispatch_async(dispatch_get_main_queue(), ^{
			if (appNameCopy && bundleIDCopy) {
				handleAppLaunch(appNameCopy, bundleIDCopy);
			}
			free(appNameCopy);
			free(bundleIDCopy);
		});
	}
}

/// Handle application termination notification
/// @param notification Notification object
- (void)applicationDidTerminate:(NSNotification *)notification {
	@autoreleasepool {
		NSRunningApplication *app = notification.userInfo[NSWorkspaceApplicationKey];
		if (!app)
			return;

		NSString *appName = app.localizedName ?: @"Unknown";
		NSString *bundleID = app.bundleIdentifier ?: @"unknown.bundle";

		char *appNameCopy = strdup([appName UTF8String]);
		char *bundleIDCopy = strdup([bundleID UTF8String]);

		dispatch_async(dispatch_get_main_queue(), ^{
			if (appNameCopy && bundleIDCopy) {
				handleAppTerminate(appNameCopy, bundleIDCopy);
			}
			free(appNameCopy);
			free(bundleIDCopy);
		});
	}
}

/// Handle application activation notification
/// @param notification Notification object
- (void)applicationDidActivate:(NSNotification *)notification {
	@autoreleasepool {
		NSRunningApplication *app = notification.userInfo[NSWorkspaceApplicationKey];
		if (!app)
			return;

		NSString *appName = app.localizedName ?: @"Unknown";
		NSString *bundleID = app.bundleIdentifier ?: @"unknown.bundle";

		// Allocate copies that Go can safely use
		char *appNameCopy = strdup([appName UTF8String]);
		char *bundleIDCopy = strdup([bundleID UTF8String]);

		if ([NSThread isMainThread]) {
			// Already on main thread, call directly
			if (appNameCopy && bundleIDCopy) {
				handleAppActivate(appNameCopy, bundleIDCopy);
			}
			free(appNameCopy);
			free(bundleIDCopy);
		} else {
			// Not on main thread, dispatch
			dispatch_async(dispatch_get_main_queue(), ^{
				if (appNameCopy && bundleIDCopy) {
					handleAppActivate(appNameCopy, bundleIDCopy);
				}
				free(appNameCopy);
				free(bundleIDCopy);
			});
		}
	}
}

/// Handle application deactivation notification
/// @param notification Notification object
- (void)applicationDidDeactivate:(NSNotification *)notification {
	@autoreleasepool {
		NSRunningApplication *app = notification.userInfo[NSWorkspaceApplicationKey];
		if (!app)
			return;

		NSString *appName = app.localizedName ?: @"Unknown";
		NSString *bundleID = app.bundleIdentifier ?: @"unknown.bundle";

		char *appNameCopy = strdup([appName UTF8String]);
		char *bundleIDCopy = strdup([bundleID UTF8String]);

		dispatch_async(dispatch_get_main_queue(), ^{
			if (appNameCopy && bundleIDCopy) {
				handleAppDeactivate(appNameCopy, bundleIDCopy);
			}
			free(appNameCopy);
			free(bundleIDCopy);
		});
	}
}

/// Handle active space change notification
/// @param notification Notification object
- (void)activeSpaceDidChange:(NSNotification *)notification {
	// Treat space change as screen parameter change to trigger overlay refresh
	[self screenParametersDidChange:notification];
}

/// Handle screen parameters change notification
/// @param notification Notification object
- (void)screenParametersDidChange:(NSNotification *)notification {
	@autoreleasepool {
		if (self.debounceTimer) {
			dispatch_source_cancel(self.debounceTimer);
			self.debounceTimer = nil;
		}
		dispatch_source_t timer =
		    dispatch_source_create(DISPATCH_SOURCE_TYPE_TIMER, 0, 0, dispatch_get_global_queue(QOS_CLASS_UTILITY, 0));
		if (timer) {
			self.debounceTimer = timer;
			dispatch_source_set_timer(timer, dispatch_time(DISPATCH_TIME_NOW, 100 * NSEC_PER_MSEC),
			                          DISPATCH_TIME_FOREVER, 10 * NSEC_PER_MSEC);
			dispatch_source_set_event_handler(timer, ^{
				handleScreenParametersChanged();
				dispatch_source_cancel(timer);
				// Clear property on main thread to avoid race
				dispatch_async(dispatch_get_main_queue(), ^{
					if (self.debounceTimer == timer) {
						self.debounceTimer = nil;
					}
				});
			});
			dispatch_resume(timer);
		}
	}
}

@end

#pragma mark - App Watcher Functions

static AppWatcherDelegate *delegate = nil;
static dispatch_queue_t watcherQueue = nil;

/// Start the application watcher
void startAppWatcher(void) {
	static dispatch_once_t onceToken;
	dispatch_once(&onceToken, ^{
		watcherQueue = dispatch_queue_create("com.neru.appwatcher", DISPATCH_QUEUE_SERIAL);
	});

	dispatch_sync(watcherQueue, ^{
		if (delegate == nil) {
			delegate = [[AppWatcherDelegate alloc] init];

			NSWorkspace *workspace = [NSWorkspace sharedWorkspace];
			NSNotificationCenter *center = [workspace notificationCenter];

			[center addObserver:delegate
			           selector:@selector(applicationDidLaunch:)
			               name:NSWorkspaceDidLaunchApplicationNotification
			             object:nil];

			[center addObserver:delegate
			           selector:@selector(applicationDidTerminate:)
			               name:NSWorkspaceDidTerminateApplicationNotification
			             object:nil];

			[center addObserver:delegate
			           selector:@selector(applicationDidActivate:)
			               name:NSWorkspaceDidActivateApplicationNotification
			             object:nil];

			[center addObserver:delegate
			           selector:@selector(applicationDidDeactivate:)
			               name:NSWorkspaceDidDeactivateApplicationNotification
			             object:nil];

			[center addObserver:delegate
			           selector:@selector(activeSpaceDidChange:)
			               name:NSWorkspaceActiveSpaceDidChangeNotification
			             object:nil];

			// Observe screen parameter changes (display add/remove, resolution changes)
			[[NSNotificationCenter defaultCenter] addObserver:delegate
			                                         selector:@selector(screenParametersDidChange:)
			                                             name:NSApplicationDidChangeScreenParametersNotification
			                                           object:nil];
		}
	});
}

/// Stop the application watcher
void stopAppWatcher(void) {
	if (watcherQueue == nil) {
		return;
	}

	dispatch_sync(watcherQueue, ^{
		if (delegate != nil) {
			NSWorkspace *workspace = [NSWorkspace sharedWorkspace];
			NSNotificationCenter *center = [workspace notificationCenter];
			[center removeObserver:delegate];

			// Remove observer from default center (for screen parameter changes)
			[[NSNotificationCenter defaultCenter] removeObserver:delegate];

			delegate = nil; // ARC will handle deallocation
		}
	});
}
