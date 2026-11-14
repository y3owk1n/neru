#ifndef BADGE_PAINT_H
#define BADGE_PAINT_H

#include <cairo/cairo.h>

// Painting shared by the X11 and Wayland overlays, which draw a badge with
// the same cairo calls and differ only in the context they hand over. Colors
// are packed ARGB, as everywhere in the overlays.

// neru_badge_set_fill makes the badge fill the context's source: top flat when
// bottom is the same color, otherwise a vertical gradient from top at y to
// bottom at y + height. The gradient pads past both ends, so a connector arrow
// beyond the badge takes the color of the edge it hangs from.
void neru_badge_set_fill(cairo_t *cr, double y, double height, unsigned int top, unsigned int bottom);

// neru_badge_paint_mask paints color through an 8-bit alpha mask of width x
// height, stored row by row with no padding, with its top-left corner at
// (x, y). A hint shadow is painted this way.
void neru_badge_paint_mask(
    cairo_t *cr, const unsigned char *alpha, int width, int height, double x, double y, unsigned int color);

#endif /* BADGE_PAINT_H */
