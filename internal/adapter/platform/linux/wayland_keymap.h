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

// Resolve the xkb key name for the given evdev scan code.
// Writes the key name into buf (up to buf_size bytes).
// Returns 0 on success, -1 on failure.
int neru_xkb_state_key_get_name(neru_xkb_state *state, uint16_t evdev_code, char *buf, size_t buf_size);

// Name a state-resolved keysym: its character when it types one, else the
// keysym name folded onto the spelling Neru binds. This is the rule
// neru_xkb_state_key_get_name applies, exposed so it can be pinned without a
// keymap. Returns 0 on success, -1 when the keysym has no name.
int neru_xkb_keysym_name(uint32_t keysym, char *buf, size_t buf_size);

void neru_xkb_state_sync_leds(neru_xkb_state *state, int num_lock_on, int caps_lock_on);

#endif
