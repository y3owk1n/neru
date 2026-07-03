//
//  secureinput.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "alert.h"
#import "secureinput.h"

#import <Carbon/Carbon.h>
#import <Cocoa/Cocoa.h>

#pragma mark - Secure Input Detection

/// Check if macOS secure input mode is currently enabled.
/// Uses IsSecureEventInputEnabled() from HIToolbox.
/// @return 1 if secure input is enabled, 0 otherwise
int NeruIsSecureInputEnabled(void) {
	// IsSecureEventInputEnabled() returns true when any application has enabled
	// secure event input, typically occurring when password fields are focused.
	return IsSecureEventInputEnabled() ? 1 : 0;
}

#pragma mark - Secure Input Notification

/// Show a notification informing the user that secure input is active.
/// Reuses the NeruShowNotification function from alert.m which handles
/// UNUserNotificationCenter for app bundles and logs to console otherwise.
void NeruShowSecureInputNotification(void) {
	NeruShowNotification(
	    "Neru: Secure Input Detected", "Mode activation blocked. A password field or secure input is active.");
}
