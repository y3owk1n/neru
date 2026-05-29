//
//  screencapture_permissions_darwin.m
//  Neru
//
//  Copyright © 2026 Neru. All rights reserved.
//

#import "screencapture.h"

#import <Cocoa/Cocoa.h>
#import <CoreGraphics/CoreGraphics.h>

#pragma mark - Permission Functions

static BOOL resetScreenCapturePermissionDecision(void) {
	NSString *bundleID = [[NSBundle mainBundle] bundleIdentifier];
	if (bundleID == nil || [bundleID length] == 0) {
		bundleID = @"com.y3owk1n.neru";
	}

	NSTask *task = [[NSTask alloc] init];
	task.launchPath = @"/usr/bin/tccutil";
	task.arguments = @[ @"reset", @"ScreenCapture", bundleID ];

	@try {
		[task launch];
		[task waitUntilExit];

		int status = [task terminationStatus];
		if (status != 0) {
			NSLog(
			    @"Neru: tccutil reset ScreenCapture %@ exited with status %d; system permission dialog may not appear",
			    bundleID, status);
			return NO;
		}
	} @catch (NSException *exception) {
		NSLog(@"Neru: failed to reset ScreenCapture permission decision: %@", exception);
		return NO;
	}

	return YES;
}

/// Check if screen capture permissions are granted
/// @return 1 if permissions are granted, 0 otherwise
int NeruCheckScreenCapturePermissions(void) {
	@autoreleasepool {
		if (@available(macOS 10.15, *)) {
			return CGPreflightScreenCaptureAccess() ? 1 : 0;
		}
		return 1;
	}
}

/// Request screen capture permissions from macOS
/// @return 1 if permissions are granted after the request, 0 otherwise
int NeruRequestScreenCapturePermissions(void) {
	@autoreleasepool {
		if (!resetScreenCapturePermissionDecision()) {
			NSLog(@"Neru: continuing with ScreenCapture permission request after reset failure");
		}

		if (@available(macOS 10.15, *)) {
			return CGRequestScreenCaptureAccess() ? 1 : 0;
		}
		return 1;
	}
}

#pragma mark - Alert Functions

static int showScreenCapturePermissionAlertOnMainThread(void) {
	while (NeruCheckScreenCapturePermissions() != 1) {
		NSAlert *alert = [[NSAlert alloc] init];
		alert.messageText = @"Screen Recording Permission Needed";
		alert.informativeText =
		    @"Neru needs Screen Recording permission to capture the screen for text recognition.\n\n"
		     "Click 'Request Permission' to open System Settings, enable Neru, then return here and click 'I've "
		     "Granted It'.\n\n"
		     "Note: macOS requires restarting Neru after granting screen recording permission for it to take effect.";
		alert.alertStyle = NSAlertStyleWarning;
		alert.icon = [NSImage imageNamed:NSImageNameCaution];

		[alert addButtonWithTitle:@"Request Permission"];
		[alert addButtonWithTitle:@"I've Granted It"];
		[alert addButtonWithTitle:@"Cancel"];

		[[alert window] setLevel:NSFloatingWindowLevel];
		[NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
		[[alert window] center];
		[[alert window] makeKeyAndOrderFront:nil];
		[NSApp activateIgnoringOtherApps:YES];

		NSModalResponse response = [alert runModal];
		[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];

		if (response == NSAlertFirstButtonReturn) {
			NeruRequestScreenCapturePermissions();
		} else if (response == NSAlertSecondButtonReturn) {
			if (NeruCheckScreenCapturePermissions() == 1) {
				return 1;  // Granted
			} else {
				// Show a confirmation alert explaining the restart requirement
				NSAlert *restartAlert = [[NSAlert alloc] init];
				restartAlert.messageText = @"Neru Restart Required";
				restartAlert.informativeText =
				    @"macOS requires restarting Neru for Screen Recording permissions to take effect.\n\n"
				     "Neru will now quit. Please relaunch the application.";
				restartAlert.alertStyle = NSAlertStyleInformational;
				[restartAlert addButtonWithTitle:@"Quit Neru"];

				[[restartAlert window] setLevel:NSFloatingWindowLevel];
				[NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
				[[restartAlert window] center];
				[[restartAlert window] makeKeyAndOrderFront:nil];
				[NSApp activateIgnoringOtherApps:YES];

				[restartAlert runModal];
				[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];

				return 3;  // Restart/Quit
			}
		} else if (response == NSAlertThirdButtonReturn) {
			return 2;  // Cancel/Exit mode
		}
	}

	return 1;  // Granted
}

/// Show the startup screen capture permission guidance alert.
/// The alert lets the user request permission and then dismiss it with Granted.
/// @return 1 if permission is granted, 2 if the user chose Quit.
int NeruShowScreenCapturePermissionAlert(void) {
	@autoreleasepool {
		__block int result = 0;

		if ([NSThread isMainThread]) {
			result = showScreenCapturePermissionAlertOnMainThread();
		} else {
			dispatch_sync(dispatch_get_main_queue(), ^{
				result = showScreenCapturePermissionAlertOnMainThread();
			});
		}

		return result;
	}
}
