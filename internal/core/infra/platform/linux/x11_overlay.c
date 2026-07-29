#include "x11_overlay.h"

#include <X11/Xatom.h>
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/extensions/Xfixes.h>
#include <X11/extensions/shape.h>
#include <cairo/cairo-xlib.h>
#include <cairo/cairo.h>
#include <stdlib.h>

static Visual *neru_x11_argb_visual(Display *display, int screen) {
	XVisualInfo vinfo;
	if (XMatchVisualInfo(display, screen, 32, TrueColor, &vinfo)) {
		return vinfo.visual;
	}
	return DefaultVisual(display, screen);
}

NeruX11Overlay *neru_x11_overlay_new(void) {
	Display *display = XOpenDisplay(NULL);
	if (display == NULL) {
		return NULL;
	}

	NeruX11Overlay *overlay = calloc(1, sizeof(NeruX11Overlay));
	if (!overlay) {
		XCloseDisplay(display);
		return NULL;
	}
	overlay->display = display;
	overlay->screen = DefaultScreen(display);
	overlay->root = RootWindow(display, overlay->screen);
	overlay->visual = neru_x11_argb_visual(display, overlay->screen);
	overlay->width = DisplayWidth(display, overlay->screen);
	overlay->height = DisplayHeight(display, overlay->screen);
	overlay->colormap = XCreateColormap(display, overlay->root, overlay->visual, AllocNone);

	XSetWindowAttributes attrs;
	attrs.override_redirect = True;
	attrs.colormap = overlay->colormap;
	attrs.background_pixel = 0;
	attrs.border_pixel = 0;
	attrs.event_mask = ExposureMask;

	overlay->window = XCreateWindow(
	    display, overlay->root, 0, 0, overlay->width, overlay->height, 0, 32, InputOutput, overlay->visual,
	    CWOverrideRedirect | CWColormap | CWBackPixel | CWBorderPixel | CWEventMask, &attrs);

	Atom dock = XInternAtom(display, "_NET_WM_WINDOW_TYPE_DOCK", False);
	Atom window_type = XInternAtom(display, "_NET_WM_WINDOW_TYPE", False);
	XChangeProperty(display, overlay->window, window_type, XA_ATOM, 32, PropModeReplace, (unsigned char *)&dock, 1);

	Atom state = XInternAtom(display, "_NET_WM_STATE", False);
	Atom above = XInternAtom(display, "_NET_WM_STATE_ABOVE", False);
	XChangeProperty(display, overlay->window, state, XA_ATOM, 32, PropModeReplace, (unsigned char *)&above, 1);

	XserverRegion region = XFixesCreateRegion(display, NULL, 0);
	XFixesSetWindowShapeRegion(display, overlay->window, ShapeInput, 0, 0, region);
	XFixesDestroyRegion(display, region);

	overlay->surface =
	    cairo_xlib_surface_create(display, overlay->window, overlay->visual, overlay->width, overlay->height);
	overlay->cr = cairo_create(overlay->surface);
	XFlush(display);

	return overlay;
}

void neru_x11_overlay_destroy(NeruX11Overlay *overlay) {
	if (overlay == NULL) {
		return;
	}
	if (overlay->cr != NULL) {
		cairo_destroy(overlay->cr);
	}
	if (overlay->surface != NULL) {
		cairo_surface_destroy(overlay->surface);
	}
	if (overlay->window != 0) {
		XDestroyWindow(overlay->display, overlay->window);
	}
	if (overlay->colormap != 0) {
		XFreeColormap(overlay->display, overlay->colormap);
	}
	if (overlay->display != NULL) {
		XCloseDisplay(overlay->display);
	}
	free(overlay);
}

void neru_x11_overlay_show(NeruX11Overlay *overlay) {
	XMapRaised(overlay->display, overlay->window);
	XFlush(overlay->display);
}

void neru_x11_overlay_hide(NeruX11Overlay *overlay) {
	XUnmapWindow(overlay->display, overlay->window);
	XFlush(overlay->display);
}

void neru_x11_overlay_clear(NeruX11Overlay *overlay) {
	cairo_save(overlay->cr);
	cairo_set_operator(overlay->cr, CAIRO_OPERATOR_CLEAR);
	cairo_paint(overlay->cr);
	cairo_restore(overlay->cr);
	cairo_surface_flush(overlay->surface);
	XClearWindow(overlay->display, overlay->window);
	XFlush(overlay->display);
}

void neru_x11_overlay_clear_buffered(NeruX11Overlay *overlay) {
	cairo_save(overlay->cr);
	cairo_set_operator(overlay->cr, CAIRO_OPERATOR_CLEAR);
	cairo_paint(overlay->cr);
	cairo_restore(overlay->cr);
}

void neru_x11_overlay_clear_rect(NeruX11Overlay *overlay, int x, int y, int width, int height) {
	if (overlay == NULL || width <= 0 || height <= 0) {
		return;
	}

	cairo_save(overlay->cr);
	cairo_set_operator(overlay->cr, CAIRO_OPERATOR_CLEAR);
	cairo_rectangle(overlay->cr, x, y, width, height);
	cairo_fill(overlay->cr);
	cairo_restore(overlay->cr);
	cairo_surface_flush(overlay->surface);
	XClearArea(overlay->display, overlay->window, x, y, (unsigned int)width, (unsigned int)height, False);
	XFlush(overlay->display);
}

void neru_x11_overlay_resize(NeruX11Overlay *overlay) {
	int width = DisplayWidth(overlay->display, overlay->screen);
	int height = DisplayHeight(overlay->display, overlay->screen);
	if (width == overlay->width && height == overlay->height) {
		return;
	}
	overlay->width = width;
	overlay->height = height;
	XResizeWindow(overlay->display, overlay->window, width, height);
	cairo_xlib_surface_set_size(overlay->surface, width, height);
	XFlush(overlay->display);
}

static void neru_x11_overlay_color(cairo_t *cr, unsigned int color) {
	double a = ((color >> 24) & 0xFF) / 255.0;
	double r = ((color >> 16) & 0xFF) / 255.0;
	double g = ((color >> 8) & 0xFF) / 255.0;
	double b = (color & 0xFF) / 255.0;
	cairo_set_source_rgba(cr, r, g, b, a);
}

void neru_x11_overlay_rect(
    NeruX11Overlay *overlay, double x, double y, double width, double height, unsigned int fill, unsigned int stroke,
    double stroke_width) {
	cairo_t *cr = overlay->cr;
	cairo_save(cr);
	cairo_rectangle(cr, x, y, width, height);
	neru_x11_overlay_color(cr, fill);
	cairo_fill_preserve(cr);
	neru_x11_overlay_color(cr, stroke);
	cairo_set_line_width(cr, stroke_width);
	cairo_stroke(cr);
	cairo_restore(cr);
}

static void neru_x11_overlay_rounded_path(cairo_t *cr, double x, double y, double width, double height, double radius) {
	double max_radius = (width < height ? width : height) / 2.0;
	if (radius > max_radius)
		radius = max_radius;
	if (radius <= 0) {
		cairo_rectangle(cr, x, y, width, height);
		return;
	}

	const double deg = 0.0174532925199432957692;
	cairo_new_sub_path(cr);
	cairo_arc(cr, x + width - radius, y + radius, radius, -90.0 * deg, 0.0 * deg);
	cairo_arc(cr, x + width - radius, y + height - radius, radius, 0.0 * deg, 90.0 * deg);
	cairo_arc(cr, x + radius, y + height - radius, radius, 90.0 * deg, 180.0 * deg);
	cairo_arc(cr, x + radius, y + radius, radius, 180.0 * deg, 270.0 * deg);
	cairo_close_path(cr);
}

void neru_x11_overlay_rounded_rect(
    NeruX11Overlay *overlay, double x, double y, double width, double height, double radius, unsigned int fill,
    unsigned int stroke, double stroke_width) {
	cairo_t *cr = overlay->cr;
	cairo_save(cr);
	neru_x11_overlay_rounded_path(cr, x, y, width, height, radius);
	neru_x11_overlay_color(cr, fill);
	cairo_fill_preserve(cr);
	neru_x11_overlay_color(cr, stroke);
	cairo_set_line_width(cr, stroke_width);
	cairo_stroke(cr);
	cairo_restore(cr);
}

// neru_x11_hint_badge_path builds a rounded-rectangle path with an optional
// triangular tail merged into one edge, as a single closed outline. edge: 0 =
// no tail, 1 = tail on the top edge (apex above), 2 = tail on the bottom edge
// (apex below). The tail base (a_left..a_right) is clamped to the edge's flat
// span so it never runs onto a rounded corner; if no room remains the tail is
// dropped. Building badge and tail as one path lets the caller fill and stroke
// once, avoiding the double-composited seam a separate overlaid triangle causes
// with translucent colors.
static void neru_x11_hint_badge_path(
    cairo_t *cr, double x, double y, double w, double h, double radius, int edge, double a_left, double a_right,
    double tip_x, double tip_y) {
	double r = radius;
	double max_r = (w < h ? w : h) / 2.0;
	if (r > max_r)
		r = max_r;
	if (r < 0)
		r = 0;

	double flat_left = x + r;
	double flat_right = x + w - r;
	if (edge != 0) {
		if (a_left < flat_left)
			a_left = flat_left;
		if (a_right > flat_right)
			a_right = flat_right;
		if (a_left >= a_right)
			edge = 0;
	}

	const double deg = 0.0174532925199432957692;
	cairo_new_sub_path(cr);
	cairo_move_to(cr, flat_left, y);
	if (edge == 1) {
		cairo_line_to(cr, a_left, y);
		cairo_line_to(cr, tip_x, tip_y);
		cairo_line_to(cr, a_right, y);
	}
	cairo_line_to(cr, flat_right, y);
	if (r > 0)
		cairo_arc(cr, x + w - r, y + r, r, -90.0 * deg, 0.0 * deg);
	cairo_line_to(cr, x + w, y + h - r);
	if (r > 0)
		cairo_arc(cr, x + w - r, y + h - r, r, 0.0 * deg, 90.0 * deg);
	if (edge == 2) {
		cairo_line_to(cr, a_right, y + h);
		cairo_line_to(cr, tip_x, tip_y);
		cairo_line_to(cr, a_left, y + h);
	}
	cairo_line_to(cr, flat_left, y + h);
	if (r > 0)
		cairo_arc(cr, x + r, y + h - r, r, 90.0 * deg, 180.0 * deg);
	cairo_line_to(cr, x, y + r);
	if (r > 0)
		cairo_arc(cr, x + r, y + r, r, 180.0 * deg, 270.0 * deg);
	cairo_close_path(cr);
}

// neru_x11_overlay_hint_badge fills and strokes a hint badge with an optional
// connector tail as one continuous outline (see neru_x11_hint_badge_path).
void neru_x11_overlay_hint_badge(
    NeruX11Overlay *overlay, double x, double y, double width, double height, double radius, int edge, double a_left,
    double a_right, double tip_x, double tip_y, unsigned int fill, unsigned int stroke, double stroke_width) {
	cairo_t *cr = overlay->cr;
	cairo_save(cr);
	neru_x11_hint_badge_path(cr, x, y, width, height, radius, edge, a_left, a_right, tip_x, tip_y);
	neru_x11_overlay_color(cr, fill);
	cairo_fill_preserve(cr);
	neru_x11_overlay_color(cr, stroke);
	cairo_set_line_width(cr, stroke_width);
	cairo_stroke(cr);
	cairo_restore(cr);
}

void neru_x11_overlay_text(
    NeruX11Overlay *overlay, const char *text, const char *font_family, double x, double y, double font_size,
    unsigned int color) {
	cairo_t *cr = overlay->cr;
	cairo_text_extents_t extents;
	cairo_save(cr);
	cairo_select_font_face(cr, font_family, CAIRO_FONT_SLANT_NORMAL, CAIRO_FONT_WEIGHT_BOLD);
	cairo_set_font_size(cr, font_size);
	cairo_text_extents(cr, text, &extents);
	neru_x11_overlay_color(cr, color);
	cairo_move_to(cr, x - (extents.width / 2.0) - extents.x_bearing, y - (extents.height / 2.0) - extents.y_bearing);
	cairo_show_text(cr, text);
	cairo_restore(cr);
}

void neru_x11_overlay_flush(NeruX11Overlay *overlay) {
	cairo_surface_flush(overlay->surface);
	XFlush(overlay->display);
}
