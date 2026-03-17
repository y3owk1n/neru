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
static int showOnboardingAlertOnMainThread(const char *configPath);

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

	// Run modal
	NSModalResponse response = [alert runModal];

	// Revert to Accessory policy after the modal is dismissed so the
	// alert stays in the foreground for the entire user interaction.
	[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];

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

/// Show a config onboarding alert for new users
/// @param configPath The default config path that will be created
/// @return 1 if user clicked Create Config, 2 if user clicked Use Defaults, 3 if user clicked Quit
int showConfigOnboardingAlert(const char *configPath) {
	@autoreleasepool {
		__block int result = 0;

		if ([NSThread isMainThread]) {
			result = showOnboardingAlertOnMainThread(configPath);
		} else {
			dispatch_sync(dispatch_get_main_queue(), ^{
				result = showOnboardingAlertOnMainThread(configPath);
			});
		}

		return result;
	}
}

/// Internal function to show onboarding alert on main thread
/// @param configPath The default config path that will be created
/// @return 1 if user clicked Create Config, 2 if user clicked Use Defaults, 3 if user clicked Quit
static int showOnboardingAlertOnMainThread(const char *configPath) {
	NSString *path = configPath ? [NSString stringWithUTF8String:configPath] : @"~/.config/neru/config.toml";

	NSAlert *alert = [[NSAlert alloc] init];
	alert.messageText = @"👋 Welcome to Neru!";
	alert.informativeText =
	    [NSString stringWithFormat:@"No configuration file found.\n\nA default config will be created at:\n%@\n\nYou "
	                               @"can run 'neru config init' later to recreate it.",
	                               path];
	alert.alertStyle = NSAlertStyleInformational;

	[alert addButtonWithTitle:@"Create Config"];
	[alert addButtonWithTitle:@"Use Defaults (No Config)"];
	[alert addButtonWithTitle:@"Quit"];

	[[alert window] setLevel:NSFloatingWindowLevel];

	[NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];

	[[alert window] center];
	[[alert window] makeKeyAndOrderFront:nil];

	[NSApp activateIgnoringOtherApps:YES];

	NSModalResponse response = [alert runModal];

	// Revert to Accessory policy after the modal is dismissed so the
	// alert stays in the foreground for the entire user interaction.
	[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];

	if (response == NSAlertFirstButtonReturn) {
		return 1;
	} else if (response == NSAlertSecondButtonReturn) {
		return 2;
	} else if (response == NSAlertThirdButtonReturn) {
		return 3;
	}

	return 0;
}

#pragma mark - Notification Functions

/// Show a macOS notification with a title and message
/// Uses osascript to display a native macOS notification (works for CLI tools)
/// @param title The notification title
/// @param message The notification message
void showNotification(const char *title, const char *message) {
	@autoreleasepool {
		NSString *nsTitle = title ? [NSString stringWithUTF8String:title] : @"Neru";
		NSString *nsMessage = message ? [NSString stringWithUTF8String:message] : @"";

		// Escape backslashes and double quotes for AppleScript string interpolation
		nsTitle = [nsTitle stringByReplacingOccurrencesOfString:@"\\" withString:@"\\\\"];
		nsTitle = [nsTitle stringByReplacingOccurrencesOfString:@"\"" withString:@"\\\""];
		nsMessage = [nsMessage stringByReplacingOccurrencesOfString:@"\\" withString:@"\\\\"];
		nsMessage = [nsMessage stringByReplacingOccurrencesOfString:@"\"" withString:@"\\\""];

		NSTask *task = [[NSTask alloc] init];
		task.executableURL = [NSURL fileURLWithPath:@"/usr/bin/osascript"];
		task.arguments = @[
			@"-e", [NSString stringWithFormat:@"display notification \"%@\" with title \"%@\"", nsMessage, nsTitle]
		];

		NSError *error = nil;
		if (![task launchAndReturnError:&error]) {
			NSLog(@"Neru: Failed to show notification: %@", error);
			return;
		}
		[task waitUntilExit];
		if (task.terminationStatus != 0) {
			NSLog(@"Neru: osascript failed with status %d", task.terminationStatus);
		}
	}
}
