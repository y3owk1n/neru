/* Minimal client protocol metadata for virtual-keyboard-unstable-v1. */

#include <stdlib.h>
#include <stdint.h>
#include "wayland-util.h"

#ifndef __has_attribute
# define __has_attribute(x) 0
#endif

#if (__has_attribute(visibility) || defined(__GNUC__) && __GNUC__ >= 4)
#define WL_PRIVATE __attribute__ ((visibility("hidden")))
#else
#define WL_PRIVATE
#endif

extern const struct wl_interface wl_seat_interface;
extern const struct wl_interface zwp_virtual_keyboard_v1_interface;

static const struct wl_interface *virtual_keyboard_unstable_v1_types[] = {
	&wl_seat_interface,
	&zwp_virtual_keyboard_v1_interface,
	NULL,
	NULL,
	NULL,
	NULL,
};

static const struct wl_message zwp_virtual_keyboard_v1_requests[] = {
	{ "keymap", "uhu", virtual_keyboard_unstable_v1_types + 2 },
	{ "key", "uuu", virtual_keyboard_unstable_v1_types + 2 },
	{ "modifiers", "uuuu", virtual_keyboard_unstable_v1_types + 2 },
	{ "destroy", "", virtual_keyboard_unstable_v1_types + 2 },
};

WL_PRIVATE const struct wl_interface zwp_virtual_keyboard_v1_interface = {
	"zwp_virtual_keyboard_v1", 1,
	4, zwp_virtual_keyboard_v1_requests,
	0, NULL,
};

static const struct wl_message zwp_virtual_keyboard_manager_v1_requests[] = {
	{ "create_virtual_keyboard", "on", virtual_keyboard_unstable_v1_types + 0 },
};

WL_PRIVATE const struct wl_interface zwp_virtual_keyboard_manager_v1_interface = {
	"zwp_virtual_keyboard_manager_v1", 1,
	1, zwp_virtual_keyboard_manager_v1_requests,
	0, NULL,
};
