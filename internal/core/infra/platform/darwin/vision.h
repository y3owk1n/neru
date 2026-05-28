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

// Captures the main display and runs Vision Framework detection.
// Returns detected regions (text, rectangles) for the full screen.
// Caller must call NeruFreeVisionResult() on the returned pointer.
// The screenBounds parameter is reserved for future cropping.
VisionResult *NeruDetectElements(CGRect screenBounds);

// Captures raw screen pixels for the main display.
// Returns a CGImageRef (caller must CFRelease).
CGImageRef NeruCaptureScreen(void);

// Frees a VisionResult previously returned by NeruDetectElements.
void NeruFreeVisionResult(VisionResult *result);
