#include "overlay_wayland.h"

#include "wlr_protocol/layer-shell.h"
#include "wlr_protocol/xdg-output.h"
#include "wlr_protocol/xdg-shell.h"

#include <cairo/cairo.h>
#include <errno.h>
#include <fcntl.h>
#include <poll.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <unistd.h>
#include <wayland-client.h>
#include <xkbcommon/xkbcommon-keysyms.h>
#include <xkbcommon/xkbcommon.h>

// Keyboard event ring buffer.
// Thread safety: all accesses happen while the Go-side displayMu mutex is
// held (shared with renderMu). The Wayland keyboard callback
// (neru_keyboard_key) fires inside wl_display_dispatch / wl_display_roundtrip,
// which only run while displayMu is held. The consumer
// (neru_wayland_overlay_get_key) is called from the keyboard poller goroutine
// which also holds displayMu. Therefore no concurrent access can occur.
//
// A ring buffer (rather than a single slot) is necessary because
// wl_display_roundtrip — called from the rendering path — may dispatch
// multiple keyboard events in a single call. With a single-slot buffer the
// second event would silently overwrite the first.
static void neru_key_ring_push(NeruWaylandOverlay *overlay, const char *key) {
	if (!key || key[0] == '\0' || overlay->key_ring.count >= NERU_KEY_RING_CAP)
		return;

	snprintf(overlay->key_ring.keys[overlay->key_ring.head], sizeof(overlay->key_ring.keys[0]), "%s", key);
	overlay->key_ring.head = (overlay->key_ring.head + 1) % NERU_KEY_RING_CAP;
	overlay->key_ring.count++;
}

static const char *neru_modifier_name_from_keysym(xkb_keysym_t keysym) {
	switch (keysym) {
	case XKB_KEY_Shift_L:
	case XKB_KEY_Shift_R:
		return "shift";
	case XKB_KEY_Control_L:
	case XKB_KEY_Control_R:
		return "ctrl";
	case XKB_KEY_Alt_L:
	case XKB_KEY_Alt_R:
		return "alt";
	case XKB_KEY_Super_L:
	case XKB_KEY_Super_R:
	case XKB_KEY_Meta_L:
	case XKB_KEY_Meta_R:
		return "cmd";
	default:
		return NULL;
	}
}

// Create anonymous shared memory
static int create_shm_file(off_t size) {
	int ret, fd;

#ifdef __NR_memfd_create
	fd = syscall(__NR_memfd_create, "neru-overlay-shm", 0);
	if (fd >= 0) {
		do {
			ret = ftruncate(fd, size);
		} while (ret < 0 && errno == EINTR);
		if (ret >= 0)
			return fd;
		close(fd);
	}
#endif

	// Fallback if no memfd
	char name[] = "/tmp/neru-shm-XXXXXX";
	fd = mkstemp(name);
	if (fd < 0)
		return -1;
	unlink(name);
	do {
		ret = ftruncate(fd, size);
	} while (ret < 0 && errno == EINTR);
	if (ret < 0) {
		close(fd);
		return -1;
	}
	return fd;
}

static void neru_layer_surface_configure(
    void *data, struct zwlr_layer_surface_v1 *layer_surface, uint32_t serial, uint32_t width, uint32_t height) {
	NeruWaylandOverlay *overlay = (NeruWaylandOverlay *)data;
	zwlr_layer_surface_v1_ack_configure(layer_surface, serial);

	for (int i = 0; i < overlay->nr_screens; i++) {
		if (overlay->screens[i].layer_surface == layer_surface) {
			if (width > 0)
				overlay->screens[i].width = width;
			if (height > 0)
				overlay->screens[i].height = height;
			break;
		}
	}

	overlay->configured = 1;
}

static void neru_layer_surface_closed(void *data, struct zwlr_layer_surface_v1 *layer_surface) {
	// No-op
}

static const struct zwlr_layer_surface_v1_listener layer_surface_listener = {
    .configure = neru_layer_surface_configure,
    .closed = neru_layer_surface_closed,
};

static void neru_overlay_registry_global(
    void *data, struct wl_registry *registry, uint32_t name, const char *interface, uint32_t version) {
	NeruWaylandOverlay *overlay = (NeruWaylandOverlay *)data;

	if (strcmp(interface, "wl_compositor") == 0) {
		overlay->compositor = wl_registry_bind(registry, name, &wl_compositor_interface, 4);
	} else if (strcmp(interface, "wl_shm") == 0) {
		overlay->shm = wl_registry_bind(registry, name, &wl_shm_interface, 1);
	} else if (strcmp(interface, "zwlr_layer_shell_v1") == 0) {
		overlay->layer_shell = wl_registry_bind(registry, name, &zwlr_layer_shell_v1_interface, 1);
	} else if (strcmp(interface, "wl_output") == 0) {
		if (overlay->nr_screens < NERU_MAX_OUTPUTS) {
			overlay->screens[overlay->nr_screens].wl_output =
			    wl_registry_bind(registry, name, &wl_output_interface, 3 < version ? 3 : version);
			overlay->nr_screens++;
		}
	} else if (strcmp(interface, "zxdg_output_manager_v1") == 0) {
		overlay->xdg_output_mgr =
		    wl_registry_bind(registry, name, &zxdg_output_manager_v1_interface, 3 < version ? 3 : version);
	} else if (strcmp(interface, "wl_seat") == 0) {
		overlay->wl_seat = wl_registry_bind(registry, name, &wl_seat_interface, 5);
	}
}

static void neru_overlay_registry_global_remove(void *data, struct wl_registry *registry, uint32_t name) {
	// No-op
}

static const struct wl_registry_listener overlay_registry_listener = {
    .global = neru_overlay_registry_global,
    .global_remove = neru_overlay_registry_global_remove,
};

static void neru_xdg_output_logical_position(void *data, struct zxdg_output_v1 *xdg_output, int32_t x, int32_t y) {
	NeruWaylandOverlayScreen *scr = (NeruWaylandOverlayScreen *)data;
	scr->x = x;
	scr->y = y;
}

static void neru_xdg_output_logical_size(void *data, struct zxdg_output_v1 *xdg_output, int32_t w, int32_t h) {
	NeruWaylandOverlayScreen *scr = (NeruWaylandOverlayScreen *)data;
	scr->width = w;
	scr->height = h;
}

static void neru_xdg_output_done(void *data, struct zxdg_output_v1 *xdg_output) {}

static void neru_xdg_output_name(void *data, struct zxdg_output_v1 *xdg_output, const char *name) {}

static void neru_xdg_output_description(void *data, struct zxdg_output_v1 *xdg_output, const char *description) {}

static const struct zxdg_output_v1_listener xdg_output_listener = {
    .logical_position = neru_xdg_output_logical_position,
    .logical_size = neru_xdg_output_logical_size,
    .done = neru_xdg_output_done,
    .name = neru_xdg_output_name,
    .description = neru_xdg_output_description,
};

// Wayland keyboard listener for key events
static void neru_keyboard_keymap(void *data, struct wl_keyboard *keyboard, uint32_t format, int32_t fd, uint32_t size) {
	NeruWaylandOverlay *overlay = (NeruWaylandOverlay *)data;
	if (format == WL_KEYBOARD_KEYMAP_FORMAT_XKB_V1) {
		char *map = mmap(NULL, size, PROT_READ, MAP_PRIVATE, fd, 0);
		if (map != MAP_FAILED) {
			if (overlay->xkb_ctx)
				xkb_context_unref(overlay->xkb_ctx);
			overlay->xkb_ctx = xkb_context_new(XKB_CONTEXT_NO_FLAGS);
			if (overlay->xkb_ctx) {
				struct xkb_keymap *keymap = xkb_keymap_new_from_string(
				    overlay->xkb_ctx, map, XKB_KEYMAP_FORMAT_TEXT_V1, XKB_KEYMAP_COMPILE_NO_FLAGS);
				if (keymap) {
					if (overlay->xkb_state)
						xkb_state_unref(overlay->xkb_state);
					overlay->xkb_state = xkb_state_new(keymap);
					xkb_keymap_unref(keymap);
				}
			}
			munmap(map, size);
		}
		close(fd);
	}
}

static void neru_keyboard_enter(
    void *data, struct wl_keyboard *keyboard, uint32_t serial, struct wl_surface *surface, struct wl_array *keys) {}

static void neru_keyboard_leave(void *data, struct wl_keyboard *keyboard, uint32_t serial, struct wl_surface *surface) {
}

// Returns a normalized base key name in buf/utf8_buf, or NULL when unknown.
static const char *neru_keyboard_base_key(
    NeruWaylandOverlay *overlay, uint32_t key, xkb_keysym_t keysym, char *buf, size_t buf_size, char *utf8_buf,
    size_t utf8_size) {
	memset(buf, 0, buf_size);
	memset(utf8_buf, 0, utf8_size);
	xkb_keysym_get_name(keysym, buf, buf_size);
	xkb_state_key_get_utf8(overlay->xkb_state, key + 8, utf8_buf, utf8_size);

	char *final_key = buf;

	if (utf8_buf[0] != '\0' && utf8_buf[1] == '\0' && utf8_buf[0] > 32 && utf8_buf[0] <= 126) {
		final_key = utf8_buf;
		if (final_key[0] >= 'A' && final_key[0] <= 'Z') {
			final_key[0] = final_key[0] + 32;
		}
	} else if (buf[0]) {
		for (size_t i = 0; buf[i]; i++) {
			if (buf[i] >= 'A' && buf[i] <= 'Z') {
				buf[i] = buf[i] + 32;
			}
		}
	}

	if (!final_key[0]) {
		return NULL;
	}

	return final_key;
}

static void neru_keyboard_key(
    void *data, struct wl_keyboard *keyboard, uint32_t serial, uint32_t time, uint32_t key, uint32_t state) {
	NeruWaylandOverlay *overlay = (NeruWaylandOverlay *)data;
	if (!overlay->xkb_state)
		return;

	xkb_keysym_t keysym = xkb_state_key_get_one_sym(overlay->xkb_state, key + 8);
	const char *modifier_name = neru_modifier_name_from_keysym(keysym);
	if (modifier_name) {
		char modifier_key[64] = {0};
		snprintf(
		    modifier_key, sizeof(modifier_key), "__modifier_%s_%s", modifier_name,
		    state == WL_KEYBOARD_KEY_STATE_PRESSED ? "down" : "up");
		neru_key_ring_push(overlay, modifier_key);

		return;
	}

	char buf[64] = {0};
	char utf8_buf[64] = {0};
	const char *final_key = neru_keyboard_base_key(overlay, key, keysym, buf, sizeof(buf), utf8_buf, sizeof(utf8_buf));
	if (!final_key) {
		return;
	}

	if (state == WL_KEYBOARD_KEY_STATE_RELEASED) {
		char release_key[128] = {0};
		snprintf(release_key, sizeof(release_key), "__keyup_%s", final_key);
		neru_key_ring_push(overlay, release_key);

		return;
	}

	if (state != WL_KEYBOARD_KEY_STATE_PRESSED) {
		return;
	}

	char mod_prefix[64] = "";
	if (xkb_state_mod_name_is_active(overlay->xkb_state, XKB_MOD_NAME_SHIFT, XKB_STATE_MODS_EFFECTIVE) > 0) {
		strcat(mod_prefix, "Shift+");
	}
	if (xkb_state_mod_name_is_active(overlay->xkb_state, XKB_MOD_NAME_CTRL, XKB_STATE_MODS_EFFECTIVE) > 0) {
		strcat(mod_prefix, "Ctrl+");
	}
	if (xkb_state_mod_name_is_active(overlay->xkb_state, XKB_MOD_NAME_ALT, XKB_STATE_MODS_EFFECTIVE) > 0) {
		strcat(mod_prefix, "Alt+");
	}
	if (xkb_state_mod_name_is_active(overlay->xkb_state, XKB_MOD_NAME_LOGO, XKB_STATE_MODS_EFFECTIVE) > 0) {
		strcat(mod_prefix, "Cmd+");
	}

	char full_key[128] = {0};
	snprintf(full_key, sizeof(full_key), "%s%s", mod_prefix, final_key);
	neru_key_ring_push(overlay, full_key);
}

static void neru_keyboard_modifiers(
    void *data, struct wl_keyboard *keyboard, uint32_t serial, uint32_t mods_depressed, uint32_t mods_latched,
    uint32_t mods_locked, uint32_t group) {
	NeruWaylandOverlay *overlay = (NeruWaylandOverlay *)data;
	if (overlay->xkb_state) {
		xkb_state_update_mask(overlay->xkb_state, mods_depressed, mods_latched, mods_locked, 0, 0, group);
	}
}

static void neru_keyboard_repeat_info(void *data, struct wl_keyboard *keyboard, int32_t rate, int32_t delay) {}

static const struct wl_keyboard_listener keyboard_listener = {
    .keymap = neru_keyboard_keymap,
    .enter = neru_keyboard_enter,
    .leave = neru_keyboard_leave,
    .key = neru_keyboard_key,
    .modifiers = neru_keyboard_modifiers,
    .repeat_info = neru_keyboard_repeat_info,
};

// Seat listener to detect keyboard
static void neru_seat_capabilities(void *data, struct wl_seat *seat, uint32_t capabilities) {
	NeruWaylandOverlay *overlay = (NeruWaylandOverlay *)data;
	if (capabilities & WL_SEAT_CAPABILITY_KEYBOARD) {
		if (!overlay->wl_keyboard) {
			overlay->wl_keyboard = wl_seat_get_keyboard(seat);
			if (overlay->wl_keyboard) {
				wl_keyboard_add_listener(overlay->wl_keyboard, &keyboard_listener, overlay);
			}
		}
	}
}

static void neru_seat_name(void *data, struct wl_seat *seat, const char *name) {}

static const struct wl_seat_listener seat_listener = {
    .capabilities = neru_seat_capabilities,
    .name = neru_seat_name,
};

NeruWaylandOverlay *neru_wayland_overlay_new(void) {
	NeruWaylandOverlay *overlay = calloc(1, sizeof(NeruWaylandOverlay));
	if (!overlay)
		return NULL;

	// EXCLUSIVE by default for keyboard capture fallback
	// SetKeyboardCaptureEnabled can change it to NONE when not needed
	overlay->keyboard_interactivity_set = ZWLR_LAYER_SURFACE_V1_KEYBOARD_INTERACTIVITY_EXCLUSIVE;

	overlay->display = wl_display_connect(NULL);
	if (!overlay->display) {
		free(overlay);
		return NULL;
	}

	overlay->registry = wl_display_get_registry(overlay->display);
	wl_registry_add_listener(overlay->registry, &overlay_registry_listener, overlay);
	wl_display_roundtrip(overlay->display);  // get globals

	if (!overlay->compositor || !overlay->layer_shell || !overlay->shm || !overlay->xdg_output_mgr) {
		wl_display_disconnect(overlay->display);
		free(overlay);
		return NULL;
	}

	for (int i = 0; i < overlay->nr_screens; i++) {
		NeruWaylandOverlayScreen *scr = &overlay->screens[i];
		scr->xdg_output = zxdg_output_manager_v1_get_xdg_output(overlay->xdg_output_mgr, scr->wl_output);
		zxdg_output_v1_add_listener(scr->xdg_output, &xdg_output_listener, scr);
	}
	wl_display_roundtrip(overlay->display);  // get screen sizes

	// Setup seat listener for keyboard
	if (overlay->wl_seat) {
		wl_seat_add_listener(overlay->wl_seat, &seat_listener, overlay);
		wl_display_roundtrip(overlay->display);
	}

	// Setup xkb context
	overlay->xkb_ctx = xkb_context_new(XKB_CONTEXT_NO_FLAGS);

	// Try to get keyboard immediately and set up listener
	if (overlay->wl_seat) {
		struct wl_keyboard *kb = wl_seat_get_keyboard(overlay->wl_seat);
		if (kb) {
			wl_keyboard_add_listener(kb, &keyboard_listener, overlay);
		}
	}

	return overlay;
}

void neru_wayland_overlay_destroy(NeruWaylandOverlay *overlay) {
	if (!overlay)
		return;

	for (int i = 0; i < overlay->nr_screens; i++) {
		NeruWaylandOverlayScreen *scr = &overlay->screens[i];
		if (scr->cr)
			cairo_destroy(scr->cr);
		if (scr->cairo_surface)
			cairo_surface_destroy(scr->cairo_surface);
		if (scr->buffer)
			wl_buffer_destroy(scr->buffer);
		if (scr->shm_data)
			munmap(scr->shm_data, scr->shm_size);
		if (scr->layer_surface)
			zwlr_layer_surface_v1_destroy(scr->layer_surface);
		if (scr->wl_surface)
			wl_surface_destroy(scr->wl_surface);
		if (scr->xdg_output)
			zxdg_output_v1_destroy(scr->xdg_output);
	}

	if (overlay->xdg_output_mgr)
		zxdg_output_manager_v1_destroy(overlay->xdg_output_mgr);
	if (overlay->layer_shell)
		zwlr_layer_shell_v1_destroy(overlay->layer_shell);
	if (overlay->registry)
		wl_registry_destroy(overlay->registry);
	if (overlay->display)
		wl_display_disconnect(overlay->display);
	free(overlay);
}

void neru_wayland_overlay_setup_buffers(NeruWaylandOverlay *overlay) {
	for (int i = 0; i < overlay->nr_screens; i++) {
		NeruWaylandOverlayScreen *scr = &overlay->screens[i];

		if (scr->layer_surface)
			continue;  // Already configured

		// Skip if dimensions aren't set yet
		if (scr->width <= 0 || scr->height <= 0)
			continue;

		scr->wl_surface = wl_compositor_create_surface(overlay->compositor);

		// We want all pointer clicks to PASS THROUGH the overlay to the window behind it.
		// On Wayland, a surface intercepts all events across its entire dimension unless
		// an input region is explicitly provided. We set an empty region here:
		struct wl_region *empty_region = wl_compositor_create_region(overlay->compositor);
		wl_surface_set_input_region(scr->wl_surface, empty_region);
		wl_region_destroy(empty_region);

		scr->layer_surface = zwlr_layer_shell_v1_get_layer_surface(
		    overlay->layer_shell, scr->wl_surface, scr->wl_output, ZWLR_LAYER_SHELL_V1_LAYER_OVERLAY, "neru");

		zwlr_layer_surface_v1_set_size(scr->layer_surface, scr->width, scr->height);
		zwlr_layer_surface_v1_set_anchor(
		    scr->layer_surface, ZWLR_LAYER_SURFACE_V1_ANCHOR_TOP | ZWLR_LAYER_SURFACE_V1_ANCHOR_LEFT |
		                            ZWLR_LAYER_SURFACE_V1_ANCHOR_RIGHT | ZWLR_LAYER_SURFACE_V1_ANCHOR_BOTTOM);
		zwlr_layer_surface_v1_set_exclusive_zone(scr->layer_surface, -1);

		// Request exclusive keyboard interactivity when overlay is shown
		// This tells the compositor to send keyboard events to this surface
		zwlr_layer_surface_v1_set_keyboard_interactivity(scr->layer_surface, overlay->keyboard_interactivity_set);

		zwlr_layer_surface_v1_add_listener(scr->layer_surface, &layer_surface_listener, overlay);
		wl_surface_commit(scr->wl_surface);
	}

	// Wait for configure events
	wl_display_roundtrip(overlay->display);

	for (int i = 0; i < overlay->nr_screens; i++) {
		NeruWaylandOverlayScreen *scr = &overlay->screens[i];
		if (scr->buffer)
			continue;

		size_t stride = ((size_t)scr->width) * 4u;
		scr->shm_size = stride * (size_t)scr->height;
		int fd = create_shm_file(scr->shm_size);
		if (fd < 0)
			continue;

		scr->shm_data = mmap(NULL, scr->shm_size, PROT_READ | PROT_WRITE, MAP_SHARED, fd, 0);
		struct wl_shm_pool *pool = wl_shm_create_pool(overlay->shm, fd, (int)scr->shm_size);
		scr->buffer = wl_shm_pool_create_buffer(pool, 0, scr->width, scr->height, (int)stride, WL_SHM_FORMAT_ARGB8888);
		wl_shm_pool_destroy(pool);
		close(fd);

		scr->cairo_surface = cairo_image_surface_create_for_data(
		    scr->shm_data, CAIRO_FORMAT_ARGB32, scr->width, scr->height, (int)stride);
		scr->cr = cairo_create(scr->cairo_surface);
	}
}

void neru_wayland_overlay_show(NeruWaylandOverlay *overlay) {
	if (!overlay)
		return;
	for (int i = 0; i < overlay->nr_screens; i++) {
		NeruWaylandOverlayScreen *scr = &overlay->screens[i];
		if (scr->wl_surface && scr->buffer) {
			wl_surface_attach(scr->wl_surface, scr->buffer, 0, 0);
			wl_surface_damage_buffer(scr->wl_surface, 0, 0, INT32_MAX, INT32_MAX);
			wl_surface_commit(scr->wl_surface);
		}
	}
	wl_display_flush(overlay->display);
}

void neru_wayland_overlay_hide(NeruWaylandOverlay *overlay) {
	if (!overlay)
		return;
	for (int i = 0; i < overlay->nr_screens; i++) {
		NeruWaylandOverlayScreen *scr = &overlay->screens[i];
		if (scr->wl_surface) {
			wl_surface_attach(scr->wl_surface, NULL, 0, 0);
			wl_surface_commit(scr->wl_surface);
		}
		// Destroy layer surface to allow proper recreation on next show
		if (scr->layer_surface) {
			zwlr_layer_surface_v1_destroy(scr->layer_surface);
			scr->layer_surface = NULL;
		}
		// Also destroy the surface
		if (scr->wl_surface) {
			wl_surface_destroy(scr->wl_surface);
			scr->wl_surface = NULL;
		}
		// Destroy buffer and cairo
		if (scr->buffer) {
			wl_buffer_destroy(scr->buffer);
			scr->buffer = NULL;
		}
		if (scr->cairo_surface) {
			cairo_surface_destroy(scr->cairo_surface);
			scr->cairo_surface = NULL;
		}
		if (scr->cr) {
			cairo_destroy(scr->cr);
			scr->cr = NULL;
		}
		if (scr->shm_data) {
			munmap(scr->shm_data, scr->shm_size);
			scr->shm_data = NULL;
		}
	}
	wl_display_flush(overlay->display);
}

void neru_wayland_overlay_set_keyboard_capture(NeruWaylandOverlay *overlay, int enabled) {
	if (!overlay)
		return;

	overlay->keyboard_interactivity_set = enabled ? ZWLR_LAYER_SURFACE_V1_KEYBOARD_INTERACTIVITY_EXCLUSIVE
	                                              : ZWLR_LAYER_SURFACE_V1_KEYBOARD_INTERACTIVITY_NONE;

	for (int i = 0; i < overlay->nr_screens; i++) {
		NeruWaylandOverlayScreen *scr = &overlay->screens[i];
		if (!scr->layer_surface || !scr->wl_surface)
			continue;

		zwlr_layer_surface_v1_set_keyboard_interactivity(scr->layer_surface, overlay->keyboard_interactivity_set);
		wl_surface_commit(scr->wl_surface);
	}

	wl_display_roundtrip(overlay->display);
}

void neru_wayland_overlay_clear(NeruWaylandOverlay *overlay) {
	if (!overlay)
		return;
	for (int i = 0; i < overlay->nr_screens; i++) {
		NeruWaylandOverlayScreen *scr = &overlay->screens[i];
		if (scr->cr) {
			cairo_save(scr->cr);
			cairo_set_operator(scr->cr, CAIRO_OPERATOR_CLEAR);
			cairo_paint(scr->cr);
			cairo_restore(scr->cr);
		}
	}
}

void neru_wayland_overlay_clear_rect(NeruWaylandOverlay *overlay, double x, double y, double width, double height) {
	if (!overlay || width <= 0 || height <= 0)
		return;
	for (int i = 0; i < overlay->nr_screens; i++) {
		NeruWaylandOverlayScreen *scr = &overlay->screens[i];
		if (!scr->cr)
			continue;

		double scr_x = x - scr->x;
		double scr_y = y - scr->y;

		cairo_t *cr = scr->cr;
		cairo_save(cr);
		cairo_set_operator(cr, CAIRO_OPERATOR_CLEAR);
		cairo_rectangle(cr, scr_x, scr_y, width, height);
		cairo_fill(cr);
		cairo_restore(cr);
	}
}

void neru_wayland_overlay_flush(NeruWaylandOverlay *overlay) { neru_wayland_overlay_show(overlay); }

static void neru_wayland_overlay_color(cairo_t *cr, unsigned int color) {
	double a = ((color >> 24) & 0xFF) / 255.0;
	double r = ((color >> 16) & 0xFF) / 255.0;
	double g = ((color >> 8) & 0xFF) / 255.0;
	double b = (color & 0xFF) / 255.0;
	cairo_set_source_rgba(cr, r, g, b, a);
}

void neru_wayland_overlay_rect(
    NeruWaylandOverlay *overlay, double x, double y, double width, double height, unsigned int fill,
    unsigned int stroke, double stroke_width) {
	for (int i = 0; i < overlay->nr_screens; i++) {
		NeruWaylandOverlayScreen *scr = &overlay->screens[i];
		if (!scr->cr)
			continue;

		// Convert global coordinates to screen-local
		double scr_x = x - scr->x;
		double scr_y = y - scr->y;

		cairo_t *cr = scr->cr;
		cairo_save(cr);
		cairo_rectangle(cr, scr_x, scr_y, width, height);
		neru_wayland_overlay_color(cr, fill);
		cairo_fill_preserve(cr);
		neru_wayland_overlay_color(cr, stroke);
		cairo_set_line_width(cr, stroke_width);
		cairo_stroke(cr);
		cairo_restore(cr);
	}
}

void neru_wayland_overlay_text(
    NeruWaylandOverlay *overlay, const char *text, const char *font_family, double x, double y, double font_size,
    unsigned int color) {
	for (int i = 0; i < overlay->nr_screens; i++) {
		NeruWaylandOverlayScreen *scr = &overlay->screens[i];
		if (!scr->cr)
			continue;

		// Convert global coordinates to screen-local
		double scr_x = x - scr->x;
		double scr_y = y - scr->y;

		cairo_t *cr = scr->cr;
		cairo_text_extents_t extents;
		cairo_save(cr);
		cairo_select_font_face(
		    cr, (font_family && font_family[0]) ? font_family : "Sans", CAIRO_FONT_SLANT_NORMAL,
		    CAIRO_FONT_WEIGHT_BOLD);
		cairo_set_font_size(cr, font_size);
		cairo_text_extents(cr, text, &extents);
		neru_wayland_overlay_color(cr, color);
		cairo_move_to(
		    cr, scr_x - (extents.width / 2.0) - extents.x_bearing, scr_y - (extents.height / 2.0) - extents.y_bearing);
		cairo_show_text(cr, text);
		cairo_restore(cr);
	}
}

// Poll for Wayland events without blocking
int neru_wayland_overlay_poll(NeruWaylandOverlay *overlay) {
	if (!overlay || !overlay->display)
		return -1;

	struct pollfd pfd = {.fd = wl_display_get_fd(overlay->display), .events = POLLIN, .revents = 0};

	int ret = poll(&pfd, 1, 0);
	if (ret > 0 && (pfd.revents & POLLIN)) {
		wl_display_dispatch(overlay->display);
	} else {
		wl_display_dispatch_pending(overlay->display);
	}
	return ret;
}

// Get next pending key from ring buffer (non-blocking).
// Returns NULL when the ring is empty.
const char *neru_wayland_overlay_get_key(NeruWaylandOverlay *overlay) {
	if (overlay->key_ring.count == 0)
		return NULL;
	const char *key = overlay->key_ring.keys[overlay->key_ring.tail];
	overlay->key_ring.tail = (overlay->key_ring.tail + 1) % NERU_KEY_RING_CAP;
	overlay->key_ring.count--;
	return key;
}
