#import "vision.h"

#import <CoreGraphics/CoreGraphics.h>
#import <Foundation/Foundation.h>
#import <Vision/Vision.h>

static CGImageRef captureScreen(void) {
	CGDirectDisplayID display = CGMainDisplayID();
	CGImageRef image = CGDisplayCreateImage(display);
	return image;
}

static NSArray<VNRectangleObservation *> *detectRectangles(CGImageRef image) {
	VNDetectRectanglesRequest *request = [[VNDetectRectanglesRequest alloc] init];
	request.maximumObservations = 100;
	request.minimumSize = 0.01;
	request.minimumAspectRatio = 0.3;
	request.maximumAspectRatio = 10.0;

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

VisionResult *NeruDetectElements(CGRect screenBounds) {
	@autoreleasepool {
		CGImageRef image = captureScreen();
		if (!image) {
			VisionResult *result = malloc(sizeof(VisionResult));
			result->regions = NULL;
			result->count = 0;
			return result;
		}

		CGFloat imgW = (CGFloat)CGImageGetWidth(image);
		CGFloat imgH = (CGFloat)CGImageGetHeight(image);
		CGRect imgRect = CGRectMake(0, 0, imgW, imgH);

		// Run Vision requests in parallel using dispatch group
		dispatch_group_t group = dispatch_group_create();
		__block NSArray<VNRectangleObservation *> *rects = nil;
		__block NSArray<VNRecognizedTextObservation *> *texts = nil;

		dispatch_group_async(group, dispatch_get_global_queue(QOS_CLASS_USER_INITIATED, 0), ^{
			rects = detectRectangles(image);
		});
		dispatch_group_async(group, dispatch_get_global_queue(QOS_CLASS_USER_INITIATED, 0), ^{
			texts = detectText(image);
		});
		dispatch_group_wait(group, DISPATCH_TIME_FOREVER);

		// Build region list from all observations
		NSMutableArray *regionList = [NSMutableArray array];

		// Add detected rectangles
		for (VNRectangleObservation *obs in rects) {
			CGRect r = [obs boundingBox];
			CGRect pixelRect = visionRectToCGRect(imgRect, r);
			[regionList addObject:@{
				@"x" : @(pixelRect.origin.x),
				@"y" : @(pixelRect.origin.y),
				@"w" : @(pixelRect.size.width),
				@"h" : @(pixelRect.size.height),
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
				@"x" : @(pixelRect.origin.x),
				@"y" : @(pixelRect.origin.y),
				@"w" : @(pixelRect.size.width),
				@"h" : @(pixelRect.size.height),
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

CGImageRef NeruCaptureScreen(void) { return captureScreen(); }

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
