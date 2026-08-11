#include "x11_screencapture.h"

#include "screencapture.h"

#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <pthread.h>
#include <stdint.h>
#include <stdlib.h>

// Xlib's default error handler calls exit() on a protocol error, and XGetImage
// answers BadMatch for any rectangle that is not wholly inside the drawable —
// which a display hotplug between the clip and the request can produce however
// carefully the region was clipped. Swapping in a handler that records the
// error instead of exiting turns "the daemon dies mid-capture" into "the
// capture failed".
//
// XSetErrorHandler is process-global, so captures serialise on this mutex and
// the previous handler is restored before returning. Another thread's X error
// landing inside that window is handled by this handler rather than the
// default — which is strictly safer for it, since returning beats exiting.
static pthread_mutex_t neru_x11_capture_mutex = PTHREAD_MUTEX_INITIALIZER;
static int neru_x11_capture_error;

static int neru_x11_capture_error_handler(Display *display, XErrorEvent *event) {
	(void)display;
	(void)event;

	neru_x11_capture_error = 1;

	return 0;
}

// neru_x11_mask_shift returns how far right a channel mask sits.
static int neru_x11_mask_shift(unsigned long mask) {
	int shift = 0;

	if (!mask) {
		return 0;
	}

	while (!(mask & 1UL)) {
		mask >>= 1;
		shift++;
	}

	return shift;
}

// neru_x11_mask_width returns how many bits a channel mask covers.
static int neru_x11_mask_width(unsigned long mask) {
	int bits = 0;

	while (mask) {
		bits += (int)(mask & 1UL);
		mask >>= 1;
	}

	return bits;
}

// neru_x11_channel extracts one 8-bit channel from a raw pixel. Visuals with
// fewer than 8 bits per channel (16-bit displays) are scaled up rather than
// rejected; the vision strategy reads shapes and text, not exact colours.
static unsigned char neru_x11_channel(unsigned long pixel, unsigned long mask, int shift, int bits) {
	if (!mask || bits <= 0) {
		return 0;
	}

	unsigned long value = (pixel & mask) >> shift;

	if (bits >= 8) {
		return (unsigned char)(value >> (bits - 8));
	}

	unsigned long max = (1UL << bits) - 1UL;

	return (unsigned char)((value * 255UL) / max);
}

// neru_x11_read_pixel returns the raw pixel at (col, row). The 32-bits-per-pixel
// case — every modern TrueColor visual — is read straight out of the image data
// rather than through XGetPixel, which is an indirect call per pixel and shows
// up as tens of milliseconds on a full-screen grab.
static unsigned long neru_x11_read_pixel(XImage *image, int col, int row, int direct) {
	if (!direct) {
		return XGetPixel(image, col, row);
	}

	const unsigned char *src =
	    (const unsigned char *)image->data + (size_t)row * (size_t)image->bytes_per_line + (size_t)col * 4u;

	if (image->byte_order == LSBFirst) {
		return (unsigned long)src[0] | ((unsigned long)src[1] << 8) | ((unsigned long)src[2] << 16) |
		       ((unsigned long)src[3] << 24);
	}

	return (unsigned long)src[3] | ((unsigned long)src[2] << 8) | ((unsigned long)src[1] << 16) |
	       ((unsigned long)src[0] << 24);
}

// neru_x11_capture_convert turns an XImage into the packed RGBA8888 buffer the
// Go side expects.
static int neru_x11_capture_convert(XImage *image, int width, int height, NeruCapture *out) {
	if (!image->red_mask || !image->green_mask || !image->blue_mask) {
		// A PseudoColor visual: the pixel value is a colormap index and the
		// masks are zero. Refusing is better than emitting garbage.
		return NERU_CAPTURE_ERR_FORMAT;
	}

	size_t size = (size_t)width * (size_t)height * 4u;

	unsigned char *pixels = malloc(size);
	if (!pixels) {
		return NERU_CAPTURE_ERR_ALLOC;
	}

	int red_shift = neru_x11_mask_shift(image->red_mask);
	int red_bits = neru_x11_mask_width(image->red_mask);
	int green_shift = neru_x11_mask_shift(image->green_mask);
	int green_bits = neru_x11_mask_width(image->green_mask);
	int blue_shift = neru_x11_mask_shift(image->blue_mask);
	int blue_bits = neru_x11_mask_width(image->blue_mask);

	int direct = image->bits_per_pixel == 32;

	for (int row = 0; row < height; row++) {
		unsigned char *dst = pixels + (size_t)row * (size_t)width * 4u;

		for (int col = 0; col < width; col++) {
			unsigned long pixel = neru_x11_read_pixel(image, col, row, direct);

			dst[0] = neru_x11_channel(pixel, image->red_mask, red_shift, red_bits);
			dst[1] = neru_x11_channel(pixel, image->green_mask, green_shift, green_bits);
			dst[2] = neru_x11_channel(pixel, image->blue_mask, blue_shift, blue_bits);
			dst[3] = 0xFF;
			dst += 4;
		}
	}

	out->pixels = pixels;
	out->width = width;
	out->height = height;

	return NERU_CAPTURE_OK;
}

int neru_x11_capture_region(int x, int y, int w, int h, NeruCapture *out) {
	if (!out) {
		return NERU_CAPTURE_ERR_REGION;
	}

	out->pixels = NULL;
	out->width = 0;
	out->height = 0;

	if (w <= 0 || h <= 0) {
		return NERU_CAPTURE_ERR_REGION;
	}

	if (!getenv("DISPLAY")) {
		return NERU_CAPTURE_ERR_NO_DISPLAY;
	}

	// Its own connection: every thread in this package owns its Display, which
	// is what lets the tree do without XInitThreads.
	Display *display = XOpenDisplay(NULL);
	if (!display) {
		return NERU_CAPTURE_ERR_NO_DISPLAY;
	}

	Window root = DefaultRootWindow(display);

	XWindowAttributes attrs;
	if (!XGetWindowAttributes(display, root, &attrs)) {
		XCloseDisplay(display);

		return NERU_CAPTURE_ERR_FAILED;
	}

	int left = x < 0 ? 0 : x;
	int top = y < 0 ? 0 : y;
	int right = x + w;
	int bottom = y + h;

	if (right > attrs.width) {
		right = attrs.width;
	}

	if (bottom > attrs.height) {
		bottom = attrs.height;
	}

	if (right <= left || bottom <= top) {
		XCloseDisplay(display);

		return NERU_CAPTURE_ERR_REGION;
	}

	int clipped_w = right - left;
	int clipped_h = bottom - top;

	pthread_mutex_lock(&neru_x11_capture_mutex);

	neru_x11_capture_error = 0;

	XErrorHandler previous = XSetErrorHandler(neru_x11_capture_error_handler);
	XImage *image =
	    XGetImage(display, root, left, top, (unsigned int)clipped_w, (unsigned int)clipped_h, AllPlanes, ZPixmap);

	// Errors arrive asynchronously; sync so the handler has run before it is
	// swapped back out.
	XSync(display, False);
	XSetErrorHandler(previous);

	int failed = neru_x11_capture_error;

	pthread_mutex_unlock(&neru_x11_capture_mutex);

	if (!image || failed) {
		if (image) {
			XDestroyImage(image);
		}

		XCloseDisplay(display);

		return NERU_CAPTURE_ERR_FAILED;
	}

	int status = neru_x11_capture_convert(image, clipped_w, clipped_h, out);

	// XDestroyImage frees image->data, which held screen pixels; wipe it first.
	if (image->data) {
		neru_capture_wipe((unsigned char *)image->data, (size_t)image->bytes_per_line * (size_t)image->height);
	}

	XDestroyImage(image);
	XCloseDisplay(display);

	return status;
}
