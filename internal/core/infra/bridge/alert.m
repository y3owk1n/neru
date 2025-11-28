//
//  alert.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "alert.h"
#import <Cocoa/Cocoa.h>

#pragma mark - Internal Function Declaration

static int showAlertOnMainThread(const char *errorMessage, const char *configPath);

#pragma mark - Alert Functions

/// Show a config validation error alert with error details and config path
/// @param errorMessage The error message to display
/// @param configPath The path to the config file
/// @return 1 if user clicked OK, 2 if user clicked Copy, 0 otherwise
int showConfigValidationErrorAlert(const char *errorMessage, const char *configPath) {
	@autoreleasepool {
		__block int result = 0;

		// Ensure we're on the main thread for UI operations
		if ([NSThread isMainThread]) {
			result = showAlertOnMainThread(errorMessage, configPath);
		} else {
			dispatch_sync(dispatch_get_main_queue(), ^{
				result = showAlertOnMainThread(errorMessage, configPath);
			});
		}

		return result;
	}
}

/// Internal function to show alert on main thread
/// @param errorMessage The error message to display
/// @param configPath The path to the config file
/// @return 1 if user clicked OK, 2 if user clicked Copy, 0 otherwise
static int showAlertOnMainThread(const char *errorMessage, const char *configPath) {
	NSString *error = errorMessage ? [NSString stringWithUTF8String:errorMessage] : @"Unknown error";
	NSString *path = configPath ? [NSString stringWithUTF8String:configPath] : @"No config file";

	NSAlert *alert = [[NSAlert alloc] init];
	alert.messageText = @"⚠️ Configuration Validation Failed";
	alert.informativeText =
	    [NSString stringWithFormat:@"Neru encountered an error while loading your configuration file:\n\n%@\n\nConfig "
	                               @"file: %@\n\nThe application will continue running with default configuration.",
	                               error, path];
	alert.alertStyle = NSAlertStyleWarning;

	// Add buttons
	[alert addButtonWithTitle:@"OK"];
	[alert addButtonWithTitle:@"Copy Path"];

	// Set icon
	alert.icon = [NSImage imageNamed:NSImageNameCaution];

	// Ensure alert is on top
	[[alert window] setLevel:NSFloatingWindowLevel];

	// Temporarily switch to Regular policy to grab focus
	[NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];

	// Center the window
	[[alert window] center];
	[[alert window] makeKeyAndOrderFront:nil];

	// Activate the app to ensure the alert is visible and focused
	[NSApp activateIgnoringOtherApps:YES];

	// Schedule switch back to Accessory policy to hide Dock icon
	// We use a small delay to ensure the focus is grabbed first
	dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(0.1 * NSEC_PER_SEC)), dispatch_get_main_queue(), ^{
		[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
	});

	// Run modal
	NSModalResponse response = [alert runModal];

	// Handle Copy Path button
	if (response == NSAlertSecondButtonReturn) {
		NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
		[pasteboard clearContents];
		[pasteboard setString:path forType:NSPasteboardTypeString];
	}

	// Check which button was clicked
	if (response == NSAlertFirstButtonReturn) {
		// OK button
		return 1;
	} else if (response == NSAlertSecondButtonReturn) {
		// Copy button
		NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
		[pasteboard clearContents];
		[pasteboard setString:path forType:NSPasteboardTypeString];
		return 2;
	}

	return 0;
}
