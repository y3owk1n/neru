#include "libei_client.h"

#include <libei.h>
#include <liboeffis.h>
#include <poll.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

struct NeruEiClient {
	struct oeffis *oeffis;
	struct ei *ei;

	struct ei_device *pointer;
	int pointer_resumed;
	int pointer_emulating;

	struct ei_device *keyboard;
	int keyboard_resumed;
	int keyboard_emulating;

	uint32_t seq;
};

static int64_t now_ms(void) {
	struct timespec ts;
	clock_gettime(CLOCK_MONOTONIC, &ts);
	return (int64_t)ts.tv_sec * 1000 + ts.tv_nsec / 1000000;
}

// wait_readable blocks until fd is readable or the deadline passes. Returns 1
// when readable, 0 on timeout, -1 on error.
static int wait_readable(int fd, int64_t deadline_ms) {
	int64_t remaining = deadline_ms - now_ms();
	if (remaining < 0) {
		remaining = 0;
	}

	struct pollfd pfd = {.fd = fd, .events = POLLIN};
	int rc = poll(&pfd, 1, (int)remaining);
	if (rc > 0) {
		return 1;
	}
	if (rc == 0) {
		return 0;
	}
	return -1;
}

// drain processes all currently queued libei events, wiring up the seat, the
// pointer/keyboard devices, and their paused/resumed lifecycle.
static void drain(NeruEiClient *c) {
	struct ei_event *e;
	while ((e = ei_get_event(c->ei)) != NULL) {
		switch (ei_event_get_type(e)) {
		case EI_EVENT_SEAT_ADDED: {
			struct ei_seat *seat = ei_event_get_seat(e);
			// Bind relative pointer alongside absolute: after an absolute warp
			// KWin updates the logical pointer (clicks land) but does not always
			// repaint the visible cursor sprite. A zero-delta relative motion on
			// the same device forces the sprite to snap to the warped position.
			ei_seat_bind_capabilities(
			    seat, EI_DEVICE_CAP_POINTER, EI_DEVICE_CAP_POINTER_ABSOLUTE, EI_DEVICE_CAP_BUTTON, EI_DEVICE_CAP_SCROLL,
			    EI_DEVICE_CAP_KEYBOARD, NULL);
			break;
		}
		case EI_EVENT_DEVICE_ADDED: {
			struct ei_device *d = ei_event_get_device(e);
			if (!c->pointer && ei_device_has_capability(d, EI_DEVICE_CAP_POINTER_ABSOLUTE)) {
				c->pointer = ei_device_ref(d);
			}
			if (!c->keyboard && ei_device_has_capability(d, EI_DEVICE_CAP_KEYBOARD)) {
				c->keyboard = ei_device_ref(d);
			}
			break;
		}
		case EI_EVENT_DEVICE_RESUMED: {
			struct ei_device *d = ei_event_get_device(e);
			if (d == c->pointer) {
				c->pointer_resumed = 1;
				c->pointer_emulating = 0;
			}
			if (d == c->keyboard) {
				c->keyboard_resumed = 1;
				c->keyboard_emulating = 0;
			}
			break;
		}
		case EI_EVENT_DEVICE_PAUSED: {
			struct ei_device *d = ei_event_get_device(e);
			if (d == c->pointer) {
				c->pointer_resumed = 0;
				c->pointer_emulating = 0;
			}
			if (d == c->keyboard) {
				c->keyboard_resumed = 0;
				c->keyboard_emulating = 0;
			}
			break;
		}
		case EI_EVENT_DEVICE_REMOVED: {
			struct ei_device *d = ei_event_get_device(e);
			if (d == c->pointer) {
				ei_device_unref(c->pointer);
				c->pointer = NULL;
				c->pointer_resumed = 0;
				c->pointer_emulating = 0;
			}
			if (d == c->keyboard) {
				ei_device_unref(c->keyboard);
				c->keyboard = NULL;
				c->keyboard_resumed = 0;
				c->keyboard_emulating = 0;
			}
			break;
		}
		default:
			break;
		}
		ei_event_unref(e);
	}
}

// pump dispatches any pending socket data and drains the resulting events.
// Non-blocking; safe to call before every emit.
static void pump(NeruEiClient *c) {
	int fd = ei_get_fd(c->ei);
	struct pollfd pfd = {.fd = fd, .events = POLLIN};
	if (poll(&pfd, 1, 0) > 0) {
		ei_dispatch(c->ei);
	}
	drain(c);
}

// ensure_emulating starts a new emulation transaction on a resumed device.
// Returns 1 when the device is ready to receive events.
static int ensure_emulating(NeruEiClient *c, struct ei_device *device, int resumed, int *emulating) {
	if (!device || !resumed) {
		return 0;
	}
	if (!*emulating) {
		ei_device_start_emulating(device, ++c->seq);
		*emulating = 1;
	}
	return 1;
}

NeruEiClient *neru_ei_connect(int timeout_ms) {
	NeruEiClient *c = calloc(1, sizeof(*c));
	if (!c) {
		return NULL;
	}

	int64_t deadline = now_ms() + (timeout_ms > 0 ? timeout_ms : 30000);

	// 1) Portal session via liboeffis -> EIS fd.
	c->oeffis = oeffis_new(NULL);
	if (!c->oeffis) {
		neru_ei_disconnect(c);
		return NULL;
	}
	oeffis_create_session(c->oeffis, OEFFIS_DEVICE_POINTER | OEFFIS_DEVICE_KEYBOARD);

	int eis_fd = -1;
	int ofd = oeffis_get_fd(c->oeffis);
	while (eis_fd < 0) {
		if (wait_readable(ofd, deadline) != 1) {
			neru_ei_disconnect(c);
			return NULL;
		}
		oeffis_dispatch(c->oeffis);
		enum oeffis_event_type ev = oeffis_get_event(c->oeffis);
		if (ev == OEFFIS_EVENT_CONNECTED_TO_EIS) {
			eis_fd = oeffis_get_eis_fd(c->oeffis);
		} else if (ev == OEFFIS_EVENT_DISCONNECTED || ev == OEFFIS_EVENT_CLOSED) {
			neru_ei_disconnect(c);
			return NULL;
		}
	}

	// 2) libei sender context attached to the EIS fd.
	c->ei = ei_new_sender(NULL);
	if (!c->ei) {
		neru_ei_disconnect(c);
		return NULL;
	}
	ei_configure_name(c->ei, "neru");
	if (ei_setup_backend_fd(c->ei, eis_fd) != 0) {
		neru_ei_disconnect(c);
		return NULL;
	}

	// 3) Pump the event loop until the absolute pointer is resumed.
	int efd = ei_get_fd(c->ei);
	while (!c->pointer_resumed) {
		if (wait_readable(efd, deadline) != 1) {
			neru_ei_disconnect(c);
			return NULL;
		}
		ei_dispatch(c->ei);
		drain(c);
	}

	return c;
}

void neru_ei_disconnect(NeruEiClient *c) {
	if (!c) {
		return;
	}
	if (c->pointer) {
		if (c->pointer_emulating) {
			ei_device_stop_emulating(c->pointer);
		}
		ei_device_unref(c->pointer);
	}
	if (c->keyboard) {
		if (c->keyboard_emulating) {
			ei_device_stop_emulating(c->keyboard);
		}
		ei_device_unref(c->keyboard);
	}
	if (c->ei) {
		ei_unref(c->ei);
	}
	if (c->oeffis) {
		oeffis_unref(c->oeffis);
	}
	free(c);
}

int neru_ei_move_abs(NeruEiClient *c, int x, int y) {
	if (!c) {
		return 0;
	}
	pump(c);
	if (!ensure_emulating(c, c->pointer, c->pointer_resumed, &c->pointer_emulating)) {
		return 0;
	}
	ei_device_pointer_motion_absolute(c->pointer, (double)x, (double)y);
	ei_device_frame(c->pointer, ei_now(c->ei));

	// Nudge with a zero-delta relative motion so KWin repaints the cursor sprite
	// at the warped position. The absolute motion alone updates the logical
	// pointer (clicks land) but the visible cursor can lag behind on KWin.
	if (ei_device_has_capability(c->pointer, EI_DEVICE_CAP_POINTER)) {
		ei_device_pointer_motion(c->pointer, 0.0, 0.0);
		ei_device_frame(c->pointer, ei_now(c->ei));
	}

	ei_dispatch(c->ei);
	return 1;
}

int neru_ei_button(NeruEiClient *c, int button, int pressed) {
	if (!c) {
		return 0;
	}
	pump(c);
	if (!ensure_emulating(c, c->pointer, c->pointer_resumed, &c->pointer_emulating)) {
		return 0;
	}
	ei_device_button_button(c->pointer, (uint32_t)button, pressed != 0);
	ei_device_frame(c->pointer, ei_now(c->ei));
	ei_dispatch(c->ei);
	return 1;
}

int neru_ei_scroll(NeruEiClient *c, int axis, int delta) {
	if (!c) {
		return 0;
	}
	pump(c);
	if (!ensure_emulating(c, c->pointer, c->pointer_resumed, &c->pointer_emulating)) {
		return 0;
	}
	if (axis == 1) {
		ei_device_scroll_delta(c->pointer, (double)delta, 0.0);
	} else {
		ei_device_scroll_delta(c->pointer, 0.0, (double)delta);
	}
	ei_device_frame(c->pointer, ei_now(c->ei));
	ei_dispatch(c->ei);
	return 1;
}

int neru_ei_key(NeruEiClient *c, int keycode, int pressed) {
	if (!c) {
		return 0;
	}
	pump(c);
	if (!ensure_emulating(c, c->keyboard, c->keyboard_resumed, &c->keyboard_emulating)) {
		return 0;
	}
	ei_device_keyboard_key(c->keyboard, (uint32_t)keycode, pressed != 0);
	ei_device_frame(c->keyboard, ei_now(c->ei));
	ei_dispatch(c->ei);
	return 1;
}

int neru_ei_has_keyboard(NeruEiClient *c) { return c && c->keyboard != NULL; }
