#ifndef PIPEWIRE_CAPTURE_H
#define PIPEWIRE_CAPTURE_H

#include "screencapture.h"

// The KDE Plasma screen-capture backend: one frame off the PipeWire node an
// xdg-desktop-portal ScreenCast session streams. KWin offers no screencopy
// protocol Neru can use, so pixels come through the portal. The node and its
// monitor's position are negotiated in Go (portal_screencast.go).
//
// Privacy: only the requested crop is copied out, the mapping goes straight
// back to PipeWire, nothing is written anywhere, and neru_capture_free wipes
// the crop.

typedef struct {
	// fd is a PipeWire remote connection from
	// org.freedesktop.portal.ScreenCast.OpenPipeWireRemote. Ownership passes to
	// neru_pipewire_capture, which closes it on every path.
	int fd;
	// node_id is the PipeWire node the portal named for the monitor to read.
	unsigned int node_id;
	// x, y, w, h are the rectangle to return, in the monitor's *logical*
	// coordinates with its top-left as the origin.
	int x;
	int y;
	int w;
	int h;
	// logical_width and logical_height are the monitor's logical size, which is
	// what turns the rectangle above into physical pixels on a scaled output.
	// Zero means the portal named no size, and the frame is then taken to be
	// unscaled.
	int logical_width;
	int logical_height;
	// timeout_ms bounds the wait for a frame to arrive.
	int timeout_ms;
} NeruPipewireRequest;

// neru_pipewire_capture reads one frame off request->node_id and fills out with
// the requested rectangle of it, in the RGBA8888 layout NeruCapture documents.
//
// On a scaled output the frame is in physical pixels, so the result is larger
// than the logical rectangle by the scale factor. A crop that does not fit the
// frame is NERU_CAPTURE_ERR_REGION rather than silently clipped.
//
// Returns NERU_CAPTURE_OK or one of the NERU_CAPTURE_ERR_* codes.
int neru_pipewire_capture(const NeruPipewireRequest *request, NeruCapture *out);

#endif /* PIPEWIRE_CAPTURE_H */
