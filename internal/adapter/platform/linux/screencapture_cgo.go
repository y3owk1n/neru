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

// A Go test file cannot use cgo, so the NERU_CAPTURE_* codes are mirrored as
// Go constants in screencapture_common.go and the error vocabulary is written
// against those. These constant expressions are what keeps the two halves
// honest: each converts a difference to uint in both directions, so a value
// that drifts stops compiling here rather than silently mapping the wrong
// failure to the wrong sentence.
const (
	_ = uint(C.NERU_CAPTURE_OK-captureStatusOK) + uint(captureStatusOK-C.NERU_CAPTURE_OK)
	_ = uint(C.NERU_CAPTURE_ERR_NO_DISPLAY-captureStatusNoDisplay) +
		uint(captureStatusNoDisplay-C.NERU_CAPTURE_ERR_NO_DISPLAY)
	_ = uint(C.NERU_CAPTURE_ERR_NO_PROTOCOL-captureStatusNoProtocol) +
		uint(captureStatusNoProtocol-C.NERU_CAPTURE_ERR_NO_PROTOCOL)
	_ = uint(C.NERU_CAPTURE_ERR_NO_OUTPUT-captureStatusNoOutput) +
		uint(captureStatusNoOutput-C.NERU_CAPTURE_ERR_NO_OUTPUT)
	_ = uint(C.NERU_CAPTURE_ERR_REGION-captureStatusRegion) +
		uint(captureStatusRegion-C.NERU_CAPTURE_ERR_REGION)
	_ = uint(C.NERU_CAPTURE_ERR_FORMAT-captureStatusFormat) +
		uint(captureStatusFormat-C.NERU_CAPTURE_ERR_FORMAT)
	_ = uint(C.NERU_CAPTURE_ERR_ALLOC-captureStatusAlloc) +
		uint(captureStatusAlloc-C.NERU_CAPTURE_ERR_ALLOC)
	_ = uint(C.NERU_CAPTURE_ERR_FAILED-captureStatusFailed) +
		uint(captureStatusFailed-C.NERU_CAPTURE_ERR_FAILED)
	_ = uint(C.NERU_CAPTURE_ERR_TIMEOUT-captureStatusTimeout) +
		uint(captureStatusTimeout-C.NERU_CAPTURE_ERR_TIMEOUT)
)

// bytesPerCapturedPixel matches the RGBA8888 layout documented on NeruCapture
// in screencapture.h, which is also image.RGBA's layout.
const bytesPerCapturedPixel = 4

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
