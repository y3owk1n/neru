#ifndef X11_SCREENCAPTURE_H
#define X11_SCREENCAPTURE_H

#include "screencapture.h"

// neru_x11_capture_region reads the pixels inside the rectangle [x, y, w, h]
// back from the root window with XGetImage.
//
// Coordinates are Neru's shared space, which is X11 root-window space, so
// nothing is translated. A rectangle not wholly inside the root window answers
// NERU_CAPTURE_ERR_REGION rather than being clipped, because XGetImage answers
// BadMatch for it and a clipped frame's top-left would not be the caller's.
//
// Returns NERU_CAPTURE_OK and fills out on success, one of the NERU_CAPTURE_ERR_*
// codes otherwise. On success the caller owns out and must release it with
// neru_capture_free.
int neru_x11_capture_region(int x, int y, int w, int h, NeruCapture *out);

#endif /* X11_SCREENCAPTURE_H */
