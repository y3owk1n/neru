#ifndef OVERLAY_WAYLAND_H
#define OVERLAY_WAYLAND_H

#include "common_defs.h"

#include <cairo/cairo.h>
#include <stddef.h>
#include <wayland-client.h>
#include <xkbcommon/xkbcommon.h>

#define NERU_KEY_RING_CAP 32

typedef struct {
	int x, y, width, height;
	struct wl_output *wl_output;
	struct zxdg_output_v1 *xdg_output;

	struct wl_surface *wl_surface;
	struct zwlr_layer_surface_v1 *layer_surface;
	struct wl_buffer *buffer;

	cairo_surface_t *cairo_surface;
	cairo_t *cr;
	void *shm_data;
	size_t shm_size;
} NeruWaylandOverlayScreen;

typedef struct {
	char keys[NERU_KEY_RING_CAP][256];
	int head;
	int tail;
	int count;
} NeruWaylandKeyRing;

typedef struct {
	struct wl_display *display;
	struct wl_registry *registry;
	struct wl_compositor *compositor;
	struct wl_shm *shm;
	struct zxdg_output_manager_v1 *xdg_output_mgr;
	struct zwlr_layer_shell_v1 *layer_shell;
	struct wl_seat *wl_seat;
	struct wl_keyboard *wl_keyboard;

	struct xkb_context *xkb_ctx;
	struct xkb_state *xkb_state;

	NeruWaylandOverlayScreen screens[NERU_MAX_OUTPUTS];
	int nr_screens;

	int configured;
	int keyboard_interactivity_set;

	int event_fd;
	int running;

	NeruWaylandKeyRing key_ring;
} NeruWaylandOverlay;

NeruWaylandOverlay *neru_wayland_overlay_new(void);
void neru_wayland_overlay_destroy(NeruWaylandOverlay *overlay);
void neru_wayland_overlay_setup_buffers(NeruWaylandOverlay *overlay);
void neru_wayland_overlay_show(NeruWaylandOverlay *overlay);
void neru_wayland_overlay_hide(NeruWaylandOverlay *overlay);
void neru_wayland_overlay_set_keyboard_capture(NeruWaylandOverlay *overlay, int enabled);
void neru_wayland_overlay_clear(NeruWaylandOverlay *overlay);
void neru_wayland_overlay_clear_rect(NeruWaylandOverlay *overlay, double x, double y, double width, double height);
void neru_wayland_overlay_flush(NeruWaylandOverlay *overlay);
void neru_wayland_overlay_rect(
    NeruWaylandOverlay *overlay, double x, double y, double width, double height, unsigned int fill,
    unsigned int stroke, double stroke_width);
void neru_wayland_overlay_text(
    NeruWaylandOverlay *overlay, const char *text, const char *font_family, double x, double y, double font_size,
    unsigned int color);
int neru_wayland_overlay_poll(NeruWaylandOverlay *overlay);
const char *neru_wayland_overlay_get_key(NeruWaylandOverlay *overlay);

#endif /* OVERLAY_WAYLAND_H */
