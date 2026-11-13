//go:build darwin

package vision

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Vision -framework CoreGraphics -framework Foundation
#include "../platform/darwin/vision.h"
*/
import "C"

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/derrors"
)

// readFrame is a CGImage capture, which Neru holds a reference to, with the
// rect in points it shows.
type readFrame = *cgFrame

type cgFrame struct {
	image  C.CGImageRef
	bounds C.CGRect
}

// retainFrame takes a reference to frame for a second owner.
func retainFrame(frame readFrame) readFrame {
	if frame != nil {
		C.CGImageRetain(frame.image)
	}

	return frame
}

// releaseFrame gives back one owner's reference.
func releaseFrame(frame readFrame) {
	if frame != nil {
		C.CGImageRelease(frame.image)
	}
}

// sameFrame reports two captures of the same pixels.
func sameFrame(a, b readFrame) bool {
	return C.NeruImagesEqual(a.image, b.image) != 0
}

// captureReadFrame captures region at readLinesScale, as a line read reads it,
// within what is left of ctx.
func captureReadFrame(ctx context.Context, region image.Rectangle) (readFrame, error) {
	timeout, err := captureTimeoutMS(ctx)
	if err != nil {
		return nil, err
	}

	var captured C.CGRect

	captureImage := C.NeruCaptureRectAtScale(cRect(region), readLinesScale, &captured, timeout)
	if uintptr(captureImage) == 0 {
		return nil, derrors.New(derrors.CodeActionFailed, "failed to capture the region to read")
	}

	return &cgFrame{image: captureImage, bounds: captured}, nil
}
