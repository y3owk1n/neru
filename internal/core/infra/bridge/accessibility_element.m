//
//  accessibility_element.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "accessibility.h"
#import "accessibility_visibility.h"
#import <Cocoa/Cocoa.h>

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
