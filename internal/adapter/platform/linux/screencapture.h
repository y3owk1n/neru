#ifndef SCREENCAPTURE_H
#define SCREENCAPTURE_H

#include <stddef.h>

// Shared result type and status codes for the Linux screen-capture backends,
// so Go has one conversion and one error vocabulary rather than one per
// backend.
//
// Privacy: a capture buffer holds arbitrary screen content. Nothing in this
// subsystem writes a byte of it anywhere but the caller's buffer, and
// neru_capture_free wipes that buffer so freed heap keeps no readable copy.

#define NERU_CAPTURE_OK 0
// No display server to capture from ($DISPLAY / $WAYLAND_DISPLAY unset, or the
// connection was refused).
#define NERU_CAPTURE_ERR_NO_DISPLAY 1
// The compositor does not advertise a protocol this backend needs.
#define NERU_CAPTURE_ERR_NO_PROTOCOL 2
// No output covers the requested region.
#define NERU_CAPTURE_ERR_NO_OUTPUT 3
// The region is empty, or falls entirely outside the screen.
#define NERU_CAPTURE_ERR_REGION 4
// The pixel format the display server offered is not one we can convert.
#define NERU_CAPTURE_ERR_FORMAT 5
// Out of memory, or the shared-memory buffer could not be created.
#define NERU_CAPTURE_ERR_ALLOC 6
// The display server accepted the request and then failed the copy.
#define NERU_CAPTURE_ERR_FAILED 7
// The display server never answered within the caller's budget.
#define NERU_CAPTURE_ERR_TIMEOUT 8

typedef struct {
	// pixels is width*height*4 bytes of RGBA8888 with no row padding, matching
	// Go's image.RGBA so the Go side is a single copy. Alpha is always 0xFF. A
	// capture is opaque, and image.RGBA is premultiplied, so a source alpha
	// would darken the image.
	unsigned char *pixels;
	int width;
	int height;
} NeruCapture;

// neru_capture_free wipes and releases a capture filled by one of the backends.
// Safe to call on a zeroed struct.
void neru_capture_free(NeruCapture *capture);

// neru_capture_begin zeroes out and validates the requested rectangle. Both
// backends open with it, so a caller never sees a stale pointer beside an error
// code. Returns NERU_CAPTURE_OK when the request is worth sending on.
int neru_capture_begin(NeruCapture *out, int w, int h);

// neru_capture_wipe zeroes size bytes of buf through a volatile pointer, so the
// compiler cannot elide the write on a buffer that is about to be freed or
// unmapped. Exposed for the backends' intermediate buffers, and for ocr.c,
// whose recognized strings are the same screen content in another shape.
void neru_capture_wipe(unsigned char *buf, size_t size);

#endif /* SCREENCAPTURE_H */
