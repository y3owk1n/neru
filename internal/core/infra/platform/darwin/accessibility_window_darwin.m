//
//  accessibility_window.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "accessibility.h"
#import "accessibility_visibility.h"
#import <Cocoa/Cocoa.h>

#pragma mark - Focus Functions

/// Set focus to element
/// @param element Element reference
/// @return 1 on success, 0 on failure
int setFocus(void *element) {
	if (!element)
		return 0;

	AXUIElementRef axElement = (AXUIElementRef)element;
	CFBooleanRef trueValue = kCFBooleanTrue;
	AXError error = AXUIElementSetAttributeValue(axElement, kAXFocusedAttribute, trueValue);

	return (error == kAXErrorSuccess) ? 1 : 0;
}

#pragma mark - Window Functions

/// Get all windows of focused application
/// @param count Output parameter for number of windows
/// @return Array of window references
void **getAllWindows(int *count) {
	if (!count)
		return NULL;

	@autoreleasepool {
		AXUIElementRef focusedApp = (AXUIElementRef)getFocusedApplication();
		if (!focusedApp) {
			*count = 0;
			return NULL;
		}

		CFTypeRef windowsValue = NULL;
		if (AXUIElementCopyAttributeValue(focusedApp, kAXWindowsAttribute, &windowsValue) != kAXErrorSuccess) {
			CFRelease(focusedApp);
			*count = 0;
			return NULL;
		}

		if (CFGetTypeID(windowsValue) != CFArrayGetTypeID()) {
			CFRelease(windowsValue);
			CFRelease(focusedApp);
			*count = 0;
			return NULL;
		}

		CFArrayRef windows = (CFArrayRef)windowsValue;
		CFIndex windowCount = CFArrayGetCount(windows);
		*count = (int)windowCount;

		void **result = (void **)malloc(windowCount * sizeof(void *));
		if (!result) {
			CFRelease(windowsValue);
			CFRelease(focusedApp);
			*count = 0;
			return NULL;
		}

		for (CFIndex i = 0; i < windowCount; i++) {
			AXUIElementRef window = (AXUIElementRef)CFArrayGetValueAtIndex(windows, i);
			CFRetain(window);
			result[i] = (void *)window;
		}

		CFRelease(windowsValue);
		CFRelease(focusedApp);
		return result;
	}
}

/// Get frontmost window
/// @return Frontmost window reference
void *getFrontmostWindow(void) {
	@autoreleasepool {
		AXUIElementRef focusedApp = (AXUIElementRef)getFocusedApplication();
		AXUIElementRef appRef = focusedApp;
		bool shouldReleaseAppRef = false;

		if (!appRef) {
			NSRunningApplication *front = [NSWorkspace sharedWorkspace].frontmostApplication;
			if (!front)
				return NULL;
			pid_t pid = front.processIdentifier;
			appRef = AXUIElementCreateApplication(pid);
			if (!appRef)
				return NULL;
			shouldReleaseAppRef = true;
		}

		AXUIElementRef window = NULL;
		AXError error = AXUIElementCopyAttributeValue(appRef, kAXFocusedWindowAttribute, (CFTypeRef *)&window);

		if (shouldReleaseAppRef && appRef) {
			CFRelease(appRef);
		}

		if (error == kAXErrorSuccess && window) {
			if (focusedApp) {
				CFRelease(focusedApp);
			}
			return (void *)window;
		}

		// Fallback: try to get the application's windows list
		if (focusedApp) {
			pid_t pid;
			if (AXUIElementGetPid(focusedApp, &pid) == kAXErrorSuccess) {
				AXUIElementRef appFromPid = AXUIElementCreateApplication(pid);
				if (appFromPid) {
					CFTypeRef windowsValue = NULL;
					if (AXUIElementCopyAttributeValue(appFromPid, kAXWindowsAttribute, &windowsValue) ==
					        kAXErrorSuccess &&
					    windowsValue && CFGetTypeID(windowsValue) == CFArrayGetTypeID()) {
						CFArrayRef windows = (CFArrayRef)windowsValue;
						if (CFArrayGetCount(windows) > 0) {
							AXUIElementRef firstWindow = (AXUIElementRef)CFArrayGetValueAtIndex(windows, 0);
							CFRetain(firstWindow);
							CFRelease(windowsValue);
							CFRelease(appFromPid);
							CFRelease(focusedApp);
							return (void *)firstWindow;
						}
					}
					if (windowsValue)
						CFRelease(windowsValue);
					CFRelease(appFromPid);
				}
			}
			CFRelease(focusedApp);
		}

		return NULL;
	}
}

/// Get application name
/// @param app Application reference
/// @return Application name string
char *getApplicationName(void *app) {
	if (!app)
		return NULL;

	AXUIElementRef axApp = (AXUIElementRef)app;
	CFTypeRef titleValue = NULL;

	if (AXUIElementCopyAttributeValue(axApp, kAXTitleAttribute, &titleValue) != kAXErrorSuccess) {
		return NULL;
	}

	char *result = NULL;
	if (CFGetTypeID(titleValue) == CFStringGetTypeID()) {
		result = cfStringToCString((CFStringRef)titleValue);
	}

	CFRelease(titleValue);
	return result;
}

/// Get bundle identifier
/// @param app Application reference
/// @return Bundle identifier string
char *getBundleIdentifier(void *app) {
	if (!app)
		return NULL;

	@autoreleasepool {
		pid_t pid;
		if (AXUIElementGetPid((AXUIElementRef)app, &pid) != kAXErrorSuccess) {
			return NULL;
		}

		NSRunningApplication *runningApp = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
		if (!runningApp)
			return NULL;

		NSString *bundleId = [runningApp bundleIdentifier];
		if (!bundleId)
			return NULL;

		return cfStringToCString((__bridge CFStringRef)bundleId);
	}
}
