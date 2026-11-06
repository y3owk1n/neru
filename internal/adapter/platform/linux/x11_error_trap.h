#ifndef X11_ERROR_TRAP_H
#define X11_ERROR_TRAP_H

#include <X11/Xlib.h>

// Xlib's default error handler calls exit() on a protocol error, so any request
// that can legitimately fail (pixels off a drawable that shrank, a property off
// a window whose client just died) would take the daemon with it. This trap
// records the error instead.
//
// XSetErrorHandler is process-global, and several Xlib users each hold their
// own Display. One trap serves them all, because two traps under two mutexes
// could interleave their save and restore and leave one caller's requests
// under the default handler, which exits. So trapped sections serialise on
// this one lock, the handler claims only its display's errors, and everything
// else is forwarded to the handler it replaced.
//
// Keep trapped sections short. Everything else that traps waits behind them.

// neru_x11_error_trap_begin installs the trap for display and takes the lock.
// Every call must be paired with neru_x11_error_trap_end on the same display.
void neru_x11_error_trap_begin(Display *display);

// neru_x11_error_trap_end syncs display so any pending error has reached the
// handler, restores the previous handler, releases the lock, and returns 1 when
// a protocol error arrived from display while the trap was installed.
int neru_x11_error_trap_end(Display *display);

// neru_x11_error_tolerate drops every protocol error from display, not just
// those inside a trapped section, until neru_x11_error_untolerate. It is for a
// display whose traffic can fail through no fault of its own, such as the
// hint-search field's input-method exchange. The handler stays installed for
// the process and forwards other displays' errors, so there is still one
// handler chain. Returns 0 when every slot is taken.
int neru_x11_error_tolerate(Display *display);
void neru_x11_error_untolerate(Display *display);

#endif /* X11_ERROR_TRAP_H */
