//go:build integration && darwin

package vision

/*
#include "../platform/darwin/vision.h"
#include <stdlib.h>
#include <string.h>
*/
import "C"

import (
	"image"
	"unsafe"
)

// readTextFromImage reads img the way ReadText reads a capture, for a test
// that cannot put pixels on the screen.
func readTextFromImage(img *image.RGBA) string {
	width, height := img.Rect.Dx(), img.Rect.Dy()
	if width == 0 || height == 0 {
		return ""
	}

	// The context keeps its buffer across calls, so the pixels are copied
	// into C memory rather than handed over as a Go pointer.
	size := C.size_t(len(img.Pix))
	buffer := C.malloc(size)
	defer C.free(buffer)

	C.memcpy(buffer, unsafe.Pointer(&img.Pix[0]), size)

	space := C.CGColorSpaceCreateDeviceRGB()
	defer C.CGColorSpaceRelease(space)

	context := C.CGBitmapContextCreate(
		buffer,
		C.size_t(width),
		C.size_t(height),
		8,
		C.size_t(img.Stride),
		space,
		C.kCGImageAlphaPremultipliedLast,
	)
	if context == 0 {
		return ""
	}
	defer C.CGContextRelease(context)

	cgImage := C.CGBitmapContextCreateImage(context)
	if uintptr(cgImage) == 0 {
		return ""
	}
	defer C.CGImageRelease(cgImage)

	text := C.NeruReadText(cgImage)
	if text == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(text))

	return C.GoString(text)
}
