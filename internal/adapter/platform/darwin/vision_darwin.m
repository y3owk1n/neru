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
// assemble, and every capture used to ask for it afresh. Only its displays and
// Neru's own application are read, so they alone are kept. The window list and
// its titles are let go with the content they came in. The kept entries last
// until the display configuration changes, or until a capture made from them
// fails. The reconfiguration callback is delivered through the main run loop,
// which the daemon runs. A process without one relies on the failed-capture
// reset alone.
// Typed id so the variable needs no availability annotation. Only macOS 14+
// code reads it.
//
// contentLock is a leaf (internal/app/modes/AGENTS.md), held only across a
// pointer read or swap, never with another lock, and nothing under it reaches
// Go.
static os_unfair_lock contentLock = OS_UNFAIR_LOCK_INIT;
static id cachedDisplays = nil;
// cachedOwnApplication is Neru's entry in the same list, or nil when the list
// did not include it.
static id cachedOwnApplication = nil;
// displaysGeneration counts resets, so a lookup that started before one
// cannot put the list it fetched back afterwards.
static uint64_t displaysGeneration = 0;

// forgetShareableContent drops the cached list and starts a new generation.
// The old list is let go after the lock, in a pool of its own, because the
// display-change callback and ScreenCaptureKit's completion queues both land
// here, and nothing should deallocate while the lock is held.
static void forgetShareableContent(void) {
	@autoreleasepool {
		os_unfair_lock_lock(&contentLock);
		id previous = cachedDisplays;
		id previousOwn = cachedOwnApplication;
		cachedDisplays = nil;
		cachedOwnApplication = nil;
		displaysGeneration++;
		os_unfair_lock_unlock(&contentLock);

		previous = nil;
		previousOwn = nil;
	}
}

static void displaysReconfigured(CGDirectDisplayID display, CGDisplayChangeSummaryFlags flags, void *info) {
	(void)display;
	(void)info;
	if ((flags & kCGDisplayBeginConfigurationFlag) == 0) {
		forgetShareableContent();
	}
}

// ownApplication finds Neru's own entry in content's applications, or nil.
static SCRunningApplication *ownApplication(SCShareableContent *content) API_AVAILABLE(macos(12.3)) {
	pid_t pid = getpid();
	for (SCRunningApplication *application in content.applications) {
		if (application.processID == pid) {
			return application;
		}
	}
	return nil;
}

// rememberedDisplays returns the cached list, or nil, with Neru's own
// application from the same lookup and the generation it belongs to, which a
// lookup made now hands back to rememberDisplays.
static NSArray<SCDisplay *> *rememberedDisplays(uint64_t *generation, SCRunningApplication **own)
    API_AVAILABLE(macos(12.3)) {
	static dispatch_once_t onceToken;
	dispatch_once(&onceToken, ^{
		CGDisplayRegisterReconfigurationCallback(displaysReconfigured, NULL);
	});

	os_unfair_lock_lock(&contentLock);
	NSArray<SCDisplay *> *displays = cachedDisplays;
	*own = cachedOwnApplication;
	*generation = displaysGeneration;
	os_unfair_lock_unlock(&contentLock);

	return displays;
}

// rememberDisplays caches a freshly fetched list, unless the display
// configuration changed while it was being fetched.
static void rememberDisplays(NSArray<SCDisplay *> *displays, SCRunningApplication *own, uint64_t generation)
    API_AVAILABLE(macos(12.3)) {
	@autoreleasepool {
		NSArray<SCDisplay *> *copied = [displays copy];

		os_unfair_lock_lock(&contentLock);
		id previous = nil;
		id previousOwn = nil;
		if (generation == displaysGeneration) {
			previous = cachedDisplays;
			previousOwn = cachedOwnApplication;
			cachedDisplays = copied;
			cachedOwnApplication = own;
		}
		os_unfair_lock_unlock(&contentLock);

		previous = nil;
		previousOwn = nil;
	}
}

// captureDisplay screenshots source, a display-local rectangle in points, or
// the whole display when source is CGRectNull. A region comes back at scale
// times its size in points. Detection asks for 1, the size the rest of the
// pipeline was written against, and copy_text's re-read of one hint asks for
// more. When own is known, the filter leaves Neru's own windows out, so a scan
// that runs while labels are on screen does not read them back as targets. The configuration leaves the pointer out
// too, because a pixel detector reads it as a control under the cursor, where a
// snap starts looking.
static void captureDisplay(
    SCDisplay *display, SCRunningApplication *own, CGRect source, double scale, NeruScreenshotState *state,
    dispatch_group_t group) API_AVAILABLE(macos(14.0)) {
	SCContentFilter *filter = own != nil ? [[SCContentFilter alloc] initWithDisplay:display
	                                                          excludingApplications:@[ own ]
	                                                               exceptingWindows:@[]]
	                                     : [[SCContentFilter alloc] initWithDisplay:display excludingWindows:@[]];
	SCStreamConfiguration *config = [[SCStreamConfiguration alloc] init];
	config.showsCursor = NO;

	if (CGRectIsNull(source)) {
		config.width = CGDisplayPixelsWide(display.displayID);
		config.height = CGDisplayPixelsHigh(display.displayID);
	} else {
		config.sourceRect = source;
		config.width = (size_t)(source.size.width * scale);
		config.height = (size_t)(source.size.height * scale);
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
// not list takes the first one, as it always has. A region capture gives up
// instead, because its source rect is local to displayID and would land on the
// wrong pixels anywhere else. Its caller then falls back to the display path.
static CGImageRef captureWithScreenCaptureKit(
    CGDirectDisplayID displayID, CGRect source, double scale, int64_t timeoutMS) {
	if (@available(macOS 14.0, *)) {
		uint64_t generation = 0;
		SCRunningApplication *rememberedOwn = nil;
		NSArray<SCDisplay *> *remembered = rememberedDisplays(&generation, &rememberedOwn);
		SCDisplay *rememberedDisplay = remembered != nil ? screenCaptureDisplayForID(displayID, remembered) : nil;

		return awaitScreenshot(timeoutMS, ^(NeruScreenshotState *state, dispatch_group_t group) {
			if (rememberedDisplay != nil) {
				captureDisplay(rememberedDisplay, rememberedOwn, source, scale, state, group);
				return;
			}

			[SCShareableContent
			    getShareableContentWithCompletionHandler:^(SCShareableContent *content, NSError *error) {
				    if (error != nil || content.displays.count == 0) {
					    dispatch_group_leave(group);
					    return;
				    }

				    SCRunningApplication *own = ownApplication(content);
				    rememberDisplays(content.displays, own, generation);

				    SCDisplay *display = screenCaptureDisplayForID(displayID, content.displays);
				    if (display == nil && !CGRectIsNull(source)) {
					    dispatch_group_leave(group);
					    return;
				    }

				    captureDisplay(
				        display != nil ? display : content.displays.firstObject, own, source, scale, state, group);
			    }];
		});
	}

	return NULL;
}

static CGImageRef captureDisplayImage(CGDirectDisplayID displayID, int64_t timeoutMS) {
	@autoreleasepool {
		if (@available(macOS 14.0, *)) {
			return captureWithScreenCaptureKit(displayID, CGRectNull, 1, timeoutMS);
		}

		return createDisplayImage(displayID);
	}
}

// displayContaining resolves which display holds rect, falling back to the
// main display when none does.
//
// CGGetDisplaysWithRect asserts on a process with no window-server connection
// yet, where CGMainDisplayID opens one. The daemon's NSApplication has always
// opened it by now, but a test binary or a CLI process has not, so it is opened
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
	return NeruCaptureRectAtScale(rect, 1, outCaptured, timeoutMS);
}

CGImageRef NeruCaptureRectAtScale(CGRect rect, double scale, CGRect *outCaptured, int64_t timeoutMS) {
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

			return captureWithScreenCaptureKit(display, source, scale, timeoutMS);
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

// readTextHeights are the heights in pixels a one-line crop is read at. The
// fast recognizer misreads a line of small UI text below about 80 pixels
// tall. Above that, any one height can still swap a letter, such as an e for
// an 8, but a different letter at each height, so the answer most heights
// agree on is usually right.
static const CGFloat readTextHeights[] = {80, 90, 100, 110, 120};
static const size_t readTextHeightCount = sizeof(readTextHeights) / sizeof(readTextHeights[0]);

// readTextMaxLineHeight is the tallest crop read as one line. A taller one,
// such as a contour hint around a whole panel, is read once at its own size,
// since scaling it to one line's height would shrink its text away.
static const size_t readTextMaxLineHeight = 160;

// scaledToHeight draws image at height pixels tall, keeping its aspect ratio.
// The caller releases the result, which is NULL on failure.
static CGImageRef scaledToHeight(CGImageRef image, CGFloat height) {
	CGFloat scale = height / (CGFloat)CGImageGetHeight(image);
	size_t width = (size_t)ceil((CGFloat)CGImageGetWidth(image) * scale);
	CGColorSpaceRef space = CGColorSpaceCreateDeviceRGB();
	CGContextRef context =
	    CGBitmapContextCreate(NULL, width, (size_t)height, 8, 0, space, (CGBitmapInfo)kCGImageAlphaPremultipliedLast);
	CGColorSpaceRelease(space);
	if (!context) {
		return NULL;
	}

	CGContextSetInterpolationQuality(context, kCGInterpolationHigh);
	CGContextDrawImage(context, CGRectMake(0, 0, width, height), image);
	CGImageRef scaled = CGBitmapContextCreateImage(context);
	CGContextRelease(context);

	return scaled;
}

// readTextOnce reads image with the fast recognizer, top to bottom, joining
// lines with a space. It returns an empty string when it finds no text.
static NSString *readTextOnce(CGImageRef image) {
	// The fast recognizer, as hint detection uses, needs no model loaded
	// first. The accurate one's first load can take tens of seconds and holds
	// up every fast request in the process meanwhile, hint detection's
	// included. Language correction stays off, since it rewrites paths, flags
	// and identifiers into dictionary words.
	VNRecognizeTextRequest *request = [[VNRecognizeTextRequest alloc] init];
	request.recognitionLevel = VNRequestTextRecognitionLevelFast;
	request.usesLanguageCorrection = NO;

	VNImageRequestHandler *handler = [[VNImageRequestHandler alloc] initWithCGImage:image options:@{}];
	NSError *error = nil;
	[handler performRequests:@[ request ] error:&error];
	if (error || request.results.count == 0) {
		return @"";
	}

	// Vision's boxes have a bottom-left origin, so the line highest up has
	// the largest y. Reading order is top to bottom, then left to right.
	// Two boxes whose middles sit within half the shorter one's height are
	// on the same line, since words on one line come back with different
	// heights.
	NSArray<VNRecognizedTextObservation *> *lines = [request.results
	    sortedArrayUsingComparator:^NSComparisonResult(VNRecognizedTextObservation *a, VNRecognizedTextObservation *b) {
		    CGFloat ay = CGRectGetMidY(a.boundingBox), by = CGRectGetMidY(b.boundingBox);
		    CGFloat sameLine = MIN(CGRectGetHeight(a.boundingBox), CGRectGetHeight(b.boundingBox)) / 2;
		    if (fabs(ay - by) > sameLine) {
			    return ay > by ? NSOrderedAscending : NSOrderedDescending;
		    }
		    return a.boundingBox.origin.x < b.boundingBox.origin.x ? NSOrderedAscending : NSOrderedDescending;
	    }];

	NSMutableArray<NSString *> *parts = [NSMutableArray arrayWithCapacity:lines.count];
	for (VNRecognizedTextObservation *line in lines) {
		NSString *text = [line topCandidates:1].firstObject.string;
		if (text.length > 0) {
			[parts addObject:text];
		}
	}

	return [parts componentsJoinedByString:@" "];
}

// readTextByVote reads image at each of readTextHeights and returns the read
// most of them give. When no read comes up more often than another, the one
// at the middle height wins.
static NSString *readTextByVote(CGImageRef image) {
	NSMutableArray<NSString *> *reads = [NSMutableArray arrayWithCapacity:readTextHeightCount];
	for (size_t i = 0; i < readTextHeightCount; i++) {
		CGImageRef scaled = scaledToHeight(image, readTextHeights[i]);
		[reads addObject:scaled ? readTextOnce(scaled) : @""];
		CGImageRelease(scaled);
	}

	NSCountedSet<NSString *> *counts = [[NSCountedSet alloc] initWithArray:reads];
	NSString *best = reads[readTextHeightCount / 2];
	for (NSString *read in reads) {
		if ([counts countForObject:read] > [counts countForObject:best]) {
			best = read;
		}
	}

	return best;
}

char *NeruReadText(CGImageRef image) {
	if (!image || CGImageGetHeight(image) == 0) {
		return NULL;
	}

	@autoreleasepool {
		NSString *text = CGImageGetHeight(image) > readTextMaxLineHeight ? readTextOnce(image) : readTextByVote(image);
		if (text.length == 0) {
			return NULL;
		}

		return strdup(text.UTF8String);
	}
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

		// Vision reports normalized coordinates with a bottom-left origin.
		// visionRectToCGRect converts them to top-left-origin pixels within the
		// image. We then scale from pixels to points and offset by the display's
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

CGContextRef NeruCreateBGRABitmapContext(
    void *data, size_t width, size_t height, size_t stride, CGColorSpaceRef space) {
	return CGBitmapContextCreate(
	    data, width, height, 8, stride, space, kCGBitmapByteOrder32Little | kCGImageAlphaPremultipliedFirst);
}

#pragma mark - Read lines

/// The ranges of the runs of non-whitespace in text.
static NSArray<NSValue *> *tokenRanges(NSString *text) {
	NSMutableArray<NSValue *> *ranges = [NSMutableArray array];
	NSCharacterSet *space = [NSCharacterSet whitespaceAndNewlineCharacterSet];
	NSUInteger i = 0, n = text.length;
	while (i < n) {
		while (i < n && [space characterIsMember:[text characterAtIndex:i]])
			i++;
		NSUInteger start = i;
		while (i < n && ![space characterIsMember:[text characterAtIndex:i]])
			i++;
		if (i > start)
			[ranges addObject:[NSValue valueWithRange:NSMakeRange(start, i - start)]];
	}
	return ranges;
}

/// A captured image's pixels as 8-bit RGBA, for measuring glyphs.
typedef struct {
	uint8_t *data;
	size_t width;
	size_t height;
	size_t stride;
} NeruPixels;

/// Draws image into a buffer the caller frees with free(pixels.data).
static NeruPixels pixelsOf(CGImageRef image) {
	NeruPixels px = {0};
	px.width = CGImageGetWidth(image);
	px.height = CGImageGetHeight(image);
	px.stride = px.width * 4;
	px.data = calloc(px.height, px.stride);
	if (!px.data)
		return px;
	CGColorSpaceRef space = CGColorSpaceCreateDeviceRGB();
	CGContextRef context = CGBitmapContextCreate(
	    px.data, px.width, px.height, 8, px.stride, space, (CGBitmapInfo)kCGImageAlphaPremultipliedLast);
	CGColorSpaceRelease(space);
	if (!context) {
		free(px.data);
		px.data = NULL;
		return px;
	}
	CGContextDrawImage(context, CGRectMake(0, 0, px.width, px.height), image);
	CGContextRelease(context);
	return px;
}

/// The luma of the pixel at x, y, top-left origin, from 0 to 255.
static int lumaAt(NeruPixels px, size_t x, size_t y) {
	const uint8_t *p = px.data + y * px.stride + x * 4;
	return (p[0] * 299 + p[1] * 587 + p[2] * 114) / 1000;
}

/// The columns of a word's box, in pixels, that each of its count glyphs
/// covers, read from where ink is: a column holds ink when a pixel in it
/// stands apart from the box's background. Runs of ink columns are glyphs.
/// More runs than glyphs, as a quote mark's two strokes make, are joined
/// across the narrowest gaps. Fewer runs, glyphs that touch, return nil.
/// ink reports the outermost ink columns whatever the count.
/// The color behind a word: the most common color along its box's edges, at
/// four bits a channel, as 0xRRGGBB. It is zero when there are no pixels.
static uint32_t backgroundOf(NeruPixels px, CGRect box) {
	if (!px.data)
		return 0;

	size_t x0 = (size_t)MAX(floor(CGRectGetMinX(box)) - 2, 0);
	size_t x1 = (size_t)MIN(ceil(CGRectGetMaxX(box)) + 2, px.width);
	size_t y0 = (size_t)MAX(floor(CGRectGetMinY(box)) - 2, 0);
	size_t y1 = (size_t)MIN(ceil(CGRectGetMaxY(box)) + 2, px.height);
	if (x1 <= x0 + 1 || y1 <= y0 + 1)
		return 0;

	uint16_t counts[1 << 12] = {0};
	uint16_t best = 0;
	for (size_t i = 0; i < 2 * (x1 - x0) + 2 * (y1 - y0); i++) {
		// Walk the top and bottom rows, then the left and right columns.
		size_t width = x1 - x0, height = y1 - y0, x, y;
		if (i < 2 * width) {
			x = x0 + i % width;
			y = i < width ? y0 : y1 - 1;
		} else {
			size_t j = i - 2 * width;
			x = j < height ? x0 : x1 - 1;
			y = y0 + j % height;
		}
		const uint8_t *p = px.data + y * px.stride + x * 4;
		uint16_t key = (uint16_t)(((p[0] >> 4) << 8) | ((p[1] >> 4) << 4) | (p[2] >> 4));
		if (counts[key] < UINT16_MAX)
			counts[key]++;
		if (counts[key] > counts[best])
			best = key;
	}

	uint32_t red = ((best >> 8) & 15) << 4, green = ((best >> 4) & 15) << 4, blue = (best & 15) << 4;
	return (red << 16) | (green << 8) | blue;
}

/// The cell of each of count glyphs in box, measured from the runs of ink.
/// seeds, when set, are the columns the recognizer placed each character in.
/// Each boundary between two of them then moves to the nearest blank column
/// within window pixels. ink is set to where the word's ink runs.
static NSArray<NSValue *> *glyphColumns(
    NeruPixels px, CGRect box, NSUInteger count, NSArray<NSValue *> *seeds, NSUInteger window, NSRange *ink) {
	*ink = NSMakeRange(NSNotFound, 0);
	if (!px.data || count == 0)
		return nil;

	size_t x0 = (size_t)MAX(floor(CGRectGetMinX(box)) - 2, 0);
	size_t x1 = (size_t)MIN(ceil(CGRectGetMaxX(box)) + 2, px.width);
	size_t y0 = (size_t)MAX(floor(CGRectGetMinY(box)), 0);
	size_t y1 = (size_t)MIN(ceil(CGRectGetMaxY(box)), px.height);
	if (x1 <= x0 + 1 || y1 <= y0 + 1)
		return nil;

	// The background is the most common luma along the box's edges.
	int histogram[256] = {0};
	for (size_t x = x0; x < x1; x++) {
		histogram[lumaAt(px, x, y0)]++;
		histogram[lumaAt(px, x, y1 - 1)]++;
	}
	for (size_t y = y0; y < y1; y++) {
		histogram[lumaAt(px, x0, y)]++;
		histogram[lumaAt(px, x1 - 1, y)]++;
	}
	int background = 0;
	for (int i = 1; i < 256; i++)
		if (histogram[i] > histogram[background])
			background = i;

	NSMutableArray<NSValue *> *runs = [NSMutableArray array];
	NSInteger start = -1;
	for (size_t x = x0; x <= x1; x++) {
		BOOL inked = NO;
		for (size_t y = y0; x < x1 && y < y1 && !inked; y++)
			inked = abs(lumaAt(px, x, y) - background) > 64;
		if (inked && start < 0)
			start = (NSInteger)x;
		if (!inked && start >= 0) {
			[runs addObject:[NSValue valueWithRange:NSMakeRange((NSUInteger)start, x - (NSUInteger)start)]];
			start = -1;
		}
	}
	if (runs.count == 0)
		return nil;

	NSRange first = runs.firstObject.rangeValue, last = runs.lastObject.rangeValue;
	*ink = NSMakeRange(first.location, NSMaxRange(last) - first.location);

	// Many wide characters, such as 川 or 明, have gaps inside them, and
	// punctuation such as 」 is set narrower, so neither the gaps nor an even
	// split find where they meet. The recognizer places each one within a
	// point or two, and moving each boundary to the nearest blank column puts
	// it in the gap between the two characters.
	if (seeds) {
		BOOL (^blank)(NSInteger) = ^BOOL(NSInteger x) {
			for (NSValue *run in runs)
				if (NSLocationInRange((NSUInteger)x, run.rangeValue))
					return NO;
			return YES;
		};
		NSMutableArray<NSNumber *> *edges = [NSMutableArray arrayWithObject:@(ink->location)];
		for (NSUInteger i = 1; i < seeds.count; i++) {
			NSInteger edge = (NSInteger)(NSMaxRange(seeds[i - 1].rangeValue) + seeds[i].rangeValue.location) / 2;
			for (NSInteger d = 0; d <= (NSInteger)window; d++) {
				if (blank(edge - d)) {
					edge -= d;
					break;
				}
				if (blank(edge + d)) {
					edge += d;
					break;
				}
			}
			[edges addObject:@(MAX(edge, edges.lastObject.integerValue + 1))];
		}
		[edges addObject:@(MAX((NSInteger)NSMaxRange(*ink), edges.lastObject.integerValue + 1))];
		NSMutableArray<NSValue *> *cells = [NSMutableArray arrayWithCapacity:seeds.count];
		for (NSUInteger i = 0; i < seeds.count; i++) {
			NSUInteger left = edges[i].unsignedIntegerValue;
			[cells addObject:[NSValue valueWithRange:NSMakeRange(left, edges[i + 1].unsignedIntegerValue - left)]];
		}
		return cells;
	}

	while (runs.count > count) {
		NSUInteger narrowest = 0;
		NSUInteger narrowestGap = NSUIntegerMax;
		for (NSUInteger i = 0; i + 1 < runs.count; i++) {
			NSUInteger gap = runs[i + 1].rangeValue.location - NSMaxRange(runs[i].rangeValue);
			if (gap < narrowestGap) {
				narrowestGap = gap;
				narrowest = i;
			}
		}
		NSRange a = runs[narrowest].rangeValue, b = runs[narrowest + 1].rangeValue;
		runs[narrowest] = [NSValue valueWithRange:NSMakeRange(a.location, NSMaxRange(b) - a.location)];
		[runs removeObjectAtIndex:narrowest + 1];
	}

	if (runs.count != count)
		return nil;

	// Each glyph's cell runs to the middle of the gaps either side of its
	// ink, and the outer ones half a gap past it, so the cells meet where
	// the characters do.
	CGFloat halfGap = 1;
	if (runs.count > 1) {
		CGFloat gaps = 0;
		for (NSUInteger i = 0; i + 1 < runs.count; i++)
			gaps += runs[i + 1].rangeValue.location - NSMaxRange(runs[i].rangeValue);
		halfGap = MAX(gaps / (runs.count - 1) / 2, 1);
	}
	NSMutableArray<NSValue *> *cells = [NSMutableArray arrayWithCapacity:runs.count];
	for (NSUInteger i = 0; i < runs.count; i++) {
		NSRange run = runs[i].rangeValue;
		CGFloat left = i == 0 ? run.location - halfGap : (NSMaxRange(runs[i - 1].rangeValue) + run.location) / 2.0;
		CGFloat right =
		    i + 1 == runs.count ? NSMaxRange(run) + halfGap : (NSMaxRange(run) + runs[i + 1].rangeValue.location) / 2.0;
		[cells
		    addObject:[NSValue valueWithRange:NSMakeRange((NSUInteger)MAX(left, 0), (NSUInteger)MAX(right - left, 1))]];
	}
	return cells;
}

/// Whether the composed character c takes two cells, as Chinese, Japanese and
/// Korean characters and fullwidth forms do.
static BOOL isWide(NSString *c) {
	UTF32Char u = 0;
	if (![c getBytes:&u
	             maxLength:sizeof(u)
	            usedLength:NULL
	              encoding:NSUTF32LittleEndianStringEncoding
	               options:0
	                 range:NSMakeRange(0, c.length)
	        remainingRange:NULL])
		return NO;
	return (u >= 0x1100 && u <= 0x115F) || (u >= 0x2E80 && u <= 0x303E) || (u >= 0x3041 && u <= 0x33FF) ||
	       (u >= 0x3400 && u <= 0x4DBF) || (u >= 0x4E00 && u <= 0x9FFF) || (u >= 0xA000 && u <= 0xA4CF) ||
	       (u >= 0xAC00 && u <= 0xD7A3) || (u >= 0xF900 && u <= 0xFAFF) || (u >= 0xFE30 && u <= 0xFE4F) ||
	       (u >= 0xFF00 && u <= 0xFF60) || (u >= 0xFFE0 && u <= 0xFFE6) || (u >= 0x20000 && u <= 0x3FFFD);
}

/// The columns the recognizer placed each composed character of the word at
/// range in. Returns nil when the word has no wide character, since the gaps
/// between its characters already show where they meet.
static NSArray<NSValue *> *seedsOf(
    VNRecognizedText *text, NSRange range, NSArray<NSValue *> *characters, CGFloat imageW) {
	BOOL wide = NO;
	for (NSValue *character in characters) {
		NSRange at = NSMakeRange(range.location + character.rangeValue.location, character.rangeValue.length);
		wide = wide || isWide([text.string substringWithRange:at]);
	}
	if (!wide)
		return nil;

	NSMutableArray<NSValue *> *seeds = [NSMutableArray arrayWithCapacity:characters.count];
	for (NSValue *character in characters) {
		NSRange at = NSMakeRange(range.location + character.rangeValue.location, character.rangeValue.length);
		NSError *error = nil;
		VNRectangleObservation *box = [text boundingBoxForRange:at error:&error];
		if (error || !box)
			return nil;
		CGRect b = box.boundingBox;
		[seeds addObject:[NSValue valueWithRange:NSMakeRange(
		                                             (NSUInteger)MAX(b.origin.x * imageW, 0),
		                                             (NSUInteger)MAX(b.size.width * imageW, 1))]];
	}
	return seeds;
}

/// Whether text is written right to left, by its first letter.
static BOOL isRightToLeft(NSString *text) {
	NSCharacterSet *letters = [NSCharacterSet letterCharacterSet];
	for (NSUInteger i = 0; i < text.length; i++) {
		unichar c = [text characterAtIndex:i];
		if (![letters characterIsMember:c])
			continue;
		return (c >= 0x0590 && c <= 0x08FF) || (c >= 0xFB1D && c <= 0xFDFF) || (c >= 0xFE70 && c <= 0xFEFF);
	}
	return NO;
}

/// The ranges of the composed characters in text.
static NSArray<NSValue *> *characterRanges(NSString *text) {
	NSMutableArray<NSValue *> *ranges = [NSMutableArray array];
	[text enumerateSubstringsInRange:NSMakeRange(0, text.length)
	                         options:NSStringEnumerationByComposedCharacterSequences
	                      usingBlock:^(NSString *piece, NSRange range, NSRange enclosing, BOOL *stop) {
		                      [ranges addObject:[NSValue valueWithRange:range]];
	                      }];
	return ranges;
}

/// Reads image's lines by deadline as NeruReadLines does, with captured as
/// the global rect the image shows.
static NeruTextResult *readLines(CGImageRef image, CGRect captured, int accurate, dispatch_time_t deadline) {
	@autoreleasepool {
		__block NSArray<VNRecognizedTextObservation *> *lines = nil;
		dispatch_group_t group = dispatch_group_create();
		CFRetain(image);
		dispatch_group_async(group, dispatch_get_global_queue(QOS_CLASS_USER_INITIATED, 0), ^{
			VNRecognizeTextRequest *request = [[VNRecognizeTextRequest alloc] init];
			request.recognitionLevel =
			    accurate ? VNRequestTextRecognitionLevelAccurate : VNRequestTextRecognitionLevelFast;
			// Off, since it rewrites paths, flags and identifiers into words.
			request.usesLanguageCorrection = NO;
			// Only the accurate level reads more than Latin script, and with
			// no language set it reads English alone. Detecting the script
			// reads Chinese, Japanese, Korean, Thai and the rest in any mix.
			if (@available(macOS 13.0, *))
				request.automaticallyDetectsLanguage = accurate != 0;
			request.minimumTextHeight = 0;
			VNImageRequestHandler *handler = [[VNImageRequestHandler alloc] initWithCGImage:image options:@{}];
			NSError *error = nil;
			[handler performRequests:@[ request ] error:&error];
			if (!error)
				lines = request.results;
			CGImageRelease(image);
		});
		BOOL finished = dispatch_group_wait(group, deadline) == 0;
		if (!finished || !lines)
			return NULL;

		NSMutableArray<NSDictionary *> *pieces = [NSMutableArray array];
		NeruPixels px = pixelsOf(image);
		CGFloat imageW = (CGFloat)CGImageGetWidth(image), imageH = (CGFloat)CGImageGetHeight(image);
		CGFloat toPointsX = captured.size.width / imageW, toPointsY = captured.size.height / imageH;
		int lineIndex = 0;
		for (VNRecognizedTextObservation *observation in lines) {
			VNRecognizedText *top = [[observation topCandidates:1] firstObject];
			if (!top || top.string.length == 0)
				continue;

			NSArray<NSValue *> *words = tokenRanges(top.string);
			for (NSUInteger wordIndex = 0; wordIndex < words.count; wordIndex++) {
				NSRange range = words[wordIndex].rangeValue;
				NSError *error = nil;
				VNRectangleObservation *box = [top boundingBoxForRange:range error:&error];
				if (error || !box)
					continue;
				CGRect b = box.boundingBox;  // normalized, bottom-left origin
				CGRect pixel = CGRectMake(
				    b.origin.x * imageW, (1.0 - b.origin.y - b.size.height) * imageH, b.size.width * imageW,
				    b.size.height * imageH);

				// Vision's word boxes run a little past the ink, and it places
				// no character exactly, so each glyph is measured from the
				// pixels, or at least the word is cut to its ink.
				NSString *word = [top.string substringWithRange:range];
				NSArray<NSValue *> *characters = characterRanges(word);
				NSRange ink;
				// A seed can be off by up to three points either way.
				NSUInteger window = (NSUInteger)ceil(3 / toPointsX);
				NSArray<NSValue *> *glyphs =
				    glyphColumns(px, pixel, characters.count, seedsOf(top, range, characters, imageW), window, &ink);
				// The cells run left to right and the characters in reading
				// order, so a word written right to left is left whole.
				if (isRightToLeft(word))
					glyphs = nil;
				uint32_t background = backgroundOf(px, pixel);
				if (ink.location != NSNotFound) {
					pixel.origin.x = ink.location;
					pixel.size.width = ink.length;
				}

				NSUInteger parts = glyphs ? characters.count : 1;
				for (NSUInteger i = 0; i < parts; i++) {
					NSString *text = glyphs ? [word substringWithRange:characters[i].rangeValue] : word;
					CGFloat x = glyphs ? glyphs[i].rangeValue.location : pixel.origin.x;
					CGFloat w = glyphs ? glyphs[i].rangeValue.length : pixel.size.width;
					[pieces addObject:@{
						@"text" : text,
						@"x" : @(captured.origin.x + x * toPointsX),
						@"y" : @(captured.origin.y + pixel.origin.y * toPointsY),
						@"w" : @(w * toPointsX),
						@"h" : @(pixel.size.height * toPointsY),
						@"line" : @(lineIndex),
						@"word" : @(wordIndex),
						@"background" : @(background),
					}];
				}
			}
			lineIndex++;
		}
		free(px.data);

		NeruTextResult *result = calloc(1, sizeof(NeruTextResult));
		if (!result)
			return NULL;
		if (pieces.count == 0)
			return result;

		result->pieces = calloc(pieces.count, sizeof(NeruTextPiece));
		if (!result->pieces) {
			free(result);
			return NULL;
		}
		result->count = (int)pieces.count;
		for (NSUInteger i = 0; i < pieces.count; i++) {
			NSDictionary *piece = pieces[i];
			result->pieces[i] = (NeruTextPiece){
			    .text = strdup([piece[@"text"] UTF8String]),
			    .x = [piece[@"x"] doubleValue],
			    .y = [piece[@"y"] doubleValue],
			    .width = [piece[@"w"] doubleValue],
			    .height = [piece[@"h"] doubleValue],
			    .line = [piece[@"line"] intValue],
			    .word = [piece[@"word"] intValue],
			    .background = [piece[@"background"] unsignedIntValue],
			};
		}
		return result;
	}
}

NeruTextResult *NeruReadLines(CGRect rect, double scale, int accurate, int64_t timeoutMS) {
	@autoreleasepool {
		// One deadline covers the capture and the read together.
		dispatch_time_t deadline = dispatch_time(DISPATCH_TIME_NOW, timeoutMS * NSEC_PER_MSEC);
		CGRect captured = CGRectZero;
		CGImageRef image = NeruCaptureRectAtScale(rect, scale, &captured, timeoutMS);
		if (!image)
			return NULL;

		NeruTextResult *result = readLines(image, captured, accurate, deadline);
		CGImageRelease(image);
		return result;
	}
}

NeruTextResult *NeruReadImageLines(CGImageRef image, CGRect bounds, int accurate, int64_t timeoutMS) {
	return readLines(image, bounds, accurate, dispatch_time(DISPATCH_TIME_NOW, timeoutMS * NSEC_PER_MSEC));
}

void NeruFreeTextResult(NeruTextResult *result) {
	if (!result)
		return;
	for (int i = 0; i < result->count; i++)
		free(result->pieces[i].text);
	free(result->pieces);
	free(result);
}
