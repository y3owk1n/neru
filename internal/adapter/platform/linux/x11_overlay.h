#ifndef X11_OVERLAY_H
#define X11_OVERLAY_H

#include <X11/Xlib.h>
#include <cairo/cairo.h>

typedef struct {
	Display *display;
	int screen;
	Window root;
	Window window;
	Visual *visual;
	Colormap colormap;
	cairo_surface_t *surface;
	cairo_t *cr;
	int width;
	int height;
} NeruX11Overlay;

NeruX11Overlay *neru_x11_overlay_new(void);
void neru_x11_overlay_destroy(NeruX11Overlay *overlay);
void neru_x11_overlay_show(NeruX11Overlay *overlay);
void neru_x11_overlay_hide(NeruX11Overlay *overlay);
void neru_x11_overlay_clear(NeruX11Overlay *overlay);
// neru_x11_overlay_blank copies what the overlay window shows into a pixmap of
// its own, clears the window and syncs, so the server has the clear before a
// capture on another connection asks. It sets *composited when a compositing
// manager runs, whose repaint a capture has to wait for. It returns the copy
// for neru_x11_overlay_unblank, or NULL when the window is not on screen. It
// also returns NULL, with *failed set and the window untouched, when the copy
// could not be made.
cairo_surface_t *neru_x11_overlay_blank(NeruX11Overlay *overlay, int *composited, int *failed);
void neru_x11_overlay_unblank(NeruX11Overlay *overlay, cairo_surface_t *saved);
void neru_x11_overlay_clear_buffered(NeruX11Overlay *overlay);
void neru_x11_overlay_clear_rect(NeruX11Overlay *overlay, int x, int y, int width, int height);
void neru_x11_overlay_resize(NeruX11Overlay *overlay);
void neru_x11_overlay_rect(
    NeruX11Overlay *overlay, double x, double y, double width, double height, unsigned int fill, unsigned int stroke,
    double stroke_width);
void neru_x11_overlay_rounded_rect(
    NeruX11Overlay *overlay, double x, double y, double width, double height, double radius, unsigned int fill,
    unsigned int stroke, double stroke_width);
void neru_x11_overlay_hint_badge(
    NeruX11Overlay *overlay, double x, double y, double width, double height, double radius, int edge, double a_left,
    double a_right, double tip_x, double tip_y, unsigned int fill, unsigned int stroke, double stroke_width);
void neru_x11_overlay_text(
    NeruX11Overlay *overlay, const char *text, const char *font_family, double x, double y, double font_size,
    unsigned int color, int bold);
void neru_x11_overlay_flush(NeruX11Overlay *overlay);
// neru_x11_overlay_scale returns the desktop-wide HiDPI UI scale from Xft.dpi
// (clamped to [1.0, 4.0]), or 1.0 when unset. See the implementation for why this
// is global rather than per-monitor on X11.
double neru_x11_overlay_scale(NeruX11Overlay *overlay);

#endif /* X11_OVERLAY_H */
