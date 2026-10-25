#import <CoreGraphics/CoreGraphics.h>

typedef struct {
	double x;
	double y;
	double width;
	double height;
	double score;
	char *label;  // text label if recognized, empty otherwise
	int isText;   // 1 if this region was detected as text
} VisionRegion;

typedef struct {
	VisionRegion *regions;
	int count;
} VisionResult;

typedef struct {
	int detectText;
	int detectRectangles;
	int requestTimeoutMS;
	int rectangleMaxCandidates;
	double rectangleMinSize;
	double rectangleMinAspect;
	double rectangleMaxAspect;
	int wordLevel;
} NeruVisionConfig;

// Captures the display containing the given rect and runs Vision Framework
// detection. On multi-monitor setups this ensures the display with the
// focused window is captured, not just the primary display.
// Returns detected regions (text, rectangles) for the captured display.
// Caller must call NeruFreeVisionResult() on the returned pointer.
VisionResult *NeruDetectElements(CGRect detectionRect, NeruVisionConfig config);

// Captures raw screen pixels for the main display.
// Returns a CGImageRef (caller must CFRelease).
CGImageRef NeruCaptureScreen(void);

// Captures the display containing rect (global top-left-origin points) and
// reports that display's bounds in the same coordinates through outBounds.
// Waits at most timeoutMS. Returns a CGImageRef (caller must CFRelease), or
// NULL when capture fails or runs out of time.
CGImageRef NeruCaptureDisplayContaining(CGRect rect, CGRect *outBounds, int64_t timeoutMS);

// Captures rect (global top-left-origin points), clipped to the display that
// holds it, at its size in points, and reports the part captured in the same
// coordinates through outCaptured. Reuses the shareable-content list between
// calls. Returns a CGImageRef (caller must CFRelease), or NULL before macOS 14
// and when capture fails or runs out of timeoutMS. Callers fall back to
// NeruCaptureDisplayContaining.
CGImageRef NeruCaptureRect(CGRect rect, CGRect *outCaptured, int64_t timeoutMS);

// Creates a bitmap context drawing into data as 32-bit little-endian BGRA with
// premultiplied alpha, stride bytes per row, in space. The flags are combined
// here rather than in Go because SDKs disagree on the parameter's C type
// (uint32_t on older ones, CGBitmapInfo on newer), and Go accepts only the one
// it was compiled against. Returns NULL on failure. The caller releases it.
CGContextRef NeruCreateBGRABitmapContext(void *data, size_t width, size_t height, size_t stride, CGColorSpaceRef space);

// Frees a VisionResult previously returned by NeruDetectElements.
void NeruFreeVisionResult(VisionResult *result);
