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

#pragma mark - Mouse Timing Constants

/// Delay between mouse down and mouse up events during a click (seconds)
static const CFTimeInterval kNeruMouseClickDownUpDelay = 0.008;

/// Delay after click processing before restoring cursor (seconds)
static const CFTimeInterval kNeruMouseClickProcessingDelay = 0.05;

/// Delay after mouse move to allow event processing (seconds)
static const CFTimeInterval kNeruMouseMoveDelay = 0.01;

/// Delay between steps during smooth mouse movement (seconds)
static const CFTimeInterval kNeruSmoothMoveStepDelay = 0.001;

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
