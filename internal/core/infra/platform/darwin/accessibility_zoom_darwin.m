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

typedef int NeruCGSConnectionID;

typedef NeruCGSConnectionID (*NeruSLSMainConnectionIDFn)(void);

/// Reads the zoom viewport center, magnification and smoothing flag.
typedef CGError (*NeruSLSGetZoomParametersFn)(
    NeruCGSConnectionID cid, CGPoint *outOrigin, double *outFactor, bool *outSmoothing);

/// Moves the zoom viewport. The trailing two arguments are undocumented; zero
/// reproduces the behavior of the system's own panning.
typedef CGError (*NeruSLSSetZoomParametersFn)(
    NeruCGSConnectionID cid, CGPoint *origin, double factor, int smoothing, int reserved, double reserved2);

static NeruCGSConnectionID gNeruZoomConnection = 0;
static NeruSLSGetZoomParametersFn gNeruGetZoomParameters = NULL;
static NeruSLSSetZoomParametersFn gNeruSetZoomParameters = NULL;
static bool gNeruZoomSPIAvailable = false;

/// Resolve the SkyLight zoom SPI once
static void neruLoadZoomSPI(void) {
	static dispatch_once_t onceToken;
	dispatch_once(&onceToken, ^{
		void *skyLight = dlopen("/System/Library/PrivateFrameworks/SkyLight.framework/SkyLight", RTLD_LAZY);
		if (!skyLight)
			return;

		NeruSLSMainConnectionIDFn mainConnectionID = (NeruSLSMainConnectionIDFn)dlsym(skyLight, "SLSMainConnectionID");
		gNeruGetZoomParameters = (NeruSLSGetZoomParametersFn)dlsym(skyLight, "SLSGetZoomParameters");
		gNeruSetZoomParameters = (NeruSLSSetZoomParametersFn)dlsym(skyLight, "SLSSetZoomParameters");

		if (!mainConnectionID || !gNeruGetZoomParameters || !gNeruSetZoomParameters)
			return;

		gNeruZoomConnection = mainConnectionID();
		gNeruZoomSPIAvailable = true;
	});
}

/// Read the current zoom viewport center and magnification
/// @param outOrigin Receives the viewport center in global CG coordinates
/// @param outFactor Receives the magnification factor
/// @param outSmoothing Receives the user's smoothing preference
/// @return true when the screen is zoomed in and the values are usable
static bool neruCurrentZoomParameters(CGPoint *outOrigin, double *outFactor, bool *outSmoothing) {
	if (!UAZoomEnabled())
		return false;

	neruLoadZoomSPI();
	if (!gNeruZoomSPIAvailable)
		return false;

	CGPoint origin = CGPointZero;
	double factor = 0.0;
	bool smoothing = false;

	if (gNeruGetZoomParameters(gNeruZoomConnection, &origin, &factor, &smoothing) != kCGErrorSuccess)
		return false;

	// A factor of 1 means zoomed all the way out — there is no viewport to pan.
	if (factor <= 1.0)
		return false;

	*outOrigin = origin;
	*outFactor = factor;
	*outSmoothing = smoothing;

	return true;
}

/// Bounds of the display the zoom viewport currently sits on
/// @param origin Viewport center in global CG coordinates
/// @return Display bounds, falling back to the main display
static CGRect neruZoomDisplayBounds(CGPoint origin) {
	CGDirectDisplayID display = kCGNullDirectDisplay;
	uint32_t matched = 0;

	if (CGGetDisplaysWithPoint(origin, 1, &display, &matched) == kCGErrorSuccess && matched > 0)
		return CGDisplayBounds(display);

	return CGDisplayBounds(CGMainDisplayID());
}

int NeruGetZoomViewport(CGRect *outViewport) {
	CGPoint origin = CGPointZero;
	double factor = 0.0;
	bool smoothing = false;

	if (!neruCurrentZoomParameters(&origin, &factor, &smoothing))
		return 0;

	CGRect bounds = neruZoomDisplayBounds(origin);
	CGFloat halfWidth = bounds.size.width / (2 * factor);
	CGFloat halfHeight = bounds.size.height / (2 * factor);

	if (outViewport)
		*outViewport = CGRectMake(origin.x - halfWidth, origin.y - halfHeight, halfWidth * 2, halfHeight * 2);

	return 1;
}

void NeruEnsureZoomViewportContainsPoint(CGPoint target) {
	CGPoint origin = CGPointZero;
	double factor = 0.0;
	bool smoothing = false;

	if (!neruCurrentZoomParameters(&origin, &factor, &smoothing))
		return;

	CGRect bounds = neruZoomDisplayBounds(origin);

	// Zoom magnifies one display at a time; the others render normally. A target
	// on one of those is already plainly visible, and panning the zoomed
	// display's viewport toward it only drags that viewport to an edge for no
	// benefit, leaving it somewhere the user did not put it.
	if (!CGRectContainsPoint(bounds, target))
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

	// The viewport cannot leave its display, and the window server clamps to
	// exactly this range. Clamping here too means a target that is unreachable —
	// most often one on a different display than the zoomed one — compares equal
	// to the current origin and costs nothing instead of re-pinning the viewport
	// to the same edge on every move.
	CGFloat minX = CGRectGetMinX(bounds) + halfWidth;
	CGFloat maxX = CGRectGetMaxX(bounds) - halfWidth;
	CGFloat minY = CGRectGetMinY(bounds) + halfHeight;
	CGFloat maxY = CGRectGetMaxY(bounds) - halfHeight;
	wanted.x = fmax(minX, fmin(maxX, wanted.x));
	wanted.y = fmax(minY, fmin(maxY, wanted.y));

	if (CGPointEqualToPoint(wanted, origin))
		return;

	// The window server keeps the cursor at a fixed position within the
	// viewport, so panning drags the cursor with it. Callers must pan first and
	// position the cursor afterwards, never the other way round.
	gNeruSetZoomParameters(gNeruZoomConnection, &wanted, factor, smoothing ? 1 : 0, 0, 0.0);
}
