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
// backend names the compositor family in the error, which matters most for the
// one that reaches here and cannot answer: KWin implements no screencopy
// protocol Neru can use, so KDE Plasma sessions get a CodeNotSupported saying
// so rather than a bare failure.
func wlrootsCaptureRegion(region image.Rectangle, backend string) (*image.RGBA, error) {
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
		return nil, captureError(status, captureCompositorLabel(backend))
	}

	return captureResult(&capture)
}

// captureCompositorLabel names the compositor family in capture errors.
func captureCompositorLabel(backend string) string {
	if backend == backendWaylandKDE {
		return "KDE Plasma (KWin)"
	}

	return "this Wayland compositor"
}
