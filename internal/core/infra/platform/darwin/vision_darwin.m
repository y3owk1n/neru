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

static SCDisplay *screenCaptureDisplayForID(CGDirectDisplayID displayID, SCShareableContent *content)
    API_AVAILABLE(macos(12.3)) {
	for (SCDisplay *display in content.displays) {
		if (display.displayID == displayID) {
			return display;
		}
	}

	return content.displays.firstObject;
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

static CGImageRef captureDisplayImageWithScreenCaptureKit(CGDirectDisplayID displayID) {
	if (@available(macOS 14.0, *)) {
		NeruScreenshotState *state = calloc(1, sizeof(NeruScreenshotState));
		if (state == NULL) {
			return NULL;
		}
		state->lock = OS_UNFAIR_LOCK_INIT;

		dispatch_group_t group = dispatch_group_create();
		dispatch_group_enter(group);

		[SCShareableContent getShareableContentWithCompletionHandler:^(SCShareableContent *content, NSError *error) {
			if (error != nil || content.displays.count == 0) {
				dispatch_group_leave(group);
				return;
			}

			SCDisplay *display = screenCaptureDisplayForID(displayID, content);
			if (display == nil) {
				dispatch_group_leave(group);
				return;
			}

			SCContentFilter *filter = [[SCContentFilter alloc] initWithDisplay:display excludingWindows:@[]];
			SCStreamConfiguration *config = [[SCStreamConfiguration alloc] init];
			config.width = CGDisplayPixelsWide(display.displayID);
			config.height = CGDisplayPixelsHigh(display.displayID);

			[SCScreenshotManager captureImageWithFilter:filter
			                              configuration:config
			                          completionHandler:^(CGImageRef image, NSError *captureError) {
				                          if (captureError == nil && image != NULL) {
					                          os_unfair_lock_lock(&state->lock);
					                          if (!state->timedOut) {
						                          state->image = CGImageRetain(image);
					                          }
					                          os_unfair_lock_unlock(&state->lock);
				                          }

				                          dispatch_group_leave(group);
			                          }];
		}];

		dispatch_time_t timeout = dispatch_time(DISPATCH_TIME_NOW, 5 * NSEC_PER_SEC);
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

	return NULL;
}

static CGImageRef captureDisplayImage(CGDirectDisplayID displayID) {
	if (@available(macOS 14.0, *)) {
		return captureDisplayImageWithScreenCaptureKit(displayID);
	}

	return createDisplayImage(displayID);
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
		// Resolve which display contains the window
		CGDirectDisplayID displays[32];
		uint32_t displayCount;
		CGError err = CGGetDisplaysWithRect(screenBounds, 32, displays, &displayCount);
		CGDirectDisplayID display;
		if (err != kCGErrorSuccess || displayCount == 0) {
			display = CGMainDisplayID();
		} else {
			display = displays[0];
		}

		CGRect displayBounds = CGDisplayBounds(display);

		CGImageRef image = captureDisplayImage(display);
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
			CGRect r = [obs boundingBox];
			CGRect pixelRect = visionRectToCGRect(imgRect, r);
			VNRecognizedText *top = [[obs topCandidates:1] firstObject];
			NSString *text = top ? top.string : @"";
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

CGImageRef NeruCaptureScreen(void) { return captureDisplayImage(CGMainDisplayID()); }

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
