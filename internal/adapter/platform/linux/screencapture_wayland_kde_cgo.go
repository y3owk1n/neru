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

// pipewireCaptureNode reads one frame off the PipeWire node the granted
// ScreenCast session named for a monitor, and returns the requested rectangle
// of it.
//
// crop is in the monitor's local logical coordinates, and logicalWidth /
// logicalHeight are that monitor's logical size. Together they turn the
// rectangle into physical pixels on a scaled output, which is the only place
// the frame's own dimensions are known.
//
// remoteFD's ownership passes to the native side, which closes it on every
// path. There is exactly one frame per descriptor. The connection is opened for
// this capture and torn down with it, so KWin is never left pushing frames of
// the user's screen at a daemon that is not reading them. When the hider cannot
// take the overlay off the screen, or ctx ends while the overlay is off, the
// native side never runs and this function closes the descriptor.
//
// The frame budget is what is left of ctx when the read starts, after the
// overlay is off, so the wait for a frame cannot outlive the activation that
// asked for it.
func pipewireCaptureNode(
	ctx context.Context,
	remoteFD int,
	nodeID uint32,
	crop image.Rectangle,
	logicalWidth int,
	logicalHeight int,
) (*image.RGBA, error) {
	handedOver := false

	// A new consumer gets a frame the compositor renders as it connects, so
	// connecting with the overlay already off is what leaves it out.
	img, err := withOwnWindowsHidden(func() (*image.RGBA, error) {
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
