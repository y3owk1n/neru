//
//  accessibility_screen.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "accessibility.h"
#import <Cocoa/Cocoa.h>

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
