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
// Returns a CGImageRef (caller must CFRelease), or NULL when capture fails.
CGImageRef NeruCaptureDisplayContaining(CGRect rect, CGRect *outBounds);

// Frees a VisionResult previously returned by NeruDetectElements.
void NeruFreeVisionResult(VisionResult *result);
