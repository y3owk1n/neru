#ifndef NERU_WAYLAND_KEYMAP_H
#define NERU_WAYLAND_KEYMAP_H

#include <stddef.h>
#include <stdint.h>

struct neru_xkb_state;
typedef struct neru_xkb_state neru_xkb_state;

// Connect to the Wayland display and retrieve the compositor's keymap
// via wl_keyboard to create an xkb_state. Returns NULL on failure.
neru_xkb_state *neru_xkb_state_create(void);

// Destroy the xkb_state and associated Wayland resources.
void neru_xkb_state_destroy(neru_xkb_state *state);

// Feed a key press (is_press=1) or release (is_press=0) to the xkb_state.
void neru_xkb_state_key(neru_xkb_state *state, uint16_t evdev_code, int is_press);

// Resolve the xkb key name for the given evdev scan code.
// Writes the key name into buf (up to buf_size bytes).
// Returns 0 on success, -1 on failure.
int neru_xkb_state_key_get_name(neru_xkb_state *state, uint16_t evdev_code, char *buf, size_t buf_size);

#endif
