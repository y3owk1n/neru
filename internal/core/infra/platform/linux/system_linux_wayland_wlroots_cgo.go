//go:build linux && cgo

package linux

/*
#cgo linux pkg-config: wayland-client xkbcommon
#cgo linux CFLAGS: -DWLR_CPLUSPLUS
#include <wayland-client.h>
#include <xkbcommon/xkbcommon.h>
#include <errno.h>
#include <fcntl.h>
#include <poll.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

// Include wlroots protocol headers relative to this package.
#include "wlr_protocol/virtual-pointer.h"
#include "wlr_protocol/virtual-keyboard.h"
#include "wlr_protocol/xdg-output.h"
#include "wlr_protocol/layer-shell.h"
#include "wlr_protocol/xdg-shell.h"

// ---------- Forward declarations ----------

#define NERU_MAX_OUTPUTS 16

// Forward declare NeruWlrootsClient (defined after NeruWaylandScreen)
typedef struct NeruWlrootsClient NeruWlrootsClient;

// Forward declare the cursor init function
static void neru_wlr_init_cursor(NeruWlrootsClient *c);

typedef struct {
	int x;
	int y;
	int w;
	int h;
	int state; // bitmask: 1=position, 2=size, 4=name
	char name[128];
	char name_valid;
	struct wl_output *wl_output;
	struct zxdg_output_v1 *xdg_output;

	struct wl_surface *discovery_surface;
} NeruWaylandScreen;

typedef struct NeruWlrootsClient {
	struct wl_display *display;
	struct wl_registry *registry;
	struct wl_compositor *compositor;
	struct wl_shm *shm;
	struct zwlr_layer_shell_v1 *layer_shell;
	struct wl_seat *seat;
	struct wl_pointer *pointer;

	struct zwlr_virtual_pointer_manager_v1 *vptr_mgr;
	struct zwlr_virtual_pointer_v1 *vptr;
	struct zwp_virtual_keyboard_manager_v1 *vkeyboard_mgr;
	struct zwp_virtual_keyboard_v1 *vkeyboard;
	int vkeyboard_ready;
	struct zxdg_output_manager_v1 *xdg_output_mgr;

	struct xkb_context *xkb_ctx;
	struct xkb_keymap *xkb_keymap;
	uint32_t mod_shift;
	uint32_t mod_ctrl;
	uint32_t mod_alt;
	uint32_t mod_logo;
	uint32_t depressed_mods;

	NeruWaylandScreen screens[NERU_MAX_OUTPUTS];
	int nr_screens;

	// Cursor position cache (updated ONLY by neru_wlr_move_absolute).
	// Wayland has no protocol to query global pointer position, so we must
	// track it client-side. Pointer enter/motion events give surface-local
	// coords that are meaningless for global tracking.
	int cursor_x;
	int cursor_y;
	int cursor_initialized;

	int connected;
} NeruWlrootsClient;

// Pointer listener callbacks — update cursor cache.
static void neru_wlr_pointer_enter(void *data,
	struct wl_pointer *pointer,
	uint32_t serial,
	struct wl_surface *surface,
	wl_fixed_t sx, wl_fixed_t sy)
{
	NeruWlrootsClient *c = (NeruWlrootsClient *)data;
	if (c && c->cursor_initialized == 0) {
		// During discovery phase, record the global pointer location based on
		// which screen triggered the enter event and surface-local coordinates.
		for (int i = 0; i < c->nr_screens; i++) {
			NeruWaylandScreen *scr = &c->screens[i];
			if (surface != NULL && surface == scr->discovery_surface) {
				c->cursor_x = scr->x + wl_fixed_to_int(sx);
				c->cursor_y = scr->y + wl_fixed_to_int(sy);
				c->cursor_initialized = 1;
				break;
			}
		}
	}
}

static void neru_wlr_pointer_leave(void *data,
	struct wl_pointer *pointer,
	uint32_t serial,
	struct wl_surface *surface)
{
	// No-op.
}

static void neru_wlr_pointer_motion(void *data,
	struct wl_pointer *pointer,
	uint32_t time,
	wl_fixed_t sx, wl_fixed_t sy)
{
	// No-op. Motion events give surface-local coords which would corrupt
	// the global cursor position tracked via neru_wlr_move_absolute.
	// This was the root cause of the "stale cache" bug: poll_cursor()
	// dispatched events which triggered this handler, overwriting the
	// correctly-set global position with meaningless surface-local coords.
	(void)data; (void)pointer; (void)time; (void)sx; (void)sy;
}

static void neru_wlr_pointer_button(void *data,
	struct wl_pointer *pointer,
	uint32_t serial,
	uint32_t time,
	uint32_t button,
	uint32_t state)
{
	// No-op.
}

static void neru_wlr_pointer_axis(void *data,
	struct wl_pointer *pointer,
	uint32_t time,
	uint32_t axis,
	wl_fixed_t value)
{
	// No-op.
}

static void neru_wlr_pointer_frame(void *data,
	struct wl_pointer *pointer)
{
	// No-op.
}

static void neru_wlr_pointer_axis_source(void *data,
	struct wl_pointer *pointer,
	uint32_t axis_source)
{
	// No-op.
}

static void neru_wlr_pointer_axis_stop(void *data,
	struct wl_pointer *pointer,
	uint32_t time,
	uint32_t axis)
{
	// No-op.
}

static void neru_wlr_pointer_axis_discrete(void *data,
	struct wl_pointer *pointer,
	uint32_t axis,
	int32_t discrete)
{
	// No-op.
}

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

static int neru_wlr_create_keymap_fd(const char *keymap, size_t size) {
	char template[] = "/tmp/neru-vkbd-keymap-XXXXXX";
	int fd = mkstemp(template);
	if (fd < 0) return -1;

	unlink(template);

	size_t written = 0;
	while (written < size) {
		ssize_t ret = write(fd, keymap + written, size - written);
		if (ret < 0) {
			if (errno == EINTR) continue;
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
	if (idx == XKB_MOD_INVALID || idx >= 32) return 0;
	return 1u << idx;
}

static int neru_wlr_setup_virtual_keyboard(NeruWlrootsClient *c) {
	if (!c || !c->vkeyboard) return 0;

	c->xkb_ctx = xkb_context_new(XKB_CONTEXT_NO_FLAGS);
	if (!c->xkb_ctx) return 0;

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

	c->xkb_keymap = xkb_keymap_new_from_names(
		c->xkb_ctx,
		&names,
		XKB_KEYMAP_COMPILE_NO_FLAGS
	);
	if (!c->xkb_keymap) return 0;

	char *keymap = xkb_keymap_get_as_string(c->xkb_keymap, XKB_KEYMAP_FORMAT_TEXT_V1);
	if (!keymap) return 0;

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

static void neru_xdg_output_logical_position(void *data,
	struct zxdg_output_v1 *xdg_output,
	int32_t x, int32_t y)
{
	NeruWaylandScreen *scr = (NeruWaylandScreen *)data;
	scr->x = x;
	scr->y = y;
	scr->state |= 1;
}

static void neru_xdg_output_logical_size(void *data,
	struct zxdg_output_v1 *xdg_output,
	int32_t w, int32_t h)
{
	NeruWaylandScreen *scr = (NeruWaylandScreen *)data;
	scr->w = w;
	scr->h = h;
	scr->state |= 2;
}

static void neru_xdg_output_done(void *data,
	struct zxdg_output_v1 *xdg_output)
{
	// No-op for v3+.
}

static void neru_xdg_output_name(void *data,
	struct zxdg_output_v1 *xdg_output,
	const char *name)
{
	NeruWaylandScreen *scr = (NeruWaylandScreen *)data;
	if (name) {
		strncpy(scr->name, name, sizeof(scr->name) - 1);
		scr->name[sizeof(scr->name) - 1] = '\0';
	}
	scr->state |= 4;
}

static void neru_xdg_output_description(void *data,
	struct zxdg_output_v1 *xdg_output,
	const char *description)
{
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

static void neru_wlr_registry_global(void *data,
	struct wl_registry *registry,
	uint32_t name, const char *interface,
	uint32_t version)
{
	NeruWlrootsClient *c = (NeruWlrootsClient *)data;

	if (strcmp(interface, "zwlr_virtual_pointer_manager_v1") == 0) {
		c->vptr_mgr = wl_registry_bind(registry, name,
			&zwlr_virtual_pointer_manager_v1_interface, 1);
	} else if (strcmp(interface, "zwp_virtual_keyboard_manager_v1") == 0) {
		c->vkeyboard_mgr = wl_registry_bind(registry, name,
			&zwp_virtual_keyboard_manager_v1_interface, 1);
	} else if (strcmp(interface, "wl_compositor") == 0) {
		c->compositor = wl_registry_bind(registry, name,
			&wl_compositor_interface, 4);
	} else if (strcmp(interface, "zwlr_layer_shell_v1") == 0) {
		c->layer_shell = wl_registry_bind(registry, name,
			&zwlr_layer_shell_v1_interface, 1);
	} else if (strcmp(interface, "wl_shm") == 0) {
		c->shm = wl_registry_bind(registry, name,
			&wl_shm_interface, 1);
	} else if (strcmp(interface, "wl_seat") == 0) {
		c->seat = wl_registry_bind(registry, name,
			&wl_seat_interface, 7 < version ? 7 : version);
		c->pointer = wl_seat_get_pointer(c->seat);
		wl_pointer_add_listener(c->pointer,
			&neru_wlr_pointer_listener, c);
	} else if (strcmp(interface, "wl_output") == 0) {
		if (c->nr_screens < NERU_MAX_OUTPUTS) {
			NeruWaylandScreen *scr = &c->screens[c->nr_screens];
			memset(scr, 0, sizeof(*scr));
			scr->wl_output = wl_registry_bind(registry, name,
				&wl_output_interface, 3 < version ? 3 : version);
			c->nr_screens++;
		}
	} else if (strcmp(interface, "zxdg_output_manager_v1") == 0) {
		c->xdg_output_mgr = wl_registry_bind(registry, name,
			&zxdg_output_manager_v1_interface, 3 < version ? 3 : version);
	}
}

static void neru_wlr_registry_global_remove(void *data,
	struct wl_registry *registry,
	uint32_t name)
{
	// TODO: handle hotplug.
}

static const struct wl_registry_listener neru_wlr_registry_listener = {
	.global = neru_wlr_registry_global,
	.global_remove = neru_wlr_registry_global_remove,
};

// ---------- Connect & initialize ----------

static NeruWlrootsClient* neru_wlr_connect(void) {
	NeruWlrootsClient *c = calloc(1, sizeof(NeruWlrootsClient));
	if (!c) return NULL;

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
		c->vptr = zwlr_virtual_pointer_manager_v1_create_virtual_pointer(
			c->vptr_mgr, c->seat);
	}

	if (c->vkeyboard_mgr && c->seat) {
		c->vkeyboard = zwp_virtual_keyboard_manager_v1_create_virtual_keyboard(
			c->vkeyboard_mgr, c->seat);
		neru_wlr_setup_virtual_keyboard(c);
	}

	// Initialize xdg_output for each screen.
	if (c->xdg_output_mgr) {
		for (int i = 0; i < c->nr_screens; i++) {
			NeruWaylandScreen *scr = &c->screens[i];
			scr->xdg_output = zxdg_output_manager_v1_get_xdg_output(
				c->xdg_output_mgr, scr->wl_output);
			zxdg_output_v1_add_listener(scr->xdg_output,
				&neru_xdg_output_listener, scr);
		}
		// Second roundtrip: receive xdg_output events.
		wl_display_roundtrip(c->display);
	}

	c->connected = 1;
	return c;
}

static void neru_wlr_disconnect(NeruWlrootsClient *c) {
	if (!c) return;

	if (c->vptr) {
		zwlr_virtual_pointer_v1_destroy(c->vptr);
	}
	if (c->vkeyboard) {
		zwp_virtual_keyboard_v1_destroy(c->vkeyboard);
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

// Initialize cursor position to the center of the screen containing the
// cursor (or first screen). Wayland has no protocol to query global
// pointer position, so we initialize to the center and then track
// position purely client-side via neru_wlr_move_absolute (matching
// warpd's pattern where ptr.x/ptr.y are only set by way_mouse_move).
static void neru_wlr_init_cursor(NeruWlrootsClient *c) {
	if (!c || c->cursor_initialized) return;

	// Use Warpd's cursor discovery trick: create invisible full-screen layer-shell surfaces
	// across all outputs, wiggle the virtual pointer, and capture the pointer_enter event.
	if (!c->layer_shell || !c->compositor || !c->pointer || !c->vptr || c->nr_screens == 0) {
		// Fallback to screen center
		c->cursor_x = c->screens[0].x + c->screens[0].w / 2;
		c->cursor_y = c->screens[0].y + c->screens[0].h / 2;
		c->cursor_initialized = 1;
		return;
	}

	struct zwlr_layer_surface_v1 *layer_surfaces[NERU_MAX_OUTPUTS] = {0};

	for (int i = 0; i < c->nr_screens; i++) {
		c->screens[i].discovery_surface = wl_compositor_create_surface(c->compositor);
		// Do not set input region, allowing it to intercept pointer events
		layer_surfaces[i] = zwlr_layer_shell_v1_get_layer_surface(
			c->layer_shell, c->screens[i].discovery_surface, c->screens[i].wl_output,
			ZWLR_LAYER_SHELL_V1_LAYER_OVERLAY, "neru_discovery"
		);
		zwlr_layer_surface_v1_set_size(layer_surfaces[i], c->screens[i].w, c->screens[i].h);
		zwlr_layer_surface_v1_set_anchor(layer_surfaces[i],
			ZWLR_LAYER_SURFACE_V1_ANCHOR_TOP | ZWLR_LAYER_SURFACE_V1_ANCHOR_LEFT |
			ZWLR_LAYER_SURFACE_V1_ANCHOR_RIGHT | ZWLR_LAYER_SURFACE_V1_ANCHOR_BOTTOM);
		zwlr_layer_surface_v1_set_exclusive_zone(layer_surfaces[i], -1);
		wl_surface_commit(c->screens[i].discovery_surface);
	}

	wl_display_roundtrip(c->display);

	// Animate the pointer to force an enter event
	zwlr_virtual_pointer_v1_motion(c->vptr, 0, wl_fixed_from_int(1), wl_fixed_from_int(1));
	zwlr_virtual_pointer_v1_frame(c->vptr);
	wl_display_flush(c->display);

	// Poll until initialized or timeout (~50ms)
	struct pollfd pfd = { .fd = wl_display_get_fd(c->display), .events = POLLIN };
	for (int attempts = 0; attempts < 5 && c->cursor_initialized == 0; attempts++) {
		if (poll(&pfd, 1, 10) > 0) {
			wl_display_dispatch(c->display);
		} else {
			wl_display_dispatch_pending(c->display);
		}
	}

	// Destroy discovery surfaces
	for (int i = 0; i < c->nr_screens; i++) {
		if (layer_surfaces[i]) zwlr_layer_surface_v1_destroy(layer_surfaces[i]);
		if (c->screens[i].discovery_surface) {
			wl_surface_destroy(c->screens[i].discovery_surface);
			c->screens[i].discovery_surface = NULL;
		}
	}
	wl_display_flush(c->display);

	// Fallback if discovery failed
	if (c->cursor_initialized == 0) {
		c->cursor_x = c->screens[0].x + c->screens[0].w / 2;
		c->cursor_y = c->screens[0].y + c->screens[0].h / 2;
		c->cursor_initialized = 1;
	}
}

// ---------- Input injection ----------

static int neru_wlr_move_absolute(NeruWlrootsClient *c, int x, int y) {
	if (!c || !c->vptr) return 0;

	// Compute the bounding box of all screens to get the virtual pointer extent.
	int minx = 0, miny = 0, maxx = 0, maxy = 0;
	for (int i = 0; i < c->nr_screens; i++) {
		NeruWaylandScreen *scr = &c->screens[i];
		if (i == 0 || scr->x < minx) minx = scr->x;
		if (i == 0 || scr->y < miny) miny = scr->y;
		int right = scr->x + scr->w;
		int bottom = scr->y + scr->h;
		if (i == 0 || right > maxx) maxx = right;
		if (i == 0 || bottom > maxy) maxy = bottom;
	}

	// Virtual pointer space starts at 0,0 even if compositor space has
	// negative origins (same workaround as warpd).
	zwlr_virtual_pointer_v1_motion_absolute(c->vptr, 0,
		wl_fixed_from_int(x - minx),
		wl_fixed_from_int(y - miny),
		wl_fixed_from_int(maxx - minx),
		wl_fixed_from_int(maxy - miny));
	zwlr_virtual_pointer_v1_frame(c->vptr);
	wl_display_flush(c->display);

	c->cursor_x = x;
	c->cursor_y = y;
	c->cursor_initialized = 1;

	return 1;
}

// Button codes for linux/input-event-codes.h
#define NERU_BTN_LEFT   0x110
#define NERU_BTN_RIGHT  0x111
#define NERU_BTN_MIDDLE 0x112

static int neru_wlr_button(NeruWlrootsClient *c, int button, int pressed) {
	if (!c || !c->vptr) return 0;

	zwlr_virtual_pointer_v1_button(c->vptr, 0, (uint32_t)button,
		pressed ? 1 : 0);
	zwlr_virtual_pointer_v1_frame(c->vptr);
	wl_display_flush(c->display);
	return 1;
}

static int neru_wlr_click(NeruWlrootsClient *c, int button) {
	if (!c || !c->vptr) return 0;

	zwlr_virtual_pointer_v1_button(c->vptr, 0, (uint32_t)button, 1);
	zwlr_virtual_pointer_v1_button(c->vptr, 0, (uint32_t)button, 0);
	zwlr_virtual_pointer_v1_frame(c->vptr);
	wl_display_flush(c->display);
	return 1;
}

static int neru_wlr_scroll(NeruWlrootsClient *c, int axis, int delta, int discrete) {
	if (!c || !c->vptr) return 0;

	// axis: 0 = vertical, 1 = horizontal
	zwlr_virtual_pointer_v1_axis_source(c->vptr, 0); // WL_POINTER_AXIS_SOURCE_WHEEL
	zwlr_virtual_pointer_v1_axis(c->vptr, 0, (uint32_t)axis, wl_fixed_from_int(delta));
	// axis_discrete helps Hyprland and other compositors properly handle scroll events.
	// discrete should be +/-1 per logical scroll step.
	if (discrete != 0) {
		zwlr_virtual_pointer_v1_axis_discrete(c->vptr, 0, (uint32_t)axis, wl_fixed_from_int(delta), discrete);
	}
	zwlr_virtual_pointer_v1_frame(c->vptr);
	wl_display_flush(c->display);
	return 1;
}

static uint32_t neru_wlr_modifier_mask(NeruWlrootsClient *c, const char *modifier) {
	if (strcmp(modifier, "shift") == 0) return c->mod_shift;
	if (strcmp(modifier, "ctrl") == 0) return c->mod_ctrl;
	if (strcmp(modifier, "alt") == 0) return c->mod_alt;
	if (strcmp(modifier, "cmd") == 0) return c->mod_logo;
	return 0;
}

static int neru_wlr_modifier_event(NeruWlrootsClient *c, const char *modifier, int is_down) {
	if (!c || !c->vkeyboard || !c->vkeyboard_ready) return 0;

	uint32_t mask = neru_wlr_modifier_mask(c, modifier);
	if (mask == 0) return 0;

	if (is_down) {
		c->depressed_mods |= mask;
	} else {
		c->depressed_mods &= ~mask;
	}

	zwp_virtual_keyboard_v1_modifiers(c->vkeyboard, c->depressed_mods, 0, 0, 0);
	// Use flush instead of roundtrip: fire-and-forget is sufficient because
	// Wayland message ordering guarantees the modifier state is applied before
	// the next pointer button event from the same client. A roundtrip blocks
	// concurrent cursor position queries waiting on the global mutex.
	wl_display_flush(c->display);

	return 1;
}

static int neru_wlr_get_cursor(NeruWlrootsClient *c, int *x, int *y) {
	if (!c) return 0;
	*x = c->cursor_x;
	*y = c->cursor_y;
	return c->cursor_initialized;
}

static int neru_wlr_screen_count(NeruWlrootsClient *c) {
	if (!c) return 0;
	return c->nr_screens;
}

static int neru_wlr_screen_info(NeruWlrootsClient *c, int idx,
	int *x, int *y, int *w, int *h, char *name_out, int name_len)
{
	if (!c || idx < 0 || idx >= c->nr_screens) return 0;
	NeruWaylandScreen *scr = &c->screens[idx];
	*x = scr->x;
	*y = scr->y;
	*w = scr->w;
	*h = scr->h;
	strncpy(name_out, scr->name, (size_t)(name_len - 1));
	name_out[name_len - 1] = '\0';
	return 1;
}

static int neru_wlr_has_virtual_pointer(NeruWlrootsClient *c) {
	return c && c->vptr != NULL;
}

static int neru_wlr_has_virtual_keyboard(NeruWlrootsClient *c) {
	return c && c->vkeyboard != NULL && c->vkeyboard_ready;
}
*/
import "C"

import (
	"fmt"
	"image"
	"os"
	"strings"
	"sync"
	"unsafe"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
	// Blank-import to link the wayland-scanner generated protocol objects.
	_ "github.com/y3owk1n/neru/internal/core/infra/platform/linux/wlr_protocol"
)

const (
	wlrootsScreenNameBufferSize = 128
	wlrootsDefaultWidth         = 1920
	wlrootsDefaultHeight        = 1080
)

type wlrootsScreen struct {
	Name   string
	Bounds image.Rectangle
}

type wlrootsState struct {
	mu sync.RWMutex

	client  *C.NeruWlrootsClient
	screens []wlrootsScreen
	ready   bool
}

var globalWlrootsState = &wlrootsState{}

func ensureWlrootsState() error {
	globalWlrootsState.mu.Lock()
	defer globalWlrootsState.mu.Unlock()

	if globalWlrootsState.ready {
		return nil
	}

	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return derrors.New(
			derrors.CodeNotSupported,
			"WAYLAND_DISPLAY is not set; wlroots backend is unavailable",
		)
	}

	client := C.neru_wlr_connect()
	if client == nil {
		return derrors.New(
			derrors.CodeActionFailed,
			"failed to connect to Wayland compositor",
		)
	}

	if C.neru_wlr_has_virtual_pointer(client) == 0 { //nolint:nlreturn
		C.neru_wlr_disconnect(client)

		return derrors.New(
			derrors.CodeActionFailed,
			"Wayland compositor does not support zwlr_virtual_pointer_v1 protocol; "+
				"this protocol is required and is provided by wlroots-based compositors (Sway, Hyprland, niri, River)",
		)
	}

	// Initialize cursor position to screen center. Wayland has no
	// protocol to query global pointer position, so we track it
	// client-side via move_absolute only (matching warpd's pattern).
	C.neru_wlr_init_cursor(client)

	// Populate screen list from the client.
	count := int(C.neru_wlr_screen_count(client)) //nolint:nlreturn
	screens := make([]wlrootsScreen, 0, count)

	for index := range count {
		var posX, posY, width, height C.int

		nameBuf := make([]C.char, wlrootsScreenNameBufferSize)
		if C.neru_wlr_screen_info( //nolint:nlreturn
			client,
			C.int(index),
			&posX,
			&posY,
			&width,
			&height,
			&nameBuf[0],
			wlrootsScreenNameBufferSize, //nolint:nlreturn
		) != 0 {
			name := C.GoString(&nameBuf[0])
			if name == "" {
				name = fmt.Sprintf("output-%d", index)
			}

			screens = append(screens, wlrootsScreen{
				Name: name,
				Bounds: image.Rect(
					int(posX),
					int(posY),
					int(posX+width),
					int(posY+height),
				),
			})
		}
	}

	// Fallback: if no screens were discovered via xdg_output, use a single
	// default screen so the rest of the system has something to work with.
	if len(screens) == 0 {
		screens = append(screens, wlrootsScreen{
			Name:   "wayland-0",
			Bounds: image.Rect(0, 0, wlrootsDefaultWidth, wlrootsDefaultHeight),
		})
	}

	globalWlrootsState.client = client
	globalWlrootsState.screens = screens
	globalWlrootsState.ready = true

	return nil
}

func wlrootsScreenBounds() (image.Rectangle, error) {
	err := ensureWlrootsState()
	if err != nil {
		return image.Rectangle{}, err
	}

	globalWlrootsState.mu.RLock()
	defer globalWlrootsState.mu.RUnlock()

	// Return bounds of the screen containing the cursor.
	cursor, _ := wlrootsCursorPositionLocked()
	for _, screen := range globalWlrootsState.screens {
		if cursor.In(screen.Bounds) {
			return screen.Bounds, nil
		}
	}

	// Fallback to first screen.
	return globalWlrootsState.screens[0].Bounds, nil
}

func wlrootsScreenBoundsByName(name string) (image.Rectangle, bool, error) {
	if name == "" {
		return image.Rectangle{}, false, nil
	}

	err := ensureWlrootsState()
	if err != nil {
		return image.Rectangle{}, false, err
	}

	globalWlrootsState.mu.RLock()
	defer globalWlrootsState.mu.RUnlock()

	for _, screen := range globalWlrootsState.screens {
		if strings.EqualFold(screen.Name, name) {
			return screen.Bounds, true, nil
		}
	}

	return image.Rectangle{}, false, nil
}

func wlrootsScreenNames() ([]string, error) {
	err := ensureWlrootsState()
	if err != nil {
		return nil, err
	}

	globalWlrootsState.mu.RLock()
	defer globalWlrootsState.mu.RUnlock()

	names := make([]string, 0, len(globalWlrootsState.screens))
	for _, screen := range globalWlrootsState.screens {
		names = append(names, screen.Name)
	}

	return names, nil
}

func wlrootsCursorPosition() (image.Point, error) {
	err := ensureWlrootsState()
	if err != nil {
		return image.Point{}, err
	}

	globalWlrootsState.mu.RLock()
	defer globalWlrootsState.mu.RUnlock()

	return wlrootsCursorPositionLocked()
}

// wlrootsCursorPositionLocked returns cursor position while holding at least RLock.
func wlrootsCursorPositionLocked() (image.Point, error) {
	client := globalWlrootsState.client
	if client == nil {
		return image.Point{}, nil
	}

	// Cursor position is tracked purely client-side via move_absolute.
	// No need to poll Wayland events — doing so previously triggered
	// the pointer motion handler which corrupted the position cache.
	var posX, posY C.int
	initialized := C.neru_wlr_get_cursor(client, &posX, &posY) //nolint:nlreturn

	// If cursor was never initialized, fall back to first screen center
	if initialized == 0 {
		if len(globalWlrootsState.screens) > 0 {
			scr := globalWlrootsState.screens[0]

			return image.Point{
				X: scr.Bounds.Min.X + scr.Bounds.Dx()/2,
				Y: scr.Bounds.Min.Y + scr.Bounds.Dy()/2,
			}, nil
		}

		return image.Point{}, nil
	}

	return image.Point{X: int(posX), Y: int(posY)}, nil
}

func wlrootsMoveCursorToPoint(point image.Point) error {
	err := ensureWlrootsState()
	if err != nil {
		return err
	}

	globalWlrootsState.mu.Lock()
	client := globalWlrootsState.client
	defer globalWlrootsState.mu.Unlock()

	if C.neru_wlr_move_absolute(client, C.int(point.X), C.int(point.Y)) == 0 { //nolint:nlreturn
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to move wlroots virtual pointer to (%d, %d)",
			point.X,
			point.Y,
		)
	}

	return nil
}

// wlrootsClick performs a mouse click at the given position using the virtual pointer.
func wlrootsClick(point image.Point, button int) error {
	err := ensureWlrootsState()
	if err != nil {
		return err
	}

	globalWlrootsState.mu.Lock()
	client := globalWlrootsState.client
	defer globalWlrootsState.mu.Unlock()

	// Move to target.
	if C.neru_wlr_move_absolute(client, C.int(point.X), C.int(point.Y)) == 0 { //nolint:nlreturn
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to move wlroots virtual pointer to (%d, %d)",
			point.X,
			point.Y,
		)
	}

	if C.neru_wlr_click(client, C.int(button)) == 0 { //nolint:nlreturn
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to perform wlroots click (button %d) at (%d, %d)",
			button,
			point.X,
			point.Y,
		)
	}

	return nil
}

// wlrootsButtonEvent presses or releases a button at the given position.
func wlrootsButtonEvent(point image.Point, button int, pressed bool) error {
	err := ensureWlrootsState()
	if err != nil {
		return err
	}

	globalWlrootsState.mu.Lock()
	client := globalWlrootsState.client
	defer globalWlrootsState.mu.Unlock()

	// Move to target.
	if C.neru_wlr_move_absolute(client, C.int(point.X), C.int(point.Y)) == 0 { //nolint:nlreturn
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to move wlroots virtual pointer to (%d, %d)",
			point.X,
			point.Y,
		)
	}

	pressedInt := 0
	if pressed {
		pressedInt = 1
	}

	if C.neru_wlr_button(client, C.int(button), C.int(pressedInt)) == 0 { //nolint:nlreturn
		return derrors.New(
			derrors.CodeActionFailed,
			"failed to perform wlroots button event",
		)
	}

	return nil
}

// wlrootsButtonRelease releases a button at the current cursor position.
func wlrootsButtonRelease(button int) error {
	err := ensureWlrootsState()
	if err != nil {
		return err
	}

	globalWlrootsState.mu.Lock()
	client := globalWlrootsState.client
	defer globalWlrootsState.mu.Unlock()

	if C.neru_wlr_button(client, C.int(button), 0) == 0 { //nolint:nlreturn
		return derrors.New(
			derrors.CodeActionFailed,
			"failed to release wlroots button",
		)
	}

	return nil
}

// wlrootsScroll sends a scroll event on the virtual pointer.
// axis: 0 = vertical, 1 = horizontal.
// delta: pixel delta for the axis event.
// discrete: discrete step count (e.g., +/-1 per logical scroll click).
func wlrootsScroll(axis, delta, discrete int) error {
	err := ensureWlrootsState()
	if err != nil {
		return err
	}

	globalWlrootsState.mu.Lock()
	client := globalWlrootsState.client
	defer globalWlrootsState.mu.Unlock()

	res := C.neru_wlr_scroll(client, C.int(axis), C.int(delta), C.int(discrete)) //nolint:nlreturn
	if res == 0 {
		return derrors.New(
			derrors.CodeActionFailed,
			"failed to perform wlroots scroll event",
		)
	}

	return nil
}

func wlrootsModifierEvent(modifier string, isDown bool) error {
	err := ensureWlrootsState()
	if err != nil {
		return err
	}

	globalWlrootsState.mu.Lock()
	client := globalWlrootsState.client
	defer globalWlrootsState.mu.Unlock()

	if C.neru_wlr_has_virtual_keyboard(client) == 0 { //nolint:nlreturn
		return derrors.New(
			derrors.CodeActionFailed,
			"Wayland compositor does not support zwp_virtual_keyboard_manager_v1 protocol; "+
				"this protocol is required for sticky modifier key injection on Wayland",
		)
	}

	cModifier := C.CString(modifier)
	defer C.free(unsafe.Pointer(cModifier)) //nolint:nlreturn

	cDown := C.int(0)
	if isDown {
		cDown = C.int(1)
	}

	if C.neru_wlr_modifier_event(client, cModifier, cDown) == 0 { //nolint:nlreturn
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to post wlroots modifier event %q",
			modifier,
		)
	}

	return nil
}

// Exported button constants for use by the accessibility adapter.
const (
	WlrBtnLeft   = 0x110
	WlrBtnRight  = 0x111
	WlrBtnMiddle = 0x112
)
