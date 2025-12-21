//
//  secureinput.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "secureinput.h"
#import <Carbon/Carbon.h>
#import <Cocoa/Cocoa.h>

#pragma mark - Secure Input Detection

/// Check if macOS secure input mode is currently enabled
/// Uses IsSecureEventInputEnabled() from the Carbon HIToolbox framework
/// @return 1 if secure input is enabled, 0 otherwise
int isSecureInputEnabled(void) {
	// IsSecureEventInputEnabled() returns true when any application has enabled
	// secure event input, typically occurring when password fields are focused
	return IsSecureEventInputEnabled() ? 1 : 0;
}

#pragma mark - Secure Input Notification

/// Show a notification informing the user that secure input is active
/// Uses osascript to display a native macOS notification
/// This approach works reliably for CLI tools without requiring an app bundle
void showSecureInputNotification(void) {
	@autoreleasepool {
		// Use osascript to show notification - works for CLI tools
		NSTask *task = [[NSTask alloc] init];
		task.executableURL = [NSURL fileURLWithPath:@"/usr/bin/osascript"];
		task.arguments = @[
			@"-e",
			@"display notification \"Mode activation blocked. A password field or secure input is active.\" with title \"Neru: Secure Input Detected\""
		];

		NSError *error = nil;
		if ([task launchAndReturnError:&error]) {
			[task waitUntilExit];
			if (task.terminationStatus != 0) {
				NSLog(@"Neru: osascript failed with status %d", task.terminationStatus);
			}
		}

		if (error) {
			NSLog(@"Neru: Failed to show secure input notification: %@", error);
		}
	}
}
