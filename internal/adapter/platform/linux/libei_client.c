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

	struct ei_seat *seat;  // Saved seat for two-stage capability bind
	int seat_caps_bound;   // Keyboard bound, pointer caps still pending

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
			struct ei_seat *s = ei_event_get_seat(e);
			if (!c->seat) {
				c->seat = ei_seat_ref(s);
				// Stage 1: bind keyboard alone. KWin's EIS implementation
				// silently drops the keyboard device when it is bound
				// alongside pointer capabilities in a single call (KDE bug
				// (https://bugs.kde.org/show_bug.cgi?id=520464, confirmed on Plasma 6). After this bind is flushed
				// and the keyboard device exists, a second bind adds the
				// pointer capabilities without affecting the keyboard.
				ei_seat_bind_capabilities(s, EI_DEVICE_CAP_KEYBOARD, NULL);
			}
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

		// Two-stage capability bind: KDE bug https://bugs.kde.org/show_bug.cgi?id=520464 causes KWin to silently
		// drop the keyboard device when keyboard and pointer are bound together
		// in a single ei_seat_bind_capabilities() call. The workaround:
		//   Stage 1 (in drain's EI_EVENT_SEAT_ADDED handler) binds keyboard
		//   alone so the keyboard device is created before any pointer caps
		//   are requested.
		//   Stage 2 (here) flushes the keyboard bind, then binds the remaining
		//   pointer capabilities. KWin sees the keyboard already exists and
		//   keeps it, while also creating the pointer devices.
		if (c->seat && !c->seat_caps_bound) {
			ei_dispatch(c->ei);
			// Drain any keyboard device events that arrived in response to
			// the stage 1 bind before queuing stage 2, so c->keyboard and
			// c->keyboard_resumed reflect the actual state.
			drain(c);
			ei_seat_bind_capabilities(
			    c->seat, EI_DEVICE_CAP_KEYBOARD, EI_DEVICE_CAP_POINTER, EI_DEVICE_CAP_POINTER_ABSOLUTE,
			    EI_DEVICE_CAP_BUTTON, EI_DEVICE_CAP_SCROLL, NULL);
			c->seat_caps_bound = 1;
		}
	}

	// 3b) Give the keyboard device a brief extra window to appear after the
	// pointer. The portal may advertise both devices but the keyboard add/resume
	// events can arrive slightly later. If keyboard was not granted by the portal
	// this loop simply times out in ~1 s and the connection still succeeds so
	// pointer/click/scroll work. The caller can check neru_ei_has_keyboard.
	{
		int64_t kbd_deadline = now_ms() + 1000;
		while (!c->keyboard_resumed && now_ms() < kbd_deadline) {
			if (wait_readable(efd, kbd_deadline) != 1) {
				break;
			}
			ei_dispatch(c->ei);
			drain(c);
		}
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
	if (c->seat) {
		ei_seat_unref(c->seat);
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

int neru_ei_has_keyboard(NeruEiClient *c) {
	if (!c) {
		return 0;
	}
	// The keyboard device may have been added/resumed after we finished waiting
	// in neru_ei_connect (which only waits for the pointer device). Pump any
	// pending EIS events so the keyboard state is visible to the caller.
	pump(c);
	// Only check device existence, not resume state: a transient pause by the
	// compositor would otherwise make the Go layer report a permanent "not
	// granted" error even though the device exists and will be resumed shortly.
	return c->keyboard != NULL;
}
