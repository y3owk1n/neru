#include "label_text.h"

#include <stdlib.h>
#include <string.h>

static void neru_label_color(cairo_t *cr, unsigned int color) {
	cairo_set_source_rgba(
	    cr, ((color >> 16) & 0xFF) / 255.0, ((color >> 8) & 0xFF) / 255.0, (color & 0xFF) / 255.0,
	    ((color >> 24) & 0xFF) / 255.0);
}

void neru_label_show(cairo_t *cr, const char *text, int matched, unsigned int color, unsigned int matched_color) {
	size_t length = strlen(text);
	if (matched <= 0 || (size_t)matched > length) {
		neru_label_color(cr, color);
		cairo_show_text(cr, text);
		return;
	}

	// cairo_show_text advances the current point past what it drew, so the
	// rest of the label starts where the typed part ends.
	char *typed = strndup(text, (size_t)matched);
	if (typed) {
		neru_label_color(cr, matched_color);
		cairo_show_text(cr, typed);
		free(typed);
	}

	neru_label_color(cr, color);
	cairo_show_text(cr, text + matched);
}
