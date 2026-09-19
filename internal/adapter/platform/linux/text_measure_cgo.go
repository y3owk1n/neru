//go:build linux && cgo

package linux

/*
#include <cairo/cairo.h>
#include <stdlib.h>

// Measures one line of text with cairo's toy text API, the one both overlay
// backends draw with (neru_x11_overlay_text, neru_wayland_overlay_text), on a
// throwaway image surface. It needs no display connection, so X11 and Wayland
// share it, and a font cairo cannot load poisons only this context. Returns 1
// on success and 0 when cairo could not measure with that family.
static int neru_text_measure_once(
    const char *text, const char *family, double size, int bold, double *out_width, double *out_height) {
	cairo_surface_t *surface = cairo_image_surface_create(CAIRO_FORMAT_ARGB32, 1, 1);
	cairo_t *cr = cairo_create(surface);
	cairo_text_extents_t text_extents;
	cairo_font_extents_t font_extents;

	cairo_select_font_face(
	    cr, family, CAIRO_FONT_SLANT_NORMAL, bold ? CAIRO_FONT_WEIGHT_BOLD : CAIRO_FONT_WEIGHT_NORMAL);
	cairo_set_font_size(cr, size);
	cairo_text_extents(cr, text, &text_extents);
	cairo_font_extents(cr, &font_extents);

	int ok = cairo_status(cr) == CAIRO_STATUS_SUCCESS;
	if (ok) {
		*out_width = text_extents.x_advance;
		*out_height = font_extents.ascent + font_extents.descent;
	}

	cairo_destroy(cr);
	cairo_surface_destroy(surface);

	return ok;
}

// neru_text_measure measures with the family asked for and, when cairo cannot
// load it, with the generic the Wayland draw path substitutes in that case
// (neru_resolve_font_family).
static int neru_text_measure(
    const char *text, const char *family, double size, int bold, double *out_width, double *out_height) {
	if (size <= 0)
		return 0;

	if (family && family[0] != '\0' && neru_text_measure_once(text, family, size, bold, out_width, out_height))
		return 1;

	return neru_text_measure_once(text, "sans-serif", size, bold, out_width, out_height);
}
*/
import "C"

import (
	"unsafe"

	"github.com/y3owk1n/neru/internal/adapter/platform/fontcache"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/ports"
)

// NewTextMeasurer returns a cairo-backed ports.TextMeasurer. Each string is
// measured on first use and remembered for the lifetime of the process.
func NewTextMeasurer() ports.TextMeasurer {
	return fontcache.NewMeasurer(measureText)
}

// measureText asks cairo on a context of its own, so it is safe on any
// goroutine and never touches a live overlay surface.
func measureText(text, family string, size float64, bold bool) (ports.TextMetrics, error) {
	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))

	cFamily := C.CString(family)
	defer C.free(unsafe.Pointer(cFamily))

	cBold := C.int(0)
	if bold {
		cBold = 1
	}

	var width, height C.double
	if C.neru_text_measure(cText, cFamily, C.double(size), cBold, &width, &height) == 0 {
		return ports.TextMetrics{}, derrors.New(
			derrors.CodeInternal,
			"cairo could not measure the text",
		)
	}

	return ports.TextMetrics{Width: float64(width), Height: float64(height)}, nil
}
