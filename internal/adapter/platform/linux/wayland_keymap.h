#ifndef NERU_WAYLAND_KEYMAP_H
#define NERU_WAYLAND_KEYMAP_H

#include <stddef.h>
#include <stdint.h>

struct neru_xkb_state;
typedef struct neru_xkb_state neru_xkb_state;

// Connect to the Wayland display and retrieve the compositor's keymap
// via wl_keyboard to create an xkb_state. Returns NULL on failure.
neru_xkb_state *neru_xkb_state_create(void);

// Read whatever the compositor has sent since the last call without blocking,
// so a keymap it replaced (a layout or option change) takes effect on the
// state. Returns 1 when the keymap was replaced, 0 when it was not, -1 when the
// display connection is gone and the state has to be rebuilt.
int neru_xkb_state_dispatch(neru_xkb_state *state);

// Destroy the xkb_state and associated Wayland resources.
void neru_xkb_state_destroy(neru_xkb_state *state);

// Feed a key press (is_press=1) or release (is_press=0) to the xkb_state.
void neru_xkb_state_key(neru_xkb_state *state, uint16_t evdev_code, int is_press);

// Resolve the xkb key name for the given evdev scan code in the live layout.
// Writes the key name into buf (up to buf_size bytes).
// Returns 0 on success, -1 on failure.
int neru_xkb_state_key_get_name(neru_xkb_state *state, uint16_t evdev_code, char *buf, size_t buf_size);

// Resolve the name a key has for Neru's own bindings. The live modifiers apply
// in the reference layout, which is the first ASCII-capable layout of the
// keymap, whatever layout is active. A keymap with no ASCII-capable layout
// resolves in the live layout. The result and buffer contract match
// neru_xkb_state_key_get_name, which always resolves in the live layout.
int neru_xkb_state_key_get_command_name(neru_xkb_state *state, uint16_t evdev_code, char *buf, size_t buf_size);

// Force the reference layout to the one whose XKB name matches name, ignoring
// case. NULL or "" returns to the automatic choice. The state keeps the name
// and matches it again against every later keymap. Returns 1 when the current
// keymap has the layout or the choice is automatic. Returns 0 when it does not,
// and the automatic choice applies until a keymap that has it arrives.
int neru_xkb_state_set_reference_layout(neru_xkb_state *state, const char *name);

// Whether the current keymap has the forced layout; 1 when none is forced.
int neru_xkb_state_reference_found(neru_xkb_state *state);

// The keymap's layouts, for listing: how many there are, and the XKB name of
// one, or NULL when it has none. The name lives until the keymap is replaced.
int neru_xkb_state_layout_count(neru_xkb_state *state);
const char *neru_xkb_state_layout_name(neru_xkb_state *state, int layout);

// The index of the reference layout, or -1 when keys resolve in the live one.
int neru_xkb_state_reference_index(neru_xkb_state *state);

// Build a state from keymap text in the format wl_keyboard.keymap delivers.
// The state has no display, so it never dispatches or receives a new keymap.
// Tests use it to pin naming against real layouts. Destroy it with
// neru_xkb_state_destroy. Returns NULL when the text does not compile.
neru_xkb_state *neru_xkb_state_create_from_keymap(const char *keymap);

// Name a state-resolved keysym: its character when it types one, else the
// keysym name folded onto the spelling Neru binds. This is the rule
// neru_xkb_state_key_get_name applies, exposed so it can be pinned without a
// keymap. Returns 0 on success, -1 when the keysym has no name.
int neru_xkb_keysym_name(uint32_t keysym, char *buf, size_t buf_size);

void neru_xkb_state_sync_leds(neru_xkb_state *state, int num_lock_on, int caps_lock_on);

#endif
