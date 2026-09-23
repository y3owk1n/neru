// Standalone native regression test; no compositor or input devices required.
// cc scripts/test-first-xkb-layout.c $(pkg-config --cflags --libs wayland-client xkbcommon) -o /tmp/neru-xkb-test
// /tmp/neru-xkb-test
#include "../internal/adapter/platform/linux/wayland_keymap.c"

#include <assert.h>
#include <stdio.h>

static void load_keymap(neru_xkb_state *state, const char *layout, const char *variant) {
	struct xkb_context *ctx = xkb_context_new(XKB_CONTEXT_NO_FLAGS);
	assert(ctx);
	struct xkb_rule_names names = {.layout = layout, .variant = variant, .options = "ctrl:swapcaps,lv3:ralt_switch"};
	struct xkb_keymap *map = xkb_keymap_new_from_names(ctx, &names, XKB_KEYMAP_COMPILE_NO_FLAGS);
	assert(map);
	char *text = xkb_keymap_get_as_string(map, XKB_KEYMAP_FORMAT_TEXT_V1);
	assert(text);
	FILE *file = tmpfile();
	assert(file);
	size_t size = strlen(text) + 1;
	assert(fwrite(text, 1, size, file) == size);
	assert(fflush(file) == 0);
	int fd = dup(fileno(file));
	assert(fd >= 0);
	neru_keyboard_keymap(&state->kr, NULL, WL_KEYBOARD_KEYMAP_FORMAT_XKB_V1, fd, size);
	state->state = state->kr.state;
	assert(state->state && state->kr.command_state);
	fclose(file);
	free(text);
	xkb_keymap_unref(map);
	xkb_context_unref(ctx);
}

static void expect_name(neru_xkb_state *state, const char *key, const char *want, int command) {
	xkb_keycode_t code = xkb_keymap_key_by_name(xkb_state_get_keymap(state->state), key);
	assert(code != XKB_KEYCODE_INVALID && code >= 8);
	char buf[64];
	int result = command ? neru_xkb_state_key_get_command_name(state, code - 8, buf, sizeof(buf))
	                     : neru_xkb_state_key_get_name(state, code - 8, buf, sizeof(buf));
	if (result != 0 || strcmp(buf, want) != 0) {
		fprintf(
		    stderr, "%s (%s): got %s, want %s\n", key, command ? "command" : "live", result ? "<error>" : buf, want);
		abort();
	}
}

static xkb_mod_mask_t mod(neru_xkb_state *state, const char *name) {
	xkb_mod_index_t index = xkb_keymap_mod_get_index(xkb_state_get_keymap(state->state), name);
	assert(index != XKB_MOD_INVALID);
	return (xkb_mod_mask_t)1 << index;
}

static void check_layout(
    neru_xkb_state *state, const char *layout, const char *variant, const char *base, const char *shifted) {
	load_keymap(state, layout, variant);
	for (unsigned group = 0; group < 2; group++) {
		// Cover depressed, latched and locked groups independently. None may
		// leak into command lookup or be mutated by it.
		for (unsigned component = 0; component < 3; component++) {
			unsigned groups[3] = {0};
			groups[component] = group;
			xkb_state_update_mask(state->state, 0, 0, 0, groups[0], groups[1], groups[2]);
			expect_name(state, "AD01", group ? "й" : base, 0);
			expect_name(state, "AD01", base, 1);
			expect_name(state, "AD01", group ? "й" : base, 0);
			assert(xkb_state_serialize_layout(state->state, XKB_STATE_LAYOUT_EFFECTIVE) == group);
			expect_name(state, "RTRN", "Return", 1);
			expect_name(state, "SPCE", "space", 1);
			expect_name(state, "BKSP", "BackSpace", 1);
			expect_name(state, "CAPS", "Control_L", 0);
		}
		xkb_state_update_mask(state->state, mod(state, "Shift"), 0, 0, 0, 0, group);
		expect_name(state, "AD01", shifted, 1);
		expect_name(state, "TAB", "Tab", 1);
		xkb_state_update_mask(state->state, 0, mod(state, "Shift"), 0, 0, 0, group);
		expect_name(state, "AD01", shifted, 1);
	}
}

int main(int argc, char **argv) {
	neru_xkb_state *state = calloc(1, sizeof(*state));
	assert(state);
	check_layout(state, "us,ru", ",", "q", "Q");
	assert(!state->kr.changed);
	check_layout(state, "us,ru", "dvorak,", "'", "\"");
	assert(state->kr.changed);  // replacement refreshed both states
	check_layout(state, "de,ru", ",", "q", "Q");
	xkb_state_update_mask(state->state, 0, 0, mod(state, "Lock"), 0, 0, 1);
	expect_name(state, "AD01", "Q", 1);
	xkb_state_update_mask(state->state, mod(state, "Mod5"), 0, 0, 0, 0, 1);
	expect_name(state, "AD01", "@", 1);  // German AltGr, not Russian
	xkb_state_update_mask(state->state, 0, 0, mod(state, "Mod2"), 0, 0, 1);
	expect_name(state, "KP7", "7", 1);
	xkb_state_update_mask(state->state, 0, 0, 0, 0, 0, 1);
	expect_name(state, "KP7", "Home", 1);
	// A non-Latin first group is resolved too; config validation is separate.
	load_keymap(state, "ru,us", ",");
	xkb_state_update_mask(state->state, 0, 0, 0, 0, 0, 1);
	expect_name(state, "AD01", "й", 1);
	expect_name(state, "AD01", "q", 0);
	if (argc > 1) {
		// Optional user layout, supplied via XKB_CONFIG_EXTRA_PATH.
		load_keymap(state, argv[1], ",");
		xkb_state_update_mask(state->state, 0, 0, 0, 0, 0, 1);
		expect_name(state, "AB07", "f", 1);
		expect_name(state, "AB07", "ь", 0);
		xkb_state_update_mask(state->state, mod(state, "Shift"), 0, 0, 0, 0, 1);
		expect_name(state, "AB07", "F", 1);
	}
	neru_xkb_state_destroy(state);
	puts("first XKB layout: all checks passed");
	return 0;
}
