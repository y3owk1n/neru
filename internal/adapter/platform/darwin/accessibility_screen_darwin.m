//
//  accessibility_screen.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "accessibility.h"
#import "accessibility_constants.h"

#import <ApplicationServices/ApplicationServices.h>
#import <Cocoa/Cocoa.h>

#pragma mark - Mission Control Detection State

// State tracking for Mission Control detection
static bool g_missionControlActive = false;
static bool g_mcDetectionEnabled = NO;                // Default to disabled — must be opted in via config
static CFAbsoluteTime g_lastDetectionTime = 0;        // Use CFAbsoluteTime (double) instead of NSDate
static NSTimeInterval g_detectionCacheTimeout = 0.5;  // Cache for 500ms
static id g_spaceChangeObserver = nil;
static dispatch_queue_t g_detectionQueue = nil;
static dispatch_source_t g_detectionTimer = nil;

// Lock for thread-safe access to shared state
static os_unfair_lock g_stateLock = OS_UNFAIR_LOCK_INIT;

// External callback declarations
extern void handleMissionControlActivated(void);
extern void handleMissionControlDeactivated(void);

// Forward declarations
void NeruUpdateMissionControlState(void);
static bool detectMissionControlActive(void);
static void initializeMissionControlDetection(void);

/// Thread-safe getter for cached Mission Control state
/// @return true if Mission Control is active
static bool getCachedMissionControlState(void) {
	os_unfair_lock_lock(&g_stateLock);
	bool result = g_missionControlActive;
	os_unfair_lock_unlock(&g_stateLock);
	return result;
}

/// Thread-safe setter for cached Mission Control state
/// @param state New state value
static void setCachedMissionControlState(bool state) {
	os_unfair_lock_lock(&g_stateLock);
	g_missionControlActive = state;
	// Use CFAbsoluteTimeGetCurrent() - plain C function, no ObjC messaging under lock
	g_lastDetectionTime = CFAbsoluteTimeGetCurrent();
	os_unfair_lock_unlock(&g_stateLock);
}

/// Thread-safe getter for the detection enabled flag
/// @return true if detection is enabled
static bool isDetectionEnabled(void) {
	os_unfair_lock_lock(&g_stateLock);
	bool result = g_mcDetectionEnabled;
	os_unfair_lock_unlock(&g_stateLock);
	return result;
}

/// Thread-safe setter for the detection enabled flag
/// @param enabled New enabled state
static void setDetectionEnabled(bool enabled) {
	os_unfair_lock_lock(&g_stateLock);
	g_mcDetectionEnabled = enabled;
	os_unfair_lock_unlock(&g_stateLock);
}

/// Thread-safe cache validity check
/// @return true if cache is still valid
static bool isCacheValid(void) {
	os_unfair_lock_lock(&g_stateLock);
	if (g_lastDetectionTime == 0) {
		os_unfair_lock_unlock(&g_stateLock);
		return false;
	}

	// Use CFAbsoluteTimeGetCurrent() - plain C function, no ObjC messaging under lock
	CFAbsoluteTime now = CFAbsoluteTimeGetCurrent();
	NSTimeInterval timeSinceLastUpdate = now - g_lastDetectionTime;
	bool valid = (timeSinceLastUpdate < g_detectionCacheTimeout);

	os_unfair_lock_unlock(&g_stateLock);
	return valid;
}

#pragma mark - Scroll Functions

/// Get scroll bounds of element
/// @param element Element reference
/// @return Scroll bounds rectangle
CGRect NeruGetScrollBounds(void *element) {
	CGRect rect = CGRectZero;
	if (!element)
		return rect;

	AXUIElementRef axElement = (AXUIElementRef)element;

	CFArrayRef attributes = CFArrayCreate(
	    NULL,
	    (const void **)(CFTypeRef[]){
	        kAXPositionAttribute,
	        kAXSizeAttribute,
	    },
	    2, &kCFTypeArrayCallBacks);

	if (!attributes)
		return rect;

	CFArrayRef values = NULL;
	AXError error = AXUIElementCopyMultipleAttributeValues(axElement, attributes, 0, &values);
	CFRelease(attributes);

	if (error != kAXErrorSuccess || !values) {
		if (values)
			CFRelease(values);
		return rect;
	}

	CFIndex count = CFArrayGetCount(values);

	if (count > 0) {
		CFTypeRef positionValue = (CFTypeRef)CFArrayGetValueAtIndex(values, 0);
		if (positionValue && CFGetTypeID(positionValue) == AXValueGetTypeID()) {
			CGPoint point;
			if (AXValueGetValue((AXValueRef)positionValue, kAXValueCGPointType, &point)) {
				rect.origin = point;
			}
		}
	}

	if (count > 1) {
		CFTypeRef sizeValue = (CFTypeRef)CFArrayGetValueAtIndex(values, 1);
		if (sizeValue && CFGetTypeID(sizeValue) == AXValueGetTypeID()) {
			CGSize size;
			if (AXValueGetValue((AXValueRef)sizeValue, kAXValueCGSizeType, &size)) {
				rect.size = size;
			}
		}
	}

	CFRelease(values);
	return rect;
}

/// Scroll at a specific point
/// @param pos The point at which to post the scroll event
/// @param deltaX Horizontal scroll amount
/// @param deltaY Vertical scroll amount
/// @param flags CGEventFlags for modifier keys (0 for none)
/// @return 1 on success, 0 on failure
int NeruScrollAtPoint(CGPoint pos, int deltaX, int deltaY, CGEventFlags flags) {
	@autoreleasepool {
		CGEventRef scrollEvent = CGEventCreateScrollWheelEvent(NULL, kCGScrollEventUnitPixel, 2, deltaY, deltaX);
		if (!scrollEvent)
			return 0;

		CGEventSetLocation(scrollEvent, pos);
		// Stamp unconditionally, zero included. A NULL-source event is born
		// carrying the combined session state's modifier flags, so skipping this
		// for an empty set does not produce an unmodified scroll — it produces
		// whatever the system currently believes is held, which is how a plain
		// scroll_down ended up zooming after a Ctrl+J binding. Every other
		// positioned event Neru posts clears its flags the same way.
		CGEventSetFlags(scrollEvent, flags);
		CGEventPost(kNeruMouseEventTapLocation, scrollEvent);
		CFRelease(scrollEvent);
		return 1;
	}
}

#pragma mark - Mission Control Detection Functions

/// How long one Dock query may take before detection gives up on it.
static const float kNeruDockQueryTimeout = 0.25f;

/// Whether Mission Control is up. While it is, the Dock's accessibility tree
/// has a child group whose identifier is "mc", and at no other time: App
/// Expose, Show Desktop and the Apps launcher leave it out.
///
/// The window list cannot answer this. The Dock keeps one full-display window
/// at its own layer, and that window is on screen whenever the Dock is visible,
/// with or without Mission Control.
///
/// A read that does not complete, such as a Dock too busy to answer within the
/// timeout, returns the last known state, so it reports no transition.
/// @return true if Mission Control is active, false otherwise
static bool detectMissionControlActive(void) {
	@autoreleasepool {
		NSRunningApplication *dock =
		    [[NSRunningApplication runningApplicationsWithBundleIdentifier:@"com.apple.dock"] firstObject];
		if (!dock) {
			return getCachedMissionControlState();
		}

		AXUIElementRef dockElement = AXUIElementCreateApplication(dock.processIdentifier);
		if (!dockElement) {
			return getCachedMissionControlState();
		}

		AXUIElementSetMessagingTimeout(dockElement, kNeruDockQueryTimeout);

		CFArrayRef children = NULL;
		AXError childrenErr = AXUIElementCopyAttributeValue(dockElement, kAXChildrenAttribute, (CFTypeRef *)&children);
		CFRelease(dockElement);

		if (childrenErr != kAXErrorSuccess || !children) {
			return getCachedMissionControlState();
		}

		bool active = false;
		CFIndex count = CFArrayGetCount(children);

		for (CFIndex i = 0; i < count && !active; i++) {
			AXUIElementRef child = (AXUIElementRef)CFArrayGetValueAtIndex(children, i);
			AXUIElementSetMessagingTimeout(child, kNeruDockQueryTimeout);

			CFTypeRef identifier = NULL;
			AXError identifierErr = AXUIElementCopyAttributeValue(child, kAXIdentifierAttribute, &identifier);

			// A child without an identifier is not "mc", which still answers the
			// question. A read that timed out answers nothing, so the last known
			// state stands.
			if (identifierErr == kAXErrorCannotComplete) {
				CFRelease(children);
				return getCachedMissionControlState();
			}

			if (identifierErr != kAXErrorSuccess || !identifier) {
				continue;
			}

			active = CFGetTypeID(identifier) == CFStringGetTypeID() &&
			         CFStringCompare((CFStringRef)identifier, CFSTR("mc"), 0) == kCFCompareEqualTo;
			CFRelease(identifier);
		}

		CFRelease(children);
		return active;
	}
}

/// Whether the window a Mission Control thumbnail stands for is on screen.
/// WindowManager names it in an undocumented "wid" attribute. It lists the
/// windows of every desktop, and only the current desktop's are on screen.
bool NeruIsElementWindowOnScreen(void *element, bool *hasWindow) {
	if (hasWindow)
		*hasWindow = false;
	if (!element)
		return false;

	CFTypeRef value = NULL;
	if (AXUIElementCopyAttributeValue((AXUIElementRef)element, CFSTR("wid"), &value) != kAXErrorSuccess || !value) {
		return false;
	}

	CGWindowID windowID = 0;
	bool isNumber = CFGetTypeID(value) == CFNumberGetTypeID() &&
	                CFNumberGetValue((CFNumberRef)value, kCFNumberSInt32Type, &windowID);
	CFRelease(value);

	if (!isNumber || windowID == 0)
		return false;

	if (hasWindow)
		*hasWindow = true;

	CFArrayRef windowIDs = CFArrayCreate(NULL, (const void **)(uintptr_t[]){windowID}, 1, NULL);
	if (!windowIDs)
		return false;

	CFArrayRef descriptions = CGWindowListCreateDescriptionFromArray(windowIDs);
	CFRelease(windowIDs);
	if (!descriptions)
		return false;

	bool onScreen = false;
	if (CFArrayGetCount(descriptions) > 0) {
		CFDictionaryRef description = (CFDictionaryRef)CFArrayGetValueAtIndex(descriptions, 0);
		CFBooleanRef isOnScreen = (CFBooleanRef)CFDictionaryGetValue(description, kCGWindowIsOnscreen);
		onScreen = isOnScreen && CFBooleanGetValue(isOnScreen);
	}

	CFRelease(descriptions);
	return onScreen;
}

/// Enable or disable Mission Control detection.
/// When disabled, the timer and window scans are completely inactive.
/// When enabled, kicks off lazy initialization of the detection system if
/// it hasn't been started yet.
void NeruSetDetectMissionControlEnabled(bool enabled) {
	setDetectionEnabled(enabled);
	if (enabled && g_detectionQueue == NULL) {
		NeruUpdateMissionControlState();
	}
}

/// Update the cached Mission Control state on the detection queue
void NeruUpdateMissionControlState(void) {
	if (!isDetectionEnabled()) {
		setCachedMissionControlState(false);
		return;
	}

	if (g_detectionQueue == NULL) {
		initializeMissionControlDetection();
		if (g_detectionQueue == NULL) {
			return;
		}
	}

	dispatch_async(g_detectionQueue, ^{
		bool oldState = getCachedMissionControlState();
		bool newState = detectMissionControlActive();
		setCachedMissionControlState(newState);

		if (oldState != newState) {
			dispatch_async(dispatch_get_main_queue(), ^{
				if (newState) {
					handleMissionControlActivated();
				} else {
					handleMissionControlDeactivated();
				}
			});
		}
	});
}

/// Notification handler for space changes.
/// Triggered by NSWorkspaceActiveSpaceDidChangeNotification.
static void spaceDidChangeNotification(NSNotification *notification) {
	(void)notification;
	NeruUpdateMissionControlState();
}

/// Initialize Mission Control detection system.
/// Sets up notification observer and initial state.
static void initializeMissionControlDetection(void) {
	static dispatch_once_t onceToken;
	dispatch_once(&onceToken, ^{
		g_detectionQueue = dispatch_queue_create("com.neru.missioncontrol.detection", DISPATCH_QUEUE_SERIAL);

		// Set up space change notification observer
		NSWorkspace *workspace = [NSWorkspace sharedWorkspace];
		NSNotificationCenter *center = [workspace notificationCenter];
		g_spaceChangeObserver = [center addObserverForName:NSWorkspaceActiveSpaceDidChangeNotification
		                                            object:nil
		                                             queue:[NSOperationQueue mainQueue]
		                                        usingBlock:^(NSNotification *note) {
			                                        spaceDidChangeNotification(note);
		                                        }];

		// Set up a periodic detection timer to handle the case where no
		// notification fires when MC opens (macOS 15+ Tahoe). This runs on
		// the background detection queue — fast path when state hasn't changed.
		g_detectionTimer = dispatch_source_create(DISPATCH_SOURCE_TYPE_TIMER, 0, 0, g_detectionQueue);
		if (g_detectionTimer) {
			dispatch_source_set_timer(
			    g_detectionTimer, dispatch_time(DISPATCH_TIME_NOW, 1 * NSEC_PER_SEC), 1 * NSEC_PER_SEC,
			    500 * NSEC_PER_MSEC);
			dispatch_source_set_event_handler(g_detectionTimer, ^{
				if (!isDetectionEnabled()) {
					return;
				}

				bool oldState = getCachedMissionControlState();
				bool newState = detectMissionControlActive();
				setCachedMissionControlState(newState);

				if (oldState != newState) {
					dispatch_async(dispatch_get_main_queue(), ^{
						if (newState) {
							handleMissionControlActivated();
						} else {
							handleMissionControlDeactivated();
						}
					});
				}
			});
			dispatch_resume(g_detectionTimer);
		}

		// Perform initial detection silently
		dispatch_async(g_detectionQueue, ^{
			if (!isDetectionEnabled()) {
				return;
			}

			bool newState = detectMissionControlActive();
			setCachedMissionControlState(newState);
		});
	});
}

#pragma mark - Screen Functions

/// Try to detect if Mission Control is currently active.
/// Uses a hybrid approach:
/// 1. NSWorkspaceActiveSpaceDidChangeNotification triggers detection when spaces change
/// 2. Cached result is returned to avoid expensive window enumeration on every call
/// 3. Cache expires after 500ms to ensure freshness
///
/// Works on Sequoia 15.1 (Tahoe) and should work on older versions.
/// Reference: https://stackoverflow.com/questions/12683225/osx-how-to-detect-if-mission-control-is-running
///
/// @return true if Mission Control is active, false otherwise
bool NeruIsMissionControlActive(void) {
	if (isDetectionEnabled()) {
		initializeMissionControlDetection();
	}

	// Return cached state if still valid
	if (isCacheValid()) {
		return getCachedMissionControlState();
	}

	// Cache expired or not yet set — perform synchronous detection and update cache
	bool result = detectMissionControlActive();
	setCachedMissionControlState(result);

	return result;
}

/// Get main screen bounds
/// @return Main screen bounds rectangle
CGRect NeruGetMainScreenBounds(void) {
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
CGRect NeruGetActiveScreenBounds(void) {
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

		// Convert NSScreen frame (bottom-left origin, Y up) to CG coordinates (top-left origin, Y down).
		// This matches the coordinate system used by accessibility APIs.
		NSRect nsFrame = activeScreen.frame;
		NSScreen *primaryScreen = [[NSScreen screens] firstObject];
		CGFloat primaryScreenHeight = primaryScreen.frame.size.height;

		CGRect cgFrame;
		cgFrame.origin.x = nsFrame.origin.x;
		cgFrame.origin.y = primaryScreenHeight - (nsFrame.origin.y + nsFrame.size.height);
		cgFrame.size.width = nsFrame.size.width;
		cgFrame.size.height = nsFrame.size.height;

		return cgFrame;
	}
}

/// Get every connected screen, name and bounds together, in NSScreen order.
/// @param outCount Output parameter for the number of screens returned
/// @return Array of outCount screens, or NULL when there are none
/// @note Caller must release the array with NeruFreeScreens()
NeruScreenInfo *NeruGetScreens(int *outCount) {
	@autoreleasepool {
		*outCount = 0;

		NSArray *screens = [NSScreen screens];
		if (!screens || screens.count == 0) {
			return NULL;
		}

		NeruScreenInfo *result = calloc(screens.count, sizeof(NeruScreenInfo));
		if (!result) {
			return NULL;
		}

		// Convert NSScreen frames (bottom-left origin, Y up) to the CG
		// coordinates the accessibility APIs use (top-left origin, Y down).
		NSScreen *primaryScreen = [screens firstObject];
		CGFloat primaryScreenHeight = primaryScreen.frame.size.height;

		int count = 0;
		for (NSScreen *screen in screens) {
			const char *utf8 = [screen.localizedName UTF8String];
			if (!utf8 || utf8[0] == '\0') {
				continue;
			}

			NSRect nsFrame = screen.frame;

			CGRect cgFrame;
			cgFrame.origin.x = nsFrame.origin.x;
			cgFrame.origin.y = primaryScreenHeight - (nsFrame.origin.y + nsFrame.size.height);
			cgFrame.size.width = nsFrame.size.width;
			cgFrame.size.height = nsFrame.size.height;

			char *name = strdup(utf8);
			if (!name) {
				continue;
			}

			result[count].name = name;
			result[count].bounds = cgFrame;
			count++;
		}

		if (count == 0) {
			free(result);
			return NULL;
		}

		*outCount = count;
		return result;
	}
}

/// Release an array returned by NeruGetScreens
void NeruFreeScreens(NeruScreenInfo *screens, int count) {
	if (!screens) {
		return;
	}

	for (int i = 0; i < count; i++) {
		free(screens[i].name);
	}

	free(screens);
}

/// Report whether the macOS Accessibility Zoom feature is currently zoomed in
/// @return true when the screen is magnified by Accessibility Zoom
/// @note Used for diagnostics — cursor positioning is zoom-independent because
///       synthetic mouse events are posted at kNeruMouseEventTapLocation.
bool NeruIsScreenZoomed(void) { return UAZoomEnabled(); }

/// Get current cursor position
/// @return Current cursor position
CGPoint NeruGetCurrentCursorPosition(void) {
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
