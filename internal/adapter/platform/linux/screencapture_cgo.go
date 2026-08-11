//go:build linux && cgo

package linux

/*
#include <stdlib.h>
#include "screencapture.h"
*/
import "C"

import (
	"image"
	"unsafe"

	"github.com/y3owk1n/neru/internal/derrors"
)

// captureResult turns a filled NeruCapture into an image.RGBA and releases the
// native buffer.
//
// The copy is deliberate and is the whole lifetime story of a captured frame:
// C owns the pixels only until this returns, neru_capture_free wipes them
// before releasing them, and the Go image is then the single copy in the
// process. Nothing here logs a dimension-plus-content pair, writes a file, or
// keeps a package-level reference — a capture cache would be a cache of the
// user's screen.
func captureResult(capture *C.NeruCapture) (*image.RGBA, error) {
	defer C.neru_capture_free(capture)

	width := int(capture.width)
	height := int(capture.height)

	if capture.pixels == nil || width <= 0 || height <= 0 {
		return nil, derrors.New(
			derrors.CodeActionFailed,
			"screen capture returned no pixels",
		)
	}

	size := width * height * bytesPerCapturedPixel
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	copy(img.Pix, unsafe.Slice((*byte)(unsafe.Pointer(capture.pixels)), size))

	return img, nil
}

// bytesPerCapturedPixel matches the RGBA8888 layout documented on NeruCapture
// in screencapture.h, which is also image.RGBA's layout.
const bytesPerCapturedPixel = 4

// captureError maps a NERU_CAPTURE_ERR_* code onto the shared error vocabulary.
// what names the backend, so a user reading the message knows which display
// server refused rather than only that "capture failed".
func captureError(status C.int, what string) error {
	switch status {
	case C.NERU_CAPTURE_ERR_NO_DISPLAY:
		return derrors.New(
			derrors.CodeNotSupported,
			"screen capture is unavailable: could not connect to "+what,
		)
	case C.NERU_CAPTURE_ERR_NO_PROTOCOL:
		return derrors.New(
			derrors.CodeNotSupported,
			what+" does not implement wlr-screencopy-unstable-v1, the protocol Neru "+
				"captures the screen with",
		)
	case C.NERU_CAPTURE_ERR_NO_OUTPUT:
		return derrors.New(
			derrors.CodeActionFailed,
			"no output of "+what+" covers the requested region",
		)
	case C.NERU_CAPTURE_ERR_REGION:
		return derrors.New(
			derrors.CodeActionFailed,
			"the requested capture region is empty or lies outside the screen",
		)
	case C.NERU_CAPTURE_ERR_FORMAT:
		return derrors.New(
			derrors.CodeNotSupported,
			what+" offered a pixel format Neru cannot read",
		)
	case C.NERU_CAPTURE_ERR_ALLOC:
		return derrors.New(
			derrors.CodeInternal,
			"could not allocate a buffer for the screen capture",
		)
	case C.NERU_CAPTURE_ERR_TIMEOUT:
		return derrors.New(
			derrors.CodeActionFailed,
			what+" did not answer the screen capture in time",
		)
	default:
		return derrors.New(
			derrors.CodeActionFailed,
			what+" failed to capture the screen",
		)
	}
}
