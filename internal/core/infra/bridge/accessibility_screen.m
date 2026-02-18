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
static NSDate *g_lastDetectionTime = nil;
static NSTimeInterval g_detectionCacheTimeout = 0.5; // Cache for 500ms
static id g_spaceChangeObserver = nil;
static dispatch_queue_t g_detectionQueue = nil;
static dispatch_once_t g_initOnceToken;

// Lock for thread-safe access to shared state
static os_unfair_lock g_stateLock = OS_UNFAIR_LOCK_INIT;

// Forward declaration
static void updateMissionControlState(void);
static bool detectMissionControlActive(void);

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
	g_lastDetectionTime = [NSDate date];
	os_unfair_lock_unlock(&g_stateLock);
}

/// Thread-safe cache validity check
/// @return true if cache is still valid
static bool isCacheValid(void) {
	os_unfair_lock_lock(&g_stateLock);
	if (!g_lastDetectionTime) {
		os_unfair_lock_unlock(&g_stateLock);
		return false;
	}
	NSTimeInterval timeSinceLastUpdate = -[g_lastDetectionTime timeIntervalSinceNow];
	bool valid = (timeSinceLastUpdate < g_detectionCacheTimeout);
	os_unfair_lock_unlock(&g_stateLock);
	return valid;
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

#pragma mark - Mission Control Detection Functions

/// Internal function to detect Mission Control state using window enumeration
/// This is the actual detection logic that looks for Dock windows
/// @return true if Mission Control is active, false otherwise
static bool detectMissionControlActive(void) {
	@autoreleasepool {
		CFArrayRef windowList = CGWindowListCopyWindowInfo(kCGWindowListOptionAll, kCGNullWindowID);

		if (!windowList) {
			return false;
		}

		// Get all screens for multi-monitor detection
		NSArray *screens = [NSScreen screens];
		if (!screens || screens.count == 0) {
			CFRelease(windowList);
			return false;
		}

		// Store all screen sizes for proper multi-monitor detection
		// This ensures we detect Mission Control windows on screens of any size
		NSMutableArray *screenSizes = [NSMutableArray arrayWithCapacity:screens.count];
		for (NSScreen *screen in screens) {
			NSValue *sizeValue = [NSValue valueWithSize:screen.frame.size];
			[screenSizes addObject:sizeValue];
		}

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

					// Check if window is fullscreen on ANY connected monitor
					// This is crucial for multi-monitor setups with different resolutions
					bool isFullscreen = false;
					for (NSValue *sizeValue in screenSizes) {
						CGSize screenSize = sizeValue.sizeValue;
						// Allow 5% tolerance for window size variations
						if (w >= screenSize.width * 0.95 && h >= screenSize.height * 0.95) {
							isFullscreen = true;
							break;
						}
					}

					// Window must have no name (Mission Control Dock windows are unnamed)
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

		// Return results: Mission Control is active when we see multiple fullscreen Dock windows
		// with high window layers (18-20)
		return (fullscreenDockWindows >= 2 && highLayerDockWindows >= 2);
	}
}

/// Update the cached Mission Control state on the detection queue
static void updateMissionControlState(void) {
	dispatch_async(g_detectionQueue, ^{
		bool newState = detectMissionControlActive();
		setCachedMissionControlState(newState);
	});
}

/// Notification handler for space changes
/// Triggered by NSWorkspaceActiveSpaceDidChangeNotification
static void spaceDidChangeNotification(NSNotification *notification) {
	(void)notification;
	// Immediately update state when space changes
	updateMissionControlState();
}

/// Initialize Mission Control detection system
/// Sets up notification observer and initial state
static void initializeMissionControlDetection(void) {
	dispatch_once(&g_initOnceToken, ^{
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

		// Initial detection
		updateMissionControlState();
	});
}

#pragma mark - Screen Functions

/// Try to detect if Mission Control is currently active
/// Uses a hybrid approach:
/// 1. NSWorkspaceActiveSpaceDidChangeNotification triggers detection when spaces change
/// 2. Cached result is returned to avoid expensive window enumeration on every call
/// 3. Cache expires after 500ms to ensure freshness
///
/// Works on Sequoia 15.1 (Tahoe) and should work on older versions
/// Reference: https://stackoverflow.com/questions/12683225/osx-how-to-detect-if-mission-control-is-running
///
/// @return true if Mission Control is active, false otherwise
bool isMissionControlActive(void) {
	// Initialize on first call - must be on main thread for NSNotificationCenter
	if (!g_detectionQueue) {
		if ([NSThread isMainThread]) {
			initializeMissionControlDetection();
		} else {
			dispatch_sync(dispatch_get_main_queue(), ^{
				initializeMissionControlDetection();
			});
		}
	}

	// Check if cache is still valid using thread-safe accessor
	if (isCacheValid()) {
		return getCachedMissionControlState();
	}

	// Cache expired or not set, do synchronous detection
	bool result = detectMissionControlActive();

	// Update cache synchronously to ensure consistency
	setCachedMissionControlState(result);

	return result;
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

/// Cleanup Mission Control detection resources
/// Should be called when the application shuts down
void cleanupMissionControlDetection(void) {
	// Must be called on main thread due to NSNotificationCenter
	if (![NSThread isMainThread]) {
		dispatch_sync(dispatch_get_main_queue(), ^{
			cleanupMissionControlDetection();
		});
		return;
	}

	// Remove notification observer
	if (g_spaceChangeObserver) {
		NSWorkspace *workspace = [NSWorkspace sharedWorkspace];
		NSNotificationCenter *center = [workspace notificationCenter];
		[center removeObserver:g_spaceChangeObserver];
		g_spaceChangeObserver = nil;
	}

	// Reset the init token so re-initialization is possible (mainly for testing)
	g_initOnceToken = 0;

	// Clear state using lock for thread safety
	os_unfair_lock_lock(&g_stateLock);
	g_missionControlActive = false;
	g_lastDetectionTime = nil;
	os_unfair_lock_unlock(&g_stateLock);

	// Note: We don't nil out g_detectionQueue as there might be pending operations
	// The queue will be cleaned up when the process exits
}
