#include <errno.h>
#include <fcntl.h>
#include <poll.h>
#include <pthread.h>
#include <stdatomic.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <sys/syscall.h>
#include <time.h>
#include <unistd.h>
#include <wayland-client.h>
#include <xkbcommon/xkbcommon.h>

// Include wlroots protocol headers relative to this package.
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
				atomic_store(&c->cursor_x, scr->x + wl_fixed_to_int(sx));
				atomic_store(&c->cursor_y, scr->y + wl_fixed_to_int(sy));
				c->entered_discovery_surface = surface;
				atomic_store(&c->cursor_initialized, 1);
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
	if (c && c->vptr && c->entered_discovery_surface) {
		for (int i = 0; i < c->nr_screens; i++) {
			NeruWaylandScreen *scr = &c->screens[i];
			if (scr->discovery_surface == c->entered_discovery_surface) {
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
				wl_fixed_t gx = wl_fixed_from_int(scr->x) + sx;
				wl_fixed_t gy = wl_fixed_from_int(scr->y) + sy;
				zwlr_virtual_pointer_v1_motion_absolute(
				    c->vptr, time, gx - wl_fixed_from_int(minx), gy - wl_fixed_from_int(miny),
				    wl_fixed_from_int(maxx - minx), wl_fixed_from_int(maxy - miny));
				zwlr_virtual_pointer_v1_frame(c->vptr);
				break;
			}
		}
	}
	(void)pointer;
}

static void neru_wlr_pointer_button(
    void *data, struct wl_pointer *pointer, uint32_t serial, uint32_t time, uint32_t button, uint32_t state) {
	NeruWlrootsClient *c = (NeruWlrootsClient *)data;
	if (c && c->vptr && c->entered_discovery_surface) {
		zwlr_virtual_pointer_v1_button(c->vptr, time, button, state);
		zwlr_virtual_pointer_v1_frame(c->vptr);
	}
	(void)pointer;
}

static void neru_wlr_pointer_axis(
    void *data, struct wl_pointer *pointer, uint32_t time, uint32_t axis, wl_fixed_t value) {
	NeruWlrootsClient *c = (NeruWlrootsClient *)data;
	if (c && c->vptr && c->entered_discovery_surface) {
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

static int neru_wlr_create_shm_file(off_t size) {
	int ret;

#ifdef __NR_memfd_create
	int fd = syscall(__NR_memfd_create, "neru-cursor-discovery-shm", 0);
	if (fd >= 0) {
		do {
			ret = ftruncate(fd, size);
		} while (ret < 0 && errno == EINTR);
		if (ret >= 0)
			return fd;
		close(fd);
	}
#endif

	char name[] = "/tmp/neru-cursor-discovery-XXXXXX";
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
	int fd = neru_wlr_create_shm_file((off_t)surface->shm_size);
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

// ---------- Registry listener ----------

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
		c->pointer = wl_seat_get_pointer(c->seat);
		wl_pointer_add_listener(c->pointer, &neru_wlr_pointer_listener, c);
	} else if (strcmp(interface, "wl_output") == 0) {
		if (c->nr_screens < NERU_MAX_OUTPUTS) {
			NeruWaylandScreen *scr = &c->screens[c->nr_screens];
			memset(scr, 0, sizeof(*scr));
			scr->wl_output = wl_registry_bind(registry, name, &wl_output_interface, 3 < version ? 3 : version);
			c->nr_screens++;
		}
	} else if (strcmp(interface, "zxdg_output_manager_v1") == 0) {
		c->xdg_output_mgr =
		    wl_registry_bind(registry, name, &zxdg_output_manager_v1_interface, 3 < version ? 3 : version);
	}
}

static void neru_wlr_registry_global_remove(void *data, struct wl_registry *registry, uint32_t name) {
	// TODO: handle hotplug.
}

static const struct wl_registry_listener neru_wlr_registry_listener = {
    .global = neru_wlr_registry_global,
    .global_remove = neru_wlr_registry_global_remove,
};

// ---------- Dispatch thread ----------

static void *neru_wlr_dispatch_loop(void *arg) {
	NeruWlrootsClient *c = (NeruWlrootsClient *)arg;
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
			break;
		}
		if (pfd.revents & POLLIN) {
			if (wl_display_read_events(c->display) < 0) {
				pthread_mutex_unlock(&c->display_mutex);
				break;
			}
			wl_display_dispatch_pending(c->display);
		} else {
			wl_display_cancel_read(c->display);
		}
		pthread_mutex_unlock(&c->display_mutex);
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

	// Create relative pointer for tracking physical cursor motion.
	if (c->rel_ptr_mgr && c->pointer) {
		c->rel_ptr = zwp_relative_pointer_manager_v1_get_relative_pointer(c->rel_ptr_mgr, c->pointer);
		zwp_relative_pointer_v1_add_listener(c->rel_ptr, &neru_wlr_relative_pointer_listener, c);
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

	if (c->vptr) {
		zwlr_virtual_pointer_v1_destroy(c->vptr);
	}
	if (c->vkeyboard) {
		zwp_virtual_keyboard_v1_destroy(c->vkeyboard);
	}
	if (c->rel_ptr) {
		zwp_relative_pointer_v1_destroy(c->rel_ptr);
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
	return c->nr_screens;
}

int neru_wlr_screen_info(NeruWlrootsClient *c, int idx, int *x, int *y, int *w, int *h, char *name_out, int name_len) {
	if (!c || idx < 0 || idx >= c->nr_screens)
		return 0;
	NeruWaylandScreen *scr = &c->screens[idx];
	*x = scr->x;
	*y = scr->y;
	*w = scr->w;
	*h = scr->h;
	strncpy(name_out, scr->name, (size_t)(name_len - 1));
	name_out[name_len - 1] = '\0';
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
