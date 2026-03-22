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

#pragma mark - Notification Delegate

/// Delegate that allows notifications to be displayed even when the app is in the foreground.
/// Without this, UNUserNotificationCenter silently suppresses foreground notifications.
@interface NeruNotificationDelegate : NSObject <UNUserNotificationCenterDelegate>
@end

@implementation NeruNotificationDelegate

- (void)userNotificationCenter:(UNUserNotificationCenter *)center
       willPresentNotification:(UNNotification *)notification
         withCompletionHandler:(void (^)(UNNotificationPresentationOptions))completionHandler {
	completionHandler(UNNotificationPresentationOptionBanner | UNNotificationPresentationOptionSound);
}

@end

#pragma mark - Notification Functions

static void showNotificationWithUNUserNotificationCenter(NSString *title, NSString *message);

/// Lazily initializes and returns the shared notification delegate.
/// The delegate is set once on UNUserNotificationCenter and kept alive for the process lifetime.
static NeruNotificationDelegate *getNotificationDelegate(void) {
	static NeruNotificationDelegate *delegate = nil;
	static dispatch_once_t onceToken;
	dispatch_once(&onceToken, ^{
		delegate = [[NeruNotificationDelegate alloc] init];
		[UNUserNotificationCenter currentNotificationCenter].delegate = delegate;
	});
	return delegate;
}

void showNotification(const char *title, const char *message) {
	@autoreleasepool {
		NSString *nsTitle = title ? [NSString stringWithUTF8String:title] : @"Neru";
		NSString *nsMessage = message ? [NSString stringWithUTF8String:message] : @"";

		NSString *bundleId = [[NSBundle mainBundle] bundleIdentifier];

		if (bundleId != nil) {
			showNotificationWithUNUserNotificationCenter(nsTitle, nsMessage);
		} else {
			NSLog(@"Neru: [%@] %@", nsTitle, nsMessage);
		}
	}
}

static void showNotificationWithUNUserNotificationCenter(NSString *title, NSString *message) {
	// Ensure the delegate is set so foreground notifications are not suppressed
	(void)getNotificationDelegate();

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
		                      UNNotificationRequest *request = [UNNotificationRequest requestWithIdentifier:identifier
		                                                                                            content:content
		                                                                                            trigger:nil];

		                      [center addNotificationRequest:request
		                               withCompletionHandler:^(NSError *_Nullable addError) {
			                               if (addError) {
				                               NSLog(@"Neru: Failed to add notification request: %@", addError);
			                               }
		                               }];
	                      }];
}
