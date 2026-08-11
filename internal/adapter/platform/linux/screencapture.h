#ifndef SCREENCAPTURE_H
#define SCREENCAPTURE_H

#include <stddef.h>

// Shared result type and status codes for the Linux screen-capture backends:
// x11_screencapture.c (XGetImage) and wlroots_screencopy.c
// (wlr-screencopy-unstable-v1). Both hand Go the same thing, so the Go side has
// one conversion and one error vocabulary rather than one per backend.
//
// Privacy: a capture buffer holds arbitrary screen content — whatever the user
// happens to have on screen, including windows Neru was never asked about.
// Nothing in this subsystem writes a byte of it anywhere but the buffer the
// caller asked for, and neru_capture_free wipes that buffer before releasing
// it so freed heap does not keep a readable copy for the next allocation.

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
	// Go's image.RGBA layout so the Go side is a single copy. Alpha is always
	// 0xFF: a screen capture is opaque, and Go's image.RGBA is
	// alpha-premultiplied, so carrying a source alpha through would darken the
	// image rather than describe it.
	unsigned char *pixels;
	int width;
	int height;
} NeruCapture;

// neru_capture_free wipes and releases a capture filled by one of the backends.
// Safe to call on a zeroed struct.
void neru_capture_free(NeruCapture *capture);

// neru_capture_wipe zeroes size bytes of buf through a volatile pointer, so the
// compiler cannot elide the write on a buffer that is about to be freed or
// unmapped. Exposed for the backends' intermediate buffers.
void neru_capture_wipe(unsigned char *buf, size_t size);

#endif /* SCREENCAPTURE_H */
