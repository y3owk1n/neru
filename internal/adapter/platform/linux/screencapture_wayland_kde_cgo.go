//go:build linux && cgo

package linux

/*
#cgo linux pkg-config: libpipewire-0.3
#include <stdlib.h>
#include "pipewire_capture.h"
*/
import "C"

import (
	"image"
)

// pipewireCaptureNode reads one frame off the PipeWire node the granted
// ScreenCast session named for a monitor, and returns the requested rectangle
// of it.
//
// crop is in the monitor's local logical coordinates, and logicalWidth /
// logicalHeight are that monitor's logical size — together they are what turns
// the rectangle into physical pixels on a scaled output, which is the only
// place the frame's own dimensions are known.
//
// remoteFD's ownership passes to the native side, which closes it on every
// path. There is exactly one frame per descriptor: the connection is opened for
// this capture and torn down with it, so KWin is never left pushing frames of
// the user's screen at a daemon that is not reading them.
//
// budgetMS is what is left of the caller's deadline, so the wait for a frame
// cannot outlive the activation that asked for it.
func pipewireCaptureNode(
	remoteFD int,
	nodeID uint32,
	crop image.Rectangle,
	logicalWidth int,
	logicalHeight int,
	budgetMS int,
) (*image.RGBA, error) {
	request := C.NeruPipewireRequest{
		fd:             C.int(remoteFD),
		node_id:        C.uint(nodeID),
		x:              C.int(crop.Min.X),
		y:              C.int(crop.Min.Y),
		w:              C.int(crop.Dx()),
		h:              C.int(crop.Dy()),
		logical_width:  C.int(logicalWidth),
		logical_height: C.int(logicalHeight),
		timeout_ms:     C.int(budgetMS),
	}

	var capture C.NeruCapture

	status := C.neru_pipewire_capture(&request, &capture)
	if status != C.NERU_CAPTURE_OK {
		return nil, pipewireCaptureError(captureStatus(status))
	}

	return captureResult(&capture)
}
