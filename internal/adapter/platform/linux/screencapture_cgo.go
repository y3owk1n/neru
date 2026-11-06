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

// Test files cannot use cgo, so screencapture_common.go mirrors the
// NERU_CAPTURE_* codes as Go constants. Each line converts the difference to
// uint both ways, so a drifted value stops compiling rather than mapping a
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

// captureResult copies a filled NeruCapture into an image.RGBA and releases the
// native buffer, which neru_capture_free wipes first. The Go image is then the
// only copy in the process. Never log its content, write it to a file, or keep
// a package-level reference, because a capture cache caches the user's screen.
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
