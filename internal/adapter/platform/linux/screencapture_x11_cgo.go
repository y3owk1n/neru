//go:build linux && cgo

package linux

/*
#cgo linux pkg-config: x11
#cgo linux LDFLAGS: -lpthread
#include <stdlib.h>
#include "x11_screencapture.h"
*/
import "C"

import (
	"image"
	"os"

	"github.com/y3owk1n/neru/internal/derrors"
)

// x11CaptureRegion reads region back from the X11 root window. Root coordinates
// are already Neru's shared space, so the rectangle crosses into C unchanged.
func x11CaptureRegion(region image.Rectangle) (*image.RGBA, error) {
	if os.Getenv("DISPLAY") == "" {
		return nil, derrors.New(
			derrors.CodeNotSupported,
			"DISPLAY is not set; X11 screen capture is unavailable",
		)
	}

	var capture C.NeruCapture

	status := C.neru_x11_capture_region(
		C.int(region.Min.X),
		C.int(region.Min.Y),
		C.int(region.Dx()),
		C.int(region.Dy()),
		&capture,
	)

	if status != C.NERU_CAPTURE_OK {
		return nil, captureError(captureStatus(status), captureLabelXServer)
	}

	return captureResult(&capture)
}
