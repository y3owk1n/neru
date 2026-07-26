#include "wayland_keymap.h"

#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <unistd.h>
#include <wayland-client.h>
#include <xkbcommon/xkbcommon.h>

struct neru_xkb_state {
	struct xkb_state *state;
	struct wl_display *display;
	struct wl_keyboard *wl_keyboard;
};

// ── wl_keyboard listener (only .keymap is used) ─────────────────────────

struct keymap_ready {
	struct xkb_state *state;
	int ready;
};

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
					kr->state = xkb_state_new(keymap);
					xkb_keymap_unref(keymap);
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
    uint32_t mods_locked, uint32_t group) {}

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

	struct keymap_ready kr = {0};
	wl_keyboard_add_listener(kb, &keyboard_listener, &kr);
	wl_display_roundtrip(display);
	wl_display_roundtrip(display);

	if (!kr.state) {
		wl_keyboard_destroy(kb);
		wl_display_disconnect(display);
		return NULL;
	}

	neru_xkb_state *state = calloc(1, sizeof(neru_xkb_state));
	if (!state) {
		xkb_state_unref(kr.state);
		wl_keyboard_destroy(kb);
		wl_display_disconnect(display);
		return NULL;
	}

	state->state = kr.state;
	state->display = display;
	state->wl_keyboard = kb;

	return state;
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

static void neru_normalize_xkb_name(char *buf, size_t buf_size) {
	static const struct {
		const char *xkb;
		const char *canon;
	} table[] = {
	    {"semicolon", ";"},
	    {"colon", ":"},
	    {"comma", ","},
	    {"period", "."},
	    {"slash", "/"},
	    {"backslash", "\\"},
	    {"apostrophe", "'"},
	    {"grave", "`"},
	    {"minus", "-"},
	    {"equal", "="},
	    {"bracketleft", "["},
	    {"bracketright", "]"},
	    {"less", "<"},
	    {"greater", ">"},
	    {"underscore", "_"},
	    {"plus", "+"},
	    {"asciitilde", "~"},
	    {"exclam", "!"},
	    {"at", "@"},
	    {"numbersign", "#"},
	    {"dollar", "$"},
	    {"percent", "%"},
	    {"asciicircum", "^"},
	    {"ampersand", "&"},
	    {"asterisk", "*"},
	    {"parenleft", "("},
	    {"parenright", ")"},
	    {"question", "?"},
	    {"quotedbl", "\""},
	    {"bar", "|"},
	    {"Page_Up", "PageUp"},
	    {"Page_Down", "PageDown"},
	    {"KP_Add", "+"},
	    {"KP_Subtract", "-"},
	    {"KP_Multiply", "*"},
	    {"KP_Divide", "/"},
	    {"KP_Enter", "Return"},
	    {"KP_Delete", "Delete"},
	    {"KP_Insert", "Insert"},
	    {"KP_Home", "Home"},
	    {"KP_End", "End"},
	    {"KP_Page_Up", "PageUp"},
	    {"KP_Page_Down", "PageDown"},
	    {"KP_Up", "Up"},
	    {"KP_Down", "Down"},
	    {"KP_Left", "Left"},
	    {"KP_Right", "Right"},
	    {"KP_Begin", "5"},
	    {"KP_Decimal", "."},
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
	/* KP_0 through KP_9 */
	size_t blen = strlen(buf);
	if (blen == 4 && buf[0] == 'K' && buf[1] == 'P' && buf[2] == '_' && buf[3] >= '0' && buf[3] <= '9') {
		buf[0] = buf[3];
		buf[1] = '\0';
	}
}

int neru_xkb_state_key_get_name(neru_xkb_state *state, uint16_t evdev_code, char *buf, size_t buf_size) {
	if (!state || !state->state || !buf || buf_size == 0)
		return -1;

	xkb_keysym_t keysym = xkb_state_key_get_one_sym(state->state, (xkb_keycode_t)evdev_code + 8);
	if (keysym == XKB_KEY_NoSymbol)
		return -1;

	xkb_keysym_get_name(keysym, buf, buf_size);
	if (buf[0] == '\0')
		return -1;

	neru_normalize_xkb_name(buf, buf_size);
	if (buf[0] == '\0')
		return -1;

	return 0;
}
