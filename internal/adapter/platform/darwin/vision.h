#import <CoreGraphics/CoreGraphics.h>

// Screen capture through ScreenCaptureKit, for the vision adapter's
// detectors and line reads. Text recognition runs in Go (yomi), not here.

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

// Creates a bitmap context drawing into data as 32-bit little-endian BGRA with
// premultiplied alpha, stride bytes per row, in space. The flags are combined
// here rather than in Go because SDKs disagree on the parameter's C type
// (uint32_t on older ones, CGBitmapInfo on newer), and Go accepts only the one
// it was compiled against. Returns NULL on failure. The caller releases it.
CGContextRef NeruCreateBGRABitmapContext(void *data, size_t width, size_t height, size_t stride, CGColorSpaceRef space);
