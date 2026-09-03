#include "overlay_wayland.h"

#include "shm_file.h"
#include "wlr_protocol/fractional-scale-v1.h"
#include "wlr_protocol/layer-shell.h"
#include "wlr_protocol/viewporter.h"
#include "wlr_protocol/xdg-output.h"
#include "wlr_protocol/xdg-shell.h"

#include <cairo/cairo.h>
#include <math.h>
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
// Thread safety: all accesses happen while the Go-side renderMu mutex is held.
// The Wayland keyboard callback
// (neru_keyboard_key) fires inside wl_display_dispatch / wl_display_roundtrip,
// which only run while renderMu is held. The consumer
// (neru_wayland_overlay_get_key) is called from the keyboard poller goroutine
// which also holds renderMu. Therefore no concurrent access can occur.
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

// Output listener to track per-output buffer scale.
static void neru_wl_output_geometry(
    void *data, struct wl_output *output, int32_t x, int32_t y, int32_t phys_w, int32_t phys_h, int32_t subpixel,
    const char *make, const char *model, int32_t transform) {}

static void neru_wl_output_mode(
    void *data, struct wl_output *output, uint32_t flags, int32_t width, int32_t height, int32_t refresh) {}

static void neru_wl_output_done(void *data, struct wl_output *output) {}

static void neru_wl_output_scale(void *data, struct wl_output *output, int32_t factor) {
	NeruWaylandOverlayScreen *scr = (NeruWaylandOverlayScreen *)data;
	scr->scale = factor;
}

static const struct wl_output_listener wl_output_listener = {
    .geometry = neru_wl_output_geometry,
    .mode = neru_wl_output_mode,
    .done = neru_wl_output_done,
    .scale = neru_wl_output_scale,
};

// Per-surface fractional-scale listener. The compositor reports its preferred
// scale as a numerator over 120 (e.g. 180 == 1.5x); we render the buffer at that
// scale and let wp_viewport map it back down to the logical output size.
static void neru_fractional_preferred_scale(
    void *data, struct wp_fractional_scale_v1 *fractional_scale, uint32_t scale) {
	NeruWaylandOverlayScreen *scr = (NeruWaylandOverlayScreen *)data;
	scr->fractional_scale_120 = (int)scale;
}

static const struct wp_fractional_scale_v1_listener fractional_scale_listener = {
    .preferred_scale = neru_fractional_preferred_scale,
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

// neru_overlay_release_screen frees every wayland/cairo/shm resource for one
// output and leaves the slot inert (all pointers NULL, num_buffers 0). Used on
// runtime output removal. The slot is tombstoned rather than compacted out of
// the array because the xdg_output / wl_output / fractional_scale / wl_buffer
// listeners all carry this slot's *address* as user_data; shifting slots would
// dangle them. Every draw/buffer loop skips inert slots (wl_output/cr NULL), and
// a later hotplug-add reuses the slot.
static void neru_overlay_release_screen(NeruWaylandOverlayScreen *scr) {
	for (int b = 0; b < scr->num_buffers; b++) {
		if (scr->crs[b])
			cairo_destroy(scr->crs[b]);
		if (scr->cairo_surfaces[b])
			cairo_surface_destroy(scr->cairo_surfaces[b]);
		if (scr->buffers[b])
			wl_buffer_destroy(scr->buffers[b]);
		if (scr->shm_datas[b])
			munmap(scr->shm_datas[b], scr->shm_sizes[b]);
		scr->crs[b] = NULL;
		scr->cairo_surfaces[b] = NULL;
		scr->buffers[b] = NULL;
		scr->shm_datas[b] = NULL;
		scr->shm_sizes[b] = 0;
		scr->busy[b] = 0;
	}
	scr->num_buffers = 0;
	scr->buffer = NULL;
	scr->cairo_surface = NULL;
	scr->cr = NULL;
	scr->shm_data = NULL;
	scr->shm_size = 0;
	scr->current_buffer = -1;
	scr->width = 0;
	scr->height = 0;
	scr->fractional_scale_120 = 0;
	if (scr->viewport) {
		wp_viewport_destroy(scr->viewport);
		scr->viewport = NULL;
	}
	if (scr->fractional_scale) {
		wp_fractional_scale_v1_destroy(scr->fractional_scale);
		scr->fractional_scale = NULL;
	}
	if (scr->layer_surface) {
		zwlr_layer_surface_v1_destroy(scr->layer_surface);
		scr->layer_surface = NULL;
	}
	if (scr->wl_surface) {
		wl_surface_destroy(scr->wl_surface);
		scr->wl_surface = NULL;
	}
	if (scr->xdg_output) {
		zxdg_output_v1_destroy(scr->xdg_output);
		scr->xdg_output = NULL;
	}
	if (scr->wl_output) {
		wl_output_destroy(scr->wl_output);
		scr->wl_output = NULL;
	}
	scr->registry_name = 0;
}

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
		// Reuse an inert slot left by a previous hotplug-remove, else append.
		NeruWaylandOverlayScreen *scr = NULL;
		for (int i = 0; i < overlay->nr_screens; i++) {
			if (overlay->screens[i].wl_output == NULL) {
				scr = &overlay->screens[i];
				break;
			}
		}
		if (scr == NULL && overlay->nr_screens < NERU_MAX_OUTPUTS) {
			scr = &overlay->screens[overlay->nr_screens];
			overlay->nr_screens++;
		}
		if (scr != NULL) {
			memset(scr, 0, sizeof(*scr));
			scr->registry_name = name;
			scr->scale = 1;
			scr->current_buffer = -1;
			scr->wl_output = wl_registry_bind(registry, name, &wl_output_interface, 3 < version ? 3 : version);
			wl_output_add_listener(scr->wl_output, &wl_output_listener, scr);
			// Past initial setup, the bulk xdg_output loop in
			// neru_wayland_overlay_new has already run, so wire this output's
			// xdg_output here to populate its logical geometry.
			if (overlay->outputs_configured && overlay->xdg_output_mgr) {
				scr->xdg_output = zxdg_output_manager_v1_get_xdg_output(overlay->xdg_output_mgr, scr->wl_output);
				zxdg_output_v1_add_listener(scr->xdg_output, &xdg_output_listener, scr);
			}
		}
	} else if (strcmp(interface, "zxdg_output_manager_v1") == 0) {
		overlay->xdg_output_mgr =
		    wl_registry_bind(registry, name, &zxdg_output_manager_v1_interface, 3 < version ? 3 : version);
	} else if (strcmp(interface, "wp_viewporter") == 0) {
		overlay->viewporter = wl_registry_bind(registry, name, &wp_viewporter_interface, 1);
	} else if (strcmp(interface, "wp_fractional_scale_manager_v1") == 0) {
		overlay->fractional_mgr = wl_registry_bind(registry, name, &wp_fractional_scale_manager_v1_interface, 1);
	} else if (strcmp(interface, "wl_seat") == 0) {
		overlay->wl_seat = wl_registry_bind(registry, name, &wl_seat_interface, 5);
	}
}

static void neru_overlay_registry_global_remove(void *data, struct wl_registry *registry, uint32_t name) {
	NeruWaylandOverlay *overlay = (NeruWaylandOverlay *)data;
	if (!overlay)
		return;

	// An output was unplugged. Tear down its surface/buffers/objects so it no
	// longer participates in cross-output buffer selection (a lingering output's
	// unreleased buffers would stall available-buffer and freeze the remaining
	// monitors). The slot is left inert and reused by a later hotplug-add.
	for (int i = 0; i < overlay->nr_screens; i++) {
		if (overlay->screens[i].wl_output != NULL && overlay->screens[i].registry_name == name) {
			neru_overlay_release_screen(&overlay->screens[i]);
			break;
		}
	}
}

static const struct wl_registry_listener overlay_registry_listener = {
    .global = neru_overlay_registry_global,
    .global_remove = neru_overlay_registry_global_remove,
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
	}
	close(fd);
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

// Buffer release listener - compositor tells us it's done reading a buffer
static void neru_buffer_release(void *data, struct wl_buffer *wl_buffer) {
	NeruWaylandOverlayScreen *scr = (NeruWaylandOverlayScreen *)data;
	for (int i = 0; i < NERU_NUM_BUFFERS; i++) {
		if (scr->buffers[i] == wl_buffer) {
			scr->busy[i] = 0;
			break;
		}
	}
}

static const struct wl_buffer_listener neru_buffer_listener = {
    .release = neru_buffer_release,
};

NeruWaylandOverlay *neru_wayland_overlay_new(void) {
	NeruWaylandOverlay *overlay = calloc(1, sizeof(NeruWaylandOverlay));
	if (!overlay)
		return NULL;

	// NONE by default so the overlay never steals keyboard focus on creation. A
	// layer-surface keyboard grab makes wlroots compositors (niri, Sway, …)
	// deactivate the focused app's toplevel, which breaks the "focused window"
	// query a hints refresh depends on. Keys are captured via the evdev grab; only
	// the non-evdev fallback (see runWayland in eventtap_linux_wayland.go) turns
	// this back to EXCLUSIVE via neru_wayland_overlay_set_keyboard_capture. This
	// mirrors macOS, whose overlay is a non-activating panel.
	overlay->keyboard_interactivity_set = ZWLR_LAYER_SURFACE_V1_KEYBOARD_INTERACTIVITY_NONE;

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
	// Initial outputs are wired; a later hotplug-added output now wires its own
	// xdg_output inline in neru_overlay_registry_global.
	overlay->outputs_configured = 1;
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
		for (int b = 0; b < scr->num_buffers; b++) {
			if (scr->crs[b])
				cairo_destroy(scr->crs[b]);
			if (scr->cairo_surfaces[b])
				cairo_surface_destroy(scr->cairo_surfaces[b]);
			if (scr->buffers[b])
				wl_buffer_destroy(scr->buffers[b]);
			if (scr->shm_datas[b])
				munmap(scr->shm_datas[b], scr->shm_sizes[b]);
		}
		scr->num_buffers = 0;
		if (scr->viewport)
			wp_viewport_destroy(scr->viewport);
		if (scr->fractional_scale)
			wp_fractional_scale_v1_destroy(scr->fractional_scale);
		if (scr->layer_surface)
			zwlr_layer_surface_v1_destroy(scr->layer_surface);
		if (scr->wl_surface)
			wl_surface_destroy(scr->wl_surface);
		if (scr->xdg_output)
			zxdg_output_v1_destroy(scr->xdg_output);
	}

	if (overlay->xkb_state)
		xkb_state_unref(overlay->xkb_state);
	if (overlay->xkb_ctx)
		xkb_context_unref(overlay->xkb_ctx);
	if (overlay->viewporter)
		wp_viewporter_destroy(overlay->viewporter);
	if (overlay->fractional_mgr)
		wp_fractional_scale_manager_v1_destroy(overlay->fractional_mgr);
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

static int neru_create_single_buffer(
    NeruWaylandOverlay *overlay, NeruWaylandOverlayScreen *scr, int buf_idx, int buf_width, int buf_height, int stride,
    double scale) {
	size_t buf_size = (size_t)stride * (size_t)buf_height;
	int fd = neru_shm_file_create("neru-overlay-shm", buf_size);
	if (fd < 0)
		return -1;

	void *data = mmap(NULL, buf_size, PROT_READ | PROT_WRITE, MAP_SHARED, fd, 0);
	if (data == MAP_FAILED) {
		close(fd);
		return -1;
	}

	struct wl_shm_pool *pool = wl_shm_create_pool(overlay->shm, fd, (int)buf_size);
	scr->buffers[buf_idx] = wl_shm_pool_create_buffer(pool, 0, buf_width, buf_height, stride, WL_SHM_FORMAT_ARGB8888);
	wl_buffer_add_listener(scr->buffers[buf_idx], &neru_buffer_listener, scr);
	wl_shm_pool_destroy(pool);
	close(fd);

	scr->shm_datas[buf_idx] = data;
	scr->shm_sizes[buf_idx] = buf_size;
	scr->busy[buf_idx] = 0;

	scr->cairo_surfaces[buf_idx] =
	    cairo_image_surface_create_for_data(data, CAIRO_FORMAT_ARGB32, buf_width, buf_height, stride);
	scr->crs[buf_idx] = cairo_create(scr->cairo_surfaces[buf_idx]);
	cairo_scale(scr->crs[buf_idx], scale, scale);

	return 0;
}

void neru_wayland_overlay_setup_buffers(NeruWaylandOverlay *overlay) {
	int new_surfaces = 0;

	for (int i = 0; i < overlay->nr_screens; i++) {
		NeruWaylandOverlayScreen *scr = &overlay->screens[i];

		if (scr->wl_output == NULL)
			continue;  // Inert slot left by a removed output

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

		// Fractional-scale + viewport: over-render the buffer at the compositor's
		// preferred fractional scale and let the viewport map it back to the
		// logical output size. Both are optional; absence falls back to the
		// integer wl_output.scale path in the buffer-creation loop below.
		if (overlay->viewporter) {
			scr->viewport = wp_viewporter_get_viewport(overlay->viewporter, scr->wl_surface);
		}
		if (overlay->fractional_mgr) {
			scr->fractional_scale =
			    wp_fractional_scale_manager_v1_get_fractional_scale(overlay->fractional_mgr, scr->wl_surface);
			wp_fractional_scale_v1_add_listener(scr->fractional_scale, &fractional_scale_listener, scr);
		}

		scr->layer_surface = zwlr_layer_shell_v1_get_layer_surface(
		    overlay->layer_shell, scr->wl_surface, scr->wl_output, ZWLR_LAYER_SHELL_V1_LAYER_OVERLAY, "neru");

		zwlr_layer_surface_v1_set_size(scr->layer_surface, scr->width, scr->height);
		zwlr_layer_surface_v1_set_anchor(
		    scr->layer_surface, ZWLR_LAYER_SURFACE_V1_ANCHOR_TOP | ZWLR_LAYER_SURFACE_V1_ANCHOR_LEFT |
		                            ZWLR_LAYER_SURFACE_V1_ANCHOR_RIGHT | ZWLR_LAYER_SURFACE_V1_ANCHOR_BOTTOM);
		zwlr_layer_surface_v1_set_exclusive_zone(scr->layer_surface, -1);

		// Apply the current keyboard-interactivity mode (NONE by default so the
		// overlay does not steal focus; the non-evdev fallback raises it to
		// EXCLUSIVE via neru_wayland_overlay_set_keyboard_capture).
		zwlr_layer_surface_v1_set_keyboard_interactivity(scr->layer_surface, overlay->keyboard_interactivity_set);

		zwlr_layer_surface_v1_add_listener(scr->layer_surface, &layer_surface_listener, overlay);
		wl_surface_commit(scr->wl_surface);

		new_surfaces = 1;
	}

	// Only roundtrip when new surfaces were created (avoids sync delay on every draw).
	if (new_surfaces) {
		wl_display_roundtrip(overlay->display);
	}

	// A second roundtrip lets the compositor deliver wp_fractional_scale_v1
	// preferred_scale events for the surfaces just committed, so buffers below
	// are sized at the correct fractional scale on first show.
	if (new_surfaces && overlay->fractional_mgr) {
		wl_display_roundtrip(overlay->display);
	}

	for (int i = 0; i < overlay->nr_screens; i++) {
		NeruWaylandOverlayScreen *scr = &overlay->screens[i];
		if (scr->num_buffers > 0)
			continue;
		if (scr->layer_surface == NULL)
			continue;  // Inert slot, or dimensions not set yet — no surface to back

		// Prefer fractional scaling (buffer over-rendered, then mapped down by the
		// viewport). Fall back to the integer wl_output.scale + set_buffer_scale
		// path when the compositor lacks the protocols or hasn't reported a scale.
		int use_fractional = (scr->viewport != NULL && scr->fractional_scale_120 > 0);
		double scale_factor;
		int buf_width, buf_height;
		int integer_scale = scr->scale > 0 ? scr->scale : 1;

		if (use_fractional) {
			scale_factor = (double)scr->fractional_scale_120 / 120.0;
			buf_width = (int)ceil((double)scr->width * scale_factor);
			buf_height = (int)ceil((double)scr->height * scale_factor);
		} else {
			scale_factor = (double)integer_scale;
			buf_width = scr->width * integer_scale;
			buf_height = scr->height * integer_scale;
		}
		size_t stride = ((size_t)buf_width) * 4u;

		int ok = 0;
		for (int b = 0; b < NERU_NUM_BUFFERS; b++) {
			if (neru_create_single_buffer(overlay, scr, b, buf_width, buf_height, (int)stride, scale_factor) == 0)
				ok++;
		}
		scr->num_buffers = ok;

		if (ok > 0) {
			// Point current pointers to buffer 0
			scr->current_buffer = 0;
			scr->buffer = scr->buffers[0];
			scr->cairo_surface = scr->cairo_surfaces[0];
			scr->cr = scr->crs[0];
			scr->shm_data = scr->shm_datas[0];
			scr->shm_size = scr->shm_sizes[0];
		}

		if (use_fractional) {
			// The buffer is over-rendered; map it back onto the logical output
			// size. buffer_scale stays at its default of 1 (the buffer dims are
			// not an integer multiple of the logical size).
			wp_viewport_set_destination(scr->viewport, scr->width, scr->height);
		} else if (scr->wl_surface) {
			wl_surface_set_buffer_scale(scr->wl_surface, integer_scale);
		}
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
			// Mark the committed buffer as busy (compositor owns it now)
			if (scr->current_buffer >= 0) {
				scr->busy[scr->current_buffer] = 1;
			}
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
		// Destroy the viewport and fractional-scale objects: both are bound to the
		// wl_surface, which is recreated (and re-queried) on the next show.
		if (scr->viewport) {
			wp_viewport_destroy(scr->viewport);
			scr->viewport = NULL;
		}
		if (scr->fractional_scale) {
			wp_fractional_scale_v1_destroy(scr->fractional_scale);
			scr->fractional_scale = NULL;
		}
		scr->fractional_scale_120 = 0;
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
		// Destroy all buffers and cairo
		for (int b = 0; b < scr->num_buffers; b++) {
			if (scr->crs[b])
				cairo_destroy(scr->crs[b]);
			if (scr->cairo_surfaces[b])
				cairo_surface_destroy(scr->cairo_surfaces[b]);
			if (scr->buffers[b])
				wl_buffer_destroy(scr->buffers[b]);
			if (scr->shm_datas[b])
				munmap(scr->shm_datas[b], scr->shm_sizes[b]);
			scr->crs[b] = NULL;
			scr->cairo_surfaces[b] = NULL;
			scr->buffers[b] = NULL;
			scr->shm_datas[b] = NULL;
			scr->shm_sizes[b] = 0;
			scr->busy[b] = 0;
		}
		scr->num_buffers = 0;
		// Reset current pointers
		scr->buffer = NULL;
		scr->cairo_surface = NULL;
		scr->cr = NULL;
		scr->shm_data = NULL;
		scr->shm_size = 0;
		scr->current_buffer = -1;
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

void neru_wayland_overlay_sync(NeruWaylandOverlay *overlay) {
	if (!overlay || !overlay->display)
		return;
	wl_display_roundtrip(overlay->display);
}

void neru_wayland_overlay_select_buffer(NeruWaylandOverlay *overlay, int index) {
	if (!overlay)
		return;
	for (int i = 0; i < overlay->nr_screens; i++) {
		NeruWaylandOverlayScreen *scr = &overlay->screens[i];
		if (index < 0 || index >= scr->num_buffers)
			continue;
		scr->buffer = scr->buffers[index];
		scr->cairo_surface = scr->cairo_surfaces[index];
		scr->cr = scr->crs[index];
		scr->shm_data = scr->shm_datas[index];
		scr->shm_size = scr->shm_sizes[index];
		scr->current_buffer = index;
	}
}

int neru_wayland_overlay_available_buffer(NeruWaylandOverlay *overlay) {
	if (!overlay || overlay->nr_screens == 0)
		return -1;

	// Return a buffer index the compositor has released on every LIVE output.
	// Outputs release buffers at different rates — most visibly a fractional-scale
	// output, which holds its viewport-mapped buffer a frame longer — so an index
	// still busy on another output attaches a buffer the compositor already owns,
	// freezing that output. Inert slots left by a removed output (num_buffers == 0)
	// are skipped; otherwise min_buffers would collapse to 0 and stall everything.
	int min_buffers = 0;
	int have_live = 0;
	for (int s = 0; s < overlay->nr_screens; s++) {
		int nb = overlay->screens[s].num_buffers;
		if (nb <= 0)
			continue;
		if (!have_live || nb < min_buffers) {
			min_buffers = nb;
			have_live = 1;
		}
	}
	if (!have_live)
		return -1;

	for (int i = 0; i < min_buffers; i++) {
		int free_on_all = 1;
		for (int s = 0; s < overlay->nr_screens; s++) {
			if (overlay->screens[s].num_buffers <= 0)
				continue;
			if (overlay->screens[s].busy[i]) {
				free_on_all = 0;
				break;
			}
		}
		if (free_on_all)
			return i;
	}
	return -1;
}

void neru_wayland_overlay_dispatch_pending(NeruWaylandOverlay *overlay) {
	if (!overlay || !overlay->display)
		return;
	wl_display_dispatch_pending(overlay->display);
}

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

static void neru_wayland_overlay_rounded_path(
    cairo_t *cr, double x, double y, double width, double height, double radius) {
	double max_radius = (width < height ? width : height) / 2.0;
	if (radius > max_radius)
		radius = max_radius;
	if (radius <= 0) {
		cairo_rectangle(cr, x, y, width, height);
		return;
	}

	// degrees-to-radians literal avoids depending on M_PI
	const double deg = 0.0174532925199432957692;
	cairo_new_sub_path(cr);
	cairo_arc(cr, x + width - radius, y + radius, radius, -90.0 * deg, 0.0 * deg);
	cairo_arc(cr, x + width - radius, y + height - radius, radius, 0.0 * deg, 90.0 * deg);
	cairo_arc(cr, x + radius, y + height - radius, radius, 90.0 * deg, 180.0 * deg);
	cairo_arc(cr, x + radius, y + radius, radius, 180.0 * deg, 270.0 * deg);
	cairo_close_path(cr);
}

void neru_wayland_overlay_rounded_rect(
    NeruWaylandOverlay *overlay, double x, double y, double width, double height, double radius, unsigned int fill,
    unsigned int stroke, double stroke_width) {
	for (int i = 0; i < overlay->nr_screens; i++) {
		NeruWaylandOverlayScreen *scr = &overlay->screens[i];
		if (!scr->cr)
			continue;

		double scr_x = x - scr->x;
		double scr_y = y - scr->y;

		cairo_t *cr = scr->cr;
		cairo_save(cr);
		neru_wayland_overlay_rounded_path(cr, scr_x, scr_y, width, height, radius);
		neru_wayland_overlay_color(cr, fill);
		cairo_fill_preserve(cr);
		neru_wayland_overlay_color(cr, stroke);
		cairo_set_line_width(cr, stroke_width);
		cairo_stroke(cr);
		cairo_restore(cr);
	}
}

// neru_wayland_hint_badge_path mirrors neru_x11_hint_badge_path: a rounded rect
// with an optional triangular tail merged into one edge as a single closed
// outline. edge: 0 = none, 1 = top-edge tail (apex above), 2 = bottom-edge tail
// (apex below). The tail base is clamped to the edge's flat span. Coordinates
// are surface-local (the caller applies the per-output offset).
static void neru_wayland_hint_badge_path(
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

// neru_wayland_overlay_hint_badge fills and strokes a hint badge with an
// optional connector tail as one continuous outline on every output.
void neru_wayland_overlay_hint_badge(
    NeruWaylandOverlay *overlay, double x, double y, double width, double height, double radius, int edge,
    double a_left, double a_right, double tip_x, double tip_y, unsigned int fill, unsigned int stroke,
    double stroke_width) {
	for (int i = 0; i < overlay->nr_screens; i++) {
		NeruWaylandOverlayScreen *scr = &overlay->screens[i];
		if (!scr->cr)
			continue;

		cairo_t *cr = scr->cr;
		cairo_save(cr);
		neru_wayland_hint_badge_path(
		    cr, x - scr->x, y - scr->y, width, height, radius, edge, a_left - scr->x, a_right - scr->x, tip_x - scr->x,
		    tip_y - scr->y);
		neru_wayland_overlay_color(cr, fill);
		cairo_fill_preserve(cr);
		neru_wayland_overlay_color(cr, stroke);
		cairo_set_line_width(cr, stroke_width);
		cairo_stroke(cr);
		cairo_restore(cr);
	}
}

// neru_resolve_font_family returns a font family cairo's toy API can actually
// render, substituting a generic fallback when the requested family fails.
//
// This guards against a subtle, catastrophic failure mode: on some
// cairo/fontconfig setups (e.g. certain Nix closures on non-NixOS hosts)
// cairo_show_text with a specific family name — even a common one like
// "DejaVu Sans" — fails with CAIRO_STATUS_NO_MEMORY, which permanently poisons
// the cairo context so every subsequent draw silently no-ops. Generic families
// ("sans-serif", "monospace", ...) resolve fine there. We probe the requested
// family on a throwaway context (never touching a real buffer context, so a
// failure cannot poison it) and fall back to "sans-serif" when it fails. The
// result is cached for the last family, since a single draw pass uses one
// family for all its glyphs.
static const char *neru_resolve_font_family(const char *family) {
	static char cache_key[128];
	static char cache_val[128];
	static int cache_valid = 0;

	if (family == NULL || family[0] == '\0')
		return "sans-serif";

	if (cache_valid && strncmp(cache_key, family, sizeof(cache_key)) == 0)
		return cache_val;

	const char *resolved = family;

	cairo_surface_t *probe = cairo_image_surface_create(CAIRO_FORMAT_ARGB32, 8, 8);
	cairo_t *cr = cairo_create(probe);
	cairo_select_font_face(cr, family, CAIRO_FONT_SLANT_NORMAL, CAIRO_FONT_WEIGHT_NORMAL);
	cairo_set_font_size(cr, 10.0);
	cairo_move_to(cr, 0.0, 6.0);
	cairo_show_text(cr, "A");
	if (cairo_status(cr) != CAIRO_STATUS_SUCCESS)
		resolved = "sans-serif";
	cairo_destroy(cr);
	cairo_surface_destroy(probe);

	snprintf(cache_key, sizeof(cache_key), "%s", family);
	snprintf(cache_val, sizeof(cache_val), "%s", resolved);
	cache_valid = 1;

	return cache_val;
}

void neru_wayland_overlay_text(
    NeruWaylandOverlay *overlay, const char *text, const char *font_family, double x, double y, double font_size,
    unsigned int color) {
	const char *resolved_family = neru_resolve_font_family(font_family);

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
		cairo_select_font_face(cr, resolved_family, CAIRO_FONT_SLANT_NORMAL, CAIRO_FONT_WEIGHT_BOLD);
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

	struct wl_display *display = overlay->display;

	int prepare_retries = 0;
	while (wl_display_prepare_read(display) != 0) {
		if (wl_display_dispatch_pending(display) < 0)
			return -1;
		if (++prepare_retries > 100)
			return -1;
	}

	wl_display_flush(display);

	struct pollfd pfd = {.fd = wl_display_get_fd(display), .events = POLLIN, .revents = 0};

	int ret = poll(&pfd, 1, 0);
	if (ret > 0 && (pfd.revents & POLLIN)) {
		if (wl_display_read_events(display) < 0)
			ret = -1;
	} else {
		wl_display_cancel_read(display);
		if (ret > 0)
			ret = -1;
	}

	wl_display_dispatch_pending(display);
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
