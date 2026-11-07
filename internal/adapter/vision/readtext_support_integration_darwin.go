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

// readTextFromImage reads img, drawn at scale times its size, the way
// ReadText reads a capture, for a test that cannot put pixels on the screen.
func readTextFromImage(img *image.RGBA, scale float64) string {
	text := ""

	withCGImage(img, func(cgImage C.CGImageRef, space C.CGColorSpaceRef) {
		width, height := img.Rect.Dx(), img.Rect.Dy()

		scaled := scaledImage(cgImage, space, width, height, scale)
		if uintptr(scaled) == 0 {
			return
		}
		defer C.CGImageRelease(scaled)

		read := C.NeruReadText(scaled)
		if read == nil {
			return
		}
		defer C.free(unsafe.Pointer(read))

		text = C.GoString(read)
	})

	return text
}

// readLinesFromImage reads img's lines with the accurate recognizer the way
// ReadLines reads a capture taken at pixelsPerPoint, as if the image showed
// the screen from its top-left corner.
func readLinesFromImage(img *image.RGBA, pixelsPerPoint int) []readPiece {
	var pieces []readPiece

	withCGImage(img, func(cgImage C.CGImageRef, _ C.CGColorSpaceRef) {
		bounds := C.CGRectMake(
			0, 0,
			C.CGFloat(img.Rect.Dx()/pixelsPerPoint), C.CGFloat(img.Rect.Dy()/pixelsPerPoint),
		)

		result := C.NeruReadImageLines(cgImage, bounds, 1, C.int64_t(readLinesTestTimeoutMS))
		if result == nil {
			return
		}
		defer C.NeruFreeTextResult(result)

		for _, piece := range unsafe.Slice(result.pieces, int(result.count)) {
			pieces = append(pieces, readPiece{
				text:  C.GoString(piece.text),
				x:     float64(piece.x),
				width: float64(piece.width),
				line:  int(piece.line),
			})
		}
	})

	return pieces
}

// readLinesTestTimeoutMS bounds a test's read, which loads the accurate
// model the first time.
const readLinesTestTimeoutMS = 30000

// readPiece is one piece ReadLines returned, in points.
type readPiece struct {
	text  string
	x     float64
	width float64
	line  int
}

// withCGImage hands use img as a CGImage, with the color space it was made
// in, and keeps the pixels alive until use returns.
func withCGImage(img *image.RGBA, use func(C.CGImageRef, C.CGColorSpaceRef)) {
	width, height := img.Rect.Dx(), img.Rect.Dy()
	if width == 0 || height == 0 {
		return
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
		return
	}
	defer C.CGContextRelease(context)

	cgImage := C.CGBitmapContextCreateImage(context)
	if uintptr(cgImage) == 0 {
		return
	}
	defer C.CGImageRelease(cgImage)

	use(cgImage, space)
}

// scaledImage draws image at scale times its width and height with high
// quality interpolation. The caller releases the result.
func scaledImage(
	image C.CGImageRef,
	space C.CGColorSpaceRef,
	width, height int,
	scale float64,
) C.CGImageRef {
	scaledWidth := C.size_t(float64(width) * scale)
	scaledHeight := C.size_t(float64(height) * scale)

	context := C.CGBitmapContextCreate(
		nil,
		scaledWidth,
		scaledHeight,
		8,
		0,
		space,
		C.kCGImageAlphaPremultipliedLast,
	)
	if context == 0 {
		return 0
	}
	defer C.CGContextRelease(context)

	C.CGContextSetInterpolationQuality(context, C.kCGInterpolationHigh)
	C.CGContextDrawImage(
		context,
		C.CGRectMake(0, 0, C.CGFloat(scaledWidth), C.CGFloat(scaledHeight)),
		image,
	)

	return C.CGBitmapContextCreateImage(context)
}
