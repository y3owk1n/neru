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
	CGPoint position;      ///< Element position
	CGSize size;           ///< Element size
	char *title;           ///< Element title
	char *role;            ///< Element role
	char *roleDescription; ///< Element role description
	bool isEnabled;        ///< Whether element is enabled
	bool isFocused;        ///< Whether element is focused
	int pid;               ///< Process identifier
} ElementInfo;

#pragma mark - Permission Functions

/// Check if accessibility permissions are granted
/// @return 1 if permissions are granted, 0 otherwise
int checkAccessibilityPermissions(void);

#pragma mark - Application Functions

/// Get system-wide accessibility element
/// @return System-wide element reference
void *getSystemWideElement(void);

/// Get currently focused application
/// @return Focused application reference
void *getFocusedApplication(void);

/// Get application by process identifier
/// @param pid Process identifier
/// @return Application reference
void *getApplicationByPID(int pid);

/// Get application by bundle identifier
/// @param bundle_id Bundle identifier
/// @return Application reference
void *getApplicationByBundleId(const char *bundle_id);

/// Get menu bar of an application
/// @param app Application reference
/// @return Menu bar reference
void *getMenuBar(void *app);

#pragma mark - Element Functions

/// Get information about an element
/// @param element Element reference
/// @return Element information structure
ElementInfo *getElementInfo(void *element);

/// Free element information structure
/// @param info Element information structure
void freeElementInfo(ElementInfo *info);

/// Get element at screen position
/// @param position Screen position
/// @return Element reference
void *getElementAtPosition(CGPoint position);

/// Get number of child elements
/// @param element Element reference
/// @return Number of children
int getChildrenCount(void *element);

/// Get child elements
/// @param element Element reference
/// @param count Output parameter for number of children
/// @return Array of child element references
void **getChildren(void *element, int *count);

/// Get visible rows of an element
/// @param element Element reference
/// @param count Output parameter for number of rows
/// @return Array of row element references
void **getVisibleRows(void *element, int *count);

/// Get center point of an element
/// @param element Element reference
/// @param outPoint Output parameter for center point
/// @return 1 on success, 0 on failure
int getElementCenter(void *element, CGPoint *outPoint);

#pragma mark - Mouse Functions

/// Move mouse cursor to position
/// @param position Target position
/// @param eventType CGEvent type (kCGEventMouseMoved or kCGEventLeftMouseDragged)
void moveMouseWithType(CGPoint position, CGEventType eventType);

/// Move mouse cursor smoothly to position
/// @param startPosition Starting position
/// @param endPosition Target position
/// @param steps Number of steps for smooth movement
/// @param delay Delay between steps in milliseconds
void moveMouseSmoothWithType(CGPoint startPosition, CGPoint endPosition, int steps, int delay, CGEventType eventType);

/// Check if element has click action
/// @param element Element reference
/// @return 1 if element is clickable, 0 otherwise
int hasClickAction(void *element);

/// Set focus to element
/// @param element Element reference
/// @return 1 on success, 0 on failure
int setFocus(void *element);

/// Get element attribute value
/// @param element Element reference
/// @param attribute Attribute name
/// @return Attribute value string
char *getElementAttribute(void *element, const char *attribute);

/// Free string allocated by getElementAttribute
/// @param str String to free
void freeString(char *str);

/// Release element reference
/// @param element Element reference
void releaseElement(void *element);

/// Retain element reference
/// @param element Element reference
void retainElement(void *element);

/// Get element hash
/// @param element Element reference
/// @return Element hash value
unsigned long getElementHash(void *element);

/// Check if two elements are equal
/// @param element1 First element reference
/// @param element2 Second element reference
/// @return 1 if equal, 0 otherwise
int areElementsEqual(void *element1, void *element2);

#pragma mark - Window Functions

/// Get all windows of focused application
/// @param count Output parameter for number of windows
/// @return Array of window references
void **getAllWindows(int *count);

/// Get frontmost window
/// @return Frontmost window reference
void *getFrontmostWindow(void);

/// Get application name
/// @param app Application reference
/// @return Application name string
char *getApplicationName(void *app);

/// Get bundle identifier
/// @param app Application reference
/// @return Bundle identifier string
char *getBundleIdentifier(void *app);

/// Set application attribute
/// @param pid Process identifier
/// @param attribute Attribute name
/// @param value Attribute value
/// @return 1 on success, 0 on failure
int setApplicationAttribute(int pid, const char *attribute, int value);

#pragma mark - Scroll Functions

/// Get scroll bounds of element
/// @param element Element reference
/// @return Scroll bounds rectangle
CGRect getScrollBounds(void *element);

/// Scroll at cursor position
/// @param deltaX Horizontal scroll amount
/// @param deltaY Vertical scroll amount
/// @return 1 on success, 0 on failure
int scrollAtCursor(int deltaX, int deltaY);

#pragma mark - Mouse Action Functions

/// Perform left click at position
/// @param position Target position
/// @param restoreCursor Whether to restore cursor position after click
/// @return 1 on success, 0 on failure
int performLeftClickAtPosition(CGPoint position, bool restoreCursor);

/// Perform right click at position
/// @param position Target position
/// @param restoreCursor Whether to restore cursor position after click
/// @return 1 on success, 0 on failure
int performRightClickAtPosition(CGPoint position, bool restoreCursor);

/// Perform middle click at position
/// @param position Target position
/// @param restoreCursor Whether to restore cursor position after click
/// @return 1 on success, 0 on failure
int performMiddleClickAtPosition(CGPoint position, bool restoreCursor);

/// Perform left mouse down at position
/// @param position Target position
/// @return 1 on success, 0 on failure
int performLeftMouseDownAtPosition(CGPoint position);

/// Perform left mouse up at position
/// @param position Target position
/// @return 1 on success, 0 on failure
int performLeftMouseUpAtPosition(CGPoint position);

/// Perform left mouse up at cursor position
/// @return 1 on success, 0 on failure
int performLeftMouseUpAtCursor(void);

#pragma mark - Screen Functions

/// Check if Mission Control is active
/// @return true if Mission Control is active, false otherwise
bool isMissionControlActive(void);

/// Cleanup Mission Control detection resources
/// Should be called when the application shuts down
void cleanupMissionControlDetection(void);

/// Get main screen bounds
/// @return Main screen bounds rectangle
CGRect getMainScreenBounds(void);

/// Get active screen bounds (screen containing cursor)
/// @return Active screen bounds rectangle
CGRect getActiveScreenBounds(void);

/// Get current cursor position
/// @return Current cursor position
CGPoint getCurrentCursorPosition(void);

#endif // ACCESSIBILITY_H
