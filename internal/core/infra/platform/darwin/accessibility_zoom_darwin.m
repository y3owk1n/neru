//
//  accessibility_zoom.m
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import "accessibility.h"
#import "accessibility_constants.h"

#import <ApplicationServices/ApplicationServices.h>
#import <Cocoa/Cocoa.h>
#include <dlfcn.h>

#pragma mark - Accessibility Zoom Viewport

// When macOS Accessibility Zoom is zoomed in, only a sub-rectangle of the
// display is on screen. Moving a real mouse to the edge of that rectangle pans
// it; synthetic events never do, at either event tap. So a cursor we position
// outside the viewport is correct but invisible, and the user is left with no
// idea where it went.
//
// UAZoomChangeFocus() is the public API for this and is a no-op when the user's
// zoom is set to follow the pointer (it drives the keyboard-focus and insertion
// -point paths only), so the viewport can only be moved through SkyLight's zoom
// SPI. Those symbols are resolved with dlsym and every entry point degrades to
// "leave the viewport alone" when they are missing, so a future macOS that drops
// them costs the follow behavior and nothing else.
//
// The per-display entry points are the ones to use. Zoom magnifies one display
// at a time, and the display-agnostic variants report whichever display the
// pointer is over while clamping a requested origin against the bounding box of
// all displays. With a second display stacked below the main one that lower
// bound sits well inside the main display, leaving a band at its top edge that
// can never be brought into view. The per-display variants clamp against the
// display itself, which is both correct and what the geometry below assumes.

typedef int NeruCGSConnectionID;

typedef NeruCGSConnectionID (*NeruSLSMainConnectionIDFn)(void);

/// Reads one display's zoom viewport center, magnification and smoothing flag.
typedef CGError (*NeruSLSGetZoomParametersForDisplayFn)(
    NeruCGSConnectionID cid, CGDirectDisplayID display, CGPoint *outOrigin, double *outFactor, bool *outSmoothing);

/// Moves one display's zoom viewport. The two reserved arguments are
/// undocumented; zero reproduces the behavior of the system's own panning.
///
/// @note smoothing is persistent user state rather than a per-call option —
///       passing anything other than the value just read from the getter
///       silently rewrites the user's preference.
typedef CGError (*NeruSLSSetZoomParametersForDisplayFn)(
    NeruCGSConnectionID cid, CGDirectDisplayID display, CGPoint *origin, int smoothing, int reserved, double factor,
    double reserved2);

static NeruCGSConnectionID gNeruZoomConnection = 0;
static NeruSLSGetZoomParametersForDisplayFn gNeruGetZoomParametersForDisplay = NULL;
static NeruSLSSetZoomParametersForDisplayFn gNeruSetZoomParametersForDisplay = NULL;
static bool gNeruZoomSPIAvailable = false;

/// Resolve the SkyLight zoom SPI once
static void neruLoadZoomSPI(void) {
	static dispatch_once_t onceToken;
	dispatch_once(&onceToken, ^{
		void *skyLight = dlopen("/System/Library/PrivateFrameworks/SkyLight.framework/SkyLight", RTLD_LAZY);
		if (!skyLight)
			return;

		NeruSLSMainConnectionIDFn mainConnectionID = (NeruSLSMainConnectionIDFn)dlsym(skyLight, "SLSMainConnectionID");
		gNeruGetZoomParametersForDisplay =
		    (NeruSLSGetZoomParametersForDisplayFn)dlsym(skyLight, "SLSGetZoomParametersForDisplay");
		gNeruSetZoomParametersForDisplay =
		    (NeruSLSSetZoomParametersForDisplayFn)dlsym(skyLight, "SLSSetZoomParametersForDisplay");

		if (!mainConnectionID || !gNeruGetZoomParametersForDisplay || !gNeruSetZoomParametersForDisplay)
			return;

		gNeruZoomConnection = mainConnectionID();
		gNeruZoomSPIAvailable = true;
	});
}

/// Find the display a point falls on
/// @param point Point in global CG coordinates
/// @param outBounds Receives that display's bounds
/// @return The display, or kCGNullDirectDisplay when the point is off-screen
static CGDirectDisplayID neruDisplayForPoint(CGPoint point, CGRect *outBounds) {
	CGDirectDisplayID display = kCGNullDirectDisplay;
	uint32_t matched = 0;

	if (CGGetDisplaysWithPoint(point, 1, &display, &matched) != kCGErrorSuccess || matched == 0)
		return kCGNullDirectDisplay;

	if (outBounds)
		*outBounds = CGDisplayBounds(display);

	return display;
}

/// Read the zoom viewport of the display a point falls on
/// @param point Point in global CG coordinates
/// @param outBounds Receives the display's bounds
/// @param outOrigin Receives the viewport center in global CG coordinates
/// @param outFactor Receives the magnification factor
/// @param outSmoothing Receives the user's smoothing preference, to be passed back unchanged
/// @return The display when it is magnified, kCGNullDirectDisplay otherwise
static CGDirectDisplayID neruZoomedDisplayForPoint(
    CGPoint point, CGRect *outBounds, CGPoint *outOrigin, double *outFactor, bool *outSmoothing) {
	if (!UAZoomEnabled())
		return kCGNullDirectDisplay;

	neruLoadZoomSPI();
	if (!gNeruZoomSPIAvailable)
		return kCGNullDirectDisplay;

	CGRect bounds = CGRectZero;
	CGDirectDisplayID display = neruDisplayForPoint(point, &bounds);
	if (display == kCGNullDirectDisplay)
		return kCGNullDirectDisplay;

	CGPoint origin = CGPointZero;
	double factor = 0.0;
	bool smoothing = false;

	if (gNeruGetZoomParametersForDisplay(gNeruZoomConnection, display, &origin, &factor, &smoothing) != kCGErrorSuccess)
		return kCGNullDirectDisplay;

	// A factor of 1 means this display is not magnified — either zoom is on a
	// different display, or it is zoomed all the way out. Either way there is no
	// viewport to pan, and the point is already displayed normally.
	if (factor <= 1.0)
		return kCGNullDirectDisplay;

	if (outBounds)
		*outBounds = bounds;
	if (outOrigin)
		*outOrigin = origin;
	if (outFactor)
		*outFactor = factor;
	if (outSmoothing)
		*outSmoothing = smoothing;

	return display;
}

int NeruGetZoomViewportForPoint(CGPoint point, CGRect *outViewport) {
	CGRect bounds = CGRectZero;
	CGPoint origin = CGPointZero;
	double factor = 0.0;
	bool smoothing = false;

	if (neruZoomedDisplayForPoint(point, &bounds, &origin, &factor, &smoothing) == kCGNullDirectDisplay)
		return 0;

	CGFloat halfWidth = bounds.size.width / (2 * factor);
	CGFloat halfHeight = bounds.size.height / (2 * factor);

	if (outViewport)
		*outViewport = CGRectMake(origin.x - halfWidth, origin.y - halfHeight, halfWidth * 2, halfHeight * 2);

	return 1;
}

void NeruEnsureZoomViewportContainsPoint(CGPoint target) {
	CGRect bounds = CGRectZero;
	CGPoint origin = CGPointZero;
	double factor = 0.0;
	bool smoothing = false;

	CGDirectDisplayID display = neruZoomedDisplayForPoint(target, &bounds, &origin, &factor, &smoothing);
	if (display == kCGNullDirectDisplay)
		return;

	CGFloat halfWidth = bounds.size.width / (2 * factor);
	CGFloat halfHeight = bounds.size.height / (2 * factor);

	// Nudge by the smallest amount that brings the target inside, so that a
	// cursor already on screen never shifts the viewport. This is what dragging
	// a real mouse into the edge of the viewport does. The margin is clamped so
	// that it can never exceed the viewport itself at extreme zoom factors.
	CGFloat marginX = fmin(kNeruZoomViewportMarginPoints, halfWidth / 2);
	CGFloat marginY = fmin(kNeruZoomViewportMarginPoints, halfHeight / 2);
	CGFloat insetWidth = halfWidth - marginX;
	CGFloat insetHeight = halfHeight - marginY;

	CGPoint wanted = origin;
	if (target.x < origin.x - insetWidth)
		wanted.x = target.x + insetWidth;
	else if (target.x > origin.x + insetWidth)
		wanted.x = target.x - insetWidth;

	if (target.y < origin.y - insetHeight)
		wanted.y = target.y + insetHeight;
	else if (target.y > origin.y + insetHeight)
		wanted.y = target.y - insetHeight;

	// The viewport cannot leave its display, and the window server clamps a
	// requested origin to exactly this range. Clamping here too means an
	// unreachable target compares equal to the current origin and costs nothing
	// instead of re-pinning the viewport to the same edge on every move.
	wanted.x = fmax(CGRectGetMinX(bounds) + halfWidth, fmin(CGRectGetMaxX(bounds) - halfWidth, wanted.x));
	wanted.y = fmax(CGRectGetMinY(bounds) + halfHeight, fmin(CGRectGetMaxY(bounds) - halfHeight, wanted.y));

	if (CGPointEqualToPoint(wanted, origin))
		return;

	// The window server keeps the cursor at a fixed position within the
	// viewport, so panning drags the cursor with it. Callers must pan first and
	// position the cursor afterwards, never the other way round.
	gNeruSetZoomParametersForDisplay(gNeruZoomConnection, display, &wanted, smoothing ? 1 : 0, 0, factor, 0.0);
}
