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

/// Get focused window plus popover windows of the focused application.
/// This keeps hint activation on one focused-app lookup and avoids asking Go
/// to fetch each window role separately just to discover AXPopover siblings.
void **getFrontmostAndPopoverWindows(int *count) {
	if (!count)
		return NULL;

	@autoreleasepool {
		*count = 0;

		AXUIElementRef focusedApp = (AXUIElementRef)getFocusedApplication();
		AXUIElementRef appRef = focusedApp;
		bool shouldReleaseAppRef = false;

		if (!appRef) {
			NSRunningApplication *front = [NSWorkspace sharedWorkspace].frontmostApplication;
			if (!front)
				return NULL;

			appRef = AXUIElementCreateApplication(front.processIdentifier);
			if (!appRef)
				return NULL;

			shouldReleaseAppRef = true;
		}

		AXUIElementRef focusedWindow = NULL;
		AXUIElementCopyAttributeValue(appRef, kAXFocusedWindowAttribute, (CFTypeRef *)&focusedWindow);

		CFTypeRef windowsValue = NULL;
		CFArrayRef windows = NULL;
		CFIndex windowCount = 0;
		if (AXUIElementCopyAttributeValue(appRef, kAXWindowsAttribute, &windowsValue) == kAXErrorSuccess &&
		    windowsValue && CFGetTypeID(windowsValue) == CFArrayGetTypeID()) {
			windows = (CFArrayRef)windowsValue;
			windowCount = CFArrayGetCount(windows);
		}

		CFIndex capacity = (focusedWindow ? 1 : 0) + windowCount;
		if (capacity == 0) {
			if (windowsValue)
				CFRelease(windowsValue);
			if (focusedWindow)
				CFRelease(focusedWindow);
			if (shouldReleaseAppRef && appRef)
				CFRelease(appRef);
			else if (focusedApp)
				CFRelease(focusedApp);
			return NULL;
		}

		void **result = (void **)malloc(capacity * sizeof(void *));
		if (!result) {
			if (windowsValue)
				CFRelease(windowsValue);
			if (focusedWindow)
				CFRelease(focusedWindow);
			if (shouldReleaseAppRef && appRef)
				CFRelease(appRef);
			else if (focusedApp)
				CFRelease(focusedApp);
			return NULL;
		}

		if (focusedWindow) {
			result[*count] = (void *)focusedWindow;
			(*count)++;
		} else if (windowCount > 0) {
			AXUIElementRef firstWindow = (AXUIElementRef)CFArrayGetValueAtIndex(windows, 0);
			if (firstWindow) {
				CFRetain(firstWindow);
				result[*count] = (void *)firstWindow;
				(*count)++;
			}
		}

		for (CFIndex i = 0; i < windowCount; i++) {
			AXUIElementRef window = (AXUIElementRef)CFArrayGetValueAtIndex(windows, i);
			if (!window)
				continue;

			if (focusedWindow && CFEqual(window, focusedWindow))
				continue;
			if (!focusedWindow && i == 0)
				continue;

			CFTypeRef roleValue = NULL;
			if (AXUIElementCopyAttributeValue(window, kAXRoleAttribute, &roleValue) != kAXErrorSuccess || !roleValue) {
				continue;
			}

			bool isPopover = CFGetTypeID(roleValue) == CFStringGetTypeID() &&
			                 CFStringCompare((CFStringRef)roleValue, CFSTR("AXPopover"), 0) == kCFCompareEqualTo;
			CFRelease(roleValue);

			if (!isPopover)
				continue;

			CFRetain(window);
			result[*count] = (void *)window;
			(*count)++;
		}

		if (windowsValue)
			CFRelease(windowsValue);

		if (shouldReleaseAppRef && appRef)
			CFRelease(appRef);
		else if (focusedApp)
			CFRelease(focusedApp);

		if (*count == 0) {
			free(result);
			return NULL;
		}

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

		// Fall back to NSWorkspace if AX couldn't find the focused app
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

		// Try focused window attribute first (fast path)
		AXUIElementRef window = NULL;
		AXError error = AXUIElementCopyAttributeValue(appRef, kAXFocusedWindowAttribute, (CFTypeRef *)&window);

		if (shouldReleaseAppRef && appRef) {
			CFRelease(appRef);
		}

		if (error == kAXErrorSuccess && window) {
			if (focusedApp)
				CFRelease(focusedApp);
			return (void *)window;
		}

		// Fallback: try the application's windows list and return the first entry
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

/// Get the frame (position + size) of the focused window
/// @return Window frame rectangle, or CGRectZero if no window is found
CGRect getFocusedWindowFrame(void) {
	@autoreleasepool {
		void *windowRef = getFrontmostWindow();
		if (!windowRef)
			return CGRectZero;

		AXUIElementRef window = (AXUIElementRef)windowRef;

		CFArrayRef attributes = CFArrayCreate(
		    NULL,
		    (const void **)(CFTypeRef[]){
		        kAXPositionAttribute,
		        kAXSizeAttribute,
		    },
		    2, &kCFTypeArrayCallBacks);

		if (!attributes) {
			CFRelease(window);
			return CGRectZero;
		}

		CFArrayRef values = NULL;
		AXError error = AXUIElementCopyMultipleAttributeValues(window, attributes, 0, &values);
		CFRelease(attributes);

		if (error != kAXErrorSuccess || !values) {
			CFRelease(window);
			return CGRectZero;
		}

		CGRect frame = CGRectZero;
		CFIndex count = CFArrayGetCount(values);

		if (count > 0) {
			CFTypeRef positionValue = (CFTypeRef)CFArrayGetValueAtIndex(values, 0);
			if (positionValue && CFGetTypeID(positionValue) == AXValueGetTypeID()) {
				CGPoint point;
				if (AXValueGetValue((AXValueRef)positionValue, kAXValueCGPointType, &point)) {
					frame.origin = point;
				}
			}
		}

		if (count > 1) {
			CFTypeRef sizeValue = (CFTypeRef)CFArrayGetValueAtIndex(values, 1);
			if (sizeValue && CFGetTypeID(sizeValue) == AXValueGetTypeID()) {
				CGSize size;
				if (AXValueGetValue((AXValueRef)sizeValue, kAXValueCGSizeType, &size)) {
					frame.size = size;
				}
			}
		}

		CFRelease(values);
		CFRelease(window);
		return frame;
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
