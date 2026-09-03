#include <errno.h>
#include <fcntl.h>
#include <poll.h>
#include <pthread.h>
#include <stdatomic.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <time.h>
#include <unistd.h>
#include <wayland-client.h>
#include <xkbcommon/xkbcommon.h>

// Include the sibling bridge headers and the wlroots protocol headers
// relative to this package.
#include "shm_file.h"
#include "wlr_protocol/foreign-toplevel.h"
#include "wlr_protocol/layer-shell.h"
#include "wlr_protocol/relative-pointer-unstable-v1.h"
#include "wlr_protocol/virtual-keyboard.h"
#include "wlr_protocol/virtual-pointer.h"
#include "wlr_protocol/xdg-output.h"
#include "wlr_protocol/xdg-shell.h"
#include "wlroots_client.h"

// Pointer listener callbacks — update cursor cache.
// Forward discovery-surface pointer events through the virtual pointer so the
// underlying application still receives them instead of having them swallowed.
static void neru_wlr_pointer_enter(
    void *data, struct wl_pointer *pointer, uint32_t serial, struct wl_surface *surface, wl_fixed_t sx, wl_fixed_t sy) {
	NeruWlrootsClient *c = (NeruWlrootsClient *)data;
	if (c) {
		c->entered_discovery_surface = NULL;
		for (int i = 0; i < c->nr_screens; i++) {
			NeruWaylandScreen *scr = &c->screens[i];
			if (surface != NULL && surface == scr->discovery_surface) {
				c->entered_discovery_surface = surface;
				atomic_store(&c->cursor_x, scr->x + wl_fixed_to_int(sx));
				atomic_store(&c->cursor_y, scr->y + wl_fixed_to_int(sy));
				atomic_store(&c->cursor_initialized, 1);

				// Immediately set empty input region so the discovery surface
				// becomes pass-through and does not swallow user input.
				if (c->compositor) {
					struct wl_region *region = wl_compositor_create_region(c->compositor);
					if (region) {
						wl_surface_set_input_region(surface, region);
						wl_region_destroy(region);
						wl_surface_commit(surface);
					}
				}
				break;
			}
		}
	}
	(void)pointer;
	(void)serial;
}

static void neru_wlr_pointer_leave(
    void *data, struct wl_pointer *pointer, uint32_t serial, struct wl_surface *surface) {
	NeruWlrootsClient *c = (NeruWlrootsClient *)data;
	if (c && surface && surface == c->entered_discovery_surface)
		c->entered_discovery_surface = NULL;
	(void)pointer;
	(void)serial;
}

static void neru_wlr_pointer_motion(
    void *data, struct wl_pointer *pointer, uint32_t time, wl_fixed_t sx, wl_fixed_t sy) {
	NeruWlrootsClient *c = (NeruWlrootsClient *)data;
	if (c && c->entered_discovery_surface) {
		for (int i = 0; i < c->nr_screens; i++) {
			NeruWaylandScreen *scr = &c->screens[i];
			if (scr->discovery_surface && scr->discovery_surface == c->entered_discovery_surface) {
				atomic_store(&c->cursor_x, scr->x + wl_fixed_to_int(sx));
				atomic_store(&c->cursor_y, scr->y + wl_fixed_to_int(sy));
				break;
			}
		}
	}
	(void)pointer;
	(void)time;
}

static void neru_wlr_sync_vptr_to_cursor(NeruWlrootsClient *c, uint32_t time) {
	if (!c || !c->vptr || c->nr_screens == 0)
		return;
	int minx = 0, miny = 0, maxx = 0, maxy = 0;
	for (int j = 0; j < c->nr_screens; j++) {
		NeruWaylandScreen *s = &c->screens[j];
		if (j == 0 || s->x < minx)
			minx = s->x;
		if (j == 0 || s->y < miny)
			miny = s->y;
		int r = s->x + s->w, b = s->y + s->h;
		if (j == 0 || r > maxx)
			maxx = r;
		if (j == 0 || b > maxy)
			maxy = b;
	}
	int cx = atomic_load(&c->cursor_x);
	int cy = atomic_load(&c->cursor_y);
	zwlr_virtual_pointer_v1_motion_absolute(
	    c->vptr, time, wl_fixed_from_int(cx - minx), wl_fixed_from_int(cy - miny), wl_fixed_from_int(maxx - minx),
	    wl_fixed_from_int(maxy - miny));
}

static void neru_wlr_pointer_button(
    void *data, struct wl_pointer *pointer, uint32_t serial, uint32_t time, uint32_t button, uint32_t state) {
	NeruWlrootsClient *c = (NeruWlrootsClient *)data;
	if (c && c->vptr && c->entered_discovery_surface) {
		neru_wlr_sync_vptr_to_cursor(c, time);
		zwlr_virtual_pointer_v1_button(c->vptr, time, button, state);
		zwlr_virtual_pointer_v1_frame(c->vptr);
	}
	(void)pointer;
	(void)serial;
}

static void neru_wlr_pointer_axis(
    void *data, struct wl_pointer *pointer, uint32_t time, uint32_t axis, wl_fixed_t value) {
	NeruWlrootsClient *c = (NeruWlrootsClient *)data;
	if (c && c->vptr && c->entered_discovery_surface) {
		neru_wlr_sync_vptr_to_cursor(c, time);
		zwlr_virtual_pointer_v1_axis(c->vptr, time, axis, value);
		zwlr_virtual_pointer_v1_frame(c->vptr);
	}
	(void)pointer;
}

static void neru_wlr_pointer_frame(void *data, struct wl_pointer *pointer) {
	(void)data;
	(void)pointer;
}

static void neru_wlr_pointer_axis_source(void *data, struct wl_pointer *pointer, uint32_t axis_source) {
	// No-op.
}

static void neru_wlr_pointer_axis_stop(void *data, struct wl_pointer *pointer, uint32_t time, uint32_t axis) {
	// No-op.
}

static void neru_wlr_pointer_axis_discrete(void *data, struct wl_pointer *pointer, uint32_t axis, int32_t discrete) {
	// No-op.
}

// ---------- Relative pointer listener ----------

static void neru_wlr_relative_motion(
    void *data, struct zwp_relative_pointer_v1 *zwp_relative_pointer_v1, uint32_t utime_hi, uint32_t utime_lo,
    wl_fixed_t dx, wl_fixed_t dy, wl_fixed_t dx_unaccel, wl_fixed_t dy_unaccel) {
	NeruWlrootsClient *c = (NeruWlrootsClient *)data;
	// Accumulate sub-pixel deltas and only commit whole pixels, preventing
	// drift on HiDPI or accelerated pointer setups where fractional motion
	// is common.
	c->cursor_x_frac += dx;
	c->cursor_y_frac += dy;
	int idx = wl_fixed_to_int(c->cursor_x_frac);
	int idy = wl_fixed_to_int(c->cursor_y_frac);
	if (idx != 0 || idy != 0) {
		c->cursor_x_frac -= wl_fixed_from_int(idx);
		c->cursor_y_frac -= wl_fixed_from_int(idy);
		atomic_fetch_add(&c->cursor_x, idx);
		atomic_fetch_add(&c->cursor_y, idy);
		int in_discovery = 0;
		for (int i = 0; i < c->nr_screens; i++) {
			if (c->screens[i].discovery_surface) {
				in_discovery = 1;
				break;
			}
		}
		if (!in_discovery)
			atomic_store(&c->cursor_initialized, 1);
	}
	(void)zwp_relative_pointer_v1;
	(void)utime_hi;
	(void)utime_lo;
	(void)dx_unaccel;
	(void)dy_unaccel;
}

static const struct zwp_relative_pointer_v1_listener neru_wlr_relative_pointer_listener = {
    .relative_motion = neru_wlr_relative_motion,
};

static const struct wl_pointer_listener neru_wlr_pointer_listener = {
    .enter = neru_wlr_pointer_enter,
    .leave = neru_wlr_pointer_leave,
    .motion = neru_wlr_pointer_motion,
    .button = neru_wlr_pointer_button,
    .axis = neru_wlr_pointer_axis,
    .frame = neru_wlr_pointer_frame,
    .axis_source = neru_wlr_pointer_axis_source,
    .axis_stop = neru_wlr_pointer_axis_stop,
    .axis_discrete = neru_wlr_pointer_axis_discrete,
};

// ---------- Seat listener ----------

// The relative pointer is what tracks physical mouse movement into the cursor
// cache, and it is built on top of the wl_pointer — so it has to be rebuilt
// every time that is, not only at connect. A pointer that came back without one
// would leave the cached cursor position silently frozen against the user's own
// hand.
static void neru_wlr_bind_relative_pointer(NeruWlrootsClient *c) {
	if (!c->rel_ptr_mgr || !c->pointer || c->rel_ptr)
		return;

	c->rel_ptr = zwp_relative_pointer_manager_v1_get_relative_pointer(c->rel_ptr_mgr, c->pointer);
	zwp_relative_pointer_v1_add_listener(c->rel_ptr, &neru_wlr_relative_pointer_listener, c);
}

// The seat tells us when a pointer exists, and only then may we ask for one.
// Our own virtual pointer is enough to bring one into being, so a seat that
// starts without one gains it as soon as neru_wlr_connect creates the virtual
// pointer — which is why connect waits a roundtrip before using c->pointer.
static void neru_wlr_seat_capabilities(void *data, struct wl_seat *seat, uint32_t capabilities) {
	NeruWlrootsClient *c = (NeruWlrootsClient *)data;
	if (!c)
		return;

	if ((capabilities & WL_SEAT_CAPABILITY_POINTER) && !c->pointer) {
		c->pointer = wl_seat_get_pointer(seat);
		wl_pointer_add_listener(c->pointer, &neru_wlr_pointer_listener, c);
		neru_wlr_bind_relative_pointer(c);
		return;
	}

	// The event is level-triggered, so a seat that loses its pointer sends it
	// again with the bit clear and the compositor treats the wl_pointer as
	// gone. Holding on to it would turn the next use into a request on a dead
	// object — a protocol error that kills the connection, which is the failure
	// this listener exists to avoid in the other direction.
	if (!(capabilities & WL_SEAT_CAPABILITY_POINTER) && c->pointer) {
		if (c->rel_ptr) {
			zwp_relative_pointer_v1_destroy(c->rel_ptr);
			c->rel_ptr = NULL;
		}
		wl_pointer_release(c->pointer);
		c->pointer = NULL;
	}
}

static void neru_wlr_seat_name(void *data, struct wl_seat *seat, const char *name) {
	(void)data;
	(void)seat;
	(void)name;
}

static const struct wl_seat_listener neru_wlr_seat_listener = {
    .capabilities = neru_wlr_seat_capabilities,
    .name = neru_wlr_seat_name,
};

typedef struct {
	NeruWlrootsClient *client;
	NeruWaylandScreen *screen;
	struct wl_surface *surface;
	struct zwlr_layer_surface_v1 *layer_surface;
	struct wl_buffer *buffer;
	void *shm_data;
	size_t shm_size;
	int width;
	int height;
	int configured;
} NeruCursorDiscoverySurface;

static int neru_wlr_discovery_attach_buffer(NeruWlrootsClient *c, NeruCursorDiscoverySurface *surface);

static void neru_wlr_discovery_configure(
    void *data, struct zwlr_layer_surface_v1 *layer_surface, uint32_t serial, uint32_t width, uint32_t height) {
	NeruCursorDiscoverySurface *surface = (NeruCursorDiscoverySurface *)data;
	zwlr_layer_surface_v1_ack_configure(layer_surface, serial);
	if (width > 0)
		surface->width = (int)width;
	if (height > 0)
		surface->height = (int)height;
	surface->configured = 1;
	neru_wlr_discovery_attach_buffer(surface->client, surface);
}

static void neru_wlr_discovery_closed(void *data, struct zwlr_layer_surface_v1 *layer_surface) {
	NeruCursorDiscoverySurface *surface = (NeruCursorDiscoverySurface *)data;
	surface->configured = -1;
}

static const struct zwlr_layer_surface_v1_listener neru_wlr_discovery_layer_listener = {
    .configure = neru_wlr_discovery_configure,
    .closed = neru_wlr_discovery_closed,
};

static int neru_wlr_discovery_attach_buffer(NeruWlrootsClient *c, NeruCursorDiscoverySurface *surface) {
	if (!c || !c->shm || !surface || !surface->surface || surface->width <= 0 || surface->height <= 0)
		return 0;
	if (surface->buffer)
		return 1;

	size_t stride = (size_t)surface->width * 4u;
	surface->shm_size = stride * (size_t)surface->height;
	int fd = neru_shm_file_create("neru-cursor-discovery-shm", surface->shm_size);
	if (fd < 0)
		return 0;

	surface->shm_data = mmap(NULL, surface->shm_size, PROT_READ | PROT_WRITE, MAP_SHARED, fd, 0);
	if (surface->shm_data == MAP_FAILED) {
		surface->shm_data = NULL;
		close(fd);
		return 0;
	}
	memset(surface->shm_data, 0, surface->shm_size);

	struct wl_shm_pool *pool = wl_shm_create_pool(c->shm, fd, (int)surface->shm_size);
	if (!pool) {
		munmap(surface->shm_data, surface->shm_size);
		surface->shm_data = NULL;
		close(fd);
		return 0;
	}

	surface->buffer =
	    wl_shm_pool_create_buffer(pool, 0, surface->width, surface->height, (int)stride, WL_SHM_FORMAT_ARGB8888);
	wl_shm_pool_destroy(pool);
	close(fd);
	if (!surface->buffer) {
		munmap(surface->shm_data, surface->shm_size);
		surface->shm_data = NULL;
		return 0;
	}

	wl_surface_attach(surface->surface, surface->buffer, 0, 0);
	wl_surface_damage_buffer(surface->surface, 0, 0, INT32_MAX, INT32_MAX);
	wl_surface_commit(surface->surface);

	return 1;
}

static void neru_wlr_discovery_destroy(NeruCursorDiscoverySurface *surface) {
	if (!surface)
		return;
	if (surface->surface) {
		wl_surface_attach(surface->surface, NULL, 0, 0);
		wl_surface_commit(surface->surface);
	}
	if (surface->layer_surface) {
		zwlr_layer_surface_v1_destroy(surface->layer_surface);
		surface->layer_surface = NULL;
	}
	if (surface->surface) {
		wl_surface_destroy(surface->surface);
		surface->surface = NULL;
	}
	if (surface->buffer) {
		wl_buffer_destroy(surface->buffer);
		surface->buffer = NULL;
	}
	if (surface->shm_data) {
		munmap(surface->shm_data, surface->shm_size);
		surface->shm_data = NULL;
	}
}

static int neru_wlr_create_keymap_fd(const char *keymap, size_t size) {
	char template[] = "/tmp/neru-vkbd-keymap-XXXXXX";
	int fd = mkstemp(template);
	if (fd < 0)
		return -1;

	unlink(template);

	size_t written = 0;
	while (written < size) {
		ssize_t ret = write(fd, keymap + written, size - written);
		if (ret < 0) {
			if (errno == EINTR)
				continue;
			close(fd);
			return -1;
		}
		written += (size_t)ret;
	}

	if (lseek(fd, 0, SEEK_SET) < 0) {
		close(fd);
		return -1;
	}

	return fd;
}

static uint32_t neru_wlr_mod_mask(xkb_mod_index_t idx) {
	if (idx == XKB_MOD_INVALID || idx >= 32)
		return 0;
	return 1u << idx;
}

static int neru_wlr_setup_virtual_keyboard(NeruWlrootsClient *c) {
	if (!c || !c->vkeyboard)
		return 0;

	c->xkb_ctx = xkb_context_new(XKB_CONTEXT_NO_FLAGS);
	if (!c->xkb_ctx)
		return 0;

	// US pc105 layout is intentionally hardcoded:
	// 1) Modifier index resolution (Shift/Ctrl/Alt/Logo) is layout-independent.
	// 2) The keymap is sent to the compositor but Neru only uses it to inject
	//    synthetic key events; actual key symbols are resolved via xkbcommon
	//    in the overlay-keyboard path, so the virtual layout never appears
	//    to the user.
	struct xkb_rule_names names = {
	    .rules = "evdev",
	    .model = "pc105",
	    .layout = "us",
	    .variant = NULL,
	    .options = NULL,
	};

	c->xkb_keymap = xkb_keymap_new_from_names(c->xkb_ctx, &names, XKB_KEYMAP_COMPILE_NO_FLAGS);
	if (!c->xkb_keymap)
		return 0;

	char *keymap = xkb_keymap_get_as_string(c->xkb_keymap, XKB_KEYMAP_FORMAT_TEXT_V1);
	if (!keymap)
		return 0;

	size_t size = strlen(keymap) + 1;
	int fd = neru_wlr_create_keymap_fd(keymap, size);
	if (fd < 0) {
		free(keymap);
		return 0;
	}

	zwp_virtual_keyboard_v1_keymap(c->vkeyboard, WL_KEYBOARD_KEYMAP_FORMAT_XKB_V1, fd, (uint32_t)size);
	close(fd);
	free(keymap);

	c->mod_shift = neru_wlr_mod_mask(xkb_keymap_mod_get_index(c->xkb_keymap, XKB_MOD_NAME_SHIFT));
	c->mod_ctrl = neru_wlr_mod_mask(xkb_keymap_mod_get_index(c->xkb_keymap, XKB_MOD_NAME_CTRL));
	c->mod_alt = neru_wlr_mod_mask(xkb_keymap_mod_get_index(c->xkb_keymap, XKB_MOD_NAME_ALT));
	c->mod_logo = neru_wlr_mod_mask(xkb_keymap_mod_get_index(c->xkb_keymap, XKB_MOD_NAME_LOGO));

	wl_display_flush(c->display);
	c->vkeyboard_ready = 1;

	return 1;
}

// ---------- xdg_output listener ----------

static void neru_xdg_output_logical_position(void *data, struct zxdg_output_v1 *xdg_output, int32_t x, int32_t y) {
	NeruWaylandScreen *scr = (NeruWaylandScreen *)data;
	scr->x = x;
	scr->y = y;
	scr->state |= 1;
}

static void neru_xdg_output_logical_size(void *data, struct zxdg_output_v1 *xdg_output, int32_t w, int32_t h) {
	NeruWaylandScreen *scr = (NeruWaylandScreen *)data;
	scr->w = w;
	scr->h = h;
	scr->state |= 2;
}

static void neru_xdg_output_done(void *data, struct zxdg_output_v1 *xdg_output) {
	// No-op for v3+.
}

static void neru_xdg_output_name(void *data, struct zxdg_output_v1 *xdg_output, const char *name) {
	NeruWaylandScreen *scr = (NeruWaylandScreen *)data;
	if (name) {
		strncpy(scr->name, name, sizeof(scr->name) - 1);
		scr->name[sizeof(scr->name) - 1] = '\0';
	}
	scr->state |= 4;
}

static void neru_xdg_output_description(void *data, struct zxdg_output_v1 *xdg_output, const char *description) {
	// No-op.
}

static const struct zxdg_output_v1_listener neru_xdg_output_listener = {
    .logical_position = neru_xdg_output_logical_position,
    .logical_size = neru_xdg_output_logical_size,
    .done = neru_xdg_output_done,
    .name = neru_xdg_output_name,
    .description = neru_xdg_output_description,
};

// ---------- Foreign-toplevel management ----------
//
// Wayland exposes no core protocol for "which application is focused". The
// wlr-foreign-toplevel-management protocol (implemented by wlroots compositors
// and KWin/KDE, but not Mutter/GNOME) fills the gap: the compositor streams a
// handle per toplevel window and marks exactly one with the ACTIVATED state.
// We track every handle and cache the activated one's app_id, which Neru uses
// as the focused application's bundle identifier for per-app configuration.
//
// All callbacks below run on the dispatch thread; they take toplevel_mutex to
// stay consistent with neru_wlr_focused_app_id, which reads under the same
// lock from a Go goroutine.

static void neru_wlr_toplevel_lock(NeruWlrootsClient *c) {
	if (c->toplevel_mutex_ready)
		pthread_mutex_lock(&c->toplevel_mutex);
}

static void neru_wlr_toplevel_unlock(NeruWlrootsClient *c) {
	if (c->toplevel_mutex_ready)
		pthread_mutex_unlock(&c->toplevel_mutex);
}

static NeruToplevel *neru_wlr_find_toplevel(NeruWlrootsClient *c, struct zwlr_foreign_toplevel_handle_v1 *handle) {
	NeruToplevel *t;
	wl_list_for_each(t, &c->toplevels, link) {
		if (t->handle == handle)
			return t;
	}
	return NULL;
}

// Recompute the cached focused app_id and title from the committed per-toplevel
// state. The last activated toplevel carrying a non-empty app_id wins; if none
// is activated the cache is cleared. Callers must hold toplevel_mutex.
static void neru_wlr_recompute_focused(NeruWlrootsClient *c) {
	// Snapshot the previous focused app_id so we only wake the Go watcher when
	// the focused application actually changes (not on every `done` commit).
	char prev[NERU_APP_ID_LEN];
	strncpy(prev, c->focused_app_id, NERU_APP_ID_LEN - 1);
	prev[NERU_APP_ID_LEN - 1] = '\0';

	const char *found = NULL;
	const char *found_title = NULL;
	NeruToplevel *t;
	wl_list_for_each(t, &c->toplevels, link) {
		if (t->activated && t->app_id[0] != '\0') {
			found = t->app_id;
			found_title = t->title;
		}
	}
	if (found) {
		strncpy(c->focused_app_id, found, NERU_APP_ID_LEN - 1);
		c->focused_app_id[NERU_APP_ID_LEN - 1] = '\0';
		strncpy(c->focused_title, found_title ? found_title : "", NERU_TITLE_LEN - 1);
		c->focused_title[NERU_TITLE_LEN - 1] = '\0';
	} else {
		c->focused_app_id[0] = '\0';
		c->focused_title[0] = '\0';
	}

	// Push a focus-change notification to Go. The write is non-blocking, so a
	// full pipe (a byte still unread) simply drops this one via EAGAIN — the
	// reader re-queries the live focused_app_id on the next wake regardless, so
	// no state is lost. Runs under toplevel_mutex; a non-blocking write never
	// stalls the dispatch thread.
	if (c->focus_pipe_ready && strcmp(prev, c->focused_app_id) != 0) {
		char b = 1;
		ssize_t n = write(c->focus_pipe[1], &b, 1);
		(void)n;
	}
}

static void neru_wlr_toplevel_title(void *data, struct zwlr_foreign_toplevel_handle_v1 *handle, const char *title) {
	NeruWlrootsClient *c = (NeruWlrootsClient *)data;
	if (!c || !title)
		return;
	neru_wlr_toplevel_lock(c);
	NeruToplevel *t = neru_wlr_find_toplevel(c, handle);
	if (t) {
		// Buffer until `done` so title commits atomically with app_id/state.
		strncpy(t->pending_title, title, NERU_TITLE_LEN - 1);
		t->pending_title[NERU_TITLE_LEN - 1] = '\0';
		t->has_pending_title = 1;
	}
	neru_wlr_toplevel_unlock(c);
}

static void neru_wlr_toplevel_app_id(void *data, struct zwlr_foreign_toplevel_handle_v1 *handle, const char *app_id) {
	NeruWlrootsClient *c = (NeruWlrootsClient *)data;
	if (!c || !app_id)
		return;
	neru_wlr_toplevel_lock(c);
	NeruToplevel *t = neru_wlr_find_toplevel(c, handle);
	if (t) {
		// Buffer until `done` so app_id and state commit atomically.
		strncpy(t->pending_app_id, app_id, NERU_APP_ID_LEN - 1);
		t->pending_app_id[NERU_APP_ID_LEN - 1] = '\0';
		t->has_pending_app_id = 1;
	}
	neru_wlr_toplevel_unlock(c);
}

static void neru_wlr_toplevel_output_enter(
    void *data, struct zwlr_foreign_toplevel_handle_v1 *handle, struct wl_output *output) {
	(void)data;
	(void)handle;
	(void)output;
}

static void neru_wlr_toplevel_output_leave(
    void *data, struct zwlr_foreign_toplevel_handle_v1 *handle, struct wl_output *output) {
	(void)data;
	(void)handle;
	(void)output;
}

static void neru_wlr_toplevel_state(
    void *data, struct zwlr_foreign_toplevel_handle_v1 *handle, struct wl_array *state) {
	NeruWlrootsClient *c = (NeruWlrootsClient *)data;
	if (!c)
		return;
	// The state event carries the full current state set each time, so derive
	// pending_activated fresh rather than toggling.
	int activated = 0;
	uint32_t *entry;
	wl_array_for_each(entry, state) {
		if (*entry == ZWLR_FOREIGN_TOPLEVEL_HANDLE_V1_STATE_ACTIVATED)
			activated = 1;
	}
	neru_wlr_toplevel_lock(c);
	NeruToplevel *t = neru_wlr_find_toplevel(c, handle);
	if (t)
		t->pending_activated = activated;
	neru_wlr_toplevel_unlock(c);
}

static void neru_wlr_toplevel_done(void *data, struct zwlr_foreign_toplevel_handle_v1 *handle) {
	NeruWlrootsClient *c = (NeruWlrootsClient *)data;
	if (!c)
		return;
	neru_wlr_toplevel_lock(c);
	NeruToplevel *t = neru_wlr_find_toplevel(c, handle);
	if (t) {
		if (t->has_pending_app_id) {
			strncpy(t->app_id, t->pending_app_id, NERU_APP_ID_LEN - 1);
			t->app_id[NERU_APP_ID_LEN - 1] = '\0';
			t->has_pending_app_id = 0;
		}
		if (t->has_pending_title) {
			strncpy(t->title, t->pending_title, NERU_TITLE_LEN - 1);
			t->title[NERU_TITLE_LEN - 1] = '\0';
			t->has_pending_title = 0;
		}
		t->activated = t->pending_activated;
		neru_wlr_recompute_focused(c);
	}
	neru_wlr_toplevel_unlock(c);
}

static void neru_wlr_toplevel_closed(void *data, struct zwlr_foreign_toplevel_handle_v1 *handle) {
	NeruWlrootsClient *c = (NeruWlrootsClient *)data;
	if (!c)
		return;
	neru_wlr_toplevel_lock(c);
	NeruToplevel *t = neru_wlr_find_toplevel(c, handle);
	if (t) {
		wl_list_remove(&t->link);
		free(t);
		neru_wlr_recompute_focused(c);
	}
	neru_wlr_toplevel_unlock(c);
	zwlr_foreign_toplevel_handle_v1_destroy(handle);
}

static void neru_wlr_toplevel_parent(
    void *data, struct zwlr_foreign_toplevel_handle_v1 *handle, struct zwlr_foreign_toplevel_handle_v1 *parent) {
	(void)data;
	(void)handle;
	(void)parent;
}

static const struct zwlr_foreign_toplevel_handle_v1_listener neru_wlr_toplevel_listener = {
    .title = neru_wlr_toplevel_title,
    .app_id = neru_wlr_toplevel_app_id,
    .output_enter = neru_wlr_toplevel_output_enter,
    .output_leave = neru_wlr_toplevel_output_leave,
    .state = neru_wlr_toplevel_state,
    .done = neru_wlr_toplevel_done,
    .closed = neru_wlr_toplevel_closed,
    .parent = neru_wlr_toplevel_parent,
};

static void neru_wlr_manager_toplevel(
    void *data, struct zwlr_foreign_toplevel_manager_v1 *manager, struct zwlr_foreign_toplevel_handle_v1 *handle) {
	NeruWlrootsClient *c = (NeruWlrootsClient *)data;
	(void)manager;
	if (!c || !handle)
		return;

	NeruToplevel *node = calloc(1, sizeof(*node));
	if (!node) {
		// Out of memory — drop tracking for this window rather than leak the
		// handle. Losing one window's focus signal is preferable to a crash.
		zwlr_foreign_toplevel_handle_v1_destroy(handle);
		return;
	}
	node->handle = handle;

	neru_wlr_toplevel_lock(c);
	wl_list_insert(&c->toplevels, &node->link);
	neru_wlr_toplevel_unlock(c);

	zwlr_foreign_toplevel_handle_v1_add_listener(handle, &neru_wlr_toplevel_listener, c);
}

static void neru_wlr_manager_finished(void *data, struct zwlr_foreign_toplevel_manager_v1 *manager) {
	NeruWlrootsClient *c = (NeruWlrootsClient *)data;
	(void)manager;
	if (!c)
		return;

	// The compositor has invalidated the manager, so every tracked handle and
	// the cached focus are now stale. Release each handle proxy (its own
	// `destroy` request — `finished` invalidates only the manager, not the
	// handles) and free the nodes so nothing leaks and focused-app queries stop
	// returning a dead window's app_id. Runs on the dispatch thread, serialized
	// with the handle callbacks; once destroyed a proxy dispatches no further
	// events, so a late `closed` is a safe no-op.
	neru_wlr_toplevel_lock(c);
	NeruToplevel *t;
	NeruToplevel *tmp;
	wl_list_for_each_safe(t, tmp, &c->toplevels, link) {
		wl_list_remove(&t->link);
		if (t->handle)
			zwlr_foreign_toplevel_handle_v1_destroy(t->handle);
		free(t);
	}
	c->focused_app_id[0] = '\0';
	c->focused_title[0] = '\0';
	neru_wlr_toplevel_unlock(c);

	// Do not destroy the manager proxy here: the server destroys it right after
	// `finished`, so sending its `stop` destructor request would target a dead
	// object. Just drop our reference; wl_display_disconnect reaps the proxy.
	c->toplevel_mgr = NULL;
}

static const struct zwlr_foreign_toplevel_manager_v1_listener neru_wlr_manager_listener = {
    .toplevel = neru_wlr_manager_toplevel,
    .finished = neru_wlr_manager_finished,
};

// ---------- Registry listener ----------

// neru_wlr_screen_signal pushes a display-configuration-change notification to
// Go. The write is non-blocking, so a full pipe (a byte still unread) drops this
// one via EAGAIN — the reader re-enumerates the live screen list on the next
// wake regardless, so no state is lost.
static void neru_wlr_screen_signal(NeruWlrootsClient *c) {
	if (c && c->screen_pipe_ready && c->screen_pipe[1] >= 0) {
		char b = 1;
		ssize_t n = write(c->screen_pipe[1], &b, 1);
		(void)n;
	}
}

// wl_output listener for the system client. Only `done` is meaningful: the
// compositor sends it after a batch of output property changes — including,
// per xdg-output v3, *after* the xdg_output logical geometry events. Signalling
// Go here (rather than from registry_global on bind) means a hotplugged output
// is re-enumerated once its geometry has actually arrived, not with zero bounds;
// it also fires on resolution/scale changes to an existing output. user_data is
// the stable client pointer, so array compaction never invalidates it.
static void neru_wlr_output_geometry(
    void *data, struct wl_output *output, int32_t x, int32_t y, int32_t phys_w, int32_t phys_h, int32_t subpixel,
    const char *make, const char *model, int32_t transform) {}

static void neru_wlr_output_mode(
    void *data, struct wl_output *output, uint32_t flags, int32_t width, int32_t height, int32_t refresh) {}

static void neru_wlr_output_scale(void *data, struct wl_output *output, int32_t factor) {}

static void neru_wlr_output_done(void *data, struct wl_output *output) {
	NeruWlrootsClient *c = (NeruWlrootsClient *)data;
	// Skip the initial discovery burst (connected == 0); that enumeration is
	// read synchronously by ensureWlrootsState. Only runtime changes wake Go.
	if (c && c->connected) {
		neru_wlr_screen_signal(c);
	}
}

static const struct wl_output_listener neru_wlr_output_listener = {
    .geometry = neru_wlr_output_geometry,
    .mode = neru_wlr_output_mode,
    .done = neru_wlr_output_done,
    .scale = neru_wlr_output_scale,
};

static void neru_wlr_registry_global(
    void *data, struct wl_registry *registry, uint32_t name, const char *interface, uint32_t version) {
	NeruWlrootsClient *c = (NeruWlrootsClient *)data;

	if (strcmp(interface, "zwlr_virtual_pointer_manager_v1") == 0) {
		c->vptr_mgr = wl_registry_bind(registry, name, &zwlr_virtual_pointer_manager_v1_interface, 1);
	} else if (strcmp(interface, "zwp_virtual_keyboard_manager_v1") == 0) {
		c->vkeyboard_mgr = wl_registry_bind(registry, name, &zwp_virtual_keyboard_manager_v1_interface, 1);
	} else if (strcmp(interface, "wl_compositor") == 0) {
		c->compositor = wl_registry_bind(registry, name, &wl_compositor_interface, 4);
	} else if (strcmp(interface, "zwlr_layer_shell_v1") == 0) {
		c->layer_shell = wl_registry_bind(registry, name, &zwlr_layer_shell_v1_interface, 1);
	} else if (strcmp(interface, "wl_shm") == 0) {
		c->shm = wl_registry_bind(registry, name, &wl_shm_interface, 1);
	} else if (strcmp(interface, "zwp_relative_pointer_manager_v1") == 0) {
		c->rel_ptr_mgr = wl_registry_bind(registry, name, &zwp_relative_pointer_manager_v1_interface, 1);
	} else if (strcmp(interface, "wl_seat") == 0) {
		c->seat = wl_registry_bind(registry, name, &wl_seat_interface, 7 < version ? 7 : version);
		// The pointer is taken from the capabilities event rather than here:
		// wl_seat.get_pointer on a seat that has never advertised a pointer is
		// a protocol error, and it kills the whole connection.  A seat with no
		// pointer yet is not hypothetical — a session with no input device
		// attached is one, and so is the headless compositor CI runs.
		wl_seat_add_listener(c->seat, &neru_wlr_seat_listener, c);
	} else if (strcmp(interface, "wl_output") == 0) {
		if (c->nr_screens < NERU_MAX_OUTPUTS) {
			NeruWaylandScreen *scr = &c->screens[c->nr_screens];
			memset(scr, 0, sizeof(*scr));
			scr->registry_name = name;
			scr->wl_output = wl_registry_bind(registry, name, &wl_output_interface, 3 < version ? 3 : version);
			// wl_output.done wakes Go once the output's geometry is current (both
			// on hotplug and on later resolution/scale changes) — see the listener.
			wl_output_add_listener(scr->wl_output, &neru_wlr_output_listener, c);
			// On runtime hotplug (after the initial connect) wire the new output's
			// xdg_output immediately so its logical geometry populates. During the
			// initial discovery roundtrip c->connected is still 0: xdg_output is set
			// up in bulk below. Go is woken by wl_output.done, not from here, so the
			// re-enumeration reads real geometry rather than a zero-area record.
			if (c->connected && c->xdg_output_mgr) {
				scr->xdg_output = zxdg_output_manager_v1_get_xdg_output(c->xdg_output_mgr, scr->wl_output);
				zxdg_output_v1_add_listener(scr->xdg_output, &neru_xdg_output_listener, scr);
			}
			c->nr_screens++;
		}
	} else if (strcmp(interface, "zxdg_output_manager_v1") == 0) {
		c->xdg_output_mgr =
		    wl_registry_bind(registry, name, &zxdg_output_manager_v1_interface, 3 < version ? 3 : version);
	} else if (strcmp(interface, "zwlr_foreign_toplevel_manager_v1") == 0) {
		c->toplevel_mgr =
		    wl_registry_bind(registry, name, &zwlr_foreign_toplevel_manager_v1_interface, 3 < version ? 3 : version);
	}
}

static void neru_wlr_registry_global_remove(void *data, struct wl_registry *registry, uint32_t name) {
	NeruWlrootsClient *c = (NeruWlrootsClient *)data;
	(void)registry;
	if (!c) {
		return;
	}

	// Find the output whose registry global id was just removed and drop it,
	// compacting the array so nr_screens stays dense. Destroy its xdg_output
	// (has a destructor request) and the wl_output proxy client-side, then wake
	// Go to re-enumerate the remaining outputs.
	for (int i = 0; i < c->nr_screens; i++) {
		if (c->screens[i].wl_output == NULL || c->screens[i].registry_name != name) {
			continue;
		}

		NeruWaylandScreen *scr = &c->screens[i];
		if (scr->xdg_output) {
			zxdg_output_v1_destroy(scr->xdg_output);
		}
		if (scr->wl_output) {
			wl_proxy_destroy((struct wl_proxy *)scr->wl_output);
		}

		for (int j = i; j < c->nr_screens - 1; j++) {
			c->screens[j] = c->screens[j + 1];
			// The moved screen's xdg_output listener still carries the old slot
			// address as user_data; re-point it to the new slot so later
			// logical_position/size/name events update the correct output rather
			// than a neighbor's (or the freed tail) slot.
			if (c->screens[j].xdg_output) {
				zxdg_output_v1_set_user_data(c->screens[j].xdg_output, &c->screens[j]);
			}
		}
		c->nr_screens--;
		memset(&c->screens[c->nr_screens], 0, sizeof(NeruWaylandScreen));

		neru_wlr_screen_signal(c);
		break;
	}
}

static const struct wl_registry_listener neru_wlr_registry_listener = {
    .global = neru_wlr_registry_global,
    .global_remove = neru_wlr_registry_global_remove,
};

// ---------- Dispatch thread ----------

static void *neru_wlr_dispatch_loop(void *arg) {
	NeruWlrootsClient *c = (NeruWlrootsClient *)arg;
	int connection_lost = 0;
	while (c->dispatch_running) {
		// Non-blocking prepare-read under lock
		pthread_mutex_lock(&c->display_mutex);
		if (wl_display_prepare_read(c->display) < 0) {
			wl_display_dispatch_pending(c->display);
			pthread_mutex_unlock(&c->display_mutex);
			continue;
		}
		pthread_mutex_unlock(&c->display_mutex);

		// Flush pending outgoing requests before blocking on poll
		// (libwayland-client protocol requirement).
		wl_display_flush(c->display);

		// Poll without lock (may block)
		struct pollfd pfd = {.fd = wl_display_get_fd(c->display), .events = POLLIN, .revents = 0};
		poll(&pfd, 1, -1);

		// Read and dispatch under lock
		pthread_mutex_lock(&c->display_mutex);
		if (pfd.revents & (POLLERR | POLLHUP)) {
			// Compositor connection broken (e.g. compositor killed).
			// Cancel the prepared read and exit the loop cleanly.
			// Do NOT clear dispatch_running — neru_wlr_disconnect
			// still needs to pthread_join this thread.
			wl_display_cancel_read(c->display);
			pthread_mutex_unlock(&c->display_mutex);
			connection_lost = 1;
			break;
		}
		if (pfd.revents & POLLIN) {
			if (wl_display_read_events(c->display) < 0) {
				pthread_mutex_unlock(&c->display_mutex);
				connection_lost = 1;
				break;
			}
			wl_display_dispatch_pending(c->display);
		} else {
			wl_display_cancel_read(c->display);
		}
		pthread_mutex_unlock(&c->display_mutex);
	}

	// If the compositor connection broke (not a clean disconnect, which exits
	// via dispatch_running == 0), close the focus pipe's write end so the Go
	// reader observes POLLHUP and restores its polling fallback rather than
	// silently degrading to the safety-sample interval. Set to -1 so
	// neru_wlr_disconnect's later close is a no-op. Safe without extra locking:
	// this thread is the only pipe writer and has stopped dispatching by here.
	if (connection_lost && c->focus_pipe[1] >= 0) {
		close(c->focus_pipe[1]);
		c->focus_pipe[1] = -1;
	}
	if (connection_lost && c->screen_pipe[1] >= 0) {
		close(c->screen_pipe[1]);
		c->screen_pipe[1] = -1;
	}

	return NULL;
}

int neru_wlr_start_dispatch(NeruWlrootsClient *c) {
	if (!c || c->dispatch_running)
		return 0;
	c->dispatch_running = 1;
	if (pthread_create(&c->dispatch_thread, NULL, neru_wlr_dispatch_loop, c) != 0) {
		c->dispatch_running = 0;
		return 0;
	}
	return 1;
}

// ---------- Connect & initialize ----------

NeruWlrootsClient *neru_wlr_connect(void) {
	NeruWlrootsClient *c = calloc(1, sizeof(NeruWlrootsClient));
	if (!c)
		return NULL;

	c->display = wl_display_connect(NULL);
	if (!c->display) {
		free(c);
		return NULL;
	}

	// Guards the foreign-toplevel bookkeeping; must be ready before the
	// manager listener starts delivering toplevel/handle events below. The
	// list head must be initialized before any toplevel event can insert.
	wl_list_init(&c->toplevels);
	pthread_mutex_init(&c->toplevel_mutex, NULL);
	c->toplevel_mutex_ready = 1;

	// Self-pipe for pushing focus changes to Go. Created before the
	// foreign-toplevel listener is bound below so the initial focus burst during
	// the discovery roundtrip is signaled too. calloc zeroed focus_pipe, but 0
	// is a valid fd, so reset to -1 before pipe() so cleanup can tell "unset"
	// from "fd 0". A failure here is non-fatal: focus_pipe_ready stays 0 and Go
	// falls back to polling.
	c->focus_pipe[0] = -1;
	c->focus_pipe[1] = -1;
	if (pipe(c->focus_pipe) == 0) {
		for (int i = 0; i < 2; i++) {
			int flags = fcntl(c->focus_pipe[i], F_GETFL, 0);
			if (flags != -1)
				fcntl(c->focus_pipe[i], F_SETFL, flags | O_NONBLOCK);
			int fdflags = fcntl(c->focus_pipe[i], F_GETFD, 0);
			if (fdflags != -1)
				fcntl(c->focus_pipe[i], F_SETFD, fdflags | FD_CLOEXEC);
		}
		c->focus_pipe_ready = 1;
	}

	// Self-pipe for pushing display-configuration changes to Go. Same lifecycle
	// and non-blocking semantics as focus_pipe above. Non-fatal on failure: Go
	// simply receives no hotplug events and the overlay follows the cursor as
	// before.
	c->screen_pipe[0] = -1;
	c->screen_pipe[1] = -1;
	if (pipe(c->screen_pipe) == 0) {
		for (int i = 0; i < 2; i++) {
			int flags = fcntl(c->screen_pipe[i], F_GETFL, 0);
			if (flags != -1)
				fcntl(c->screen_pipe[i], F_SETFL, flags | O_NONBLOCK);
			int fdflags = fcntl(c->screen_pipe[i], F_GETFD, 0);
			if (fdflags != -1)
				fcntl(c->screen_pipe[i], F_SETFD, fdflags | FD_CLOEXEC);
		}
		c->screen_pipe_ready = 1;
	}

	c->registry = wl_display_get_registry(c->display);
	wl_registry_add_listener(c->registry, &neru_wlr_registry_listener, c);

	// First roundtrip: discover globals.
	wl_display_roundtrip(c->display);

	// Create virtual pointer if manager was found.
	if (c->vptr_mgr) {
		c->vptr = zwlr_virtual_pointer_manager_v1_create_virtual_pointer(c->vptr_mgr, c->seat);
	}

	if (c->vkeyboard_mgr && c->seat) {
		c->vkeyboard = zwp_virtual_keyboard_manager_v1_create_virtual_keyboard(c->vkeyboard_mgr, c->seat);
		neru_wlr_setup_virtual_keyboard(c);
	}

	// Let the seat answer before anything reads c->pointer: the pointer is
	// taken from the capabilities event, and on a seat that had none the
	// virtual pointer created just above is what brings it into existence.
	wl_display_roundtrip(c->display);

	// Create relative pointer for tracking physical cursor motion. The seat
	// handler usually got here first — the roundtrip above is what lets it —
	// and this is the case where the manager arrived after the capability.
	neru_wlr_bind_relative_pointer(c);

	// Subscribe to foreign-toplevel events. Binding the manager makes the
	// compositor replay a `toplevel` event for every existing window; a
	// roundtrip drains that initial burst (plus each handle's app_id/state/
	// done) so the focused app_id is populated before the daemon needs it.
	if (c->toplevel_mgr) {
		zwlr_foreign_toplevel_manager_v1_add_listener(c->toplevel_mgr, &neru_wlr_manager_listener, c);
		wl_display_roundtrip(c->display);
	}

	// Initialize xdg_output for each screen.
	if (c->xdg_output_mgr) {
		for (int i = 0; i < c->nr_screens; i++) {
			NeruWaylandScreen *scr = &c->screens[i];
			scr->xdg_output = zxdg_output_manager_v1_get_xdg_output(c->xdg_output_mgr, scr->wl_output);
			zxdg_output_v1_add_listener(scr->xdg_output, &neru_xdg_output_listener, scr);
		}
		// Second roundtrip: receive xdg_output events.
		wl_display_roundtrip(c->display);
	}

	// Initialize display mutex. Dispatch thread is started later
	// via neru_wlr_start_dispatch() to avoid reader_count conflicts
	// with neru_wlr_init_cursor() which also does roundtrips.
	pthread_mutex_init(&c->display_mutex, NULL);

	c->connected = 1;
	return c;
}

void neru_wlr_disconnect(NeruWlrootsClient *c) {
	if (!c)
		return;

	// Stop the dispatch thread.
	int had_dispatch = c->dispatch_running;
	c->dispatch_running = 0;
	// Wake it up by sending a sync request so it exits the poll.
	pthread_mutex_lock(&c->display_mutex);
	if (c->display) {
		struct wl_callback *cb = wl_display_sync(c->display);
		wl_display_flush(c->display);
		if (cb)
			wl_callback_destroy(cb);
	}
	pthread_mutex_unlock(&c->display_mutex);
	if (had_dispatch)
		pthread_join(c->dispatch_thread, NULL);
	pthread_mutex_destroy(&c->display_mutex);

	// Close the focus self-pipe now that the dispatch thread (the only writer)
	// has joined, so no write can race the close. Closing the read end makes any
	// blocked Go reader observe EOF/POLLHUP.
	if (c->focus_pipe_ready) {
		if (c->focus_pipe[0] >= 0)
			close(c->focus_pipe[0]);
		if (c->focus_pipe[1] >= 0)
			close(c->focus_pipe[1]);
		c->focus_pipe[0] = -1;
		c->focus_pipe[1] = -1;
		c->focus_pipe_ready = 0;
	}

	if (c->screen_pipe_ready) {
		if (c->screen_pipe[0] >= 0)
			close(c->screen_pipe[0]);
		if (c->screen_pipe[1] >= 0)
			close(c->screen_pipe[1]);
		c->screen_pipe[0] = -1;
		c->screen_pipe[1] = -1;
		c->screen_pipe_ready = 0;
	}

	if (c->vptr) {
		zwlr_virtual_pointer_v1_destroy(c->vptr);
	}
	if (c->vkeyboard) {
		zwp_virtual_keyboard_v1_destroy(c->vkeyboard);
	}
	if (c->rel_ptr) {
		zwp_relative_pointer_v1_destroy(c->rel_ptr);
	}
	// Tear down foreign-toplevel tracking. The dispatch thread is already
	// joined at this point, so no callbacks can race these destroys. The list
	// may be empty (e.g. after a manager `finished` event cleared it).
	if (c->toplevel_mutex_ready) {
		NeruToplevel *t;
		NeruToplevel *tmp;
		wl_list_for_each_safe(t, tmp, &c->toplevels, link) {
			wl_list_remove(&t->link);
			if (t->handle)
				zwlr_foreign_toplevel_handle_v1_destroy(t->handle);
			free(t);
		}
	}
	if (c->toplevel_mgr) {
		zwlr_foreign_toplevel_manager_v1_destroy(c->toplevel_mgr);
		c->toplevel_mgr = NULL;
	}
	if (c->toplevel_mutex_ready) {
		pthread_mutex_destroy(&c->toplevel_mutex);
		c->toplevel_mutex_ready = 0;
	}
	if (c->xkb_keymap) {
		xkb_keymap_unref(c->xkb_keymap);
	}
	if (c->xkb_ctx) {
		xkb_context_unref(c->xkb_ctx);
	}
	for (int i = 0; i < c->nr_screens; i++) {
		if (c->screens[i].xdg_output) {
			zxdg_output_v1_destroy(c->screens[i].xdg_output);
		}
	}
	if (c->display) {
		wl_display_disconnect(c->display);
	}
	free(c);
}

int neru_wlr_refresh_cursor(NeruWlrootsClient *c) {
	if (!c || !c->layer_shell || !c->compositor || !c->shm || !c->pointer || c->nr_screens == 0)
		return 0;

	NeruCursorDiscoverySurface surfaces[NERU_MAX_OUTPUTS] = {0};
	int created = 0;
	int was_initialized = atomic_load(&c->cursor_initialized);
	int old_x = atomic_load(&c->cursor_x);
	int old_y = atomic_load(&c->cursor_y);

	atomic_store(&c->cursor_initialized, 0);

	pthread_mutex_lock(&c->display_mutex);
	for (int i = 0; i < c->nr_screens; i++) {
		NeruWaylandScreen *scr = &c->screens[i];
		if (!scr->wl_output || scr->w <= 0 || scr->h <= 0)
			continue;

		NeruCursorDiscoverySurface *surface = &surfaces[created];
		surface->client = c;
		surface->screen = scr;
		surface->width = scr->w;
		surface->height = scr->h;
		surface->surface = wl_compositor_create_surface(c->compositor);
		if (!surface->surface)
			continue;

		scr->discovery_surface = surface->surface;
		surface->layer_surface = zwlr_layer_shell_v1_get_layer_surface(
		    c->layer_shell, surface->surface, scr->wl_output, ZWLR_LAYER_SHELL_V1_LAYER_OVERLAY, "neru_cursor_sync");
		if (!surface->layer_surface) {
			scr->discovery_surface = NULL;
			wl_surface_destroy(surface->surface);
			surface->surface = NULL;
			continue;
		}

		zwlr_layer_surface_v1_set_size(surface->layer_surface, scr->w, scr->h);
		zwlr_layer_surface_v1_set_anchor(
		    surface->layer_surface, ZWLR_LAYER_SURFACE_V1_ANCHOR_TOP | ZWLR_LAYER_SURFACE_V1_ANCHOR_LEFT |
		                                ZWLR_LAYER_SURFACE_V1_ANCHOR_RIGHT | ZWLR_LAYER_SURFACE_V1_ANCHOR_BOTTOM);
		zwlr_layer_surface_v1_set_exclusive_zone(surface->layer_surface, -1);
		zwlr_layer_surface_v1_set_keyboard_interactivity(
		    surface->layer_surface, ZWLR_LAYER_SURFACE_V1_KEYBOARD_INTERACTIVITY_NONE);
		zwlr_layer_surface_v1_add_listener(surface->layer_surface, &neru_wlr_discovery_layer_listener, surface);
		wl_surface_commit(surface->surface);
		created++;
	}
	wl_display_flush(c->display);
	pthread_mutex_unlock(&c->display_mutex);

	if (created == 0) {
		if (was_initialized) {
			atomic_store(&c->cursor_x, old_x);
			atomic_store(&c->cursor_y, old_y);
			atomic_store(&c->cursor_initialized, 1);
		}
		return 0;
	}

	if (atomic_load(&c->dispatch_running)) {
		for (int attempt = 0; attempt < 50 && atomic_load(&c->cursor_initialized) == 0; attempt++) {
			usleep(2000);
		}
	} else {
		pthread_mutex_lock(&c->display_mutex);
		for (int attempt = 0; attempt < 4 && atomic_load(&c->cursor_initialized) == 0; attempt++) {
			wl_display_roundtrip(c->display);
		}
		pthread_mutex_unlock(&c->display_mutex);
	}

	int discovered = atomic_load(&c->cursor_initialized);

	pthread_mutex_lock(&c->display_mutex);
	for (int i = 0; i < created; i++) {
		if (surfaces[i].screen)
			surfaces[i].screen->discovery_surface = NULL;
		neru_wlr_discovery_destroy(&surfaces[i]);
	}
	wl_display_flush(c->display);
	pthread_mutex_unlock(&c->display_mutex);

	if (!discovered && was_initialized) {
		atomic_store(&c->cursor_x, old_x);
		atomic_store(&c->cursor_y, old_y);
		atomic_store(&c->cursor_initialized, 1);
	}

	return discovered;
}

// Initialize cursor position. Wayland has no global pointer-position query, so
// Neru briefly maps transparent layer-shell surfaces and reads wl_pointer.enter
// coordinates. If a compositor refuses discovery, startup falls back to the
// first screen center and later mode activations can try to refresh again.
void neru_wlr_init_cursor(NeruWlrootsClient *c) {
	if (!c || atomic_load(&c->cursor_initialized))
		return;

	if (neru_wlr_refresh_cursor(c))
		return;

	if (c->nr_screens > 0) {
		atomic_store(&c->cursor_x, c->screens[0].x + c->screens[0].w / 2);
		atomic_store(&c->cursor_y, c->screens[0].y + c->screens[0].h / 2);
		atomic_store(&c->cursor_initialized, 1);
	}
}

// ---------- Input injection ----------

int neru_wlr_move_absolute(NeruWlrootsClient *c, int x, int y) {
	if (!c || !c->vptr)
		return 0;

	// Compute the bounding box of all screens to get the virtual pointer extent.
	int minx = 0, miny = 0, maxx = 0, maxy = 0;
	for (int i = 0; i < c->nr_screens; i++) {
		NeruWaylandScreen *scr = &c->screens[i];
		if (i == 0 || scr->x < minx)
			minx = scr->x;
		if (i == 0 || scr->y < miny)
			miny = scr->y;
		int right = scr->x + scr->w;
		int bottom = scr->y + scr->h;
		if (i == 0 || right > maxx)
			maxx = right;
		if (i == 0 || bottom > maxy)
			maxy = bottom;
	}

	pthread_mutex_lock(&c->display_mutex);
	zwlr_virtual_pointer_v1_motion_absolute(
	    c->vptr, 0, wl_fixed_from_int(x - minx), wl_fixed_from_int(y - miny), wl_fixed_from_int(maxx - minx),
	    wl_fixed_from_int(maxy - miny));
	zwlr_virtual_pointer_v1_frame(c->vptr);
	wl_display_flush(c->display);
	c->cursor_x_frac = 0;
	c->cursor_y_frac = 0;
	pthread_mutex_unlock(&c->display_mutex);

	atomic_store(&c->cursor_x, x);
	atomic_store(&c->cursor_y, y);
	atomic_store(&c->cursor_initialized, 1);

	return 1;
}

int neru_wlr_move_relative(NeruWlrootsClient *c, int dx, int dy) {
	if (!c || !c->vptr)
		return 0;

	pthread_mutex_lock(&c->display_mutex);
	zwlr_virtual_pointer_v1_motion(c->vptr, 0, wl_fixed_from_int(dx), wl_fixed_from_int(dy));
	zwlr_virtual_pointer_v1_frame(c->vptr);
	wl_display_flush(c->display);
	pthread_mutex_unlock(&c->display_mutex);

	// Update cache synchronously — relative-motion events from the compositor
	// never reach us because this client never owns pointer focus (all our
	// surfaces have empty input regions after init). Without this, every
	// MoveCursorBy call drifts the cache further from the real cursor.
	atomic_fetch_add(&c->cursor_x, dx);
	atomic_fetch_add(&c->cursor_y, dy);
	atomic_store(&c->cursor_initialized, 1);

	return 1;
}

// Button codes for linux/input-event-codes.h
#define NERU_BTN_LEFT 0x110
#define NERU_BTN_RIGHT 0x111
#define NERU_BTN_MIDDLE 0x112

int neru_wlr_button(NeruWlrootsClient *c, int button, int pressed) {
	if (!c || !c->vptr)
		return 0;

	pthread_mutex_lock(&c->display_mutex);
	zwlr_virtual_pointer_v1_button(c->vptr, 0, (uint32_t)button, pressed ? 1 : 0);
	zwlr_virtual_pointer_v1_frame(c->vptr);
	wl_display_flush(c->display);
	pthread_mutex_unlock(&c->display_mutex);
	return 1;
}

int neru_wlr_click(NeruWlrootsClient *c, int button) {
	if (!c || !c->vptr)
		return 0;

	pthread_mutex_lock(&c->display_mutex);
	zwlr_virtual_pointer_v1_button(c->vptr, 0, (uint32_t)button, 1);
	zwlr_virtual_pointer_v1_button(c->vptr, 0, (uint32_t)button, 0);
	zwlr_virtual_pointer_v1_frame(c->vptr);
	wl_display_flush(c->display);
	pthread_mutex_unlock(&c->display_mutex);
	return 1;
}

int neru_wlr_scroll(NeruWlrootsClient *c, int axis, int delta, int discrete) {
	if (!c || !c->vptr)
		return 0;

	pthread_mutex_lock(&c->display_mutex);
	zwlr_virtual_pointer_v1_axis_source(c->vptr, 0);
	if (discrete != 0) {
		zwlr_virtual_pointer_v1_axis_discrete(c->vptr, 0, (uint32_t)axis, wl_fixed_from_int(delta), discrete);
	} else {
		zwlr_virtual_pointer_v1_axis(c->vptr, 0, (uint32_t)axis, wl_fixed_from_int(delta));
	}
	zwlr_virtual_pointer_v1_frame(c->vptr);
	wl_display_flush(c->display);
	pthread_mutex_unlock(&c->display_mutex);
	return 1;
}

int neru_wlr_scroll_continuous(NeruWlrootsClient *c, int axis, double value) {
	if (!c || !c->vptr)
		return 0;

	pthread_mutex_lock(&c->display_mutex);
	// Source "continuous" rather than "wheel": a wheel source tells the client
	// the value came off a detented device, and a toolkit is entitled to round
	// it back to detents on that word alone.  What this sends is a distance in
	// a continuous space, which is what the enum's continuous member means, and
	// unlike "finger" it implies no kinetic scrolling and needs no axis_stop.
	zwlr_virtual_pointer_v1_axis_source(c->vptr, WL_POINTER_AXIS_SOURCE_CONTINUOUS);
	// axis rather than axis_discrete: wlroots leaves delta_discrete at zero for
	// this request, and a zero delta_discrete is what makes the compositor send
	// the fraction on to the client instead of holding it in a v120 accumulator
	// until a whole notch adds up.
	zwlr_virtual_pointer_v1_axis(c->vptr, 0, (uint32_t)axis, wl_fixed_from_double(value));
	zwlr_virtual_pointer_v1_frame(c->vptr);
	wl_display_flush(c->display);
	pthread_mutex_unlock(&c->display_mutex);
	return 1;
}

int neru_wlr_scroll_batch(NeruWlrootsClient *c, int axis, int *deltas, int *discretes, int count) {
	if (!c || !c->vptr || !deltas || !discretes || count <= 0)
		return 0;

	pthread_mutex_lock(&c->display_mutex);
	for (int i = 0; i < count; i++) {
		zwlr_virtual_pointer_v1_axis_source(c->vptr, 0);
		zwlr_virtual_pointer_v1_axis_discrete(c->vptr, 0, (uint32_t)axis, wl_fixed_from_int(deltas[i]), discretes[i]);
		zwlr_virtual_pointer_v1_frame(c->vptr);
	}
	// Ignore flush return value — the events are queued in the client
	// output buffer and will be flushed by the dispatch loop.  Returning 0
	// on EAGAIN (transient buffer-full) is worse than ignoring it:
	// it would cause the entire batch to be reported as failed even though
	// delivery is guaranteed.
	wl_display_flush(c->display);
	pthread_mutex_unlock(&c->display_mutex);
	return 1;
}

static uint32_t neru_wlr_modifier_mask(NeruWlrootsClient *c, const char *modifier) {
	if (strcmp(modifier, "shift") == 0)
		return c->mod_shift;
	if (strcmp(modifier, "ctrl") == 0)
		return c->mod_ctrl;
	if (strcmp(modifier, "alt") == 0)
		return c->mod_alt;
	if (strcmp(modifier, "cmd") == 0)
		return c->mod_logo;
	return 0;
}

int neru_wlr_modifier_event(NeruWlrootsClient *c, const char *modifier, int is_down) {
	if (!c || !c->vkeyboard || !c->vkeyboard_ready)
		return 0;

	uint32_t mask = neru_wlr_modifier_mask(c, modifier);
	if (mask == 0)
		return 0;

	if (is_down) {
		c->depressed_mods |= mask;
	} else {
		c->depressed_mods &= ~mask;
	}

	pthread_mutex_lock(&c->display_mutex);
	zwp_virtual_keyboard_v1_modifiers(c->vkeyboard, c->depressed_mods, 0, 0, 0);
	wl_display_flush(c->display);
	pthread_mutex_unlock(&c->display_mutex);

	return 1;
}

// ---------- Request barrier ----------

// neru_wlr_sync_request is one outstanding wl_display.sync. The callback owns
// it and frees it, so a waiter that gives up early leaves nothing dangling.
typedef struct {
	NeruWlrootsClient *client;
	unsigned int id;
} NeruWlrootsSyncRequest;

static void neru_wlr_sync_done(void *data, struct wl_callback *cb, uint32_t serial) {
	(void)serial;
	NeruWlrootsSyncRequest *req = (NeruWlrootsSyncRequest *)data;
	// Callbacks are answered in issue order, so storing rather than maxing is
	// monotonic: id here is always at least the one stored before it.
	atomic_store(&req->client->sync_completed, req->id);
	wl_callback_destroy(cb);
	free(req);
}

static const struct wl_callback_listener neru_wlr_sync_listener = {
    .done = neru_wlr_sync_done,
};

// neru_wlr_sync_poll_interval_us is how often the waiter checks. Two hundred
// microseconds is well under the round trip it is waiting on and coarse enough
// not to spin.
#define NERU_WLR_SYNC_POLL_INTERVAL_US 200

int neru_wlr_sync(NeruWlrootsClient *c, int timeout_ms) {
	if (!c || !c->display || timeout_ms <= 0)
		return 0;

	// With no dispatch thread running, nothing would ever read the answer, so
	// the waiter has to do the reading itself. This is the same split
	// neru_wlr_refresh_cursor makes: roundtrip when this thread owns the
	// display, poll an atomic when the dispatch thread does — calling
	// wl_display_roundtrip while that thread sits in wl_display_prepare_read is
	// what the split exists to avoid.
	if (!atomic_load(&c->dispatch_running)) {
		pthread_mutex_lock(&c->display_mutex);
		int rc = wl_display_roundtrip(c->display);
		pthread_mutex_unlock(&c->display_mutex);
		return rc >= 0;
	}

	NeruWlrootsSyncRequest *req = calloc(1, sizeof(*req));
	if (!req)
		return 0;

	// want is read here and never again: the moment the mutex is dropped the
	// dispatch thread may answer this sync, and the callback frees req.
	pthread_mutex_lock(&c->display_mutex);
	req->client = c;
	req->id = ++c->sync_issued;
	unsigned int want = req->id;
	struct wl_callback *cb = wl_display_sync(c->display);
	if (cb)
		wl_callback_add_listener(cb, &neru_wlr_sync_listener, req);
	wl_display_flush(c->display);
	if (!cb) {
		free(req);
		pthread_mutex_unlock(&c->display_mutex);
		return 0;
	}
	pthread_mutex_unlock(&c->display_mutex);

	for (int waited = 0; waited < timeout_ms * 1000; waited += NERU_WLR_SYNC_POLL_INTERVAL_US) {
		// Unsigned subtraction so a wrapped counter still compares correctly.
		if ((int)(atomic_load(&c->sync_completed) - want) >= 0)
			return 1;
		usleep(NERU_WLR_SYNC_POLL_INTERVAL_US);
	}

	return (int)(atomic_load(&c->sync_completed) - want) >= 0;
}

int neru_wlr_get_cursor(NeruWlrootsClient *c, int *x, int *y) {
	if (!c)
		return 0;
	*x = atomic_load(&c->cursor_x);
	*y = atomic_load(&c->cursor_y);
	return atomic_load(&c->cursor_initialized);
}

// Update the cached cursor position when input is injected outside the
// virtual-pointer protocol (e.g. libei on KDE) so neru_wlr_get_cursor stays
// consistent with the last known compositor coordinates.
void neru_wlr_set_cursor(NeruWlrootsClient *c, int x, int y) {
	if (!c)
		return;
	atomic_store(&c->cursor_x, x);
	atomic_store(&c->cursor_y, y);
	atomic_store(&c->cursor_initialized, 1);
}

int neru_wlr_screen_count(NeruWlrootsClient *c) {
	if (!c)
		return 0;

	// The dispatch thread mutates nr_screens/screens[] (registry add/remove and
	// xdg_output geometry callbacks) while holding display_mutex — it runs the
	// wayland callbacks inside wl_display_dispatch_pending under that lock. Take
	// the same lock here so a hotplug re-enumeration from Go never reads a torn
	// count. The critical section touches no wayland call, so it cannot deadlock
	// with the dispatch thread.
	pthread_mutex_lock(&c->display_mutex);
	int count = c->nr_screens;
	pthread_mutex_unlock(&c->display_mutex);

	return count;
}

int neru_wlr_screen_info(NeruWlrootsClient *c, int idx, int *x, int *y, int *w, int *h, char *name_out, int name_len) {
	if (!c || idx < 0)
		return 0;

	// Read screens[idx] under the same display_mutex the dispatch thread holds
	// while writing it (see neru_wlr_screen_count). The index is re-validated
	// inside the lock because a concurrent hotplug-remove can shrink nr_screens.
	pthread_mutex_lock(&c->display_mutex);
	if (idx >= c->nr_screens) {
		pthread_mutex_unlock(&c->display_mutex);

		return 0;
	}

	NeruWaylandScreen *scr = &c->screens[idx];
	*x = scr->x;
	*y = scr->y;
	*w = scr->w;
	*h = scr->h;
	strncpy(name_out, scr->name, (size_t)(name_len - 1));
	name_out[name_len - 1] = '\0';
	pthread_mutex_unlock(&c->display_mutex);

	return 1;
}

int neru_wlr_has_virtual_pointer(NeruWlrootsClient *c) { return c && c->vptr != NULL; }

int neru_wlr_has_virtual_keyboard(NeruWlrootsClient *c) { return c && c->vkeyboard != NULL && c->vkeyboard_ready; }

int neru_wlr_key(NeruWlrootsClient *c, uint32_t keycode, int pressed) {
	if (!c || !c->vkeyboard || !c->vkeyboard_ready)
		return 0;

	struct timespec ts;
	clock_gettime(CLOCK_MONOTONIC, &ts);
	uint32_t time = (uint32_t)(ts.tv_sec * 1000 + ts.tv_nsec / 1000000);

	pthread_mutex_lock(&c->display_mutex);
	zwp_virtual_keyboard_v1_key(c->vkeyboard, time, keycode, pressed ? 1 : 0);
	wl_display_flush(c->display);
	pthread_mutex_unlock(&c->display_mutex);
	return 1;
}

int neru_wlr_has_toplevel_manager(NeruWlrootsClient *c) { return c && c->toplevel_mgr != NULL; }

int neru_wlr_focused_app_id(NeruWlrootsClient *c, char *out, int out_len) {
	if (!c || !out || out_len <= 0)
		return 0;

	int ok = 0;
	neru_wlr_toplevel_lock(c);
	if (c->focused_app_id[0] != '\0') {
		strncpy(out, c->focused_app_id, (size_t)(out_len - 1));
		out[out_len - 1] = '\0';
		ok = 1;
	} else {
		out[0] = '\0';
	}
	neru_wlr_toplevel_unlock(c);
	return ok;
}

int neru_wlr_focused_app_identity(NeruWlrootsClient *c, char *app_out, int app_len, char *title_out, int title_len) {
	if (!c || !app_out || app_len <= 0 || !title_out || title_len <= 0)
		return 0;

	int ok = 0;
	// Read app_id and title under a single lock so they always describe the same
	// toplevel; a focus commit cannot interleave between them.
	neru_wlr_toplevel_lock(c);
	if (c->focused_app_id[0] != '\0') {
		strncpy(app_out, c->focused_app_id, (size_t)(app_len - 1));
		app_out[app_len - 1] = '\0';
		strncpy(title_out, c->focused_title, (size_t)(title_len - 1));
		title_out[title_len - 1] = '\0';
		ok = 1;
	} else {
		app_out[0] = '\0';
		title_out[0] = '\0';
	}
	neru_wlr_toplevel_unlock(c);
	return ok;
}

int neru_wlr_focus_event_fd(NeruWlrootsClient *c) {
	if (!c || !c->focus_pipe_ready)
		return -1;
	return c->focus_pipe[0];
}

int neru_wlr_screen_event_fd(NeruWlrootsClient *c) {
	if (!c || !c->screen_pipe_ready)
		return -1;
	return c->screen_pipe[0];
}
