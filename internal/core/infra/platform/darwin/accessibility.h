//
//  accessibility.h
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#ifndef ACCESSIBILITY_H
#define ACCESSIBILITY_H

#import <ApplicationServices/ApplicationServices.h>
#import <Foundation/Foundation.h>

#pragma mark - Element Information

/// Structure containing information about an accessibility element
typedef struct {
	CGPoint position;          ///< Element position
	CGSize size;               ///< Element size
	char *title;               ///< Element title
	char *description;         ///< Element description
	char *value;               ///< Element value
	char *identifier;          ///< Element identifier (for widget detection)
	char *role;                ///< Element role
	char *subrole;             ///< Element subrole
	char *roleDescription;     ///< Element role description
	bool isEnabled;            ///< Whether element is enabled
	bool hasEnabledAttribute;  ///< Whether element supports AXEnabled attribute
	bool isFocused;            ///< Whether element is focused
	int pid;                   ///< Process identifier
	bool isHidden;             ///< Whether element is AX-hidden (CSS visibility:hidden etc.)
	bool isVisible;            ///< Whether element is AX-visible (default true when unsupported)
	bool hasPressAction;       ///< Whether element has AXPress action (pre-fetched)
	bool hasShowMenuAction;    ///< Whether element has AXShowMenu action (pre-fetched)
	bool preActionsFetched;    ///< Whether actions were successfully pre-fetched
} ElementInfo;

#pragma mark - Permission Functions

/// Check if accessibility permissions are granted
/// @return 1 if permissions are granted, 0 otherwise
int NeruCheckAccessibilityPermissions(void);

/// Request accessibility permissions from macOS
/// @return 1 if permissions are granted after the request, 0 otherwise
int NeruRequestAccessibilityPermissions(void);

#pragma mark - Application Functions

/// Get system-wide accessibility element
/// @return System-wide element reference
void *NeruGetSystemWideElement(void);

/// Get currently focused application
/// @return Focused application reference
void *NeruGetFocusedApplication(void);

/// Get application by process identifier
/// @param pid Process identifier
/// @return Application reference
void *NeruGetApplicationByPID(int pid);

/// Get application by bundle identifier
/// @param bundle_id Bundle identifier
/// @return Application reference
void *NeruGetApplicationByBundleId(const char *bundle_id);

/// Get menu bar of an application
/// @param app Application reference
/// @return Menu bar reference
void *NeruGetMenuBar(void *app);

#pragma mark - Element Functions

/// Get information about an element
/// @param element Element reference
/// @return Element information structure
ElementInfo *NeruGetElementInfo(void *element);

/// Free element information structure
/// @param info Element information structure
void NeruFreeElementInfo(ElementInfo *info);

/// Get element at screen position
/// @param position Screen position
/// @return Element reference
void *NeruGetElementAtPosition(CGPoint position);

/// Get child elements
/// @param element Element reference
/// @param count Output parameter for number of children
/// @return Array of child element references
void **NeruGetChildren(void *element, int *count);

/// Get visible rows of an element
/// @param element Element reference
/// @param count Output parameter for number of rows
/// @return Array of row element references
void **NeruGetVisibleRows(void *element, int *count);

/// Get center point of an element
/// @param element Element reference
/// @param outPoint Output parameter for center point
/// @return 1 on success, 0 on failure
int NeruGetElementCenter(void *element, CGPoint *outPoint);

#pragma mark - Mouse Functions

/// Move mouse cursor to position
/// @param position Target position
/// @param eventType CGEvent type (kCGEventMouseMoved or kCGEventLeftMouseDragged)
void NeruMoveMouseWithType(CGPoint position, CGEventType eventType);

/// Post a single mouse move event (for async animation)
/// @param position Target position
/// @param eventType CGEvent type (kCGEventMouseMoved or kCGEventLeftMouseDragged)
void NeruPostMouseMoveEvent(CGPoint position, CGEventType eventType);

/// Check if element has click action
/// @param element Element reference
/// @param skipVisCheck If true, skip the expensive hit-test visibility check
/// @param preHidden Pre-fetched AXHidden value (from ElementInfo)
/// @param preVisible Pre-fetched AXVisible value (from ElementInfo)
/// @param preEnabled Pre-fetched AXEnabled value (from ElementInfo)
/// @param hasEnabledAttr Whether AXEnabled attribute exists on this element
/// @param preRole Pre-fetched AXRole value (from ElementInfo, caller retains)
/// @param preIsWidget Pre-computed widget flag (from ElementInfo identifier)
/// @param centerX Pre-computed center X (from ElementInfo position + size)
/// @param centerY Pre-computed center Y (from ElementInfo position + size)
/// @param preHasPressAction Pre-fetched AXPress action flag (from ElementInfo)
/// @param preHasShowMenuAction Pre-fetched AXShowMenu action flag (from ElementInfo)
/// @param preActionsFetched Whether action names were successfully pre-fetched
/// @return 1 if element is clickable, 0 otherwise
int NeruHasClickAction(
    void *element, bool skipVisCheck, bool preHidden, bool preVisible, bool preEnabled, bool hasEnabledAttr,
    const char *preRole, bool preIsWidget, double centerX, double centerY, bool preHasPressAction,
    bool preHasShowMenuAction, bool preActionsFetched);

/// Fast visibility check using a pre-computed center point (avoids redundant AX position fetch)
/// @param element Element reference
/// @param center Pre-computed center point (from ElementInfo — already fetched during tree building)
/// @return 1 if element or one of its descendants is hit-test visible at the given point, 0 otherwise
int NeruIsElementVisibleAtPoint(void *element, CGPoint center);

/// Set focus to element
/// @param element Element reference
/// @return 1 on success, 0 on failure
int NeruSetFocus(void *element);

/// Get element attribute value
/// @param element Element reference
/// @param attribute Attribute name
/// @return Attribute value string
char *NeruGetElementAttribute(void *element, const char *attribute);

/// Free string allocated by NeruGetElementAttribute
/// @param str String to free
void NeruFreeString(char *str);

/// Release element reference
/// @param element Element reference
void NeruReleaseElement(void *element);

/// Retain element reference
/// @param element Element reference
void NeruRetainElement(void *element);

/// Get element hash
/// @param element Element reference
/// @return Element hash value
unsigned long NeruGetElementHash(void *element);

/// Check if two elements are equal
/// @param element1 First element reference
/// @param element2 Second element reference
/// @return 1 if equal, 0 otherwise
int NeruAreElementsEqual(void *element1, void *element2);

#pragma mark - Window Functions

/// Get all windows of focused application
/// @param count Output parameter for number of windows
/// @return Array of window references
void **NeruGetAllWindows(int *count);

/// Get the focused window plus AXPopover windows of the focused application
/// @param count Output parameter for number of returned windows
/// @return Array of retained window references. Caller frees the array and releases refs.
void **NeruGetFrontmostAndPopoverWindows(int *count);

/// Get frontmost window
/// @return Frontmost window reference
void *NeruGetFrontmostWindow(void);

/// Get the frame (position + size) of the focused window
/// @return Window frame rectangle, or CGRectZero if no window is found
CGRect NeruGetFocusedWindowFrame(void);

/// Get application name
/// @param app Application reference
/// @return Application name string
char *NeruGetApplicationName(void *app);

/// Get bundle identifier
/// @param app Application reference
/// @return Bundle identifier string
char *NeruGetBundleIdentifier(void *app);

/// Get bundle identifier from PID directly (avoids creating an AX element ref)
/// @param pid Process identifier
/// @return Bundle identifier string, or NULL if not found
char *NeruGetBundleIDForPID(int pid);

/// Set application attribute
/// @param pid Process identifier
/// @param attribute Attribute name
/// @param value Attribute value
/// @return 1 on success, 0 on failure
int NeruSetApplicationAttribute(int pid, const char *attribute, int value);

#pragma mark - Scroll Functions

/// Get scroll bounds of element
/// @param element Element reference
/// @return Scroll bounds rectangle
CGRect NeruGetScrollBounds(void *element);

/// Scroll at a specific point
/// @param pos The point at which to post the scroll event
/// @param deltaX Horizontal scroll amount
/// @param deltaY Vertical scroll amount
/// @return 1 on success, 0 on failure
int NeruScrollAtPoint(CGPoint pos, int deltaX, int deltaY);

#pragma mark - Mouse Action Functions

/// Perform left click at position
/// @param position Target position
/// @param restoreCursor Whether to restore cursor position after click
/// @param flags CGEventFlags for modifier keys (0 for none)
/// @return 1 on success, 0 on failure
int NeruPerformLeftClickAtPosition(CGPoint position, bool restoreCursor, CGEventFlags flags);

/// Perform right click at position
/// @param position Target position
/// @param restoreCursor Whether to restore cursor position after click
/// @param flags CGEventFlags for modifier keys (0 for none)
/// @return 1 on success, 0 on failure
int NeruPerformRightClickAtPosition(CGPoint position, bool restoreCursor, CGEventFlags flags);

/// Perform middle click at position
/// @param position Target position
/// @param restoreCursor Whether to restore cursor position after click
/// @param flags CGEventFlags for modifier keys (0 for none)
/// @return 1 on success, 0 on failure
int NeruPerformMiddleClickAtPosition(CGPoint position, bool restoreCursor, CGEventFlags flags);

/// Perform left mouse down at position
/// @param position Target position
/// @param flags CGEventFlags for modifier keys (0 for none)
/// @return 1 on success, 0 on failure
int NeruPerformLeftMouseDownAtPosition(CGPoint position, CGEventFlags flags);

/// Perform left mouse up at position
/// @param position Target position
/// @param flags CGEventFlags for modifier keys (0 for none)
/// @return 1 on success, 0 on failure
int NeruPerformLeftMouseUpAtPosition(CGPoint position, CGEventFlags flags);

/// Perform left mouse up at cursor position
/// @return 1 on success, 0 on failure
int NeruPerformLeftMouseUpAtCursor(void);

#pragma mark - Screen Functions

/// Check if Mission Control is active
/// @return true if Mission Control is active, false otherwise
bool NeruIsMissionControlActive(void);

/// Update the cached Mission Control state and trigger transition callbacks
void NeruUpdateMissionControlState(void);

/// Enable or disable Mission Control detection.
/// When disabled, no timer, window scans, or callbacks are active.
void NeruSetDetectMissionControlEnabled(bool enabled);

/// Get main screen bounds
/// @return Main screen bounds rectangle
CGRect NeruGetMainScreenBounds(void);

/// Get active screen bounds (screen containing cursor)
/// @return Active screen bounds rectangle
CGRect NeruGetActiveScreenBounds(void);

/// Get all connected screen names as a NUL-separated string
/// @param outLen Output parameter for the total byte length of the returned buffer
/// @return NUL-separated localized display names, or empty string if no screens
/// @note Caller must free the returned string with free()
/// @note NUL is used as the delimiter because display names may theoretically contain commas
char *NeruGetScreenNames(int *outLen);

/// Get screen bounds by localized display name (case-insensitive)
/// @param name Display name to match (e.g. "Built-in Retina Display", "DELL U2720Q")
/// @param found Output parameter set to 1 if screen was found, 0 otherwise
/// @return Screen bounds rectangle in CG coordinates, or CGRectZero if not found
CGRect NeruGetScreenBoundsByName(const char *name, int *found);

/// Get current cursor position
/// @return Current cursor position
CGPoint NeruGetCurrentCursorPosition(void);

#endif  // ACCESSIBILITY_H
