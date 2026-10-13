#include "textfield_x11.h"

#include "locale_guard.h"
#include "x11_error_trap.h"

#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/keysym.h>
#include <locale.h>
#include <poll.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

#define NERU_X11_TEXT_FIELD_MAX_QUERY 1024

// NERU_X11_TEXT_FIELD_MAP_TIMEOUT_MS bounds the wait for the window to map.
// The caller bounds the whole open as well. This only keeps prepare from
// waiting on a server that never maps the window.
#define NERU_X11_TEXT_FIELD_MAP_TIMEOUT_MS 250

struct NeruX11TextField {
	Display *display;
	int tolerated;
	Window window;
	Window previous_focus;
	XIM im;
	XIC ic;
	int finished;  // Return or Escape ended the search
	int flags;
	char committed[NERU_X11_TEXT_FIELD_MAX_QUERY];
};

static void neru_x11_text_field_append(NeruX11TextField *field, const char *text) {
	size_t used = strlen(field->committed);
	size_t add = strlen(text);
	if (used + add >= sizeof(field->committed)) {
		return;
	}
	memcpy(field->committed + used, text, add + 1);
	field->flags |= NERU_TEXT_FIELD_CHANGED;
}

static void neru_x11_text_field_delete_last(NeruX11TextField *field) {
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

// neru_x11_text_field_im_destroyed runs when the input-method server goes
// away. The method and its context are gone with it, and keys fall back to
// XLookupString.
static void neru_x11_text_field_im_destroyed(XIM im, XPointer client_data, XPointer call_data) {
	NeruX11TextField *field = (NeruX11TextField *)client_data;
	field->im = NULL;
	field->ic = NULL;
}

// neru_x11_text_field_open_im opens the input method XMODIFIERS names, or the
// built-in one that applies the locale's compose rules when there is none.
//
// XIM works in the locale the process has when the method opens, and a Go
// process runs in the C locale, which has no compose rules and which IBus and
// fcitx refuse. So LC_CTYPE is switched to the user's locale for the open and
// switched straight back, under the guard tesseract's engine creation takes
// (locale_guard.h). The method keeps the locale it was opened in.
static XIM neru_x11_text_field_open_im(Display *display) {
	neru_locale_guard_lock();

	char saved[256] = "C";
	const char *current = setlocale(LC_CTYPE, NULL);
	if (current) {
		snprintf(saved, sizeof(saved), "%s", current);
	}

	if (!setlocale(LC_CTYPE, "") || !XSupportsLocale()) {
		setlocale(LC_CTYPE, "C.UTF-8");
	}

	XIM im = NULL;
	if (XSupportsLocale()) {
		XSetLocaleModifiers("");
		im = XOpenIM(display, NULL, NULL, NULL);
		if (!im) {
			XSetLocaleModifiers("@im=none");
			im = XOpenIM(display, NULL, NULL, NULL);
		}
	}

	setlocale(LC_CTYPE, saved);
	neru_locale_guard_unlock();

	return im;
}

// neru_x11_text_field_create_ic picks the simplest style the method offers,
// letting it draw any composition in a window of its own.
static XIC neru_x11_text_field_create_ic(NeruX11TextField *field) {
	XIMStyles *styles = NULL;
	if (XGetIMValues(field->im, XNQueryInputStyle, &styles, NULL) || !styles) {
		return NULL;
	}

	XIMStyle chosen = 0;
	const XIMStyle wanted[] = {XIMPreeditNothing | XIMStatusNothing, XIMPreeditNone | XIMStatusNone};
	for (size_t w = 0; w < sizeof(wanted) / sizeof(wanted[0]) && !chosen; w++) {
		for (int i = 0; i < styles->count_styles; i++) {
			if (styles->supported_styles[i] == wanted[w]) {
				chosen = wanted[w];
				break;
			}
		}
	}
	XFree(styles);
	if (!chosen) {
		return NULL;
	}

	return XCreateIC(
	    field->im, XNInputStyle, chosen, XNClientWindow, field->window, XNFocusWindow, field->window, NULL);
}

static long neru_x11_text_field_ms_until(const struct timespec *deadline) {
	struct timespec now;
	clock_gettime(CLOCK_MONOTONIC, &now);
	return (deadline->tv_sec - now.tv_sec) * 1000L + (deadline->tv_nsec - now.tv_nsec) / 1000000L;
}

// neru_x11_text_field_wait_mapped waits for MapNotify on the window, which
// XSetInputFocus needs.
static int neru_x11_text_field_wait_mapped(NeruX11TextField *field) {
	struct timespec deadline;
	clock_gettime(CLOCK_MONOTONIC, &deadline);
	deadline.tv_nsec += NERU_X11_TEXT_FIELD_MAP_TIMEOUT_MS * 1000000L;
	deadline.tv_sec += deadline.tv_nsec / 1000000000L;
	deadline.tv_nsec %= 1000000000L;

	for (;;) {
		XEvent event;
		if (XCheckTypedWindowEvent(field->display, field->window, MapNotify, &event)) {
			return 1;
		}
		long remaining = neru_x11_text_field_ms_until(&deadline);
		if (remaining <= 0) {
			return 0;
		}
		struct pollfd ready = {.fd = ConnectionNumber(field->display), .events = POLLIN};
		if (poll(&ready, 1, (int)remaining) > 0) {
			XEventsQueued(field->display, QueuedAfterReading);
		}
	}
}

NeruX11TextField *neru_x11_text_field_prepare(int x, int y, int width, int height) {
	if (width <= 0 || height <= 0) {
		return NULL;
	}

	NeruX11TextField *field = calloc(1, sizeof(NeruX11TextField));
	if (!field) {
		return NULL;
	}

	field->display = XOpenDisplay(NULL);
	if (!field->display) {
		neru_x11_text_field_close(field);
		return NULL;
	}
	// Without tolerance, an input-method window vanishing mid-search would
	// reach Xlib's default handler, which exits.
	field->tolerated = neru_x11_error_tolerate(field->display);
	if (!field->tolerated) {
		neru_x11_text_field_close(field);
		return NULL;
	}

	// One pixel at the bottom-left corner of the box. The window has to be
	// mapped to hold the focus, and an input method places its candidates next
	// to it.
	XSetWindowAttributes attributes = {0};
	attributes.override_redirect = True;
	attributes.event_mask = KeyPressMask | KeyReleaseMask | FocusChangeMask | StructureNotifyMask;
	field->window = XCreateWindow(
	    field->display, DefaultRootWindow(field->display), x, y + height - 1, 1, 1, 0, CopyFromParent, InputOutput,
	    CopyFromParent, CWOverrideRedirect | CWEventMask, &attributes);

	field->im = neru_x11_text_field_open_im(field->display);
	if (field->im) {
		XIMCallback destroyed = {.client_data = (XPointer)field, .callback = neru_x11_text_field_im_destroyed};
		XSetIMValues(field->im, XNDestroyCallback, &destroyed, NULL);
		field->ic = neru_x11_text_field_create_ic(field);
	}
	if (field->ic) {
		long filtered = 0;
		XGetICValues(field->ic, XNFilterEvents, &filtered, NULL);
		XSelectInput(field->display, field->window, attributes.event_mask | filtered);
	}

	XMapRaised(field->display, field->window);
	XFlush(field->display);

	if (!neru_x11_text_field_wait_mapped(field)) {
		neru_x11_text_field_close(field);
		return NULL;
	}

	return field;
}

int neru_x11_text_field_focus(NeruX11TextField *field) {
	int revert = 0;
	XGetInputFocus(field->display, &field->previous_focus, &revert);

	neru_x11_error_trap_begin(field->display);
	XSetInputFocus(field->display, field->window, RevertToParent, CurrentTime);
	int failed = neru_x11_error_trap_end(field->display);

	Window focus = None;
	XGetInputFocus(field->display, &focus, &revert);
	if (failed || focus != field->window) {
		return 0;
	}

	if (field->ic) {
		XSetICFocus(field->ic);
	}
	XFlush(field->display);

	return 1;
}

int neru_x11_text_field_fd(NeruX11TextField *field) { return ConnectionNumber(field->display); }

static void neru_x11_text_field_key(NeruX11TextField *field, XKeyEvent *key) {
	char buffer[64];
	char *text = buffer;
	KeySym keysym = NoSymbol;
	Status status = XLookupNone;
	int length;

	if (field->ic) {
		length = Xutf8LookupString(field->ic, key, text, sizeof(buffer) - 1, &keysym, &status);
		// A long input-method commit does not fit, so ask again with room for it.
		if (status == XBufferOverflow) {
			text = malloc(length + 1);
			if (!text) {
				return;
			}
			length = Xutf8LookupString(field->ic, key, text, length, &keysym, &status);
		}
	} else {
		length = XLookupString(key, text, sizeof(buffer) - 1, &keysym, NULL);
		// Without an input method the bytes are Latin-1, so only ASCII is
		// taken as text.
		status = length > 0 && (unsigned char)text[0] < 0x80 ? XLookupBoth : XLookupKeySym;
	}

	switch (keysym) {
	case XK_Escape:
		field->flags |= NERU_TEXT_FIELD_CANCEL;
		field->finished = 1;
		break;
	case XK_Return:
	case XK_KP_Enter:
		field->flags |= NERU_TEXT_FIELD_CONFIRM;
		field->finished = 1;
		break;
	case XK_BackSpace:
		neru_x11_text_field_delete_last(field);
		break;
	default:
		// A chord with Ctrl, Alt or Super types nothing into the search.
		if (!(key->state & (ControlMask | Mod1Mask | Mod4Mask)) && (status == XLookupChars || status == XLookupBoth) &&
		    length > 0 && (unsigned char)text[0] >= 0x20 && text[0] != 0x7f) {
			text[length] = '\0';
			neru_x11_text_field_append(field, text);
		}
	}

	if (text != buffer) {
		free(text);
	}
}

int neru_x11_text_field_dispatch(NeruX11TextField *field) {
	while (XPending(field->display)) {
		XEvent event;
		XNextEvent(field->display, &event);

		// The input method takes the keys it composes with.
		if (XFilterEvent(&event, None)) {
			continue;
		}
		if (field->finished || event.xany.window != field->window) {
			continue;
		}

		switch (event.type) {
		case KeyPress:
			neru_x11_text_field_key(field, &event.xkey);
			break;
		case FocusOut:
			// The user moved focus to another window. A grab starting or ending
			// moves no focus, so those are not the user leaving.
			if (event.xfocus.mode == NotifyNormal) {
				field->flags |= NERU_TEXT_FIELD_CANCEL;
				field->finished = 1;
				field->previous_focus = None;
			}
			break;
		}
	}

	int flags = field->flags;
	field->flags = 0;
	return flags;
}

const char *neru_x11_text_field_query(NeruX11TextField *field) { return field->committed; }

void neru_x11_text_field_close(NeruX11TextField *field) {
	if (!field) {
		return;
	}
	if (field->display && field->window && field->previous_focus != None) {
		Window focus = None;
		int revert = 0;
		XGetInputFocus(field->display, &focus, &revert);
		if (focus == field->window) {
			neru_x11_error_trap_begin(field->display);
			XSetInputFocus(field->display, field->previous_focus, RevertToParent, CurrentTime);
			neru_x11_error_trap_end(field->display);
		}
	}
	if (field->ic) {
		XDestroyIC(field->ic);
	}
	if (field->im) {
		XCloseIM(field->im);
	}
	if (field->display) {
		if (field->window) {
			XDestroyWindow(field->display, field->window);
		}
		XSync(field->display, False);
		if (field->tolerated) {
			neru_x11_error_untolerate(field->display);
		}
		XCloseDisplay(field->display);
	}
	explicit_bzero(field->committed, sizeof(field->committed));
	free(field);
}
