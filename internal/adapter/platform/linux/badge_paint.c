#include "badge_paint.h"

#include <string.h>

int neru_badge_look_equal(const NeruBadgeLook *a, const NeruBadgeLook *b) {
	return a->width == b->width && a->height == b->height && a->pad == b->pad && a->scale == b->scale &&
	       a->fill == b->fill && a->stroke == b->stroke && a->stroke_width == b->stroke_width &&
	       a->text_color == b->text_color && a->font_size == b->font_size && a->bold == b->bold &&
	       strcmp(a->text, b->text) == 0 && strcmp(a->family, b->family) == 0;
}

static void neru_badge_color(cairo_t *cr, unsigned int color) {
	double a = ((color >> 24) & 0xFF) / 255.0;
	double r = ((color >> 16) & 0xFF) / 255.0;
	double g = ((color >> 8) & 0xFF) / 255.0;
	double b = (color & 0xFF) / 255.0;
	cairo_set_source_rgba(cr, r, g, b, a);
}

void neru_badge_paint(cairo_t *cr, const NeruBadgeLook *look) {
	cairo_save(cr);
	cairo_set_operator(cr, CAIRO_OPERATOR_CLEAR);
	cairo_paint(cr);
	cairo_restore(cr);

	// The same rectangle and stroke the shared surface used to draw the badge
	// with, so moving it to a surface of its own changes no pixel of it.
	cairo_save(cr);
	cairo_rectangle(cr, look->pad, look->pad, look->width, look->height);
	neru_badge_color(cr, look->fill);
	cairo_fill_preserve(cr);
	neru_badge_color(cr, look->stroke);
	cairo_set_line_width(cr, look->stroke_width);
	cairo_stroke(cr);
	cairo_restore(cr);

	// Centered on the same integer point drawTextCentered picks.
	double center_x = look->pad + look->width / 2;
	double center_y = look->pad + look->height / 2;
	cairo_text_extents_t extents;
	cairo_save(cr);
	cairo_select_font_face(
	    cr, look->family, CAIRO_FONT_SLANT_NORMAL, look->bold ? CAIRO_FONT_WEIGHT_BOLD : CAIRO_FONT_WEIGHT_NORMAL);
	cairo_set_font_size(cr, look->font_size);
	cairo_text_extents(cr, look->text, &extents);
	neru_badge_color(cr, look->text_color);
	cairo_move_to(
	    cr, center_x - (extents.width / 2.0) - extents.x_bearing,
	    center_y - (extents.height / 2.0) - extents.y_bearing);
	cairo_show_text(cr, look->text);
	cairo_restore(cr);
}
