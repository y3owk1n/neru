#ifndef X11_OVERLAY_H
#define X11_OVERLAY_H

#include "badge_paint.h"

#include <X11/Xlib.h>
#include <cairo/cairo.h>

// NeruX11Badge is an indicator badge in an override-redirect window of its
// own beside the overlay window. A window of its own means moving or hiding it
// leaves whatever the overlay drew under it alone. It is a sibling rather than
// a child, because a child's pixels replace the overlay's instead of being
// blended over them, and moving one exposes the overlay's transparent
// background where it was.
typedef struct {
	Window window;
	cairo_surface_t *surface;
	NeruBadgeLook look;   // what the window holds, family resolved
	NeruBadgeLook asked;  // the look the caller asked for it with
	int x, y;             // the window's position, in device pixels
	int painted;
	int visible;
	int blanked;  // taken off for a capture, to be put back
} NeruX11Badge;

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
	int mapped;  // the overlay window is shown, so badges may be

	NeruX11Badge badges[NERU_BADGE_SLOTS];
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
// could not be made. With a width and height above zero it clears only that
// rectangle, and the rest of the window stays on screen.
cairo_surface_t *neru_x11_overlay_blank(
    NeruX11Overlay *overlay, int x, int y, int width, int height, int *composited, int *failed);
void neru_x11_overlay_unblank(NeruX11Overlay *overlay, cairo_surface_t *saved);
// neru_x11_overlay_composited reports whether a compositing manager owns the
// screen. Without one the server keeps no window's pixels for it, so a badge
// moving off the overlay exposes the overlay's background where it was.
int neru_x11_overlay_composited(NeruX11Overlay *overlay);
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
// neru_x11_overlay_badge shows look in badge slot with its window's top-left
// corner at (x, y), in device pixels, and reports whether it is on screen. It
// paints only when look changed, and shows nothing while the overlay window
// is hidden.
int neru_x11_overlay_badge(NeruX11Overlay *overlay, int slot, int x, int y, const NeruBadgeLook *look);
// neru_x11_overlay_hide_badge takes badge slot off the screen.
void neru_x11_overlay_hide_badge(NeruX11Overlay *overlay, int slot);
// neru_x11_overlay_scale returns the desktop-wide HiDPI UI scale from Xft.dpi
// (clamped to [1.0, 4.0]), or 1.0 when unset. See the implementation for why this
// is global rather than per-monitor on X11.
double neru_x11_overlay_scale(NeruX11Overlay *overlay);

#endif /* X11_OVERLAY_H */
