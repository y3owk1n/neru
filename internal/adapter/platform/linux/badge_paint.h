#ifndef BADGE_PAINT_H
#define BADGE_PAINT_H

#include <cairo/cairo.h>

// NERU_BADGE_SLOTS is how many indicator badges an overlay keeps a surface
// for: the mode indicator and the sticky-modifiers indicator.
#define NERU_BADGE_SLOTS 2

// NeruBadgeLook is everything an indicator badge's pixels depend on. Two
// equal looks paint the same badge, so a badge that only moved is not painted
// again.
typedef struct {
	int width;   // badge rectangle, without the stroke padding
	int height;  // around it
	int pad;     // transparent margin each side, for the stroke
	double scale;
	unsigned int fill;
	unsigned int stroke;
	double stroke_width;
	unsigned int text_color;
	double font_size;
	int bold;
	char text[128];
	char family[256];
} NeruBadgeLook;

// neru_badge_look_equal reports whether two looks paint the same pixels.
int neru_badge_look_equal(const NeruBadgeLook *a, const NeruBadgeLook *b);

// neru_badge_paint clears cr and paints look into it: a filled and stroked
// rectangle at (pad, pad), with its text centered in it. cr is in the badge
// surface's own coordinates, with any HiDPI scale already applied.
void neru_badge_paint(cairo_t *cr, const NeruBadgeLook *look);

#endif /* BADGE_PAINT_H */
