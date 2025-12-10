//
//  accessibility.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "accessibility.h"
#import <Cocoa/Cocoa.h>
#include <sys/time.h>
#include <unistd.h>

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

		// Check if attribute is settable
		Boolean isSettable = false;
		AXError checkError = AXUIElementIsAttributeSettable(appRef, attrName, &isSettable);
		if (checkError != kAXErrorSuccess || !isSettable) {
			CFRelease(attrName);
			CFRelease(appRef);
			return 0;
		}

		CFBooleanRef boolValue = value ? kCFBooleanTrue : kCFBooleanFalse;
		AXError error = AXUIElementSetAttributeValue(appRef, attrName, boolValue);
		CFRelease(attrName);
		CFRelease(appRef);
		return (error == kAXErrorSuccess) ? 1 : 0;
	}
}

#pragma mark - Visibility Functions

/// Helper function to check if a point is actually visible (not occluded by other windows)
/// @param point Screen position to check
/// @param elementPid Process identifier of the element
/// @return true if point is visible, false otherwise
static bool isPointVisible(CGPoint point, pid_t elementPid) {
	AXUIElementRef systemWide = AXUIElementCreateSystemWide();
	if (!systemWide)
		return true;

	AXUIElementRef elementAtPoint = NULL;
	AXError error = AXUIElementCopyElementAtPosition(systemWide, point.x, point.y, &elementAtPoint);
	CFRelease(systemWide);

	if (error != kAXErrorSuccess || !elementAtPoint) {
		return true; // Assume visible if we can't check
	}

	// Get the PID of the element at this point
	pid_t pidAtPoint;
	bool isVisible = false;

	if (AXUIElementGetPid(elementAtPoint, &pidAtPoint) == kAXErrorSuccess) {
		// The point is visible if it belongs to the same application
		isVisible = (pidAtPoint == elementPid);
	}

	CFRelease(elementAtPoint);
	return isVisible;
}

/// Helper function to check if an element is occluded by checking multiple sample points
/// @param elementRect Element bounds
/// @param elementPid Process identifier of the element
/// @return true if element is occluded, false otherwise
static bool isElementOccluded(CGRect elementRect, pid_t elementPid) {
	// Sample 5 points: center and 4 corners (slightly inset)
	CGFloat inset = 2.0; // Inset from edges to avoid border issues

	CGPoint samplePoints[5] = {
	    // Center
	    CGPointMake(elementRect.origin.x + elementRect.size.width / 2,
	                elementRect.origin.y + elementRect.size.height / 2),
	    // Top-left
	    CGPointMake(elementRect.origin.x + inset, elementRect.origin.y + inset),
	    // Top-right
	    CGPointMake(elementRect.origin.x + elementRect.size.width - inset, elementRect.origin.y + inset),
	    // Bottom-left
	    CGPointMake(elementRect.origin.x + inset, elementRect.origin.y + elementRect.size.height - inset),
	    // Bottom-right
	    CGPointMake(elementRect.origin.x + elementRect.size.width - inset,
	                elementRect.origin.y + elementRect.size.height - inset)};

	// Element is considered visible if at least 2 sample points are visible
	int visiblePoints = 0;
	for (int i = 0; i < 5; i++) {
		if (isPointVisible(samplePoints[i], elementPid)) {
			visiblePoints++;
			if (visiblePoints >= 2) {
				return false; // Not occluded
			}
		}
	}

	return true; // Occluded (less than 2 points visible)
}

#pragma mark - Element Accessor Functions

/// Get system-wide accessibility element
/// @return System-wide element reference
void *getSystemWideElement(void) {
	AXUIElementRef systemWide = AXUIElementCreateSystemWide();
	return (void *)systemWide;
}

/// Get focused application
/// @return Focused application reference
void *getFocusedApplication(void) {
	@autoreleasepool {
		AXUIElementRef systemWide = AXUIElementCreateSystemWide();
		if (systemWide) {
			AXUIElementRef focusedApp = NULL;
			AXError error =
			    AXUIElementCopyAttributeValue(systemWide, kAXFocusedApplicationAttribute, (CFTypeRef *)&focusedApp);

			CFRelease(systemWide);

			if (error == kAXErrorSuccess && focusedApp) {
				return (void *)focusedApp;
			}
		}

		// Fallback: use NSWorkspace frontmostApplication
		NSRunningApplication *front = [NSWorkspace sharedWorkspace].frontmostApplication;
		if (!front)
			return NULL;
		pid_t pid = front.processIdentifier;
		AXUIElementRef axApp = AXUIElementCreateApplication(pid);
		return (void *)axApp;
	}
}

/// Get the menu bar element of an application
/// @param app Application reference
/// @return Menu bar element reference
void *getMenuBar(void *app) {
	if (!app)
		return NULL;

	AXUIElementRef axApp = (AXUIElementRef)app;
	AXUIElementRef menubar = NULL;
	AXError error = AXUIElementCopyAttributeValue(axApp, kAXMenuBarAttribute, (CFTypeRef *)&menubar);
	if (error != kAXErrorSuccess) {
		return NULL;
	}
	return (void *)menubar;
}

/// Get application by PID
/// @param pid Process identifier
/// @return Application reference
void *getApplicationByPID(int pid) {
	AXUIElementRef app = AXUIElementCreateApplication(pid);
	return (void *)app;
}

/// Get application by bundle identifier
/// @param bundle_id Bundle identifier
/// @return Application reference
void *getApplicationByBundleId(const char *bundle_id) {
	if (!bundle_id)
		return NULL;

	@autoreleasepool {
		NSString *bundleIdStr = [NSString stringWithUTF8String:bundle_id];
		NSArray<NSRunningApplication *> *apps =
		    [NSRunningApplication runningApplicationsWithBundleIdentifier:bundleIdStr];
		if (apps.count == 0) {
			return NULL;
		}
		NSRunningApplication *app = apps.firstObject;
		pid_t pid = app.processIdentifier;
		AXUIElementRef axApp = AXUIElementCreateApplication(pid);
		return (void *)axApp;
	}
}

#pragma mark - String Conversion Functions

/// Helper function to convert CFStringRef to C string
/// @param cfStr CFString reference
/// @return C string (must be freed by caller)
char *cfStringToCString(CFStringRef cfStr) {
	if (!cfStr)
		return NULL;

	CFIndex length = CFStringGetLength(cfStr);
	CFIndex maxSize = CFStringGetMaximumSizeForEncoding(length, kCFStringEncodingUTF8) + 1;
	char *buffer = (char *)malloc(maxSize);
	if (!buffer)
		return NULL;

	if (CFStringGetCString(cfStr, buffer, maxSize, kCFStringEncodingUTF8)) {
		return buffer;
	}

	free(buffer);
	return NULL;
}

#pragma mark - Element Information Functions

/// Get element information
/// @param element Element reference
/// @return Element information structure
ElementInfo *getElementInfo(void *element) {
	if (!element)
		return NULL;

	@autoreleasepool {
		AXUIElementRef axElement = (AXUIElementRef)element;
		ElementInfo *info = (ElementInfo *)calloc(1, sizeof(ElementInfo));
		if (!info)
			return NULL;

		// Get position
		CFTypeRef positionValue = NULL;
		if (AXUIElementCopyAttributeValue(axElement, kAXPositionAttribute, &positionValue) == kAXErrorSuccess) {
			CGPoint point;
			if (AXValueGetValue(positionValue, kAXValueCGPointType, &point)) {
				info->position = point;
			}
			CFRelease(positionValue);
		}

		// Get size
		CFTypeRef sizeValue = NULL;
		if (AXUIElementCopyAttributeValue(axElement, kAXSizeAttribute, &sizeValue) == kAXErrorSuccess) {
			CGSize size;
			if (AXValueGetValue(sizeValue, kAXValueCGSizeType, &size)) {
				info->size = size;
			}
			CFRelease(sizeValue);
		}

		// Get title
		CFTypeRef titleValue = NULL;
		if (AXUIElementCopyAttributeValue(axElement, kAXTitleAttribute, &titleValue) == kAXErrorSuccess) {
			if (CFGetTypeID(titleValue) == CFStringGetTypeID()) {
				info->title = cfStringToCString((CFStringRef)titleValue);
			}
			CFRelease(titleValue);
		}

		// Get role
		CFTypeRef roleValue = NULL;
		if (AXUIElementCopyAttributeValue(axElement, kAXRoleAttribute, &roleValue) == kAXErrorSuccess) {
			if (CFGetTypeID(roleValue) == CFStringGetTypeID()) {
				info->role = cfStringToCString((CFStringRef)roleValue);
			}
			CFRelease(roleValue);
		}

		// Get role description
		CFTypeRef roleDescValue = NULL;
		if (AXUIElementCopyAttributeValue(axElement, kAXRoleDescriptionAttribute, &roleDescValue) == kAXErrorSuccess) {
			if (CFGetTypeID(roleDescValue) == CFStringGetTypeID()) {
				info->roleDescription = cfStringToCString((CFStringRef)roleDescValue);
			}
			CFRelease(roleDescValue);
		}

		// Get enabled state
		CFTypeRef enabledValue = NULL;
		if (AXUIElementCopyAttributeValue(axElement, kAXEnabledAttribute, &enabledValue) == kAXErrorSuccess) {
			if (CFGetTypeID(enabledValue) == CFBooleanGetTypeID()) {
				info->isEnabled = CFBooleanGetValue((CFBooleanRef)enabledValue);
			}
			CFRelease(enabledValue);
		}

		// Get focused state
		CFTypeRef focusedValue = NULL;
		if (AXUIElementCopyAttributeValue(axElement, kAXFocusedAttribute, &focusedValue) == kAXErrorSuccess) {
			if (CFGetTypeID(focusedValue) == CFBooleanGetTypeID()) {
				info->isFocused = CFBooleanGetValue((CFBooleanRef)focusedValue);
			}
			CFRelease(focusedValue);
		}

		// Get PID
		pid_t pid;
		if (AXUIElementGetPid(axElement, &pid) == kAXErrorSuccess) {
			info->pid = pid;
		}

		return info;
	}
}

/// Free element info
/// @param info Element information structure
void freeElementInfo(ElementInfo *info) {
	if (!info)
		return;

	if (info->title)
		free(info->title);
	if (info->role)
		free(info->role);
	if (info->roleDescription)
		free(info->roleDescription);
	free(info);
}

#pragma mark - Position Functions

/// Get element at screen position
/// @param position Screen position
/// @return Element reference
void *getElementAtPosition(CGPoint position) {
	AXUIElementRef systemWide = AXUIElementCreateSystemWide();
	if (!systemWide)
		return NULL;

	AXUIElementRef element = NULL;
	AXError error = AXUIElementCopyElementAtPosition(systemWide, position.x, position.y, &element);

	CFRelease(systemWide);

	if (error != kAXErrorSuccess) {
		return NULL;
	}

	return (void *)element;
}

#pragma mark - Child Element Functions

/// Get number of child elements
/// @param element Element reference
/// @return Number of children
int getChildrenCount(void *element) {
	if (!element)
		return 0;

	AXUIElementRef axElement = (AXUIElementRef)element;
	CFTypeRef childrenValue = NULL;

	if (AXUIElementCopyAttributeValue(axElement, kAXChildrenAttribute, &childrenValue) != kAXErrorSuccess) {
		return 0;
	}

	if (CFGetTypeID(childrenValue) != CFArrayGetTypeID()) {
		CFRelease(childrenValue);
		return 0;
	}

	CFIndex count = CFArrayGetCount((CFArrayRef)childrenValue);
	CFRelease(childrenValue);

	return (int)count;
}

/// Get child elements
/// @param element Element reference
/// @param count Output parameter for number of children
/// @return Array of child element references
void **getChildren(void *element, int *count) {
	if (!element || !count)
		return NULL;

	AXUIElementRef axElement = (AXUIElementRef)element;
	CFTypeRef childrenValue = NULL;

	if (AXUIElementCopyAttributeValue(axElement, kAXChildrenAttribute, &childrenValue) != kAXErrorSuccess) {
		*count = 0;
		return NULL;
	}

	if (CFGetTypeID(childrenValue) != CFArrayGetTypeID()) {
		CFRelease(childrenValue);
		*count = 0;
		return NULL;
	}

	CFArrayRef children = (CFArrayRef)childrenValue;
	CFIndex childCount = CFArrayGetCount(children);
	*count = (int)childCount;

	void **result = (void **)malloc(childCount * sizeof(void *));
	if (!result) {
		CFRelease(childrenValue);
		*count = 0;
		return NULL;
	}

	for (CFIndex i = 0; i < childCount; i++) {
		AXUIElementRef child = (AXUIElementRef)CFArrayGetValueAtIndex(children, i);
		CFRetain(child);
		result[i] = (void *)child;
	}

	CFRelease(childrenValue);
	return result;
}

/// Get visible rows of an element
/// @param element Element reference
/// @param count Output parameter for number of rows
/// @return Array of row element references
void **getVisibleRows(void *element, int *count) {
	if (!element || !count)
		return NULL;

	AXUIElementRef axElement = (AXUIElementRef)element;
	CFTypeRef rowsValue = NULL;

	if (AXUIElementCopyAttributeValue(axElement, kAXVisibleRowsAttribute, &rowsValue) != kAXErrorSuccess) {
		*count = 0;
		return NULL;
	}

	if (CFGetTypeID(rowsValue) != CFArrayGetTypeID()) {
		CFRelease(rowsValue);
		*count = 0;
		return NULL;
	}

	CFArrayRef rows = (CFArrayRef)rowsValue;
	CFIndex rowCount = CFArrayGetCount(rows);
	*count = (int)rowCount;

	void **result = (void **)malloc(rowCount * sizeof(void *));
	if (!result) {
		CFRelease(rowsValue);
		*count = 0;
		return NULL;
	}

	for (CFIndex i = 0; i < rowCount; i++) {
		AXUIElementRef row = (AXUIElementRef)CFArrayGetValueAtIndex(rows, i);
		CFRetain(row);
		result[i] = (void *)row;
	}

	CFRelease(rowsValue);
	return result;
}

#pragma mark - Constants

static CFStringRef kAXLinkRole = CFSTR("AXLink");
static CFStringRef kAXCheckboxRole = CFSTR("AXCheckBox");
static CFStringRef kAXFocusableAttribute = CFSTR("AXFocusable");
static CFStringRef kAXVisibleAttribute = CFSTR("AXVisible");

#pragma mark - Click Action Functions

/// Check if element has click action
/// @param element Element reference
/// @return 1 if element is clickable, 0 otherwise
int hasClickAction(void *element) {
	if (!element)
		return 0;

	AXUIElementRef axElement = (AXUIElementRef)element;

	// Ignore hidden or disabled elements early
	CFBooleanRef hidden = NULL;
	if (AXUIElementCopyAttributeValue(axElement, kAXHiddenAttribute, (CFTypeRef *)&hidden) == kAXErrorSuccess &&
	    hidden) {
		if (CFBooleanGetValue(hidden)) {
			CFRelease(hidden);
			return 0;
		}
		CFRelease(hidden);
	}

	CFBooleanRef enabled = NULL;
	bool isEnabled = true;
	if (AXUIElementCopyAttributeValue(axElement, kAXEnabledAttribute, (CFTypeRef *)&enabled) == kAXErrorSuccess &&
	    enabled) {
		isEnabled = CFBooleanGetValue(enabled);
		CFRelease(enabled);
	}
	if (!isEnabled)
		return 0;

	// Get role for role-specific fallbacks
	CFStringRef role = NULL;
	if (AXUIElementCopyAttributeValue(axElement, kAXRoleAttribute, (CFTypeRef *)&role) != kAXErrorSuccess) {
		role = NULL;
	}

	// Explicit actions are the strongest signal
	CFArrayRef actions = NULL;
	if (AXUIElementCopyActionNames(axElement, &actions) == kAXErrorSuccess && actions) {
		CFIndex count = CFArrayGetCount(actions);
		for (CFIndex i = 0; i < count; i++) {
			CFStringRef action = (CFStringRef)CFArrayGetValueAtIndex(actions, i);
			if (CFStringCompare(action, kAXPressAction, 0) == kCFCompareEqualTo ||
			    CFStringCompare(action, CFSTR("AXShowMenu"), 0) == kCFCompareEqualTo ||
			    CFStringCompare(action, CFSTR("AXConfirm"), 0) == kCFCompareEqualTo ||
			    CFStringCompare(action, CFSTR("AXPick"), 0) == kCFCompareEqualTo ||
			    CFStringCompare(action, CFSTR("AXRaise"), 0) == kCFCompareEqualTo) {
				CFRelease(actions);
				if (role)
					CFRelease(role);
				return 1;
			}
		}
		CFRelease(actions);
	}

	// Some elements support AXPress even if not listed in action names
	CFStringRef pressDesc = NULL;
	if (AXUIElementCopyActionDescription(axElement, kAXPressAction, &pressDesc) == kAXErrorSuccess && pressDesc) {
		CFRelease(pressDesc);
		if (role)
			CFRelease(role);
		return 1;
	}

	// Focusable and enabled controls are clickable
	CFBooleanRef focusable = NULL;
	if (AXUIElementCopyAttributeValue(axElement, kAXFocusableAttribute, (CFTypeRef *)&focusable) == kAXErrorSuccess &&
	    focusable) {
		if (CFBooleanGetValue(focusable)) {
			CFRelease(focusable);
			CGPoint center;
			pid_t pid;
			bool visible = true;
			if (getElementCenter((void *)axElement, &center) && AXUIElementGetPid(axElement, &pid) == kAXErrorSuccess) {
				visible = isPointVisible(center, pid);
			}
			if (role)
				CFRelease(role);
			return visible ? 1 : 0;
		}
		CFRelease(focusable);
	}

	// Role-specific fallback for links
	if (role && CFStringCompare(role, kAXLinkRole, 0) == kCFCompareEqualTo) {
		CFTypeRef urlAttr = NULL;
		if (AXUIElementCopyAttributeValue(axElement, kAXURLAttribute, &urlAttr) == kAXErrorSuccess && urlAttr) {
			CFRelease(urlAttr);
			CFRelease(role);
			return 1;
		}
	}

	if (role)
		CFRelease(role);

	// Final check: visible bounding box and not occluded
	CGPoint center;
	pid_t pid;
	if (getElementCenter((void *)axElement, &center) && AXUIElementGetPid(axElement, &pid) == kAXErrorSuccess) {
		return isPointVisible(center, pid) ? 1 : 0;
	}

	return 0;
}

/// Get the center point of an element
/// @param element Element reference
/// @param outPoint Output parameter for center point
/// @return 1 on success, 0 on failure
int getElementCenter(void *element, CGPoint *outPoint) {
	if (!element || !outPoint)
		return 0;

	AXUIElementRef axElement = (AXUIElementRef)element;
	*outPoint = CGPointZero;

	CFTypeRef positionRef = NULL;
	AXError error = AXUIElementCopyAttributeValue(axElement, kAXPositionAttribute, &positionRef);

	if (error != kAXErrorSuccess || !positionRef) {
		return 0;
	}

	if (!AXValueGetValue((AXValueRef)positionRef, kAXValueCGPointType, outPoint)) {
		CFRelease(positionRef);
		return 0;
	}
	CFRelease(positionRef);

	// Get size and offset to center
	CFTypeRef sizeRef = NULL;
	if (AXUIElementCopyAttributeValue(axElement, kAXSizeAttribute, &sizeRef) == kAXErrorSuccess && sizeRef) {
		CGSize size;
		if (AXValueGetValue((AXValueRef)sizeRef, kAXValueCGSizeType, &size)) {
			outPoint->x += size.width / 2.0;
			outPoint->y += size.height / 2.0;
		}
		CFRelease(sizeRef);
	}

	return 1;
}

#pragma mark - Mouse Functions

// Timing constants for mouse click operations
static const CFTimeInterval kMouseClickDownUpDelay = 0.008;    // Delay between down and up events
static const CFTimeInterval kMouseClickProcessingDelay = 0.04; // Delay after click processing

/// Move mouse cursor to position
/// @param position Target position
void moveMouse(CGPoint position) {
	CGEventRef move = CGEventCreateMouseEvent(NULL, kCGEventMouseMoved, position, kCGMouseButtonLeft);
	if (move) {
		CGEventPost(kCGHIDEventTap, move);
		CFRelease(move);
		CFRunLoopRunInMode(kCFRunLoopDefaultMode, 0.01, false);
	}
}

/// Move mouse cursor smoothly to position
/// @param startPosition Starting position
/// @param endPosition Target position
/// @param steps Number of steps for smooth movement
/// @param delay Delay between steps in milliseconds
void moveMouseSmooth(CGPoint startPosition, CGPoint endPosition, int steps, int delay) {
	if (steps <= 0)
		steps = 10;
	if (delay <= 0)
		delay = 5;

	for (int i = 1; i <= steps; i++) {
		double progress = (double)i / (double)steps;
		CGPoint currentPos = CGPointMake(startPosition.x + (endPosition.x - startPosition.x) * progress,
		                                 startPosition.y + (endPosition.y - startPosition.y) * progress);

		CGEventRef move = CGEventCreateMouseEvent(NULL, kCGEventMouseMoved, currentPos, kCGMouseButtonLeft);
		if (move) {
			CGEventPost(kCGHIDEventTap, move);
			CFRelease(move);
			CFRunLoopRunInMode(kCFRunLoopDefaultMode, 0.001, false);
		}

		// Small delay for smooth movement
		usleep(delay * 1000);
	}
}

#pragma mark - Mouse Action Functions

/// Release the left button without moving
/// @return 1 on success, 0 on failure
int performLeftMouseUpAtCursor(void) {
	CGEventRef currentEvent = CGEventCreate(NULL);
	if (!currentEvent)
		return 0;

	CGPoint currentPos = CGEventGetLocation(currentEvent);
	CFRelease(currentEvent);

	CGEventRef up = CGEventCreateMouseEvent(NULL, kCGEventLeftMouseUp, currentPos, kCGMouseButtonLeft);
	if (!up)
		return 0;

	// Clear all modifier flags to ensure clean mouse up
	CGEventSetFlags(up, 0);
	CGEventPost(kCGHIDEventTap, up);
	CFRelease(up);

	CFRunLoopRunInMode(kCFRunLoopDefaultMode, 0.05, false);
	return 1;
}

/// Generic click at position
/// @param pos Target position
/// @param downEvent Mouse down event type
/// @param upEvent Mouse up event type
/// @param button Mouse button
/// @param restoreCursor Whether to restore cursor position after click
/// @return 1 on success, 0 on failure
static int performClickAtPosition(CGPoint pos, CGEventType downEvent, CGEventType upEvent, CGMouseButton button,
                                  bool restoreCursor) {
	CGPoint originalPosition = CGPointZero;
	if (restoreCursor) {
		CGEventRef currentEvent = CGEventCreate(NULL);
		if (currentEvent) {
			originalPosition = CGEventGetLocation(currentEvent);
			CFRelease(currentEvent);
		}
	}

	moveMouse(pos);

	CGEventRef down = CGEventCreateMouseEvent(NULL, downEvent, pos, button);
	CGEventRef up = CGEventCreateMouseEvent(NULL, upEvent, pos, button);
	if (!down || !up) {
		if (down)
			CFRelease(down);
		if (up)
			CFRelease(up);
		if (restoreCursor)
			moveMouse(originalPosition);
		return 0;
	}
	// Clear all modifier flags to ensure clean click without Cmd/Shift/etc
	CGEventSetFlags(down, 0);
	CGEventSetFlags(up, 0);

	// Post mouse down, allow the system to process it, then post mouse up.
	CGEventPost(kCGHIDEventTap, down);
	// Give the event loop a short moment to register the down event before sending up.
	CFRunLoopRunInMode(kCFRunLoopDefaultMode, kMouseClickDownUpDelay, false);

	CGEventPost(kCGHIDEventTap, up);
	CFRelease(down);
	CFRelease(up);

	// Allow a small amount of time for the click to be processed by the system
	// before restoring the cursor to avoid clicks landing in-transit.
	CFRunLoopRunInMode(kCFRunLoopDefaultMode, kMouseClickProcessingDelay, false);

	if (restoreCursor)
		moveMouse(originalPosition);
	return 1;
}

/// State tracking for click detection
static struct {
	CGPoint lastPosition;         ///< Last click position
	struct timeval lastClickTime; ///< Last click time
	int clickCount;               ///< Current click count
} clickState = {0};

/// Get current time in milliseconds
/// @return Current time in milliseconds
static long long getCurrentTimeMs(void) {
	struct timeval tv;
	gettimeofday(&tv, NULL);
	return (long long)tv.tv_sec * 1000 + tv.tv_usec / 1000;
}

/// Perform left click at position
/// @param position Target position
/// @param restoreCursor Whether to restore cursor position after click
/// @return 1 on success, 0 on failure
int performLeftClickAtPosition(CGPoint position, bool restoreCursor) {
	CGPoint originalPosition = CGPointZero;
	if (restoreCursor) {
		CGEventRef currentEvent = CGEventCreate(NULL);
		if (currentEvent) {
			originalPosition = CGEventGetLocation(currentEvent);
			CFRelease(currentEvent);
		}
	}

	// Get current time
	long long currentTime = getCurrentTimeMs();
	long long lastTime = (long long)clickState.lastClickTime.tv_sec * 1000 + clickState.lastClickTime.tv_usec / 1000;
	long long timeDiff = currentTime - lastTime;

	// Check if this is a multi-click (within 500ms and at same position)
	// macOS typically uses 500ms as the double-click interval
	double distance =
	    sqrt(pow(position.x - clickState.lastPosition.x, 2) + pow(position.y - clickState.lastPosition.y, 2));

	if (timeDiff < 500 && distance < 5.0) {
		// Same location, quick succession - increment click count
		clickState.clickCount++;
	} else {
		// New click sequence
		clickState.clickCount = 1;
	}

	// Update state
	clickState.lastPosition = position;
	gettimeofday(&clickState.lastClickTime, NULL);

	moveMouse(position);

	CGEventRef down = CGEventCreateMouseEvent(NULL, kCGEventLeftMouseDown, position, kCGMouseButtonLeft);
	CGEventRef up = CGEventCreateMouseEvent(NULL, kCGEventLeftMouseUp, position, kCGMouseButtonLeft);

	if (!down || !up) {
		if (down)
			CFRelease(down);
		if (up)
			CFRelease(up);
		if (restoreCursor)
			moveMouse(originalPosition);
		return 0;
	}

	// Clear all modifier flags to ensure clean click
	CGEventSetFlags(down, 0);
	CGEventSetFlags(up, 0);

	// Set the click count
	CGEventSetIntegerValueField(down, kCGMouseEventClickState, clickState.clickCount);
	CGEventSetIntegerValueField(up, kCGMouseEventClickState, clickState.clickCount);

	// Post mouse down and allow a short moment before posting mouse up to ensure
	// the system attributes the down/up pair to the target location.
	CGEventPost(kCGHIDEventTap, down);
	CFRunLoopRunInMode(kCFRunLoopDefaultMode, kMouseClickDownUpDelay, false);

	CGEventPost(kCGHIDEventTap, up);
	CFRelease(down);
	CFRelease(up);

	// Wait briefly to let the OS process the click before potentially moving the cursor back.
	CFRunLoopRunInMode(kCFRunLoopDefaultMode, kMouseClickProcessingDelay, false);

	if (restoreCursor)
		moveMouse(originalPosition);
	return 1;
}

/// Perform right click at position
/// @param position Target position
/// @param restoreCursor Whether to restore cursor position after click
/// @return 1 on success, 0 on failure
int performRightClickAtPosition(CGPoint position, bool restoreCursor) {
	return performClickAtPosition(position, kCGEventRightMouseDown, kCGEventRightMouseUp, kCGMouseButtonRight,
	                              restoreCursor);
}

/// Perform middle click at position
/// @param position Target position
/// @param restoreCursor Whether to restore cursor position after click
/// @return 1 on success, 0 on failure
int performMiddleClickAtPosition(CGPoint position, bool restoreCursor) {
	return performClickAtPosition(position, kCGEventOtherMouseDown, kCGEventOtherMouseUp, kCGMouseButtonCenter,
	                              restoreCursor);
}

/// Perform left mouse down at position
/// @param position Target position
/// @return 1 on success, 0 on failure
int performLeftMouseDownAtPosition(CGPoint position) {
	moveMouse(position);
	CFRunLoopRunInMode(kCFRunLoopDefaultMode, 0.05, false);
	CGEventRef down = CGEventCreateMouseEvent(NULL, kCGEventLeftMouseDown, position, kCGMouseButtonLeft);
	if (!down)
		return 0;
	// Clear all modifier flags to ensure clean mouse down
	CGEventSetFlags(down, 0);
	CGEventPost(kCGHIDEventTap, down);
	CFRelease(down);
	CFRunLoopRunInMode(kCFRunLoopDefaultMode, 0.05, false);
	return 1;
}

/// Perform left mouse up at position
/// @param position Target position
/// @return 1 on success, 0 on failure
int performLeftMouseUpAtPosition(CGPoint position) {
	moveMouse(position);
	CFRunLoopRunInMode(kCFRunLoopDefaultMode, 0.05, false);
	CGEventRef up = CGEventCreateMouseEvent(NULL, kCGEventLeftMouseUp, position, kCGMouseButtonLeft);
	if (!up)
		return 0;
	// Clear all modifier flags to ensure clean mouse up
	CGEventSetFlags(up, 0);
	CGEventPost(kCGHIDEventTap, up);
	CFRelease(up);
	CFRunLoopRunInMode(kCFRunLoopDefaultMode, 0.05, false);
	return 1;
}

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

#pragma mark - Attribute Functions

/// Get element attribute value
/// @param element Element reference
/// @param attribute Attribute name
/// @return Attribute value string
char *getElementAttribute(void *element, const char *attribute) {
	if (!element || !attribute)
		return NULL;

	AXUIElementRef axElement = (AXUIElementRef)element;
	CFStringRef attrName = CFStringCreateWithCString(NULL, attribute, kCFStringEncodingUTF8);
	if (!attrName)
		return NULL;

	CFTypeRef value = NULL;
	AXError error = AXUIElementCopyAttributeValue(axElement, attrName, &value);
	CFRelease(attrName);

	if (error != kAXErrorSuccess || !value) {
		return NULL;
	}

	char *result = NULL;

	if (CFGetTypeID(value) == CFStringGetTypeID()) {
		result = cfStringToCString((CFStringRef)value);
	} else if (CFGetTypeID(value) == AXValueGetTypeID()) {
		AXValueType valueType = AXValueGetType((AXValueRef)value);

		if (valueType == kAXValueCGRectType) {
			CGRect rect;
			if (AXValueGetValue((AXValueRef)value, kAXValueCGRectType, &rect)) {
				result = malloc(128);
				if (result) {
					snprintf(result, 128, "{{%.1f, %.1f}, {%.1f, %.1f}}", rect.origin.x, rect.origin.y, rect.size.width,
					         rect.size.height);
				}
			}
		} else if (valueType == kAXValueCGPointType) {
			CGPoint point;
			if (AXValueGetValue((AXValueRef)value, kAXValueCGPointType, &point)) {
				result = malloc(64);
				if (result) {
					snprintf(result, 64, "{%.1f, %.1f}", point.x, point.y);
				}
			}
		} else if (valueType == kAXValueCGSizeType) {
			CGSize size;
			if (AXValueGetValue((AXValueRef)value, kAXValueCGSizeType, &size)) {
				result = malloc(64);
				if (result) {
					snprintf(result, 64, "{%.1f, %.1f}", size.width, size.height);
				}
			}
		}
	}

	CFRelease(value);
	return result;
}

/// Free string allocated by getElementAttribute
/// @param str String to free
void freeString(char *str) {
	if (str)
		free(str);
}

/// Release element reference
/// @param element Element reference
void releaseElement(void *element) {
	if (element) {
		CFRelease((AXUIElementRef)element);
	}
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

#pragma mark - Scroll Functions

/// Get scroll bounds of element
/// @param element Element reference
/// @return Scroll bounds rectangle
CGRect getScrollBounds(void *element) {
	CGRect rect = CGRectZero;
	if (!element)
		return rect;

	AXUIElementRef axElement = (AXUIElementRef)element;

	CFTypeRef positionValue = NULL;
	CFTypeRef sizeValue = NULL;

	if (AXUIElementCopyAttributeValue(axElement, kAXPositionAttribute, &positionValue) == kAXErrorSuccess) {
		CGPoint point;
		if (AXValueGetValue(positionValue, kAXValueCGPointType, &point)) {
			rect.origin = point;
		}
		CFRelease(positionValue);
	}

	if (AXUIElementCopyAttributeValue(axElement, kAXSizeAttribute, &sizeValue) == kAXErrorSuccess) {
		CGSize size;
		if (AXValueGetValue(sizeValue, kAXValueCGSizeType, &size)) {
			rect.size = size;
		}
		CFRelease(sizeValue);
	}

	return rect;
}

/// Scroll at current cursor position only
/// @param deltaX Horizontal scroll amount
/// @param deltaY Vertical scroll amount
/// @return 1 on success, 0 on failure
int scrollAtCursor(int deltaX, int deltaY) {
	CGEventRef event = CGEventCreate(NULL);
	if (!event)
		return 0;

	CGPoint cursorPos = CGEventGetLocation(event);
	CFRelease(event);

	CGEventRef scrollEvent = CGEventCreateScrollWheelEvent(NULL, kCGScrollEventUnitPixel, 2, deltaY, deltaX);

	if (scrollEvent) {
		CGEventSetLocation(scrollEvent, cursorPos);
		CGEventPost(kCGHIDEventTap, scrollEvent);
		CFRelease(scrollEvent);
		return 1;
	}

	return 0;
}

#pragma mark - Screen Functions

/// Try to detect if Mission Control is active
/// Works on Sequoia 15.1 (Tahoe)
/// Maybe works on older versions as per stackoverflow result
/// (https://stackoverflow.com/questions/12683225/osx-how-to-detect-if-mission-control-is-running)
/// @return true if Mission Control is active, false otherwise
bool isMissionControlActive(void) {
	bool result = false;

	@autoreleasepool {
		CFArrayRef windowList = CGWindowListCopyWindowInfo(kCGWindowListOptionAll, kCGNullWindowID);

		if (!windowList) {
			return false;
		}

		// Get all screens and calculate total bounds
		NSArray *screens = [NSScreen screens];
		if (!screens || screens.count == 0) {
			CFRelease(windowList);
			return false;
		}

		// Find the largest screen dimensions to use as reference
		// This prevents false positives on multi-monitor setups
		CGFloat maxWidth = 0;
		CGFloat maxHeight = 0;
		for (NSScreen *screen in screens) {
			CGSize size = screen.frame.size;
			if (size.width > maxWidth)
				maxWidth = size.width;
			if (size.height > maxHeight)
				maxHeight = size.height;
		}

		CGSize screenSize = CGSizeMake(maxWidth, maxHeight);

		CFIndex count = CFArrayGetCount(windowList);
		int fullscreenDockWindows = 0;
		int highLayerDockWindows = 0;

		for (CFIndex i = 0; i < count; i++) {
			CFDictionaryRef windowInfo = (CFDictionaryRef)CFArrayGetValueAtIndex(windowList, i);
			if (!windowInfo)
				continue;

			CFStringRef ownerName = (CFStringRef)CFDictionaryGetValue(windowInfo, kCGWindowOwnerName);

			if (ownerName && CFStringCompare(ownerName, CFSTR("Dock"), 0) == kCFCompareEqualTo) {
				CFStringRef windowName = (CFStringRef)CFDictionaryGetValue(windowInfo, kCGWindowName);

				CFDictionaryRef bounds = (CFDictionaryRef)CFDictionaryGetValue(windowInfo, kCGWindowBounds);

				CFNumberRef windowLayer = (CFNumberRef)CFDictionaryGetValue(windowInfo, kCGWindowLayer);

				if (bounds) {
					double x = 0, y = 0, w = 0, h = 0;

					CFNumberRef xValue = (CFNumberRef)CFDictionaryGetValue(bounds, CFSTR("X"));
					CFNumberRef yValue = (CFNumberRef)CFDictionaryGetValue(bounds, CFSTR("Y"));
					CFNumberRef wValue = (CFNumberRef)CFDictionaryGetValue(bounds, CFSTR("Width"));
					CFNumberRef hValue = (CFNumberRef)CFDictionaryGetValue(bounds, CFSTR("Height"));

					if (xValue)
						CFNumberGetValue(xValue, kCFNumberDoubleType, &x);
					if (yValue)
						CFNumberGetValue(yValue, kCFNumberDoubleType, &y);
					if (wValue)
						CFNumberGetValue(wValue, kCFNumberDoubleType, &w);
					if (hValue)
						CFNumberGetValue(hValue, kCFNumberDoubleType, &h);

					// Count fullscreen Dock windows (works on Sequoia 15.1)
					// Note: y < 0 check removed as it causes false positives on multi-monitor setups
					// where screens can be positioned above the primary screen
					bool isFullscreen = (w >= screenSize.width * 0.95 && h >= screenSize.height * 0.95);
					bool hasNoName = (!windowName || CFStringGetLength(windowName) == 0);

					if (isFullscreen && hasNoName && windowLayer) {
						int layer = 0;
						CFNumberGetValue(windowLayer, kCFNumberIntType, &layer);

						fullscreenDockWindows++;
						if (layer >= 18 && layer <= 20) {
							highLayerDockWindows++;
						}
					}
				}
			}
		}

		CFRelease(windowList);

		// Return results from old or new OS method
		if (!result && fullscreenDockWindows >= 2 && highLayerDockWindows >= 2) {
			result = true;
		}

		return result;
	}
}

/// Get main screen bounds
/// @return Main screen bounds rectangle
CGRect getMainScreenBounds(void) {
	@autoreleasepool {
		NSScreen *mainScreen = [NSScreen mainScreen];
		if (!mainScreen) {
			return CGRectZero;
		}
		NSRect frame = mainScreen.frame;
		return NSRectToCGRect(frame);
	}
}

/// Get active screen bounds (screen containing cursor)
/// @return Active screen bounds rectangle
CGRect getActiveScreenBounds(void) {
	@autoreleasepool {
		// Get current mouse location in screen coordinates
		NSPoint mouseLoc = [NSEvent mouseLocation];

		// Find the screen containing the mouse cursor
		NSScreen *activeScreen = nil;
		for (NSScreen *screen in [NSScreen screens]) {
			if (NSPointInRect(mouseLoc, screen.frame)) {
				activeScreen = screen;
				break;
			}
		}

		// Fall back to main screen if mouse is somehow not on any screen
		if (!activeScreen) {
			activeScreen = [NSScreen mainScreen];
		}

		if (!activeScreen) {
			return CGRectZero;
		}

		// Convert NSScreen frame (bottom-left origin, Y up) to CG coordinates (top-left origin, Y down)
		// This matches the coordinate system used by accessibility APIs
		NSRect nsFrame = activeScreen.frame;

		// Get the primary screen height to flip Y coordinate
		NSScreen *primaryScreen = [[NSScreen screens] firstObject];
		CGFloat primaryScreenHeight = primaryScreen.frame.size.height;

		// Convert to CG coordinates
		CGRect cgFrame;
		cgFrame.origin.x = nsFrame.origin.x;
		cgFrame.origin.y = primaryScreenHeight - (nsFrame.origin.y + nsFrame.size.height);
		cgFrame.size.width = nsFrame.size.width;
		cgFrame.size.height = nsFrame.size.height;

		return cgFrame;
	}
}

/// Get current cursor position
/// @return Current cursor position
CGPoint getCurrentCursorPosition(void) {
	@autoreleasepool {
		CGEventRef event = CGEventCreate(NULL);
		if (!event) {
			return CGPointZero;
		}

		CGPoint position = CGEventGetLocation(event);
		CFRelease(event);

		return position;
	}
}
