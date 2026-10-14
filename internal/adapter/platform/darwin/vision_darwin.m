#import "vision.h"

#import <CoreGraphics/CoreGraphics.h>
#import <Foundation/Foundation.h>
#import <ScreenCaptureKit/ScreenCaptureKit.h>
#import <Vision/Vision.h>
#import <dlfcn.h>
#import <os/lock.h>

static VisionResult *emptyVisionResult(void) {
	VisionResult *result = malloc(sizeof(VisionResult));
	result->regions = NULL;
	result->count = 0;
	return result;
}

typedef CGImageRef (*CGDisplayCreateImageFunc)(CGDirectDisplayID display);

typedef struct {
	os_unfair_lock lock;
	CGImageRef image;
	BOOL timedOut;
} NeruScreenshotState;

static CGImageRef createDisplayImage(CGDirectDisplayID display) {
	static CGDisplayCreateImageFunc createImage = NULL;
	static dispatch_once_t onceToken;

	dispatch_once(&onceToken, ^{
		createImage = (CGDisplayCreateImageFunc)dlsym(RTLD_DEFAULT, "CGDisplayCreateImage");
	});

	if (createImage == NULL) {
		return NULL;
	}

	return createImage(display);
}

static SCDisplay *screenCaptureDisplayForID(CGDirectDisplayID displayID, NSArray<SCDisplay *> *displays)
    API_AVAILABLE(macos(12.3)) {
	for (SCDisplay *display in displays) {
		if (display.displayID == displayID) {
			return display;
		}
	}

	return nil;
}

static void releaseScreenshotState(NeruScreenshotState *state) {
	if (state == NULL) {
		return;
	}

	if (state->image != NULL) {
		CGImageRelease(state->image);
	}

	free(state);
}

// storeScreenshot keeps image for the waiting caller, unless the wait already
// gave up, in which case releaseScreenshotState frees it with the state.
static void storeScreenshot(NeruScreenshotState *state, CGImageRef image) {
	if (image == NULL) {
		return;
	}

	os_unfair_lock_lock(&state->lock);
	if (!state->timedOut && state->image == NULL) {
		state->image = CGImageRetain(image);
	}
	os_unfair_lock_unlock(&state->lock);
}

// defaultCaptureTimeoutMS bounds a capture whose caller set no deadline.
static const int64_t defaultCaptureTimeoutMS = 5000;

// awaitScreenshot runs start, which must store any image through
// storeScreenshot and leave group exactly once, and waits up to timeoutMS for
// it. The caller owns the returned image.
static CGImageRef awaitScreenshot(
    int64_t timeoutMS, void (^start)(NeruScreenshotState *state, dispatch_group_t group)) {
	NeruScreenshotState *state = calloc(1, sizeof(NeruScreenshotState));
	if (state == NULL) {
		return NULL;
	}
	state->lock = OS_UNFAIR_LOCK_INIT;

	dispatch_group_t group = dispatch_group_create();
	dispatch_group_enter(group);

	start(state, group);

	dispatch_time_t timeout = dispatch_time(DISPATCH_TIME_NOW, timeoutMS * NSEC_PER_MSEC);
	if (dispatch_group_wait(group, timeout) != 0) {
		os_unfair_lock_lock(&state->lock);
		state->timedOut = YES;
		os_unfair_lock_unlock(&state->lock);

		dispatch_group_notify(group, dispatch_get_global_queue(QOS_CLASS_UTILITY, 0), ^{
			releaseScreenshotState(state);
		});

		return NULL;
	}

	os_unfair_lock_lock(&state->lock);
	CGImageRef image = state->image;
	state->image = NULL;
	os_unfair_lock_unlock(&state->lock);

	releaseScreenshotState(state);

	return image;
}

// The shareable-content list costs ScreenCaptureKit tens of milliseconds to
// assemble, and every capture used to ask for it afresh. Only its displays are
// read, so they alone are kept: the window list and its titles are let go with
// the content they came in. The displays are kept until the display
// configuration changes, or until a capture made from them fails. The
// reconfiguration callback is delivered through the main run loop, which the
// daemon runs; a process without one relies on the failed-capture reset alone.
// Typed id so the variable needs no availability annotation; only macOS 14+
// code reads it.
//
// contentLock is a leaf (internal/app/modes/AGENTS.md): held only across a
// pointer read or swap, never with another lock, and nothing under it reaches
// Go.
static os_unfair_lock contentLock = OS_UNFAIR_LOCK_INIT;
static id cachedDisplays = nil;

// replaceDisplays swaps the cached list under the lock and lets the old one
// go after it, in a pool of its own: the display-change callback and
// ScreenCaptureKit's completion queues both land here, and nothing should
// deallocate while the lock is held.
static void replaceDisplays(id displays) {
	@autoreleasepool {
		os_unfair_lock_lock(&contentLock);
		id previous = cachedDisplays;
		cachedDisplays = displays;
		os_unfair_lock_unlock(&contentLock);

		previous = nil;
	}
}

static void forgetShareableContent(void) { replaceDisplays(nil); }

static void displaysReconfigured(CGDirectDisplayID display, CGDisplayChangeSummaryFlags flags, void *info) {
	(void)display;
	(void)info;
	if ((flags & kCGDisplayBeginConfigurationFlag) == 0) {
		forgetShareableContent();
	}
}

static NSArray<SCDisplay *> *rememberedDisplays(void) API_AVAILABLE(macos(12.3)) {
	static dispatch_once_t onceToken;
	dispatch_once(&onceToken, ^{
		CGDisplayRegisterReconfigurationCallback(displaysReconfigured, NULL);
	});

	os_unfair_lock_lock(&contentLock);
	NSArray<SCDisplay *> *displays = cachedDisplays;
	os_unfair_lock_unlock(&contentLock);

	return displays;
}

static void rememberDisplays(NSArray<SCDisplay *> *displays) API_AVAILABLE(macos(12.3)) {
	replaceDisplays([displays copy]);
}

// captureDisplay screenshots source, a display-local rectangle in points, or
// the whole display when source is CGRectNull, at its size in points: the size
// the rest of the pipeline was written against.
static void captureDisplay(SCDisplay *display, CGRect source, NeruScreenshotState *state, dispatch_group_t group)
    API_AVAILABLE(macos(14.0)) {
	SCContentFilter *filter = [[SCContentFilter alloc] initWithDisplay:display excludingWindows:@[]];
	SCStreamConfiguration *config = [[SCStreamConfiguration alloc] init];

	if (CGRectIsNull(source)) {
		config.width = CGDisplayPixelsWide(display.displayID);
		config.height = CGDisplayPixelsHigh(display.displayID);
	} else {
		config.sourceRect = source;
		config.width = (size_t)source.size.width;
		config.height = (size_t)source.size.height;
	}

	[SCScreenshotManager captureImageWithFilter:filter
	                              configuration:config
	                          completionHandler:^(CGImageRef image, NSError *captureError) {
		                          if (captureError == nil) {
			                          storeScreenshot(state, image);
		                          } else {
			                          forgetShareableContent();
		                          }

		                          dispatch_group_leave(group);
	                          }];
}

// captureWithScreenCaptureKit captures source (see captureDisplay) on
// displayID, from the remembered displays when they still list it and from a
// fresh fetch otherwise. A whole-display capture of a display the fetch does
// not list takes the first one, as it always has; a region capture gives up
// instead, because its source rect is local to displayID and would land on the
// wrong pixels anywhere else. Its caller then falls back to the display path.
static CGImageRef captureWithScreenCaptureKit(CGDirectDisplayID displayID, CGRect source, int64_t timeoutMS) {
	if (@available(macOS 14.0, *)) {
		NSArray<SCDisplay *> *remembered = rememberedDisplays();
		SCDisplay *rememberedDisplay = remembered != nil ? screenCaptureDisplayForID(displayID, remembered) : nil;

		return awaitScreenshot(timeoutMS, ^(NeruScreenshotState *state, dispatch_group_t group) {
			if (rememberedDisplay != nil) {
				captureDisplay(rememberedDisplay, source, state, group);
				return;
			}

			[SCShareableContent
			    getShareableContentWithCompletionHandler:^(SCShareableContent *content, NSError *error) {
				    if (error != nil || content.displays.count == 0) {
					    dispatch_group_leave(group);
					    return;
				    }

				    rememberDisplays(content.displays);

				    SCDisplay *display = screenCaptureDisplayForID(displayID, content.displays);
				    if (display == nil && !CGRectIsNull(source)) {
					    dispatch_group_leave(group);
					    return;
				    }

				    captureDisplay(display != nil ? display : content.displays.firstObject, source, state, group);
			    }];
		});
	}

	return NULL;
}

static CGImageRef captureDisplayImage(CGDirectDisplayID displayID, int64_t timeoutMS) {
	@autoreleasepool {
		if (@available(macOS 14.0, *)) {
			return captureWithScreenCaptureKit(displayID, CGRectNull, timeoutMS);
		}

		return createDisplayImage(displayID);
	}
}

// displayContaining resolves which display holds rect, falling back to the
// main display when none does.
//
// CGGetDisplaysWithRect asserts on a process with no window-server connection
// yet, where CGMainDisplayID opens one. The daemon's NSApplication has always
// opened it by now; a test binary or a CLI process has not, so it is opened
// here once rather than trusted.
static CGDirectDisplayID displayContaining(CGRect rect) {
	static dispatch_once_t connected;
	dispatch_once(&connected, ^{
		(void)CGMainDisplayID();
	});

	CGDirectDisplayID displays[32];
	uint32_t displayCount;
	CGError err = CGGetDisplaysWithRect(rect, 32, displays, &displayCount);
	if (err != kCGErrorSuccess || displayCount == 0) {
		return CGMainDisplayID();
	}
	return displays[0];
}

CGImageRef NeruCaptureDisplayContaining(CGRect rect, CGRect *outBounds, int64_t timeoutMS) {
	CGDirectDisplayID display = displayContaining(rect);
	if (outBounds) {
		*outBounds = CGDisplayBounds(display);
	}
	return captureDisplayImage(display, timeoutMS);
}

// NeruCaptureRect goes through captureImageWithFilter with a source rect.
// +[SCScreenshotManager captureImageInRect:] would skip the content lookup and
// return sooner, but it leaks a CGImage and the IOSurface behind it on every
// call, whatever the caller does with the image, which `leaks` shows from a
// plain Objective-C loop. A daemon that captures on every activation cannot
// carry that.
CGImageRef NeruCaptureRect(CGRect rect, CGRect *outCaptured, int64_t timeoutMS) {
	if (@available(macOS 14.0, *)) {
		@autoreleasepool {
			CGDirectDisplayID display = displayContaining(rect);
			CGRect bounds = CGDisplayBounds(display);
			CGRect captured = CGRectIntegral(CGRectIntersection(rect, bounds));
			if (CGRectIsEmpty(captured)) {
				return NULL;
			}

			if (outCaptured) {
				*outCaptured = captured;
			}

			CGRect source = CGRectOffset(captured, -bounds.origin.x, -bounds.origin.y);

			return captureWithScreenCaptureKit(display, source, timeoutMS);
		}
	}

	return NULL;
}

static NSArray<VNRectangleObservation *> *detectRectangles(CGImageRef image, NeruVisionConfig config) {
	VNDetectRectanglesRequest *request = [[VNDetectRectanglesRequest alloc] init];
	request.maximumObservations = config.rectangleMaxCandidates;
	request.minimumSize = config.rectangleMinSize;
	request.minimumAspectRatio = config.rectangleMinAspect;
	request.maximumAspectRatio = config.rectangleMaxAspect;

	VNImageRequestHandler *handler = [[VNImageRequestHandler alloc] initWithCGImage:image options:@{}];
	NSError *error = nil;
	[handler performRequests:@[ request ] error:&error];
	if (error) {
		return @[];
	}
	return request.results ?: @[];
}

static NSArray<VNRecognizedTextObservation *> *detectText(CGImageRef image) {
	VNRecognizeTextRequest *request = [[VNRecognizeTextRequest alloc] init];
	request.recognitionLevel = VNRequestTextRecognitionLevelFast;
	request.usesLanguageCorrection = NO;

	VNImageRequestHandler *handler = [[VNImageRequestHandler alloc] initWithCGImage:image options:@{}];
	NSError *error = nil;
	[handler performRequests:@[ request ] error:&error];
	if (error) {
		return @[];
	}
	return request.results ?: @[];
}

static CGRect visionRectToCGRect(CGRect imageBounds, CGRect normalizedRect) {
	CGFloat x = normalizedRect.origin.x * imageBounds.size.width;
	CGFloat y = (1.0 - normalizedRect.origin.y - normalizedRect.size.height) * imageBounds.size.height;
	CGFloat w = normalizedRect.size.width * imageBounds.size.width;
	CGFloat h = normalizedRect.size.height * imageBounds.size.height;
	return CGRectMake(x, y, w, h);
}

VisionResult *NeruDetectElements(CGRect screenBounds, NeruVisionConfig config) {
	@autoreleasepool {
		CGRect displayBounds;
		CGImageRef image = NeruCaptureDisplayContaining(screenBounds, &displayBounds, defaultCaptureTimeoutMS);
		if (!image) {
			return emptyVisionResult();
		}

		CGFloat imgW = (CGFloat)CGImageGetWidth(image);
		CGFloat imgH = (CGFloat)CGImageGetHeight(image);

		// Compute scale from the actual image pixels vs display bounds (points).
		// CGDisplayPixelsWide can return point-count on some systems, so we rely
		// on the captured image for the true pixel resolution.
		// Scale handles high DPI / retina screens.
		CGFloat scaleX = imgW / displayBounds.size.width;
		CGFloat scaleY = imgH / displayBounds.size.height;

		CGRect imgRect = CGRectMake(0, 0, imgW, imgH);

		// Vision coordinates are relative to the image (top-left origin, pixels).
		// visionRectToCGRect converts to bottom-left position within the image.
		// We then scale from pixels to points and offset by the display's
		// global origin to get global top-left-origin screen coordinates.
		CGFloat originX = displayBounds.origin.x;
		CGFloat originY = displayBounds.origin.y;

		// Run Vision requests in parallel using dispatch group
		dispatch_group_t group = dispatch_group_create();
		__block NSArray<VNRectangleObservation *> *rects = nil;
		__block NSArray<VNRecognizedTextObservation *> *texts = nil;

		CFRetain(image);
		if (config.detectRectangles) {
			dispatch_group_async(group, dispatch_get_global_queue(QOS_CLASS_USER_INITIATED, 0), ^{
				rects = detectRectangles(image, config);
			});
		}
		if (config.detectText) {
			dispatch_group_async(group, dispatch_get_global_queue(QOS_CLASS_USER_INITIATED, 0), ^{
				texts = detectText(image);
			});
		}
		dispatch_time_t timeout = dispatch_time(DISPATCH_TIME_NOW, (int64_t)config.requestTimeoutMS * 1000000LL);
		if (dispatch_group_wait(group, timeout) != 0) {
			dispatch_group_notify(group, dispatch_get_global_queue(QOS_CLASS_UTILITY, 0), ^{
				CGImageRelease(image);
			});
			CGImageRelease(image);
			return emptyVisionResult();
		}
		CGImageRelease(image);

		// Build region list from all observations
		NSMutableArray *regionList = [NSMutableArray array];

		// Add detected rectangles
		for (VNRectangleObservation *obs in rects) {
			CGRect r = [obs boundingBox];
			CGRect pixelRect = visionRectToCGRect(imgRect, r);
			[regionList addObject:@{
				@"x" : @(originX + pixelRect.origin.x / scaleX),
				@"y" : @(originY + pixelRect.origin.y / scaleY),
				@"w" : @(pixelRect.size.width / scaleX),
				@"h" : @(pixelRect.size.height / scaleY),
				@"score" : @(obs.confidence),
				@"isText" : @(0),
				@"label" : @""
			}];
		}

		// Add detected text regions
		for (VNRecognizedTextObservation *obs in texts) {
			VNRecognizedText *top = [[obs topCandidates:1] firstObject];
			NSString *text = top ? top.string : @"";

			if (config.wordLevel && top && text.length > 0) {
				// Split into word-level regions
				[text enumerateSubstringsInRange:NSMakeRange(0, text.length)
				                         options:NSStringEnumerationByWords
				                      usingBlock:^(
				                          NSString *word, NSRange wordRange, NSRange enclosingRange, BOOL *stop) {
					                      NSError *err = nil;
					                      VNRectangleObservation *wordObs = [top boundingBoxForRange:wordRange
					                                                                           error:&err];
					                      if (err != nil || wordObs == nil) {
						                      return;
					                      }
					                      CGRect wordBox = [wordObs boundingBox];
					                      if (CGRectIsEmpty(wordBox) || CGRectIsNull(wordBox)) {
						                      return;
					                      }
					                      CGRect pixelRect = visionRectToCGRect(imgRect, wordBox);
					                      [regionList addObject:@{
						                      @"x" : @(originX + pixelRect.origin.x / scaleX),
						                      @"y" : @(originY + pixelRect.origin.y / scaleY),
						                      @"w" : @(pixelRect.size.width / scaleX),
						                      @"h" : @(pixelRect.size.height / scaleY),
						                      @"score" : @(obs.confidence),
						                      @"isText" : @(1),
						                      @"label" : word
					                      }];
				                      }];
			} else {
				CGRect r = [obs boundingBox];
				CGRect pixelRect = visionRectToCGRect(imgRect, r);
				[regionList addObject:@{
					@"x" : @(originX + pixelRect.origin.x / scaleX),
					@"y" : @(originY + pixelRect.origin.y / scaleY),
					@"w" : @(pixelRect.size.width / scaleX),
					@"h" : @(pixelRect.size.height / scaleY),
					@"score" : @(obs.confidence),
					@"isText" : @(1),
					@"label" : text ?: @""
				}];
			}
		}

		// Build C result
		VisionResult *result = malloc(sizeof(VisionResult));
		result->count = (int)[regionList count];
		result->regions = malloc(sizeof(VisionRegion) * result->count);

		for (int i = 0; i < result->count; i++) {
			NSDictionary *dict = regionList[i];
			result->regions[i].x = [dict[@"x"] doubleValue];
			result->regions[i].y = [dict[@"y"] doubleValue];
			result->regions[i].width = [dict[@"w"] doubleValue];
			result->regions[i].height = [dict[@"h"] doubleValue];
			result->regions[i].score = [dict[@"score"] doubleValue];
			result->regions[i].isText = [dict[@"isText"] intValue];
			NSString *labelStr = dict[@"label"];
			result->regions[i].label = labelStr ? strdup([labelStr UTF8String]) : NULL;
		}

		CGImageRelease(image);
		return result;
	}
}

CGImageRef NeruCaptureScreen(void) { return captureDisplayImage(CGMainDisplayID(), defaultCaptureTimeoutMS); }

void NeruFreeVisionResult(VisionResult *result) {
	if (result) {
		if (result->regions) {
			for (int i = 0; i < result->count; i++) {
				if (result->regions[i].label) {
					free(result->regions[i].label);
				}
			}
			free(result->regions);
		}
		free(result);
	}
}
