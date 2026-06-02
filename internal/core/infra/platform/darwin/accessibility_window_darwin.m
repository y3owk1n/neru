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
int NeruSetFocus(void *element) {
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
void **NeruGetAllWindows(int *count) {
	if (!count)
		return NULL;

	@autoreleasepool {
		AXUIElementRef focusedApp = (AXUIElementRef)NeruGetFocusedApplication();
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
void **NeruGetFrontmostAndPopoverWindows(int *count) {
	if (!count)
		return NULL;

	@autoreleasepool {
		*count = 0;

		AXUIElementRef focusedApp = (AXUIElementRef)NeruGetFocusedApplication();
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

		// Batch fetch focused window and all windows in a single AX call
		CFArrayRef windowAttrs = CFArrayCreate(
		    NULL,
		    (CFTypeRef[]){
		        kAXFocusedWindowAttribute,
		        kAXWindowsAttribute,
		    },
		    2, &kCFTypeArrayCallBacks);
		if (!windowAttrs) {
			if (shouldReleaseAppRef && appRef)
				CFRelease(appRef);
			else if (focusedApp)
				CFRelease(focusedApp);
			return NULL;
		}

		CFArrayRef windowValues = NULL;
		AXError batchError = AXUIElementCopyMultipleAttributeValues(appRef, windowAttrs, 0, &windowValues);
		CFRelease(windowAttrs);

		AXUIElementRef focusedWindow = NULL;
		CFTypeRef windowsValue = NULL;
		CFArrayRef windows = NULL;
		CFIndex windowCount = 0;

		if (batchError == kAXErrorSuccess && windowValues && CFArrayGetCount(windowValues) >= 2) {
			CFTypeRef focusedVal = (CFTypeRef)CFArrayGetValueAtIndex(windowValues, 0);
			if (focusedVal && CFGetTypeID(focusedVal) != CFNullGetTypeID()) {
				focusedWindow = (AXUIElementRef)focusedVal;
				CFRetain(focusedWindow);
			}

			CFTypeRef windowsVal = (CFTypeRef)CFArrayGetValueAtIndex(windowValues, 1);
			if (windowsVal && CFGetTypeID(windowsVal) == CFArrayGetTypeID()) {
				windowsValue = windowsVal;
				CFRetain(windowsValue);
				windows = (CFArrayRef)windowsValue;
				windowCount = CFArrayGetCount(windows);
			}
		}

		if (windowValues)
			CFRelease(windowValues);

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
				if (roleValue)
					CFRelease(roleValue);
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
void *NeruGetFrontmostWindow(void) {
	@autoreleasepool {
		AXUIElementRef focusedApp = (AXUIElementRef)NeruGetFocusedApplication();
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

		// Batch fetch focused window and all windows in a single AX call
		CFArrayRef windowAttrs = CFArrayCreate(
		    NULL,
		    (CFTypeRef[]){
		        kAXFocusedWindowAttribute,
		        kAXWindowsAttribute,
		    },
		    2, &kCFTypeArrayCallBacks);
		if (!windowAttrs) {
			if (shouldReleaseAppRef && appRef)
				CFRelease(appRef);
			else if (focusedApp)
				CFRelease(focusedApp);
			return NULL;
		}

		CFArrayRef windowValues = NULL;
		AXError batchError = AXUIElementCopyMultipleAttributeValues(appRef, windowAttrs, 0, &windowValues);
		CFRelease(windowAttrs);

		AXUIElementRef window = NULL;
		CFArrayRef windows = NULL;

		if (batchError == kAXErrorSuccess && windowValues && CFArrayGetCount(windowValues) >= 2) {
			CFTypeRef focusedVal = (CFTypeRef)CFArrayGetValueAtIndex(windowValues, 0);
			if (focusedVal && CFGetTypeID(focusedVal) != CFNullGetTypeID()) {
				window = (AXUIElementRef)focusedVal;
				CFRetain(window);
			}

			CFTypeRef windowsVal = (CFTypeRef)CFArrayGetValueAtIndex(windowValues, 1);
			if (windowsVal && CFGetTypeID(windowsVal) == CFArrayGetTypeID()) {
				windows = (CFArrayRef)windowsVal;
				CFRetain(windows);
			}
		}

		if (windowValues)
			CFRelease(windowValues);

		if (shouldReleaseAppRef && appRef) {
			CFRelease(appRef);
		}

		if (window) {
			if (focusedApp)
				CFRelease(focusedApp);
			if (windows)
				CFRelease(windows);
			return (void *)window;
		}

		// Fallback: use first window from the windows list
		if (windows && CFArrayGetCount(windows) > 0) {
			AXUIElementRef firstWindow = (AXUIElementRef)CFArrayGetValueAtIndex(windows, 0);
			CFRetain(firstWindow);
			CFRelease(windows);
			if (focusedApp)
				CFRelease(focusedApp);
			return (void *)firstWindow;
		}

		if (windows)
			CFRelease(windows);

		if (focusedApp)
			CFRelease(focusedApp);

		return NULL;
	}
}

static CGPoint getWindowPosition(AXUIElementRef window) {
	CFTypeRef positionValue = NULL;
	if (AXUIElementCopyAttributeValue(window, kAXPositionAttribute, &positionValue) == kAXErrorSuccess &&
	    positionValue) {
		CGPoint point = CGPointZero;
		if (CFGetTypeID(positionValue) == AXValueGetTypeID()) {
			AXValueGetValue((AXValueRef)positionValue, kAXValueCGPointType, &point);
		}
		CFRelease(positionValue);
		return point;
	}
	return CGPointZero;
}

/// Get all focusable windows on the active space across all running applications.
/// Filters out non-focusable windows (minimized, hidden, off-space, non-window roles).
/// @param count Output parameter for number of windows
/// @return Array of window element references
void **NeruGetAllFocusableWindowsOnActiveSpace(int *count) {
	if (!count)
		return NULL;

	@autoreleasepool {
		*count = 0;

		NSArray *runningApps = [[NSWorkspace sharedWorkspace].runningApplications
		    sortedArrayUsingComparator:^NSComparisonResult(NSRunningApplication *obj1, NSRunningApplication *obj2) {
			    if (obj1.processIdentifier < obj2.processIdentifier) {
				    return NSOrderedAscending;
			    } else if (obj1.processIdentifier > obj2.processIdentifier) {
				    return NSOrderedDescending;
			    }
			    return NSOrderedSame;
		    }];
		CFMutableArrayRef windowsCollector = CFArrayCreateMutable(NULL, 0, &kCFTypeArrayCallBacks);
		if (!windowsCollector)
			return NULL;

		for (NSRunningApplication *app in runningApps) {
			if (app.activationPolicy != NSApplicationActivationPolicyRegular)
				continue;
			if (app.hidden)
				continue;

			pid_t pid = app.processIdentifier;
			AXUIElementRef appElement = AXUIElementCreateApplication(pid);
			if (!appElement)
				continue;

			CFTypeRef windowsValue = NULL;
			AXError error = AXUIElementCopyAttributeValue(appElement, kAXWindowsAttribute, &windowsValue);
			if (error != kAXErrorSuccess || !windowsValue) {
				CFRelease(appElement);
				continue;
			}

			if (CFGetTypeID(windowsValue) != CFArrayGetTypeID()) {
				CFRelease(windowsValue);
				CFRelease(appElement);
				continue;
			}

			CFArrayRef windows = (CFArrayRef)windowsValue;
			CFIndex windowCount = CFArrayGetCount(windows);

			for (CFIndex i = 0; i < windowCount; i++) {
				AXUIElementRef window = (AXUIElementRef)CFArrayGetValueAtIndex(windows, i);
				if (!window)
					continue;

				// Batch-fetch attributes for efficiency
				CFStringRef attrs[] = {
				    kAXRoleAttribute,
				    kAXMinimizedAttribute,
				    CFSTR("AXWindowIsOnActiveSpace"),
				};
				CFArrayRef attrArray = CFArrayCreate(NULL, (const void **)attrs, 3, &kCFTypeArrayCallBacks);
				if (!attrArray)
					continue;

				CFArrayRef values = NULL;
				AXError batchError = AXUIElementCopyMultipleAttributeValues(window, attrArray, 0, &values);
				CFRelease(attrArray);

				if (batchError != kAXErrorSuccess || !values) {
					if (values)
						CFRelease(values);
					continue;
				}

				bool shouldInclude = false;

				// role: must be AXWindow
				if (CFArrayGetCount(values) > 0) {
					CFTypeRef roleVal = (CFTypeRef)CFArrayGetValueAtIndex(values, 0);
					if (roleVal && CFGetTypeID(roleVal) == CFStringGetTypeID() &&
					    CFStringCompare((CFStringRef)roleVal, CFSTR("AXWindow"), 0) == kCFCompareEqualTo) {
						shouldInclude = true;
					}
				}

				// minimized: exclude if true
				if (shouldInclude && CFArrayGetCount(values) > 1) {
					CFTypeRef minVal = (CFTypeRef)CFArrayGetValueAtIndex(values, 1);
					if (minVal && CFGetTypeID(minVal) == CFBooleanGetTypeID() &&
					    CFBooleanGetValue((CFBooleanRef)minVal)) {
						shouldInclude = false;
					}
				}

				// on active space: exclude if false or unsupported
				if (shouldInclude && CFArrayGetCount(values) > 2) {
					CFTypeRef spaceVal = (CFTypeRef)CFArrayGetValueAtIndex(values, 2);
					if (spaceVal && CFGetTypeID(spaceVal) == CFBooleanGetTypeID() &&
					    !CFBooleanGetValue((CFBooleanRef)spaceVal)) {
						shouldInclude = false;
					}
				}

				CFRelease(values);

				if (shouldInclude) {
					CFArrayAppendValue(windowsCollector, window);
				}
			}

			CFRelease(windowsValue);
			CFRelease(appElement);
		}

		CFIndex total = CFArrayGetCount(windowsCollector);
		if (total == 0) {
			CFRelease(windowsCollector);
			return NULL;
		}

		// Pre-compute window positions and PIDs to avoid O(N log N) IPC calls during sorting
		NSMutableDictionary<NSValue *, NSValue *> *positions = [NSMutableDictionary dictionaryWithCapacity:total];
		NSMutableDictionary<NSValue *, NSNumber *> *pids = [NSMutableDictionary dictionaryWithCapacity:total];
		for (CFIndex i = 0; i < total; i++) {
			AXUIElementRef w = (AXUIElementRef)CFArrayGetValueAtIndex(windowsCollector, i);
			CGPoint pos = getWindowPosition(w);
			positions[[NSValue valueWithPointer:w]] = [NSValue valueWithBytes:&pos objCType:@encode(CGPoint)];

			pid_t pid = 0;
			AXUIElementGetPid(w, &pid);
			pids[[NSValue valueWithPointer:w]] = @(pid);
		}

		// Sort the collected windows stably by screen coordinates (y-coordinate first, then x-coordinate)
		NSArray *sortedWindows =
		    [(__bridge NSArray *)windowsCollector sortedArrayUsingComparator:^NSComparisonResult(id obj1, id obj2) {
			    AXUIElementRef w1 = (__bridge AXUIElementRef)obj1;
			    AXUIElementRef w2 = (__bridge AXUIElementRef)obj2;

			    NSValue *key1 = [NSValue valueWithPointer:w1];
			    NSValue *key2 = [NSValue valueWithPointer:w2];

			    CGPoint p1 = CGPointZero;
			    CGPoint p2 = CGPointZero;
			    [positions[key1] getValue:&p1];
			    [positions[key2] getValue:&p2];

			    if (p1.y < p2.y)
				    return NSOrderedAscending;
			    if (p1.y > p2.y)
				    return NSOrderedDescending;
			    if (p1.x < p2.x)
				    return NSOrderedAscending;
			    if (p1.x > p2.x)
				    return NSOrderedDescending;

			    int pid1 = [pids[key1] intValue];
			    int pid2 = [pids[key2] intValue];
			    if (pid1 < pid2)
				    return NSOrderedAscending;
			    if (pid1 > pid2)
				    return NSOrderedDescending;

			    return NSOrderedSame;
		    }];

		void **result = (void **)malloc(total * sizeof(void *));
		if (!result) {
			CFRelease(windowsCollector);
			return NULL;
		}

		for (CFIndex i = 0; i < total; i++) {
			result[i] = (void *)(__bridge AXUIElementRef)sortedWindows[i];
			CFRetain(result[i]);
		}

		CFRelease(windowsCollector);
		*count = (int)total;
		return result;
	}
}

/// Activate a window: brings its application to the foreground and sets
/// keyboard focus on the window.
/// @param window Window element reference
/// @return 1 on success, 0 on failure
int NeruActivateWindow(void *window) {
	if (!window)
		return 0;

	@autoreleasepool {
		AXUIElementRef axWindow = (AXUIElementRef)window;

		pid_t pid;
		if (AXUIElementGetPid(axWindow, &pid) != kAXErrorSuccess)
			return 0;

		NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
		if (!app)
			return 0;

		[app activateWithOptions:NSApplicationActivateIgnoringOtherApps];

		// Set kAXMainAttribute to make it the main window of the application
		AXUIElementSetAttributeValue(axWindow, kAXMainAttribute, kCFBooleanTrue);

		// Set kAXFocusedAttribute to give it keyboard focus
		AXError error = AXUIElementSetAttributeValue(axWindow, kAXFocusedAttribute, kCFBooleanTrue);

		// Perform AXRaise action to bring the window to the front
		AXUIElementPerformAction(axWindow, kAXRaiseAction);

		return (error == kAXErrorSuccess) ? 1 : 0;
	}
}

/// Get the frame (position + size) of the focused window
/// @return Window frame rectangle, or CGRectZero if no window is found
CGRect NeruGetFocusedWindowFrame(void) {
	@autoreleasepool {
		void *windowRef = NeruGetFrontmostWindow();
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
			if (values)
				CFRelease(values);
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
char *NeruGetApplicationName(void *app) {
	if (!app)
		return NULL;

	AXUIElementRef axApp = (AXUIElementRef)app;
	CFTypeRef titleValue = NULL;

	if (AXUIElementCopyAttributeValue(axApp, kAXTitleAttribute, &titleValue) != kAXErrorSuccess) {
		return NULL;
	}

	char *result = NULL;
	if (CFGetTypeID(titleValue) == CFStringGetTypeID()) {
		result = NeruCFStringToCString((CFStringRef)titleValue);
	}

	CFRelease(titleValue);
	return result;
}

/// Get bundle identifier
/// @param app Application reference
/// @return Bundle identifier string
char *NeruGetBundleIdentifier(void *app) {
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

		return NeruCFStringToCString((__bridge CFStringRef)bundleId);
	}
}

/// Get bundle identifier from PID directly
/// @param pid Process identifier
/// @return Bundle identifier string, or NULL if not found
char *NeruGetBundleIDForPID(int pid) {
	@autoreleasepool {
		NSRunningApplication *runningApp = [NSRunningApplication runningApplicationWithProcessIdentifier:(pid_t)pid];
		if (!runningApp)
			return NULL;

		NSString *bundleId = [runningApp bundleIdentifier];
		if (!bundleId)
			return NULL;

		return NeruCFStringToCString((__bridge CFStringRef)bundleId);
	}
}
