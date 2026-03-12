//
//  accessibility_visibility.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "accessibility_visibility.h"
#import "accessibility_constants.h"
#import <Cocoa/Cocoa.h>

#pragma mark - Visibility Functions

/// Helper function to check if a point is actually visible (not occluded by other windows)
/// @param point Screen position to check
/// @param elementPid Process identifier of the element
/// @return true if point is visible, false otherwise
bool isPointVisible(CGPoint point, pid_t elementPid) {
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
	CGFloat inset = kNeruVisibilityInsetPoints;

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
			if (visiblePoints >= kNeruMinVisibleSamplePoints) {
				return false; // Not occluded
			}
		}
	}

	return true; // Occluded (less than 2 points visible)
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
