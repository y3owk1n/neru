#ifndef X11_SCREENCAPTURE_H
#define X11_SCREENCAPTURE_H

#include "screencapture.h"

// neru_x11_capture_region reads the pixels inside the rectangle [x, y, w, h]
// back from the root window with XGetImage.
//
// Coordinates are in Neru's shared space: global origin at the top-left, Y down.
// That is exactly X11 root-window space, so nothing is translated here. A rectangle
// not wholly inside the root window answers NERU_CAPTURE_ERR_REGION rather than
// being clipped, both because XGetImage answers BadMatch (a protocol error,
// fatal by Xlib default) for one and because a clipped frame would no longer
// cover what the caller asked for. The buffer's own origin is (0, 0), and
// the caller's region says where those pixels are.
//
// Returns NERU_CAPTURE_OK and fills out on success, one of the NERU_CAPTURE_ERR_*
// codes otherwise. On success the caller owns out and must release it with
// neru_capture_free.
int neru_x11_capture_region(int x, int y, int w, int h, NeruCapture *out);

#endif /* X11_SCREENCAPTURE_H */
