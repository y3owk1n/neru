#include "wayland_keymap.h"

#include <poll.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <unistd.h>
#include <wayland-client.h>
#include <xkbcommon/xkbcommon.h>

struct keymap_ready {
	struct xkb_state *state;
	int ready;
	// Set when a keymap arrived after the first one, so the reader that
	// dispatches this display knows the state it resolves names against has
	// been replaced (neru_xkb_state_dispatch reports and clears it).
	int changed;
};

struct neru_xkb_state {
	struct xkb_state *state;
	struct wl_display *display;
	struct wl_keyboard *wl_keyboard;
	struct keymap_ready kr;  // listener data, alive for lifetime of this struct
};

// ── wl_keyboard listener (only .keymap is used) ─────────────────────────

static void neru_keyboard_keymap(
    void *data, struct wl_keyboard *wl_keyboard, uint32_t format, int32_t fd, uint32_t size) {
	struct keymap_ready *kr = data;
	if (format == WL_KEYBOARD_KEYMAP_FORMAT_XKB_V1) {
		char *map_str = mmap(NULL, size, PROT_READ, MAP_PRIVATE, fd, 0);
		if (map_str != MAP_FAILED) {
			struct xkb_context *ctx = xkb_context_new(XKB_CONTEXT_NO_FLAGS);
			if (ctx) {
				struct xkb_keymap *keymap =
				    xkb_keymap_new_from_string(ctx, map_str, XKB_KEYMAP_FORMAT_TEXT_V1, XKB_KEYMAP_COMPILE_NO_FLAGS);
				if (keymap) {
					struct xkb_state *fresh = xkb_state_new(keymap);
					xkb_keymap_unref(keymap);
					if (fresh) {
						// A later keymap replaces the first: the compositor
						// changed its layout or options, and every name
						// resolved from here on has to follow it.
						if (kr->state) {
							xkb_state_unref(kr->state);
							kr->changed = 1;
						}
						kr->state = fresh;
					}
				}
				xkb_context_unref(ctx);
			}
			munmap(map_str, size);
		}
	}
	close(fd);
	kr->ready = 1;
}

static void neru_keyboard_enter(
    void *data, struct wl_keyboard *wl_keyboard, uint32_t serial, struct wl_surface *surface, struct wl_array *keys) {}

static void neru_keyboard_leave(
    void *data, struct wl_keyboard *wl_keyboard, uint32_t serial, struct wl_surface *surface) {}

static void neru_keyboard_key(
    void *data, struct wl_keyboard *wl_keyboard, uint32_t serial, uint32_t time, uint32_t key, uint32_t state) {}

static void neru_keyboard_modifiers(
    void *data, struct wl_keyboard *wl_keyboard, uint32_t serial, uint32_t mods_depressed, uint32_t mods_latched,
    uint32_t mods_locked, uint32_t group) {
	struct keymap_ready *kr = data;
	if (kr->state) {
		xkb_state_update_mask(kr->state, mods_depressed, mods_latched, mods_locked, 0, 0, group);
	}
}

static void neru_keyboard_repeat_info(void *data, struct wl_keyboard *wl_keyboard, int32_t rate, int32_t delay) {}

static const struct wl_keyboard_listener keyboard_listener = {
    .keymap = neru_keyboard_keymap,
    .enter = neru_keyboard_enter,
    .leave = neru_keyboard_leave,
    .key = neru_keyboard_key,
    .modifiers = neru_keyboard_modifiers,
    .repeat_info = neru_keyboard_repeat_info,
};

// ── wl_seat listener ────────────────────────────────────────────────────

static void neru_seat_capabilities(void *data, struct wl_seat *seat, uint32_t capabilities) {}

static void neru_seat_name(void *data, struct wl_seat *seat, const char *name) {}

static const struct wl_seat_listener seat_listener = {
    .capabilities = neru_seat_capabilities,
    .name = neru_seat_name,
};

// ── wl_registry listener ────────────────────────────────────────────────

struct registry_data {
	struct wl_seat *seat;
};

static void neru_registry_global(
    void *data, struct wl_registry *registry, uint32_t name, const char *interface, uint32_t version) {
	struct registry_data *rd = data;
	if (strcmp(interface, "wl_seat") == 0) {
		rd->seat = wl_registry_bind(registry, name, &wl_seat_interface, version < 7 ? version : 7);
	}
}

static void neru_registry_global_remove(void *data, struct wl_registry *registry, uint32_t name) {}

static const struct wl_registry_listener registry_listener = {
    .global = neru_registry_global,
    .global_remove = neru_registry_global_remove,
};

// ── Public API ──────────────────────────────────────────────────────────

neru_xkb_state *neru_xkb_state_create(void) {
	struct wl_display *display = wl_display_connect(NULL);
	if (!display)
		return NULL;

	struct wl_registry *registry = wl_display_get_registry(display);
	if (!registry) {
		wl_display_disconnect(display);
		return NULL;
	}

	struct registry_data rd = {0};
	wl_registry_add_listener(registry, &registry_listener, &rd);
	wl_display_roundtrip(display);
	wl_display_roundtrip(display);

	if (!rd.seat) {
		wl_registry_destroy(registry);
		wl_display_disconnect(display);
		return NULL;
	}

	struct wl_keyboard *kb = wl_seat_get_keyboard(rd.seat);
	wl_registry_destroy(registry);

	if (!kb) {
		wl_display_disconnect(display);
		return NULL;
	}

	neru_xkb_state *state = calloc(1, sizeof(neru_xkb_state));
	if (!state) {
		wl_keyboard_destroy(kb);
		wl_display_disconnect(display);
		return NULL;
	}

	state->display = display;
	state->wl_keyboard = kb;
	state->kr = (struct keymap_ready){0};

	wl_keyboard_add_listener(kb, &keyboard_listener, &state->kr);
	wl_display_roundtrip(display);
	wl_display_roundtrip(display);

	if (!state->kr.state) {
		neru_xkb_state_destroy(state);
		return NULL;
	}

	state->state = state->kr.state;

	return state;
}

int neru_xkb_state_dispatch(neru_xkb_state *state) {
	if (!state || !state->display)
		return -1;

	// The libwayland read protocol: announce the intent to read, flush what
	// is queued, read only if the socket has something, and never block.
	while (wl_display_prepare_read(state->display) != 0) {
		if (wl_display_dispatch_pending(state->display) < 0)
			return -1;
	}

	wl_display_flush(state->display);

	struct pollfd pfd = {.fd = wl_display_get_fd(state->display), .events = POLLIN};
	if (poll(&pfd, 1, 0) > 0) {
		if (wl_display_read_events(state->display) < 0)
			return -1;
	} else {
		wl_display_cancel_read(state->display);
	}

	if (wl_display_dispatch_pending(state->display) < 0)
		return -1;

	if (!state->kr.changed)
		return 0;

	state->kr.changed = 0;
	state->state = state->kr.state;
	return 1;
}

void neru_xkb_state_destroy(neru_xkb_state *state) {
	if (!state)
		return;

	if (state->state)
		xkb_state_unref(state->state);
	if (state->wl_keyboard)
		wl_keyboard_destroy(state->wl_keyboard);
	if (state->display)
		wl_display_disconnect(state->display);

	free(state);
}

void neru_xkb_state_key(neru_xkb_state *state, uint16_t evdev_code, int is_press) {
	if (!state || !state->state)
		return;

	xkb_state_update_key(state->state, (xkb_keycode_t)evdev_code + 8, is_press ? XKB_KEY_DOWN : XKB_KEY_UP);
}

// neru_normalize_xkb_name maps the keysym names that stand for a named key onto the
// spelling Neru binds. Only keysyms with no character of their own reach this
// table: everything printable was already named by its character in
// neru_xkb_keysym_name, which is what keeps this list short.
//
// The keypad reports keysyms of its own while NumLock is off, and they mean the
// same keys as their main-keyboard equivalents, so they fold onto the same
// names. With NumLock on it reports KP_0 through KP_9 and the operators, which
// carry a character and never get here. ISO_Left_Tab is what XKB calls the Tab
// key once Shift has chosen the level: the same key, and a binding written
// Shift+Tab has to reach it.
//
// The rows are keyed by the name xkb_keysym_get_name answers, which is the
// first name xkbcommon-keysyms.h lists for a keysym and not always the
// familiar one: the page keys are Prior and Next there, never Page_Up and
// Page_Down. A row keyed by the alias is dead, which is how PageUp went
// unmatched on this backend.
static void neru_normalize_xkb_name(char *buf, size_t buf_size) {
	static const struct {
		const char *xkb;
		const char *canon;
	} table[] = {
	    // clang-format off: one row per line is what the pin in
	    // internal/architecture/wayland_keypad_folds_test.go reads.
	    {"ISO_Left_Tab", "Tab"},
	    {"Prior", "PageUp"},
	    {"Next", "PageDown"},
	    {"KP_Enter", "Return"},
	    {"KP_Delete", "Delete"},
	    {"KP_Insert", "Insert"},
	    {"KP_Home", "Home"},
	    {"KP_End", "End"},
	    {"KP_Prior", "PageUp"},
	    {"KP_Next", "PageDown"},
	    {"KP_Up", "Up"},
	    {"KP_Down", "Down"},
	    {"KP_Left", "Left"},
	    {"KP_Right", "Right"},
	    {"KP_Begin", "5"},
	    {"Caps_Lock", "CapsLock"},
	    // clang-format on
	};

	for (size_t i = 0; i < sizeof(table) / sizeof(table[0]); i++) {
		if (strcmp(buf, table[i].xkb) == 0) {
			size_t len = strlen(table[i].canon);
			if (len < buf_size) {
				memmove(buf, table[i].canon, len + 1);
			}
			return;
		}
	}
}

// neru_xkb_keysym_name names a state-resolved keysym the way the X11 tap names
// one: by the character it types when it types one, and by name otherwise.
//
// Character first is what makes the two Linux backends agree. Shift has already
// chosen the level by the time a keysym exists, and the keysym name for the
// shifted level is not the character: Shift+[ is "braceleft", and on a layout
// that is not us the names run to "sterling" and "adiaeresis". A name table
// written by hand can never be complete, and every key it misses is a binding
// that matches on X11 and not here. Asking libxkbcommon for the character
// covers every printable keysym at once, and only the keys with no character
// (Tab, Return, the navigation keys, the keypad with NumLock off) go by name.
//
// Space is deliberately left to the name path: it is a named key ("Space"),
// and a bare " " would be trimmed to nothing downstream. Control characters
// (Tab, Return, BackSpace, Delete) have no printable form and take the same
// path.
int neru_xkb_keysym_name(uint32_t keysym, char *buf, size_t buf_size) {
	if (!buf || buf_size == 0)
		return -1;

	if (keysym == XKB_KEY_NoSymbol)
		return -1;

	int written = xkb_keysym_to_utf8(keysym, buf, buf_size);
	if (written > 1 && (unsigned char)buf[0] > 0x20 && (unsigned char)buf[0] != 0x7f)
		return 0;

	xkb_keysym_get_name(keysym, buf, buf_size);
	if (buf[0] == '\0')
		return -1;

	neru_normalize_xkb_name(buf, buf_size);
	if (buf[0] == '\0')
		return -1;

	return 0;
}

int neru_xkb_state_key_get_name(neru_xkb_state *state, uint16_t evdev_code, char *buf, size_t buf_size) {
	if (!state || !state->state || !buf || buf_size == 0)
		return -1;

	xkb_keysym_t keysym = xkb_state_key_get_one_sym(state->state, (xkb_keycode_t)evdev_code + 8);

	return neru_xkb_keysym_name(keysym, buf, buf_size);
}

void neru_xkb_state_sync_leds(neru_xkb_state *state, int num_lock_on, int caps_lock_on) {
	if (!state || !state->state)
		return;

	struct xkb_keymap *keymap = xkb_state_get_keymap(state->state);
	if (!keymap)
		return;

	xkb_mod_mask_t depressed = xkb_state_serialize_mods(state->state, XKB_STATE_MODS_DEPRESSED);
	xkb_mod_mask_t latched = xkb_state_serialize_mods(state->state, XKB_STATE_MODS_LATCHED);
	xkb_mod_mask_t locked = xkb_state_serialize_mods(state->state, XKB_STATE_MODS_LOCKED);
	xkb_layout_index_t group = xkb_state_serialize_layout(state->state, XKB_STATE_LAYOUT_LOCKED);

	xkb_mod_mask_t new_locked = locked;
	int changed = 0;

	xkb_mod_index_t num_idx = xkb_keymap_mod_get_index(keymap, "Mod2");
	if (num_idx != XKB_MOD_INVALID) {
		int cur = (locked >> num_idx) & 1;
		if (cur != num_lock_on) {
			new_locked ^= (xkb_mod_mask_t)1 << num_idx;
			changed = 1;
		}
	}

	xkb_mod_index_t caps_idx = xkb_keymap_mod_get_index(keymap, "Lock");
	if (caps_idx != XKB_MOD_INVALID) {
		int cur = (locked >> caps_idx) & 1;
		if (cur != caps_lock_on) {
			new_locked ^= (xkb_mod_mask_t)1 << caps_idx;
			changed = 1;
		}
	}

	if (changed) {
		xkb_state_update_mask(state->state, depressed, latched, new_locked, 0, 0, group);
	}
}
