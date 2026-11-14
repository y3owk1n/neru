#ifndef LABEL_TEXT_H
#define LABEL_TEXT_H

#include <cairo/cairo.h>

// neru_label_show draws text from the current point in color, with its first
// matched bytes, the part of a hint label already typed, in matched_color.
// The two runs are drawn one after the other from the same current point, so
// the label is laid out exactly as one run would be. Colors are packed ARGB.
// Shared by the X11 and Wayland overlays, which differ only in the context.
void neru_label_show(cairo_t *cr, const char *text, int matched, unsigned int color, unsigned int matched_color);

#endif /* LABEL_TEXT_H */
