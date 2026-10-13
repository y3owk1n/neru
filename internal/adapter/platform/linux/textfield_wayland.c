#include "textfield_wayland.h"

#include "shm_file.h"
#include "wlr_protocol/layer-shell.h"
#include "wlr_protocol/text-input-v3.h"
#include "wlr_protocol/xdg-output.h"

#include <errno.h>
#include <poll.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <sys/timerfd.h>
#include <time.h>
#include <unistd.h>
#include <wayland-client.h>
#include <xkbcommon/xkbcommon-compose.h>
#include <xkbcommon/xkbcommon-keysyms.h>
#include <xkbcommon/xkbcommon.h>

#define NERU_TEXT_FIELD_MAX_OUTPUTS 16
#define NERU_TEXT_FIELD_MAX_QUERY 1024

// NERU_TEXT_FIELD_OPEN_TIMEOUT_MS bounds the compositor round trips opening
// the field. The caller holds the mode handler's lock, so a compositor that
// stops answering costs at most this, and search falls back to the key stream.
#define NERU_TEXT_FIELD_OPEN_TIMEOUT_MS 250

typedef struct {
	struct wl_output *output;
	struct zxdg_output_v1 *xdg_output;
	int x, y, width, height;
} NeruTextFieldOutput;

struct NeruTextField {
	struct wl_display *display;
	struct wl_registry *registry;
	struct wl_compositor *compositor;
	struct wl_shm *shm;
	struct wl_seat *seat;
	struct wl_keyboard *keyboard;
	struct zwlr_layer_shell_v1 *layer_shell;
	struct zxdg_output_manager_v1 *output_manager;
	struct zwp_text_input_manager_v3 *text_input_manager;
	struct zwp_text_input_v3 *text_input;

	NeruTextFieldOutput outputs[NERU_TEXT_FIELD_MAX_OUTPUTS];
	int output_count;

	struct wl_surface *surface;
	struct zwlr_layer_surface_v1 *layer_surface;
	struct wl_buffer *buffer;
	int width, height;
	int focused;   // the surface holds the keyboard
	int finished;  // Return or Escape ended the search

	struct xkb_context *xkb_context;
	struct xkb_keymap *xkb_keymap;
	struct xkb_state *xkb_state;
	struct xkb_compose_state *compose_state;

	int repeat_fd;
	uint32_t repeat_key;
	int32_t repeat_rate, repeat_delay;

	char committed[NERU_TEXT_FIELD_MAX_QUERY];
	char *preedit;
	char *pending_preedit;
	char *pending_commit;
	char query[NERU_TEXT_FIELD_MAX_QUERY * 2];

	int flags;
};

// Text the field holds.

// neru_text_field_finished reports whether Return or Escape already ended the
// search. Input after it is dropped, so a batch holding both keys, or a key
// typed after Return, cannot change what the first one decided.
static int neru_text_field_finished(NeruTextField *field) { return field->finished; }

static void neru_text_field_append(NeruTextField *field, const char *text) {
	size_t used = strlen(field->committed);
	size_t add = strlen(text);
	if (used + add >= sizeof(field->committed)) {
		return;
	}
	memcpy(field->committed + used, text, add + 1);
	field->flags |= NERU_TEXT_FIELD_CHANGED;
}

static void neru_text_field_delete_last(NeruTextField *field) {
	size_t used = strlen(field->committed);
	if (used == 0) {
		return;
	}
	// Step back over UTF-8 continuation bytes to the start of the last rune.
	size_t end = used - 1;
	while (end > 0 && (field->committed[end] & 0xC0) == 0x80) {
		end--;
	}
	field->committed[end] = '\0';
	field->flags |= NERU_TEXT_FIELD_CHANGED;
}

// Keyboard.

static int neru_text_field_mod_active(NeruTextField *field, const char *name) {
	return xkb_state_mod_name_is_active(field->xkb_state, name, XKB_STATE_MODS_EFFECTIVE) > 0;
}

static void neru_text_field_key_press(NeruTextField *field, uint32_t key) {
	if (!field->xkb_state || neru_text_field_finished(field)) {
		return;
	}

	xkb_keysym_t keysym = xkb_state_key_get_one_sym(field->xkb_state, key + 8);

	if (field->compose_state && xkb_compose_state_feed(field->compose_state, keysym) == XKB_COMPOSE_FEED_ACCEPTED) {
		switch (xkb_compose_state_get_status(field->compose_state)) {
		case XKB_COMPOSE_COMPOSING:
			return;
		case XKB_COMPOSE_COMPOSED: {
			char composed[64];
			xkb_compose_state_get_utf8(field->compose_state, composed, sizeof(composed));
			xkb_compose_state_reset(field->compose_state);
			neru_text_field_append(field, composed);
			return;
		}
		case XKB_COMPOSE_CANCELLED:
			xkb_compose_state_reset(field->compose_state);
			return;
		case XKB_COMPOSE_NOTHING:
			break;
		}
	}

	switch (keysym) {
	case XKB_KEY_Escape:
		field->flags |= NERU_TEXT_FIELD_CANCEL;
		field->finished = 1;
		return;
	case XKB_KEY_Return:
	case XKB_KEY_KP_Enter:
		field->flags |= NERU_TEXT_FIELD_CONFIRM;
		field->finished = 1;
		return;
	case XKB_KEY_BackSpace:
		neru_text_field_delete_last(field);
		return;
	}

	// A chord with Ctrl, Alt or Super types nothing into the search.
	if (neru_text_field_mod_active(field, XKB_MOD_NAME_CTRL) || neru_text_field_mod_active(field, XKB_MOD_NAME_ALT) ||
	    neru_text_field_mod_active(field, XKB_MOD_NAME_LOGO)) {
		return;
	}

	char text[64];
	int length = xkb_state_key_get_utf8(field->xkb_state, key + 8, text, sizeof(text));
	if (length <= 0 || (unsigned char)text[0] < 0x20 || text[0] == 0x7f) {
		return;
	}
	neru_text_field_append(field, text);
}

static void neru_text_field_stop_repeat(NeruTextField *field) {
	field->repeat_key = 0;
	struct itimerspec off = {0};
	timerfd_settime(field->repeat_fd, 0, &off, NULL);
}

static void neru_text_field_start_repeat(NeruTextField *field, uint32_t key) {
	if (field->repeat_rate <= 0 || !field->xkb_keymap || !xkb_keymap_key_repeats(field->xkb_keymap, key + 8)) {
		return;
	}
	field->repeat_key = key;
	long interval_ns = 1000000000L / field->repeat_rate;
	struct itimerspec timer = {
	    .it_value = {.tv_sec = field->repeat_delay / 1000, .tv_nsec = (field->repeat_delay % 1000) * 1000000L},
	    .it_interval = {.tv_sec = interval_ns / 1000000000L, .tv_nsec = interval_ns % 1000000000L},
	};
	timerfd_settime(field->repeat_fd, 0, &timer, NULL);
}

static void neru_keyboard_keymap_cb(
    void *data, struct wl_keyboard *keyboard, uint32_t format, int32_t fd, uint32_t size) {
	NeruTextField *field = data;
	if (format != WL_KEYBOARD_KEYMAP_FORMAT_XKB_V1) {
		close(fd);
		return;
	}
	char *map = mmap(NULL, size, PROT_READ, MAP_PRIVATE, fd, 0);
	close(fd);
	if (map == MAP_FAILED) {
		return;
	}
	struct xkb_keymap *keymap =
	    xkb_keymap_new_from_string(field->xkb_context, map, XKB_KEYMAP_FORMAT_TEXT_V1, XKB_KEYMAP_COMPILE_NO_FLAGS);
	munmap(map, size);
	if (!keymap) {
		return;
	}
	if (field->xkb_state) {
		xkb_state_unref(field->xkb_state);
	}
	if (field->xkb_keymap) {
		xkb_keymap_unref(field->xkb_keymap);
	}
	field->xkb_keymap = keymap;
	field->xkb_state = xkb_state_new(keymap);
}

static void neru_keyboard_enter_cb(
    void *data, struct wl_keyboard *keyboard, uint32_t serial, struct wl_surface *surface, struct wl_array *keys) {
	NeruTextField *field = data;
	if (surface == field->surface) {
		field->focused = 1;
	}
}

static void neru_keyboard_leave_cb(
    void *data, struct wl_keyboard *keyboard, uint32_t serial, struct wl_surface *surface) {
	NeruTextField *field = data;
	field->focused = 0;
	neru_text_field_stop_repeat(field);
}

static void neru_keyboard_key_cb(
    void *data, struct wl_keyboard *keyboard, uint32_t serial, uint32_t time, uint32_t key, uint32_t state) {
	NeruTextField *field = data;
	if (state == WL_KEYBOARD_KEY_STATE_PRESSED) {
		neru_text_field_key_press(field, key);
		neru_text_field_start_repeat(field, key);
	} else if (key == field->repeat_key) {
		neru_text_field_stop_repeat(field);
	}
}

static void neru_keyboard_modifiers_cb(
    void *data, struct wl_keyboard *keyboard, uint32_t serial, uint32_t depressed, uint32_t latched, uint32_t locked,
    uint32_t group) {
	NeruTextField *field = data;
	if (field->xkb_state) {
		xkb_state_update_mask(field->xkb_state, depressed, latched, locked, 0, 0, group);
	}
}

static void neru_keyboard_repeat_info_cb(void *data, struct wl_keyboard *keyboard, int32_t rate, int32_t delay) {
	NeruTextField *field = data;
	field->repeat_rate = rate;
	field->repeat_delay = delay;
}

static const struct wl_keyboard_listener neru_text_field_keyboard_listener = {
    .keymap = neru_keyboard_keymap_cb,
    .enter = neru_keyboard_enter_cb,
    .leave = neru_keyboard_leave_cb,
    .key = neru_keyboard_key_cb,
    .modifiers = neru_keyboard_modifiers_cb,
    .repeat_info = neru_keyboard_repeat_info_cb,
};

// Input-method text. A composition in progress shows in the query, as it does
// in the macOS field, and replaces the previous one when the method sends done.

static void neru_text_input_enter_cb(void *data, struct zwp_text_input_v3 *text_input, struct wl_surface *surface) {
	NeruTextField *field = data;
	zwp_text_input_v3_enable(text_input);
	zwp_text_input_v3_set_content_type(
	    text_input, ZWP_TEXT_INPUT_V3_CONTENT_HINT_NONE, ZWP_TEXT_INPUT_V3_CONTENT_PURPOSE_NORMAL);
	zwp_text_input_v3_set_cursor_rectangle(text_input, 0, 0, field->width, field->height);
	zwp_text_input_v3_commit(text_input);
}

static void neru_text_input_leave_cb(void *data, struct zwp_text_input_v3 *text_input, struct wl_surface *surface) {
	zwp_text_input_v3_disable(text_input);
	zwp_text_input_v3_commit(text_input);
}

static void neru_text_input_preedit_cb(
    void *data, struct zwp_text_input_v3 *text_input, const char *text, int32_t cursor_begin, int32_t cursor_end) {
	NeruTextField *field = data;
	free(field->pending_preedit);
	field->pending_preedit = strdup(text ? text : "");
}

static void neru_text_input_commit_cb(void *data, struct zwp_text_input_v3 *text_input, const char *text) {
	NeruTextField *field = data;
	free(field->pending_commit);
	field->pending_commit = strdup(text ? text : "");
}

static void neru_text_input_delete_surrounding_cb(
    void *data, struct zwp_text_input_v3 *text_input, uint32_t before_length, uint32_t after_length) {}

static void neru_text_input_done_cb(void *data, struct zwp_text_input_v3 *text_input, uint32_t serial) {
	NeruTextField *field = data;
	if (neru_text_field_finished(field)) {
		return;
	}
	if (field->pending_commit) {
		neru_text_field_append(field, field->pending_commit);
		free(field->pending_commit);
		field->pending_commit = NULL;
	}
	free(field->preedit);
	field->preedit = field->pending_preedit;
	field->pending_preedit = NULL;
	field->flags |= NERU_TEXT_FIELD_CHANGED;
}

static const struct zwp_text_input_v3_listener neru_text_field_text_input_listener = {
    .enter = neru_text_input_enter_cb,
    .leave = neru_text_input_leave_cb,
    .preedit_string = neru_text_input_preedit_cb,
    .commit_string = neru_text_input_commit_cb,
    .delete_surrounding_text = neru_text_input_delete_surrounding_cb,
    .done = neru_text_input_done_cb,
};

// Outputs, so the field lands on the screen holding the search box.

static void neru_xdg_output_position_cb(void *data, struct zxdg_output_v1 *xdg_output, int32_t x, int32_t y) {
	NeruTextFieldOutput *output = data;
	output->x = x;
	output->y = y;
}

static void neru_xdg_output_size_cb(void *data, struct zxdg_output_v1 *xdg_output, int32_t width, int32_t height) {
	NeruTextFieldOutput *output = data;
	output->width = width;
	output->height = height;
}

static void neru_xdg_output_done_cb(void *data, struct zxdg_output_v1 *xdg_output) {}

static void neru_xdg_output_name_cb(void *data, struct zxdg_output_v1 *xdg_output, const char *name) {}

static void neru_xdg_output_description_cb(void *data, struct zxdg_output_v1 *xdg_output, const char *description) {}

static const struct zxdg_output_v1_listener neru_text_field_xdg_output_listener = {
    .logical_position = neru_xdg_output_position_cb,
    .logical_size = neru_xdg_output_size_cb,
    .done = neru_xdg_output_done_cb,
    .name = neru_xdg_output_name_cb,
    .description = neru_xdg_output_description_cb,
};

// Surface.

static void neru_text_field_buffer_release_cb(void *data, struct wl_buffer *buffer) {}

static const struct wl_buffer_listener neru_text_field_buffer_listener = {
    .release = neru_text_field_buffer_release_cb,
};

// neru_text_field_attach gives the surface a fully transparent buffer, which a
// layer surface needs before the compositor maps it and gives it the keyboard.
static void neru_text_field_attach(NeruTextField *field) {
	int stride = field->width * 4;
	int size = stride * field->height;
	int fd = neru_shm_file_create("neru-text-field", size);
	if (fd < 0) {
		return;
	}
	void *pixels = mmap(NULL, size, PROT_READ | PROT_WRITE, MAP_SHARED, fd, 0);
	if (pixels != MAP_FAILED) {
		memset(pixels, 0, size);
		munmap(pixels, size);
	}
	struct wl_shm_pool *pool = wl_shm_create_pool(field->shm, fd, size);
	close(fd);
	if (field->buffer) {
		wl_buffer_destroy(field->buffer);
	}
	field->buffer = wl_shm_pool_create_buffer(pool, 0, field->width, field->height, stride, WL_SHM_FORMAT_ARGB8888);
	wl_shm_pool_destroy(pool);
	wl_buffer_add_listener(field->buffer, &neru_text_field_buffer_listener, field);
	wl_surface_attach(field->surface, field->buffer, 0, 0);
	wl_surface_damage(field->surface, 0, 0, field->width, field->height);
	wl_surface_commit(field->surface);
}

static void neru_layer_surface_configure_cb(
    void *data, struct zwlr_layer_surface_v1 *layer_surface, uint32_t serial, uint32_t width, uint32_t height) {
	NeruTextField *field = data;
	zwlr_layer_surface_v1_ack_configure(layer_surface, serial);
	if (!field->buffer) {
		neru_text_field_attach(field);
	}
}

static void neru_layer_surface_closed_cb(void *data, struct zwlr_layer_surface_v1 *layer_surface) {
	NeruTextField *field = data;
	field->flags |= NERU_TEXT_FIELD_CANCEL;
}

static const struct zwlr_layer_surface_v1_listener neru_text_field_layer_listener = {
    .configure = neru_layer_surface_configure_cb,
    .closed = neru_layer_surface_closed_cb,
};

// Bounded round trips.

static void neru_text_field_sync_done_cb(void *data, struct wl_callback *callback, uint32_t serial) {
	*(int *)data = 1;
	wl_callback_destroy(callback);
}

static const struct wl_callback_listener neru_text_field_sync_listener = {
    .done = neru_text_field_sync_done_cb,
};

static long neru_text_field_ms_until(const struct timespec *deadline) {
	struct timespec now;
	clock_gettime(CLOCK_MONOTONIC, &now);
	return (deadline->tv_sec - now.tv_sec) * 1000L + (deadline->tv_nsec - now.tv_nsec) / 1000000L;
}

// neru_text_field_roundtrip waits until the compositor has handled every
// request sent so far, like wl_display_roundtrip, but gives up at deadline.
// It returns 0 when the compositor answered in time.
static int neru_text_field_roundtrip(NeruTextField *field, const struct timespec *deadline) {
	int done = 0;
	struct wl_callback *callback = wl_display_sync(field->display);
	wl_callback_add_listener(callback, &neru_text_field_sync_listener, &done);

	while (!done) {
		if (wl_display_flush(field->display) < 0 && errno != EAGAIN) {
			return -1;
		}
		if (wl_display_prepare_read(field->display) != 0) {
			if (wl_display_dispatch_pending(field->display) < 0) {
				return -1;
			}
			continue;
		}
		long remaining = neru_text_field_ms_until(deadline);
		struct pollfd ready = {.fd = wl_display_get_fd(field->display), .events = POLLIN};
		int polled = remaining > 0 ? poll(&ready, 1, (int)remaining) : 0;
		if (polled <= 0) {
			wl_display_cancel_read(field->display);
			if (polled < 0 && errno == EINTR) {
				continue;
			}
			return -1;
		}
		if (wl_display_read_events(field->display) < 0 || wl_display_dispatch_pending(field->display) < 0) {
			return -1;
		}
	}
	return 0;
}

// Registry.

static void neru_seat_capabilities_cb(void *data, struct wl_seat *seat, uint32_t capabilities) {
	NeruTextField *field = data;
	if ((capabilities & WL_SEAT_CAPABILITY_KEYBOARD) && !field->keyboard) {
		field->keyboard = wl_seat_get_keyboard(seat);
		wl_keyboard_add_listener(field->keyboard, &neru_text_field_keyboard_listener, field);
	}
}

static void neru_seat_name_cb(void *data, struct wl_seat *seat, const char *name) {}

static const struct wl_seat_listener neru_text_field_seat_listener = {
    .capabilities = neru_seat_capabilities_cb,
    .name = neru_seat_name_cb,
};

static void neru_registry_global_cb(
    void *data, struct wl_registry *registry, uint32_t name, const char *interface, uint32_t version) {
	NeruTextField *field = data;
	if (strcmp(interface, wl_compositor_interface.name) == 0) {
		field->compositor = wl_registry_bind(registry, name, &wl_compositor_interface, 4);
	} else if (strcmp(interface, wl_shm_interface.name) == 0) {
		field->shm = wl_registry_bind(registry, name, &wl_shm_interface, 1);
	} else if (strcmp(interface, wl_seat_interface.name) == 0 && !field->seat) {
		field->seat = wl_registry_bind(registry, name, &wl_seat_interface, version < 5 ? version : 5);
		wl_seat_add_listener(field->seat, &neru_text_field_seat_listener, field);
	} else if (strcmp(interface, wl_output_interface.name) == 0 && field->output_count < NERU_TEXT_FIELD_MAX_OUTPUTS) {
		field->outputs[field->output_count++].output = wl_registry_bind(registry, name, &wl_output_interface, 1);
	} else if (strcmp(interface, zwlr_layer_shell_v1_interface.name) == 0) {
		field->layer_shell = wl_registry_bind(registry, name, &zwlr_layer_shell_v1_interface, 1);
	} else if (strcmp(interface, zxdg_output_manager_v1_interface.name) == 0) {
		field->output_manager = wl_registry_bind(registry, name, &zxdg_output_manager_v1_interface, 2);
	} else if (strcmp(interface, zwp_text_input_manager_v3_interface.name) == 0) {
		field->text_input_manager = wl_registry_bind(registry, name, &zwp_text_input_manager_v3_interface, 1);
	}
}

static void neru_registry_global_remove_cb(void *data, struct wl_registry *registry, uint32_t name) {}

static const struct wl_registry_listener neru_text_field_registry_listener = {
    .global = neru_registry_global_cb,
    .global_remove = neru_registry_global_remove_cb,
};

// neru_text_field_output_at returns the output holding x, y, or NULL to let
// the compositor choose, with origin set to that output's top-left corner.
static NeruTextFieldOutput *neru_text_field_output_at(NeruTextField *field, int x, int y) {
	for (int i = 0; i < field->output_count; i++) {
		NeruTextFieldOutput *output = &field->outputs[i];
		if (x >= output->x && y >= output->y && x < output->x + output->width && y < output->y + output->height) {
			return output;
		}
	}
	return NULL;
}

static void neru_text_field_compose_init(NeruTextField *field) {
	const char *locale = getenv("LC_ALL");
	if (!locale || !*locale) {
		locale = getenv("LC_CTYPE");
	}
	if (!locale || !*locale) {
		locale = getenv("LANG");
	}
	if (!locale || !*locale) {
		locale = "C";
	}
	struct xkb_compose_table *table =
	    xkb_compose_table_new_from_locale(field->xkb_context, locale, XKB_COMPOSE_COMPILE_NO_FLAGS);
	if (table) {
		field->compose_state = xkb_compose_state_new(table, XKB_COMPOSE_STATE_NO_FLAGS);
		xkb_compose_table_unref(table);
	}
}

NeruTextField *neru_text_field_open(int x, int y, int width, int height) {
	if (width <= 0 || height <= 0) {
		return NULL;
	}

	NeruTextField *field = calloc(1, sizeof(NeruTextField));
	if (!field) {
		return NULL;
	}
	field->repeat_fd = -1;
	field->width = width;
	field->height = height;

	field->display = wl_display_connect(NULL);
	field->xkb_context = xkb_context_new(XKB_CONTEXT_NO_FLAGS);
	field->repeat_fd = timerfd_create(CLOCK_MONOTONIC, TFD_NONBLOCK | TFD_CLOEXEC);
	if (!field->display || !field->xkb_context || field->repeat_fd < 0) {
		neru_text_field_close(field);
		return NULL;
	}

	struct timespec deadline;
	clock_gettime(CLOCK_MONOTONIC, &deadline);
	deadline.tv_nsec += NERU_TEXT_FIELD_OPEN_TIMEOUT_MS * 1000000L;
	deadline.tv_sec += deadline.tv_nsec / 1000000000L;
	deadline.tv_nsec %= 1000000000L;

	field->registry = wl_display_get_registry(field->display);
	wl_registry_add_listener(field->registry, &neru_text_field_registry_listener, field);
	if (neru_text_field_roundtrip(field, &deadline) != 0) {
		neru_text_field_close(field);
		return NULL;
	}

	if (!field->compositor || !field->shm || !field->seat || !field->layer_shell) {
		neru_text_field_close(field);
		return NULL;
	}

	if (field->output_manager) {
		for (int i = 0; i < field->output_count; i++) {
			NeruTextFieldOutput *output = &field->outputs[i];
			output->xdg_output = zxdg_output_manager_v1_get_xdg_output(field->output_manager, output->output);
			zxdg_output_v1_add_listener(output->xdg_output, &neru_text_field_xdg_output_listener, output);
		}
	}
	// The second round trip delivers the seat's keyboard and the output layout.
	if (neru_text_field_roundtrip(field, &deadline) != 0 || !field->keyboard) {
		neru_text_field_close(field);
		return NULL;
	}

	neru_text_field_compose_init(field);

	NeruTextFieldOutput *output = neru_text_field_output_at(field, x, y);
	int left = output ? x - output->x : x;
	int top = output ? y - output->y : y;

	field->surface = wl_compositor_create_surface(field->compositor);
	struct wl_region *empty = wl_compositor_create_region(field->compositor);
	wl_surface_set_input_region(field->surface, empty);
	wl_region_destroy(empty);

	field->layer_surface = zwlr_layer_shell_v1_get_layer_surface(
	    field->layer_shell, field->surface, output ? output->output : NULL, ZWLR_LAYER_SHELL_V1_LAYER_OVERLAY,
	    "neru-hint-search");
	zwlr_layer_surface_v1_add_listener(field->layer_surface, &neru_text_field_layer_listener, field);
	zwlr_layer_surface_v1_set_size(field->layer_surface, width, height);
	zwlr_layer_surface_v1_set_anchor(
	    field->layer_surface, ZWLR_LAYER_SURFACE_V1_ANCHOR_TOP | ZWLR_LAYER_SURFACE_V1_ANCHOR_LEFT);
	zwlr_layer_surface_v1_set_margin(field->layer_surface, top, 0, 0, left);
	zwlr_layer_surface_v1_set_exclusive_zone(field->layer_surface, -1);
	zwlr_layer_surface_v1_set_keyboard_interactivity(
	    field->layer_surface, ZWLR_LAYER_SURFACE_V1_KEYBOARD_INTERACTIVITY_EXCLUSIVE);

	if (field->text_input_manager) {
		field->text_input = zwp_text_input_manager_v3_get_text_input(field->text_input_manager, field->seat);
		zwp_text_input_v3_add_listener(field->text_input, &neru_text_field_text_input_listener, field);
	}

	// Hand the field over only once it holds the keyboard. The caller turns the
	// key capture off when it gets the field, and until the surface has focus
	// the compositor still sends keys to the window the user was in.
	wl_surface_commit(field->surface);
	while (!field->focused) {
		if (neru_text_field_roundtrip(field, &deadline) != 0 || (field->flags & NERU_TEXT_FIELD_CANCEL)) {
			neru_text_field_close(field);
			return NULL;
		}
	}
	field->flags = 0;
	wl_display_flush(field->display);

	return field;
}

int neru_text_field_display_fd(NeruTextField *field) { return wl_display_get_fd(field->display); }

int neru_text_field_repeat_fd(NeruTextField *field) { return field->repeat_fd; }

static int neru_text_field_take_flags(NeruTextField *field) {
	int flags = field->flags;
	field->flags = 0;
	wl_display_flush(field->display);
	return flags;
}

int neru_text_field_dispatch(NeruTextField *field) {
	// The caller polled the descriptor readable, so the read does not block.
	if (wl_display_prepare_read(field->display) == 0 && wl_display_read_events(field->display) < 0) {
		return -1;
	}
	if (wl_display_dispatch_pending(field->display) < 0) {
		return -1;
	}
	return neru_text_field_take_flags(field);
}

int neru_text_field_repeat(NeruTextField *field) {
	uint64_t expirations = 0;
	if (read(field->repeat_fd, &expirations, sizeof(expirations)) != sizeof(expirations) || !field->repeat_key) {
		return 0;
	}
	neru_text_field_key_press(field, field->repeat_key);
	return neru_text_field_take_flags(field);
}

const char *neru_text_field_query(NeruTextField *field) {
	snprintf(field->query, sizeof(field->query), "%s%s", field->committed, field->preedit ? field->preedit : "");
	return field->query;
}

void neru_text_field_close(NeruTextField *field) {
	if (!field) {
		return;
	}
	if (field->text_input) {
		zwp_text_input_v3_destroy(field->text_input);
	}
	if (field->layer_surface) {
		zwlr_layer_surface_v1_destroy(field->layer_surface);
	}
	if (field->buffer) {
		wl_buffer_destroy(field->buffer);
	}
	if (field->surface) {
		wl_surface_destroy(field->surface);
	}
	if (field->keyboard) {
		wl_keyboard_destroy(field->keyboard);
	}
	for (int i = 0; i < field->output_count; i++) {
		if (field->outputs[i].xdg_output) {
			zxdg_output_v1_destroy(field->outputs[i].xdg_output);
		}
		wl_output_destroy(field->outputs[i].output);
	}
	if (field->text_input_manager) {
		zwp_text_input_manager_v3_destroy(field->text_input_manager);
	}
	if (field->output_manager) {
		zxdg_output_manager_v1_destroy(field->output_manager);
	}
	if (field->layer_shell) {
		zwlr_layer_shell_v1_destroy(field->layer_shell);
	}
	if (field->seat) {
		wl_seat_destroy(field->seat);
	}
	if (field->shm) {
		wl_shm_destroy(field->shm);
	}
	if (field->compositor) {
		wl_compositor_destroy(field->compositor);
	}
	if (field->registry) {
		wl_registry_destroy(field->registry);
	}
	if (field->display) {
		wl_display_flush(field->display);
		wl_display_disconnect(field->display);
	}
	if (field->compose_state) {
		xkb_compose_state_unref(field->compose_state);
	}
	if (field->xkb_state) {
		xkb_state_unref(field->xkb_state);
	}
	if (field->xkb_keymap) {
		xkb_keymap_unref(field->xkb_keymap);
	}
	if (field->xkb_context) {
		xkb_context_unref(field->xkb_context);
	}
	if (field->repeat_fd >= 0) {
		close(field->repeat_fd);
	}
	free(field->preedit);
	free(field->pending_preedit);
	free(field->pending_commit);
	explicit_bzero(field->committed, sizeof(field->committed));
	explicit_bzero(field->query, sizeof(field->query));
	free(field);
}
