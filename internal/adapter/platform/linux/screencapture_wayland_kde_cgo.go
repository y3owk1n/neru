//go:build linux && cgo

package linux

/*
#cgo linux pkg-config: libpipewire-0.3
#include <stdlib.h>
#include "pipewire_capture.h"
*/
import "C"

import (
	"context"
	"image"

	"golang.org/x/sys/unix"
)

// pipewireCaptureNode reads one frame off the ScreenCast node for a monitor and
// returns the requested rectangle of it. crop is in the monitor's local logical
// coordinates and logicalWidth/logicalHeight are its logical size, so the
// native side can scale to physical pixels once it sees the frame's size.
//
// The native side takes ownership of remoteFD and closes it on every path. One
// descriptor carries one frame, so KWin never streams the user's screen at a
// daemon that is not reading. If the hider fails or ctx ends before the
// handover, this function closes it instead. The frame budget is what is left
// of ctx once the overlay is off, and the overlay comes off, over region in
// global pixels, only when hideOverlay is set.
func pipewireCaptureNode(
	ctx context.Context,
	remoteFD int,
	nodeID uint32,
	crop image.Rectangle,
	logicalWidth int,
	logicalHeight int,
	hideOverlay bool,
	region image.Rectangle,
) (*image.RGBA, error) {
	handedOver := false

	// A new consumer gets a frame the compositor renders as it connects, so
	// connecting with the overlay already off is what leaves it out.
	img, err := withOwnWindowsHidden(hideOverlay, region, func() (*image.RGBA, error) {
		err := captureCanceled(ctx)
		if err != nil {
			return nil, err
		}

		handedOver = true

		request := C.NeruPipewireRequest{
			fd:             C.int(remoteFD),
			node_id:        C.uint(nodeID),
			x:              C.int(crop.Min.X),
			y:              C.int(crop.Min.Y),
			w:              C.int(crop.Dx()),
			h:              C.int(crop.Dy()),
			logical_width:  C.int(logicalWidth),
			logical_height: C.int(logicalHeight),
			timeout_ms:     C.int(captureBudgetMS(ctx)),
		}

		var capture C.NeruCapture

		status := C.neru_pipewire_capture(&request, &capture)
		if status != C.NERU_CAPTURE_OK {
			return nil, pipewireCaptureError(captureStatus(status))
		}

		return captureResult(&capture)
	})

	if !handedOver {
		_ = unix.Close(remoteFD)
	}

	return img, err
}
