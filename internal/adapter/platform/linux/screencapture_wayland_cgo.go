//go:build linux && cgo

package linux

/*
#cgo linux pkg-config: wayland-client
#include <stdlib.h>
#include "wlroots_screencopy.h"
*/
import "C"

import (
	"image"
	"os"

	// Blank-import to link the wayland-scanner generated protocol objects
	// (zwlr_screencopy_manager_v1, zxdg_output_manager_v1).
	_ "github.com/y3owk1n/neru/internal/adapter/platform/linux/wlr_protocol"
	"github.com/y3owk1n/neru/internal/derrors"
)

// wlrootsCaptureRegion reads region back through wlr-screencopy-unstable-v1.
//
// Only the wlroots family reaches here. KWin shares this client stack for
// everything else — wl_output, xdg-output, layer shell — but advertises no
// screencopy manager, so CaptureScreenRegion sends KDE to the portal backend
// before this branch is considered.
func wlrootsCaptureRegion(region image.Rectangle) (*image.RGBA, error) {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return nil, derrors.New(
			derrors.CodeNotSupported,
			"WAYLAND_DISPLAY is not set; Wayland screen capture is unavailable",
		)
	}

	var capture C.NeruCapture

	status := C.neru_screencopy_capture_region(
		C.int(region.Min.X),
		C.int(region.Min.Y),
		C.int(region.Dx()),
		C.int(region.Dy()),
		C.int(screenCaptureTimeoutMS),
		&capture,
	)

	if status != C.NERU_CAPTURE_OK {
		return nil, captureError(captureStatus(status), captureLabelCompositor)
	}

	return captureResult(&capture)
}
