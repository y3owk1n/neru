#include "x11_error_trap.h"

#include <pthread.h>
#include <stdatomic.h>

static pthread_mutex_t neru_x11_error_trap_mutex = PTHREAD_MUTEX_INITIALIZER;
static Display *neru_x11_error_trap_display;
static XErrorHandler neru_x11_error_trap_previous;
static int neru_x11_error_trap_seen;

// neru_x11_error_trap_handler records errors from the trapped display and hands
// every other display's back to whoever was handling them, so a trapped section
// neither swallows another subsystem's error nor fails itself over one.
static int neru_x11_error_trap_handler(Display *display, XErrorEvent *event) {
	if (display != neru_x11_error_trap_display) {
		if (neru_x11_error_trap_previous) {
			return neru_x11_error_trap_previous(display, event);
		}

		return 0;
	}

	neru_x11_error_trap_seen = 1;

	return 0;
}

void neru_x11_error_trap_begin(Display *display) {
	pthread_mutex_lock(&neru_x11_error_trap_mutex);

	neru_x11_error_trap_seen = 0;
	neru_x11_error_trap_display = display;
	neru_x11_error_trap_previous = XSetErrorHandler(neru_x11_error_trap_handler);
}

int neru_x11_error_trap_end(Display *display) {
	// Errors arrive asynchronously; sync so the handler has run before it is
	// swapped back out.
	XSync(display, False);
	XSetErrorHandler(neru_x11_error_trap_previous);

	int seen = neru_x11_error_trap_seen;

	neru_x11_error_trap_display = NULL;
	neru_x11_error_trap_previous = NULL;

	pthread_mutex_unlock(&neru_x11_error_trap_mutex);

	return seen;
}

// Displays whose errors are dropped outside trapped sections as well. A slot
// is read from the error handler, which can run on any thread doing Xlib
// work, so the slots are atomic rather than guarded by the trap mutex.
#define NERU_X11_TOLERATED_SLOTS 4

static _Atomic(Display *) neru_x11_tolerated[NERU_X11_TOLERATED_SLOTS];
static XErrorHandler neru_x11_tolerant_previous;
static int neru_x11_tolerant_installed;

static int neru_x11_tolerant_handler(Display *display, XErrorEvent *event) {
	for (int i = 0; i < NERU_X11_TOLERATED_SLOTS; i++) {
		if (atomic_load(&neru_x11_tolerated[i]) == display) {
			return 0;
		}
	}

	return neru_x11_tolerant_previous ? neru_x11_tolerant_previous(display, event) : 0;
}

int neru_x11_error_tolerate(Display *display) {
	pthread_mutex_lock(&neru_x11_error_trap_mutex);

	if (!neru_x11_tolerant_installed) {
		neru_x11_tolerant_previous = XSetErrorHandler(neru_x11_tolerant_handler);
		neru_x11_tolerant_installed = 1;
	}

	int added = 0;
	for (int i = 0; i < NERU_X11_TOLERATED_SLOTS && !added; i++) {
		Display *empty = NULL;
		added = atomic_compare_exchange_strong(&neru_x11_tolerated[i], &empty, display);
	}

	pthread_mutex_unlock(&neru_x11_error_trap_mutex);

	return added;
}

void neru_x11_error_untolerate(Display *display) {
	for (int i = 0; i < NERU_X11_TOLERATED_SLOTS; i++) {
		Display *expected = display;
		atomic_compare_exchange_strong(&neru_x11_tolerated[i], &expected, NULL);
	}
}
