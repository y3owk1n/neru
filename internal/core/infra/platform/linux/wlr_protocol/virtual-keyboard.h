/* Minimal client protocol declarations for virtual-keyboard-unstable-v1. */

#ifndef VIRTUAL_KEYBOARD_UNSTABLE_V1_CLIENT_PROTOCOL_H
#define VIRTUAL_KEYBOARD_UNSTABLE_V1_CLIENT_PROTOCOL_H

#include <stdint.h>
#include <stddef.h>
#include "wayland-client.h"

#ifdef  __cplusplus
extern "C" {
#endif

struct wl_seat;
struct zwp_virtual_keyboard_manager_v1;
struct zwp_virtual_keyboard_v1;

extern const struct wl_interface zwp_virtual_keyboard_manager_v1_interface;
extern const struct wl_interface zwp_virtual_keyboard_v1_interface;

#ifndef ZWP_VIRTUAL_KEYBOARD_V1_ERROR_ENUM
#define ZWP_VIRTUAL_KEYBOARD_V1_ERROR_ENUM
enum zwp_virtual_keyboard_v1_error {
	ZWP_VIRTUAL_KEYBOARD_V1_ERROR_NO_KEYMAP = 0,
};
#endif /* ZWP_VIRTUAL_KEYBOARD_V1_ERROR_ENUM */

#ifndef ZWP_VIRTUAL_KEYBOARD_MANAGER_V1_ERROR_ENUM
#define ZWP_VIRTUAL_KEYBOARD_MANAGER_V1_ERROR_ENUM
enum zwp_virtual_keyboard_manager_v1_error {
	ZWP_VIRTUAL_KEYBOARD_MANAGER_V1_ERROR_UNAUTHORIZED = 0,
};
#endif /* ZWP_VIRTUAL_KEYBOARD_MANAGER_V1_ERROR_ENUM */

#define ZWP_VIRTUAL_KEYBOARD_V1_KEYMAP 0
#define ZWP_VIRTUAL_KEYBOARD_V1_KEY 1
#define ZWP_VIRTUAL_KEYBOARD_V1_MODIFIERS 2
#define ZWP_VIRTUAL_KEYBOARD_V1_DESTROY 3

#define ZWP_VIRTUAL_KEYBOARD_MANAGER_V1_CREATE_VIRTUAL_KEYBOARD 0

static inline void
zwp_virtual_keyboard_v1_keymap(struct zwp_virtual_keyboard_v1 *zwp_virtual_keyboard_v1, uint32_t format, int32_t fd, uint32_t size)
{
	wl_proxy_marshal((struct wl_proxy *) zwp_virtual_keyboard_v1,
			 ZWP_VIRTUAL_KEYBOARD_V1_KEYMAP, format, fd, size);
}

static inline void
zwp_virtual_keyboard_v1_key(struct zwp_virtual_keyboard_v1 *zwp_virtual_keyboard_v1, uint32_t time, uint32_t key, uint32_t state)
{
	wl_proxy_marshal((struct wl_proxy *) zwp_virtual_keyboard_v1,
			 ZWP_VIRTUAL_KEYBOARD_V1_KEY, time, key, state);
}

static inline void
zwp_virtual_keyboard_v1_modifiers(struct zwp_virtual_keyboard_v1 *zwp_virtual_keyboard_v1, uint32_t mods_depressed, uint32_t mods_latched, uint32_t mods_locked, uint32_t group)
{
	wl_proxy_marshal((struct wl_proxy *) zwp_virtual_keyboard_v1,
			 ZWP_VIRTUAL_KEYBOARD_V1_MODIFIERS, mods_depressed, mods_latched, mods_locked, group);
}

static inline void
zwp_virtual_keyboard_v1_destroy(struct zwp_virtual_keyboard_v1 *zwp_virtual_keyboard_v1)
{
	wl_proxy_marshal((struct wl_proxy *) zwp_virtual_keyboard_v1,
			 ZWP_VIRTUAL_KEYBOARD_V1_DESTROY);
	wl_proxy_destroy((struct wl_proxy *) zwp_virtual_keyboard_v1);
}

static inline struct zwp_virtual_keyboard_v1 *
zwp_virtual_keyboard_manager_v1_create_virtual_keyboard(struct zwp_virtual_keyboard_manager_v1 *zwp_virtual_keyboard_manager_v1, struct wl_seat *seat)
{
	struct wl_proxy *id;

	id = wl_proxy_marshal_constructor((struct wl_proxy *) zwp_virtual_keyboard_manager_v1,
			 ZWP_VIRTUAL_KEYBOARD_MANAGER_V1_CREATE_VIRTUAL_KEYBOARD, &zwp_virtual_keyboard_v1_interface, seat, NULL);

	return (struct zwp_virtual_keyboard_v1 *) id;
}

#ifdef  __cplusplus
}
#endif

#endif
