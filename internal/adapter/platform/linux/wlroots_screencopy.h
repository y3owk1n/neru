#ifndef WLROOTS_SCREENCOPY_H
#define WLROOTS_SCREENCOPY_H

#include "screencapture.h"

// neru_screencopy_capture_region captures the rectangle [x, y, w, h] through
// wlr-screencopy-unstable-v1.
//
// Coordinates are Neru's shared space, which on Wayland is the logical space
// xdg_output reports. The region must lie wholly inside one output and is
// translated into its local logical coordinates. A region spanning two outputs
// answers NERU_CAPTURE_ERR_NO_OUTPUT rather than coming back cropped, so the
// result always covers exactly what was asked for.
//
// On a scaled output the buffer is in *physical* pixels, so it can be larger
// than the region by the scale factor, as a Retina capture is on macOS.
//
// This opens its own short-lived wl_display connection rather than borrowing
// the one in wlroots_client.c, because screencopy needs synchronous roundtrips,
// and that client has a dispatch thread owning its reads. overlay_wayland.c and
// wayland_keymap.c take the same approach for the same reason.
//
// The frame is in the output's buffer orientation. transform receives the
// output's wl_output.transform, which the caller undoes to get the image as it
// appears on screen.
//
// timeout_ms bounds the whole exchange. Returns NERU_CAPTURE_OK and fills out on
// success, one of the NERU_CAPTURE_ERR_* codes otherwise. On success the caller
// owns out and must release it with neru_capture_free.
int neru_screencopy_capture_region(int x, int y, int w, int h, int timeout_ms, NeruCapture *out, int *transform);

#endif /* WLROOTS_SCREENCOPY_H */
