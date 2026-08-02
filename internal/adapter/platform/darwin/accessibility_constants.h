//
//  accessibility_constants.h
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#import <CoreFoundation/CoreFoundation.h>
#import <CoreGraphics/CoreGraphics.h>

#ifdef __cplusplus
extern "C" {
#endif

#pragma mark - Event Posting Constants

/// Tap location for every synthetic mouse event we post.
///
/// This is deliberately the session tap rather than the HID tap. When the macOS
/// Accessibility Zoom feature is zoomed in, the window server rewrites the
/// location of pointer-motion events entering at the HID tap, treating the
/// posted point as a coordinate in zoomed-viewport space:
///
///     landed = zoomOrigin + (posted - displayCenter) / zoomFactor
///
/// so an absolute move lands hundreds of points away from its target and every
/// cursor-driven feature (hints, grid, move_mouse, drag, scroll) misses. Events
/// posted at the session tap enter above that transform and land exactly on the
/// requested point whether or not zoom is active.
static const CGEventTapLocation kNeruMouseEventTapLocation = kCGSessionEventTap;

#pragma mark - Accessibility Zoom Constants

/// Margin kept between a point and the edge of the Accessibility Zoom viewport
/// when panning to reveal it (points, in unmagnified global coordinates).
/// Without it the point lands exactly on the boundary and the cursor drawn
/// there is half clipped.
static const CGFloat kNeruZoomViewportMarginPoints = 8.0;

#pragma mark - Mouse Timing Constants

/// Delay between mouse down and mouse up events during a click (seconds)
static const CFTimeInterval kNeruMouseClickDownUpDelay = 0.008;

/// Delay after click processing before restoring cursor (seconds)
static const CFTimeInterval kNeruMouseClickProcessingDelay = 0.05;

/// Delay after mouse move to allow event processing (seconds)
static const CFTimeInterval kNeruMouseMoveDelay = 0.01;

#pragma mark - Click Detection Constants

/// Maximum time between clicks to be considered a multi-click sequence (milliseconds)
static const CFTimeInterval kNeruDoubleClickIntervalMs = 500.0;

/// Maximum distance between clicks to be considered at the same position (points)
static const CGFloat kNeruDoubleClickDistancePoints = 5.0;

#pragma mark - Visibility Constants

/// Inset from element edges when sampling visibility points (points)
static const CGFloat kNeruVisibilityInsetPoints = 2.0;

/// Minimum number of visible sample points to consider element visible
static const int kNeruMinVisibleSamplePoints = 2;

#ifdef __cplusplus
}
#endif
