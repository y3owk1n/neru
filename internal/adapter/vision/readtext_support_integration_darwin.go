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

		result := C.NeruReadImageLines(
			cgImage,
			bounds,
			1,
			nil,
			C.int64_t(readLinesTestTimeoutMS),
			nil,
		)
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

// readLinesFromImageCanceled reads img as readLinesFromImage does, with its
// cancel flag set before the read starts, and reports whether the read
// returned anything.
func readLinesFromImageCanceled(img *image.RGBA, pixelsPerPoint int) bool {
	returned := false

	withCGImage(img, func(cgImage C.CGImageRef, _ C.CGColorSpaceRef) {
		bounds := C.CGRectMake(
			0, 0,
			C.CGFloat(img.Rect.Dx()/pixelsPerPoint), C.CGFloat(img.Rect.Dy()/pixelsPerPoint),
		)

		cancel := C.int(1)

		result := C.NeruReadImageLines(
			cgImage, bounds, 1, nil, C.int64_t(readLinesTestTimeoutMS), &cancel,
		)
		if result != nil {
			returned = true

			C.NeruFreeTextResult(result)
		}
	})

	return returned
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
