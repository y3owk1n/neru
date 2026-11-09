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

// One whitespace-separated word of a read line, with its box in global
// top-left points.
typedef struct {
	char *text;
	double x;
	double y;
	double width;
	double height;
	int line;             // index of the line it was read on
	int word;             // index of the whitespace-separated word it is part of, in its line
	uint32_t background;  // the color behind its word, 0xRRGGBB
	double borderAbove;   // points from its word up to the nearest border, 0 for none
	double borderBelow;   // points from its word down to the nearest border, 0 for none
} NeruTextPiece;

typedef struct {
	NeruTextPiece *pieces;
	int count;
} NeruTextResult;

// Captures rect (global top-left points) at scale times its size and reads its
// text, with the accurate recognizer when accurate is 1 and the fast one
// otherwise, language correction off. Waits at most timeoutMS for the capture
// and as long again for the read. Returns NULL when either fails. Caller frees
// the result with NeruFreeTextResult.
NeruTextResult *NeruReadLines(CGRect rect, double scale, int accurate, int64_t timeoutMS);

// Reads image's text as NeruReadLines reads a capture, with bounds as the
// global rect, in points, the image shows. The helper process that reads text
// lines accurately reads its image with it, and so do tests. An accurate read
// also reads with language, such as "zh-Hans", or with the Chinese, Japanese
// or Korean language the user prefers when it is empty or NULL. When cancel is
// not NULL, the read stops and returns NULL once *cancel is nonzero.
NeruTextResult *NeruReadImageLines(
    CGImageRef image, CGRect bounds, int accurate, const char *language, int64_t timeoutMS, const int *cancel);

// Draws a line each of Latin, Arabic, Devanagari and Chinese text, black on
// white. Reading it accurately has Vision compile every recognition model a
// later read can need. Returns NULL when it cannot draw. Caller releases the
// image.
CF_RETURNS_RETAINED CGImageRef NeruTextSampleImage(void);

// NeruCancelRead sets flag, which stops the NeruReadImageLines it was passed to.
void NeruCancelRead(int *flag);

// Frees what NeruReadLines returned.
void NeruFreeTextResult(NeruTextResult *result);

// Reports whether two captures hold the same pixels, 1 when they do. A read
// that waits for the screen to settle compares its captures with it.
int NeruImagesEqual(CGImageRef first, CGImageRef second);

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

// NeruCaptureRectAtScale is NeruCaptureRect with the region captured at scale
// times its size in points, for reading small text from it.
CGImageRef NeruCaptureRectAtScale(CGRect rect, double scale, CGRect *outCaptured, int64_t timeoutMS);

// NeruReadText reads the text in image, top to bottom, joining lines with a
// space. A one-line image is read at several heights, and the read most of
// them agree on wins. It returns a malloc'd UTF-8 string the caller frees, or NULL when the
// image holds no text.
char *NeruReadText(CGImageRef image);

// Creates a bitmap context drawing into data as 32-bit little-endian BGRA with
// premultiplied alpha, stride bytes per row, in space. The flags are combined
// here rather than in Go because SDKs disagree on the parameter's C type
// (uint32_t on older ones, CGBitmapInfo on newer), and Go accepts only the one
// it was compiled against. Returns NULL on failure. The caller releases it.
CGContextRef NeruCreateBGRABitmapContext(void *data, size_t width, size_t height, size_t stride, CGColorSpaceRef space);

// Frees a VisionResult previously returned by NeruDetectElements.
void NeruFreeVisionResult(VisionResult *result);
