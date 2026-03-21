//
//  accessibility_permissions.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "accessibility.h"

#import <Cocoa/Cocoa.h>

#pragma mark - Permission Functions

/// Check if accessibility permissions are granted
/// @return 1 if permissions are granted, 0 otherwise
int checkAccessibilityPermissions(void) {
	@autoreleasepool {
		NSDictionary *options = @{(__bridge id)kAXTrustedCheckOptionPrompt : @YES};
		Boolean trusted = AXIsProcessTrustedWithOptions((__bridge CFDictionaryRef)options);
		return trusted ? 1 : 0;
	}
}

#pragma mark - Application Functions

/// Set application attribute
/// @param pid Process identifier
/// @param attribute Attribute name
/// @param value Attribute value
/// @return 1 on success, 0 on failure
int setApplicationAttribute(int pid, const char *attribute, int value) {
	if (!attribute)
		return 0;

	@autoreleasepool {
		AXUIElementRef appRef = AXUIElementCreateApplication(pid);
		if (!appRef)
			return 0;

		CFStringRef attrName = CFStringCreateWithCString(NULL, attribute, kCFStringEncodingUTF8);
		if (!attrName) {
			CFRelease(appRef);
			return 0;
		}

		// Check if attribute is settable before attempting to set it
		Boolean isSettable = false;
		AXError checkError = AXUIElementIsAttributeSettable(appRef, attrName, &isSettable);
		if (checkError != kAXErrorSuccess || !isSettable) {
			CFRelease(attrName);
			CFRelease(appRef);
			return 0;
		}

		// Set the attribute value
		CFBooleanRef boolValue = value ? kCFBooleanTrue : kCFBooleanFalse;
		AXError error = AXUIElementSetAttributeValue(appRef, attrName, boolValue);

		CFRelease(attrName);
		CFRelease(appRef);
		return (error == kAXErrorSuccess) ? 1 : 0;
	}
}
