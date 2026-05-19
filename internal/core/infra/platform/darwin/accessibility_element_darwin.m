//
//  accessibility_element.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "accessibility.h"
#import "accessibility_visibility.h"

#import <Cocoa/Cocoa.h>
#include <pthread.h>

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

/// Get element information using batched attribute queries
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

		info->isVisible = true;  // default visible when attribute not available

		static CFStringRef kVisibleAttr = CFSTR("AXVisible");

		CFArrayRef attributes = CFArrayCreate(
		    NULL,
		    (const void **)(CFTypeRef[]){
		        kAXPositionAttribute,
		        kAXSizeAttribute,
		        kAXTitleAttribute,
		        kAXDescriptionAttribute,
		        kAXValueAttribute,
		        kAXIdentifierAttribute,
		        kAXRoleAttribute,
		        kAXSubroleAttribute,
		        kAXRoleDescriptionAttribute,
		        kAXEnabledAttribute,
		        kAXFocusedAttribute,
		        kAXHiddenAttribute,
		        kVisibleAttr,
		    },
		    13, &kCFTypeArrayCallBacks);

		if (!attributes) {
			free(info);
			return NULL;
		}

		CFArrayRef values = NULL;
		AXError error = AXUIElementCopyMultipleAttributeValues(axElement, attributes, 0, &values);
		CFRelease(attributes);

		if (error != kAXErrorSuccess || !values) {
			pid_t pid;
			if (AXUIElementGetPid(axElement, &pid) == kAXErrorSuccess) {
				info->pid = pid;
			}
			return info;
		}

		// With option=0, values always has exactly 13 entries (one per requested attribute).
		// Slots for unsupported/errored attributes hold an AX error placeholder (CFNumber),
		// which the CFGetTypeID checks below will correctly reject.
		CFTypeRef positionValue = (CFTypeRef)CFArrayGetValueAtIndex(values, 0);
		if (positionValue && CFGetTypeID(positionValue) == AXValueGetTypeID()) {
			CGPoint point;
			if (AXValueGetValue((AXValueRef)positionValue, kAXValueCGPointType, &point)) {
				info->position = point;
			}
		}

		CFTypeRef sizeValue = (CFTypeRef)CFArrayGetValueAtIndex(values, 1);
		if (sizeValue && CFGetTypeID(sizeValue) == AXValueGetTypeID()) {
			CGSize size;
			if (AXValueGetValue((AXValueRef)sizeValue, kAXValueCGSizeType, &size)) {
				info->size = size;
			}
		}

		CFTypeRef titleValue = (CFTypeRef)CFArrayGetValueAtIndex(values, 2);
		if (titleValue && CFGetTypeID(titleValue) == CFStringGetTypeID()) {
			info->title = cfStringToCString((CFStringRef)titleValue);
		}

		CFTypeRef descValue = (CFTypeRef)CFArrayGetValueAtIndex(values, 3);
		if (descValue && CFGetTypeID(descValue) == CFStringGetTypeID()) {
			info->description = cfStringToCString((CFStringRef)descValue);
		}

		CFTypeRef valueValue = (CFTypeRef)CFArrayGetValueAtIndex(values, 4);
		if (valueValue && CFGetTypeID(valueValue) == CFStringGetTypeID()) {
			info->value = cfStringToCString((CFStringRef)valueValue);
		}

		CFTypeRef identifierValue = (CFTypeRef)CFArrayGetValueAtIndex(values, 5);
		if (identifierValue && CFGetTypeID(identifierValue) == CFStringGetTypeID()) {
			info->identifier = cfStringToCString((CFStringRef)identifierValue);
		}

		CFTypeRef roleValue = (CFTypeRef)CFArrayGetValueAtIndex(values, 6);
		if (roleValue && CFGetTypeID(roleValue) == CFStringGetTypeID()) {
			info->role = cfStringToCString((CFStringRef)roleValue);
		}

		CFTypeRef subroleValue = (CFTypeRef)CFArrayGetValueAtIndex(values, 7);
		if (subroleValue && CFGetTypeID(subroleValue) == CFStringGetTypeID()) {
			info->subrole = cfStringToCString((CFStringRef)subroleValue);
		}

		CFTypeRef roleDescValue = (CFTypeRef)CFArrayGetValueAtIndex(values, 8);
		if (roleDescValue && CFGetTypeID(roleDescValue) == CFStringGetTypeID()) {
			info->roleDescription = cfStringToCString((CFStringRef)roleDescValue);
		}

		CFTypeRef enabledValue = (CFTypeRef)CFArrayGetValueAtIndex(values, 9);
		if (enabledValue && CFGetTypeID(enabledValue) == CFBooleanGetTypeID()) {
			info->isEnabled = CFBooleanGetValue((CFBooleanRef)enabledValue);
			info->hasEnabledAttribute = true;
		}

		CFTypeRef focusedValue = (CFTypeRef)CFArrayGetValueAtIndex(values, 10);
		if (focusedValue && CFGetTypeID(focusedValue) == CFBooleanGetTypeID()) {
			info->isFocused = CFBooleanGetValue((CFBooleanRef)focusedValue);
		}

		CFTypeRef hiddenValue = (CFTypeRef)CFArrayGetValueAtIndex(values, 11);
		if (hiddenValue && CFGetTypeID(hiddenValue) == CFBooleanGetTypeID()) {
			info->isHidden = CFBooleanGetValue((CFBooleanRef)hiddenValue);
		}

		CFTypeRef visibleValue = (CFTypeRef)CFArrayGetValueAtIndex(values, 12);
		if (visibleValue && CFGetTypeID(visibleValue) == CFBooleanGetTypeID()) {
			info->isVisible = CFBooleanGetValue((CFBooleanRef)visibleValue);
		}

		CFRelease(values);

		// Pre-fetch action names to eliminate a separate AX call during clickability checks.
		// This is done after the batched attribute fetch since action names are only needed
		// for interactive elements, but the cost is negligible for non-interactive ones.
		CFArrayRef actionNames = NULL;
		if (AXUIElementCopyActionNames(axElement, &actionNames) == kAXErrorSuccess && actionNames) {
			info->preActionsFetched = true;
			CFIndex actionCount = CFArrayGetCount(actionNames);
			for (CFIndex i = 0; i < actionCount; i++) {
				CFStringRef action = (CFStringRef)CFArrayGetValueAtIndex(actionNames, i);
				if (!action)
					continue;
				if (CFStringCompare(action, kAXPressAction, 0) == kCFCompareEqualTo ||
				    CFStringCompare(action, CFSTR("AXConfirm"), 0) == kCFCompareEqualTo ||
				    CFStringCompare(action, CFSTR("AXPick"), 0) == kCFCompareEqualTo ||
				    CFStringCompare(action, CFSTR("AXRaise"), 0) == kCFCompareEqualTo) {
					info->hasPressAction = true;
				}
				if (CFStringCompare(action, CFSTR("AXShowMenu"), 0) == kCFCompareEqualTo) {
					info->hasShowMenuAction = true;
				}
			}
			CFRelease(actionNames);
		}

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
	if (info->description)
		free(info->description);
	if (info->value)
		free(info->value);
	if (info->identifier)
		free(info->identifier);
	if (info->role)
		free(info->role);
	if (info->subrole)
		free(info->subrole);
	if (info->roleDescription)
		free(info->roleDescription);

	free(info);
}

#pragma mark - Cached System-Wide Element

static AXUIElementRef cachedSystemWideElement = NULL;
static pthread_mutex_t systemWideMutex = PTHREAD_MUTEX_INITIALIZER;

static AXUIElementRef getCachedSystemWideElement(void) {
	pthread_mutex_lock(&systemWideMutex);
	if (!cachedSystemWideElement) {
		cachedSystemWideElement = AXUIElementCreateSystemWide();
	}
	if (cachedSystemWideElement) {
		CFRetain(cachedSystemWideElement);
	}
	pthread_mutex_unlock(&systemWideMutex);
	return cachedSystemWideElement;
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

	CFArrayRef attributes = CFArrayCreate(
	    NULL,
	    (const void **)(CFTypeRef[]){
	        kAXPositionAttribute,
	        kAXSizeAttribute,
	    },
	    2, &kCFTypeArrayCallBacks);

	if (!attributes)
		return 0;

	CFArrayRef values = NULL;
	AXError error = AXUIElementCopyMultipleAttributeValues(axElement, attributes, 0, &values);
	CFRelease(attributes);

	if (error != kAXErrorSuccess || !values) {
		return 0;
	}

	CFIndex count = CFArrayGetCount(values);

	if (count < 1) {
		CFRelease(values);
		return 0;
	}

	CFTypeRef positionRef = (CFTypeRef)CFArrayGetValueAtIndex(values, 0);
	if (!positionRef || CFGetTypeID(positionRef) != AXValueGetTypeID()) {
		CFRelease(values);
		return 0;
	}

	if (!AXValueGetValue((AXValueRef)positionRef, kAXValueCGPointType, outPoint)) {
		CFRelease(values);
		return 0;
	}

	if (count > 1) {
		CFTypeRef sizeRef = (CFTypeRef)CFArrayGetValueAtIndex(values, 1);
		if (sizeRef && CFGetTypeID(sizeRef) == AXValueGetTypeID()) {
			CGSize size;
			if (AXValueGetValue((AXValueRef)sizeRef, kAXValueCGSizeType, &size)) {
				outPoint->x += size.width / 2.0;
				outPoint->y += size.height / 2.0;
			}
		}
	}

	CFRelease(values);
	return 1;
}

#pragma mark - Child Element Functions

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

#pragma mark - Click Action Functions

static bool elementOrAncestorMatches(AXUIElementRef element, AXUIElementRef target) {
	if (!element || !target) {
		return false;
	}

	AXUIElementRef current = element;
	CFRetain(current);

	// Limit parent chain walk to 16 levels. Most UI hierarchies are shallow
	// enough that the target (or a descendant) is found well before this.
	// 64 was excessive and caused unnecessary AX calls for obscured elements.
	for (int depth = 0; current && depth < 16; depth++) {
		if (CFEqual(current, target)) {
			CFRelease(current);
			return true;
		}

		AXUIElementRef parent = NULL;
		AXError error = AXUIElementCopyAttributeValue(current, kAXParentAttribute, (CFTypeRef *)&parent);
		CFRelease(current);

		if (error != kAXErrorSuccess || !parent) {
			return false;
		}

		current = parent;
	}

	if (current) {
		CFRelease(current);
	}

	return false;
}

/// Fast visibility check using a pre-computed center point
/// Skips hidden/visible attribute checks (caller handles those) and position fetch.
/// Caller should compute center from ElementInfo (already fetched during tree building).
/// @param element Element reference
/// @param center Pre-computed center point
/// @return 1 if the element itself is the hit-test result at the point, or the result is a descendant of the element, 0
/// otherwise
int isElementVisibleAtPoint(void *element, CGPoint center) {
	if (!element)
		return 0;

	AXUIElementRef axElement = (AXUIElementRef)element;

	AXUIElementRef systemWide = getCachedSystemWideElement();
	if (!systemWide)
		return 1;

	AXUIElementRef hitElement = NULL;
	AXError error = AXUIElementCopyElementAtPosition(systemWide, center.x, center.y, &hitElement);
	CFRelease(systemWide);

	if (error != kAXErrorSuccess || !hitElement)
		return 0;

	bool visible = elementOrAncestorMatches(hitElement, axElement);
	CFRelease(hitElement);

	return visible ? 1 : 0;
}

/// Get element frame (AXPosition + AXSize) via batch attribute fetch.
static bool getElementFrame(AXUIElementRef element, CGRect *outFrame) {
	if (!element || !outFrame)
		return false;

	CFTypeRef attrs[] = {kAXPositionAttribute, kAXSizeAttribute};
	CFArrayRef attrArray = CFArrayCreate(NULL, (const void **)attrs, 2, &kCFTypeArrayCallBacks);
	if (!attrArray)
		return false;

	CFArrayRef values = NULL;
	AXError err = AXUIElementCopyMultipleAttributeValues(element, attrArray, 0, &values);
	CFRelease(attrArray);

	if (err != kAXErrorSuccess || !values || CFArrayGetCount(values) < 2) {
		if (values)
			CFRelease(values);
		return false;
	}

	CGPoint position = CGPointZero;
	CGSize size = CGSizeZero;

	CFTypeRef posRef = CFArrayGetValueAtIndex(values, 0);
	if (posRef && CFGetTypeID(posRef) == AXValueGetTypeID())
		AXValueGetValue((AXValueRef)posRef, kAXValueCGPointType, &position);

	CFTypeRef sizeRef = CFArrayGetValueAtIndex(values, 1);
	if (sizeRef && CFGetTypeID(sizeRef) == AXValueGetTypeID())
		AXValueGetValue((AXValueRef)sizeRef, kAXValueCGSizeType, &size);

	CFRelease(values);

	outFrame->origin = position;
	outFrame->size = size;
	return true;
}

/// Check if element has click action
/// @param element Element reference
/// @param skipVisCheck If true, skip the expensive hit-test visibility check
/// @param preHidden Pre-fetched AXHidden value
/// @param preVisible Pre-fetched AXVisible value
/// @param preEnabled Pre-fetched AXEnabled value
/// @param hasEnabledAttr Whether AXEnabled attribute exists on this element
/// @param preRole Pre-fetched AXRole as C string
/// @param centerX Pre-computed center X (from ElementInfo)
/// @param centerY Pre-computed center Y (from ElementInfo)
/// @param preActionsFetched Whether action names were successfully pre-fetched
/// @return 1 if element is clickable, 0 otherwise
int hasClickAction(
    void *element, bool skipVisCheck, bool preHidden, bool preVisible, bool preEnabled, bool hasEnabledAttr,
    const char *preRole, bool preIsWidget, double centerX, double centerY, bool preHasPressAction,
    bool preHasShowMenuAction, bool preActionsFetched) {
	if (!element)
		return 0;

	AXUIElementRef axElement = (AXUIElementRef)element;

	if (preHidden || !preVisible)
		return 0;

	// Use pre-computed center from ElementInfo (no redundant AX call).
	CGPoint visCenter = CGPointMake(centerX, centerY);

#define visHit (!skipVisCheck ? isElementVisibleAtPoint((void *)axElement, visCenter) : 1)

	// Use pre-fetched action flags from ElementInfo (eliminates AXUIElementCopyActionNames call).
	// Explicit actions are the strongest signal, so we check for them first
	if (preHasPressAction || preHasShowMenuAction) {
		if (hasEnabledAttr && !preEnabled)
			return 0;

		if (!visHit)
			return 0;

		return 1;
	}

	if (!preActionsFetched) {
		// Fall back to fetching action names when pre-fetch was unavailable or
		// did not match AXPress/AXShowMenu. Check all known click actions here
		// so that a transient pre-fetch failure does not cause false negatives.
		CFArrayRef actions = NULL;
		if (AXUIElementCopyActionNames(axElement, &actions) == kAXErrorSuccess && actions) {
			CFIndex count = CFArrayGetCount(actions);

			for (CFIndex i = 0; i < count; i++) {
				CFStringRef action = (CFStringRef)CFArrayGetValueAtIndex(actions, i);

				if (!action)
					continue;

				if (CFStringCompare(action, kAXPressAction, 0) == kCFCompareEqualTo ||
				    CFStringCompare(action, CFSTR("AXShowMenu"), 0) == kCFCompareEqualTo ||
				    CFStringCompare(action, CFSTR("AXConfirm"), 0) == kCFCompareEqualTo ||
				    CFStringCompare(action, CFSTR("AXPick"), 0) == kCFCompareEqualTo ||
				    CFStringCompare(action, CFSTR("AXRaise"), 0) == kCFCompareEqualTo) {
					CFRelease(actions);

					if (hasEnabledAttr && !preEnabled)
						return 0;

					if (!visHit)
						return 0;

					return 1;
				}
			}

			CFRelease(actions);
		}
	}

	// Exclude known container/structural roles unless they are widgets
	if (preRole) {
		bool isScrollArea = strcmp(preRole, "AXScrollArea") == 0;
		bool isGroup = strcmp(preRole, "AXGroup") == 0;
		bool isSplitGroup = strcmp(preRole, "AXSplitGroup") == 0;

		if (isScrollArea || isGroup || isSplitGroup) {
			if (preIsWidget) {
				if (hasEnabledAttr && !preEnabled)
					return 0;

				if (!visHit)
					return 0;

				return 1;
			}

			return 0;
		}
	}

	// Some elements support AXPress even if not listed in action names
	CFStringRef pressDesc = NULL;
	if (AXUIElementCopyActionDescription(axElement, kAXPressAction, &pressDesc) == kAXErrorSuccess && pressDesc) {
		CFRelease(pressDesc);

		if (!visHit)
			return 0;

		return 1;
	}

	// Role-specific fallback for links
	if (preRole && strcmp(preRole, "AXLink") == 0) {
		CFTypeRef urlAttr = NULL;
		if (AXUIElementCopyAttributeValue(axElement, kAXURLAttribute, &urlAttr) == kAXErrorSuccess && urlAttr) {
			CFRelease(urlAttr);

			if (!visHit)
				return 0;

			return 1;
		}
	}

	// Visibility check: hit-test at center to filter obscured/scroll-clipped elements.
	if (!skipVisCheck) {
		return isElementVisibleAtPoint((void *)axElement, visCenter);
	}

	return 1;

#undef visHit
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
					snprintf(
					    result, 128, "{{%.1f, %.1f}, {%.1f, %.1f}}", rect.origin.x, rect.origin.y, rect.size.width,
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

/// Retain element reference
/// @param element Element reference
void retainElement(void *element) {
	if (element) {
		CFRetain((AXUIElementRef)element);
	}
}

#pragma mark - Identity Functions

/// Get element hash
/// @param element Element reference
/// @return Element hash value
unsigned long getElementHash(void *element) {
	if (!element)
		return 0;

	return CFHash((AXUIElementRef)element);
}

/// Check if two elements are equal
/// @param element1 First element reference
/// @param element2 Second element reference
/// @return 1 if equal, 0 otherwise
int areElementsEqual(void *element1, void *element2) {
	if (!element1 || !element2)
		return element1 == element2;

	return CFEqual((AXUIElementRef)element1, (AXUIElementRef)element2) ? 1 : 0;
}
