#include "x11_error_trap.h"

#include <pthread.h>

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
