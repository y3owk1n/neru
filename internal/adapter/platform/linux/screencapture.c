#include "screencapture.h"

#include <stdlib.h>

void neru_capture_wipe(unsigned char *buf, size_t size) {
	if (!buf) {
		return;
	}

	// volatile so the compiler keeps the stores it would otherwise drop as dead
	// on memory that is about to be freed or unmapped.
	volatile unsigned char *cursor = buf;

	while (size--) {
		*cursor++ = 0;
	}
}

void neru_capture_free(NeruCapture *capture) {
	if (!capture || !capture->pixels) {
		return;
	}

	size_t size = (size_t)capture->width * (size_t)capture->height * 4u;

	neru_capture_wipe(capture->pixels, size);
	free(capture->pixels);

	capture->pixels = NULL;
	capture->width = 0;
	capture->height = 0;
}
