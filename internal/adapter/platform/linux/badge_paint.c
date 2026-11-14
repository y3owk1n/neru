#include "badge_paint.h"

#include <string.h>

static void neru_badge_unpack(unsigned int color, double *r, double *g, double *b, double *a) {
	*a = ((color >> 24) & 0xFF) / 255.0;
	*r = ((color >> 16) & 0xFF) / 255.0;
	*g = ((color >> 8) & 0xFF) / 255.0;
	*b = (color & 0xFF) / 255.0;
}

void neru_badge_set_fill(cairo_t *cr, double y, double height, unsigned int top, unsigned int bottom) {
	double r, g, b, a;
	neru_badge_unpack(top, &r, &g, &b, &a);
	if (top == bottom || height <= 0) {
		cairo_set_source_rgba(cr, r, g, b, a);
		return;
	}

	cairo_pattern_t *gradient = cairo_pattern_create_linear(0, y, 0, y + height);
	cairo_pattern_add_color_stop_rgba(gradient, 0, r, g, b, a);
	neru_badge_unpack(bottom, &r, &g, &b, &a);
	cairo_pattern_add_color_stop_rgba(gradient, 1, r, g, b, a);
	cairo_pattern_set_extend(gradient, CAIRO_EXTEND_PAD);
	cairo_set_source(cr, gradient);
	cairo_pattern_destroy(gradient);
}

void neru_badge_paint_mask(
    cairo_t *cr, const unsigned char *alpha, int width, int height, double x, double y, unsigned int color) {
	if (!alpha || width <= 0 || height <= 0 || ((color >> 24) & 0xFF) == 0)
		return;

	// An A8 surface pads each row to the stride cairo picks, so the mask is
	// copied in row by row rather than wrapped in place.
	cairo_surface_t *mask = cairo_image_surface_create(CAIRO_FORMAT_A8, width, height);
	if (cairo_surface_status(mask) != CAIRO_STATUS_SUCCESS) {
		cairo_surface_destroy(mask);
		return;
	}

	cairo_surface_flush(mask);
	unsigned char *data = cairo_image_surface_get_data(mask);
	int stride = cairo_image_surface_get_stride(mask);
	for (int row = 0; row < height; row++)
		memcpy(data + (size_t)row * (size_t)stride, alpha + (size_t)row * (size_t)width, (size_t)width);
	cairo_surface_mark_dirty(mask);

	double r, g, b, a;
	neru_badge_unpack(color, &r, &g, &b, &a);
	cairo_save(cr);
	cairo_set_source_rgba(cr, r, g, b, a);
	cairo_mask_surface(cr, mask, x, y);
	cairo_restore(cr);
	cairo_surface_destroy(mask);
}
