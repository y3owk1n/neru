#ifndef WLROOTS_SCREENCOPY_H
#define WLROOTS_SCREENCOPY_H

#include "screencapture.h"

// neru_screencopy_capture_region captures the rectangle [x, y, w, h] through
// wlr-screencopy-unstable-v1.
//
// Coordinates are Neru's shared space — global origin, top-left, Y down,
// unscaled — which on Wayland is the logical space xdg_output reports. The
// region must lie wholly inside one output: it is translated into that output's
// local logical coordinates, which is what capture_output_region takes, and a
// region spanning two outputs answers NERU_CAPTURE_ERR_NO_OUTPUT rather than
// coming back cropped. What is returned always covers exactly what was asked
// for.
//
// On a scaled output the compositor answers with a *physical*-pixel buffer, so
// the returned width and height can be larger than the requested region by the
// output's scale factor; that is the same thing a Retina capture does on macOS.
// The buffer's own origin is (0, 0) — it is the caller's region that says where
// those pixels are.
//
// This opens its own short-lived wl_display connection rather than borrowing
// the one in wlroots_client.c: screencopy needs synchronous roundtrips, and
// that client has a dispatch thread owning its reads. overlay_wayland.c and
// wayland_keymap.c take the same approach for the same reason.
//
// timeout_ms bounds the whole exchange. Returns NERU_CAPTURE_OK and fills out on
// success, one of the NERU_CAPTURE_ERR_* codes otherwise; on success the caller
// owns out and must release it with neru_capture_free.
int neru_screencopy_capture_region(int x, int y, int w, int h, int timeout_ms, NeruCapture *out);

#endif /* WLROOTS_SCREENCOPY_H */
