//
//  accessibility_screen.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "accessibility.h"

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
/// @return 1 on success, 0 on failure
int NeruScrollAtPoint(CGPoint pos, int deltaX, int deltaY) {
	@autoreleasepool {
		CGEventRef scrollEvent = CGEventCreateScrollWheelEvent(NULL, kCGScrollEventUnitPixel, 2, deltaY, deltaX);
		if (!scrollEvent)
			return 0;

		CGEventSetLocation(scrollEvent, pos);
		CGEventPost(kCGHIDEventTap, scrollEvent);
		CFRelease(scrollEvent);
		return 1;
	}
}

#pragma mark - Mission Control Detection Functions

/// Internal function to detect Mission Control state using window enumeration
/// Detects MC across multiple macOS versions by checking for:
///   1. "Mission Control" app windows (macOS 13 and earlier)
///   2. Dock overlay windows at elevated layers (macOS 14 Sonoma, layers ~18-20)
///   3. Dock overlay windows at broader ranges (macOS 15 Sequoia/Tahoe)
/// @return true if Mission Control is active, false otherwise
static bool detectMissionControlActive(void) {
	@autoreleasepool {
		CFArrayRef windowList = CGWindowListCopyWindowInfo(kCGWindowListOptionAll, kCGNullWindowID);
		if (!windowList) {
			return false;
		}

		CFIndex count = CFArrayGetCount(windowList);
		int dockHighLayerWindows = 0;
		int dockOverlayWindows = 0;

		for (CFIndex i = 0; i < count; i++) {
			CFDictionaryRef windowInfo = (CFDictionaryRef)CFArrayGetValueAtIndex(windowList, i);
			if (!windowInfo)
				continue;

			CFStringRef ownerName = (CFStringRef)CFDictionaryGetValue(windowInfo, kCGWindowOwnerName);
			if (!ownerName)
				continue;

			// Check if Mission Control app is visible (macOS 13 and earlier)
			if (CFStringCompare(ownerName, CFSTR("Mission Control"), 0) == kCFCompareEqualTo) {
				CFRelease(windowList);
				return YES;
			}

			if (CFStringCompare(ownerName, CFSTR("Dock"), 0) != kCFCompareEqualTo)
				continue;

			CFNumberRef windowLayer = (CFNumberRef)CFDictionaryGetValue(windowInfo, kCGWindowLayer);
			if (!windowLayer)
				continue;

			int layer = 0;
			CFNumberGetValue(windowLayer, kCFNumberIntType, &layer);

			// Layers 18-20: Dock MC overlays on macOS 14 Sonoma
			if (layer >= 18 && layer <= 20) {
				dockHighLayerWindows++;
				if (dockHighLayerWindows >= 2) {
					CFRelease(windowList);
					return YES;
				}
			}

			// Layers 14-25: broader range covering macOS 15 Sequoia/Tahoe
			// where the window manager may use different layers
			if (layer >= 14 && layer <= 25) {
				dockOverlayWindows++;
				if (dockOverlayWindows >= 3) {
					CFRelease(windowList);
					return YES;
				}
			}
		}

		CFRelease(windowList);
		return NO;
	}
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

/// Get all connected screen names as a NUL-separated string
/// @param outLen Output parameter for the total byte length of the returned buffer
/// @return NUL-separated localized display names, or empty string if no screens
/// @note Caller must free the returned string with free()
/// @note NUL is used as the delimiter because display names may theoretically contain commas
char *NeruGetScreenNames(int *outLen) {
	@autoreleasepool {
		*outLen = 0;

		NSArray *screens = [NSScreen screens];
		if (!screens || screens.count == 0) {
			return strdup("");
		}

		// Build a NUL-separated list: "name1\0name2\0"
		NSMutableData *data = [NSMutableData data];
		for (NSScreen *screen in screens) {
			const char *utf8 = [screen.localizedName UTF8String];
			// Append name including its terminating NUL
			[data appendBytes:utf8 length:strlen(utf8) + 1];
		}

		char *result = (char *)malloc(data.length);
		if (result) {
			memcpy(result, data.bytes, data.length);
			*outLen = (int)data.length;
		}

		return result;
	}
}

/// Get screen bounds by localized display name (case-insensitive)
/// @param name Display name to match (e.g. "Built-in Retina Display", "DELL U2720Q")
/// @param found Output parameter set to 1 if screen was found, 0 otherwise
/// @return Screen bounds rectangle in CG coordinates, or CGRectZero if not found
CGRect NeruGetScreenBoundsByName(const char *name, int *found) {
	@autoreleasepool {
		*found = 0;

		if (!name) {
			return CGRectZero;
		}

		NSString *targetName = [NSString stringWithUTF8String:name];
		if (!targetName || targetName.length == 0) {
			return CGRectZero;
		}

		// Find the screen matching the given name
		NSScreen *matchedScreen = nil;
		for (NSScreen *screen in [NSScreen screens]) {
			if ([screen.localizedName caseInsensitiveCompare:targetName] == NSOrderedSame) {
				matchedScreen = screen;
				break;
			}
		}

		if (!matchedScreen) {
			return CGRectZero;
		}

		*found = 1;

		// Convert NSScreen frame (bottom-left origin, Y up) to CG coordinates (top-left origin, Y down)
		NSRect nsFrame = matchedScreen.frame;
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
