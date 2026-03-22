//
//  alert.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "alert.h"

#import <Cocoa/Cocoa.h>
#import <UserNotifications/UserNotifications.h>

#pragma mark - Internal Function Declaration

static int showAlertOnMainThread(const char *errorMessage, const char *configPath);
static int showOnboardingAlertOnMainThread(const char *configPath);

#pragma mark - Alert Functions

/// Show a config validation error alert with error details and config path
/// @param errorMessage The error message to display
/// @param configPath The path to the config file
/// @return 1 if user clicked OK, 2 if user clicked Copy Path, 0 otherwise
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
/// @return 1 if user clicked OK, 2 if user clicked Copy Path, 0 otherwise
static int showAlertOnMainThread(const char *errorMessage, const char *configPath) {
	NSString *error = errorMessage ? [NSString stringWithUTF8String:errorMessage] : @"Unknown error";
	NSString *path = configPath ? [NSString stringWithUTF8String:configPath] : @"No config file";

	// Configure alert content
	NSAlert *alert = [[NSAlert alloc] init];
	alert.messageText = @"⚠️ Configuration Validation Failed";
	alert.informativeText =
	    [NSString stringWithFormat:@"Neru encountered an error while loading your configuration file:\n\n%@\n\nConfig "
	                               @"file: %@",
	                               error, path];
	alert.alertStyle = NSAlertStyleWarning;
	alert.icon = [NSImage imageNamed:NSImageNameCaution];

	// Add buttons
	[alert addButtonWithTitle:@"OK"];
	[alert addButtonWithTitle:@"Copy Path"];

	// Bring alert to front and ensure it receives focus
	[[alert window] setLevel:NSFloatingWindowLevel];
	[NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
	[[alert window] center];
	[[alert window] makeKeyAndOrderFront:nil];
	[NSApp activateIgnoringOtherApps:YES];

	// Run modal and revert activation policy once dismissed
	NSModalResponse response = [alert runModal];
	[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];

	// Handle button response
	if (response == NSAlertFirstButtonReturn) {
		return 1;
	} else if (response == NSAlertSecondButtonReturn) {
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

	// Configure alert content
	NSAlert *alert = [[NSAlert alloc] init];
	alert.messageText = @"👋 Welcome to Neru!";
	alert.informativeText =
	    [NSString stringWithFormat:@"No configuration file found.\n\nA default config will be created at:\n%@\n\nYou "
	                               @"can run 'neru config init' later to recreate it.",
	                               path];
	alert.alertStyle = NSAlertStyleInformational;

	// Add buttons
	[alert addButtonWithTitle:@"Create Config"];
	[alert addButtonWithTitle:@"Use Defaults (No Config)"];
	[alert addButtonWithTitle:@"Quit"];

	// Bring alert to front and ensure it receives focus
	[[alert window] setLevel:NSFloatingWindowLevel];
	[NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
	[[alert window] center];
	[[alert window] makeKeyAndOrderFront:nil];
	[NSApp activateIgnoringOtherApps:YES];

	// Run modal and revert activation policy once dismissed
	NSModalResponse response = [alert runModal];
	[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];

	// Handle button response
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

static void showNotificationWithUNUserNotificationCenter(NSString *title, NSString *message);

static void showNotificationFallbackOsascript(NSString *title, NSString *message);

void showNotification(const char *title, const char *message) {
	@autoreleasepool {
		NSString *nsTitle = title ? [NSString stringWithUTF8String:title] : @"Neru";
		NSString *nsMessage = message ? [NSString stringWithUTF8String:message] : @"";

		NSString *bundleId = [[NSBundle mainBundle] bundleIdentifier];
		NSLog(@"Neru: showNotification bundleIdentifier=%@", bundleId);

		if (bundleId != nil) {
			showNotificationWithUNUserNotificationCenter(nsTitle, nsMessage);
		} else {
			showNotificationFallbackOsascript(nsTitle, nsMessage);
		}
	}
}

static void showNotificationWithUNUserNotificationCenter(NSString *title, NSString *message) {
	UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];

	[center requestAuthorizationWithOptions:(UNAuthorizationOptionAlert | UNAuthorizationOptionSound)
	                      completionHandler:^(BOOL granted, NSError *_Nullable error) {
		                      if (error) {
			                      NSLog(@"Neru: Notification authorization error: %@", error);
			                      if (!granted) {
				                      return;
			                      }
		                      }

		                      if (!granted) {
			                      NSLog(@"Neru: Notification authorization denied");
			                      return;
		                      }

		                      UNMutableNotificationContent *content = [[UNMutableNotificationContent alloc] init];
		                      content.title = title;
		                      content.body = message;
		                      content.sound = [UNNotificationSound defaultSound];

		                      NSString *identifier = [[NSUUID UUID] UUIDString];
		                      UNTimeIntervalNotificationTrigger *trigger =
		                          [UNTimeIntervalNotificationTrigger triggerWithTimeInterval:0.1 repeats:NO];
		                      UNNotificationRequest *request = [UNNotificationRequest requestWithIdentifier:identifier
		                                                                                            content:content
		                                                                                            trigger:trigger];

		                      [center addNotificationRequest:request
		                               withCompletionHandler:^(NSError *_Nullable addError) {
			                               if (addError) {
				                               NSLog(@"Neru: Failed to add notification request: %@", addError);
			                               }
		                               }];
	                      }];
}

static void showNotificationFallbackOsascript(NSString *title, NSString *message) {
	// Escape backslashes and double quotes for AppleScript string interpolation
	title = [title stringByReplacingOccurrencesOfString:@"\\" withString:@"\\\\"];
	title = [title stringByReplacingOccurrencesOfString:@"\"" withString:@"\\\""];
	message = [message stringByReplacingOccurrencesOfString:@"\\" withString:@"\\\\"];
	message = [message stringByReplacingOccurrencesOfString:@"\"" withString:@"\\\""];

	// Launch osascript to post the notification
	NSTask *task = [[NSTask alloc] init];
	task.executableURL = [NSURL fileURLWithPath:@"/usr/bin/osascript"];
	task.arguments =
	    @[ @"-e", [NSString stringWithFormat:@"display notification \"%@\" with title \"%@\"", message, title] ];

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
