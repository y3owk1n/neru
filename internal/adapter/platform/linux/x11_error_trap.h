#ifndef X11_ERROR_TRAP_H
#define X11_ERROR_TRAP_H

#include <X11/Xlib.h>

// Xlib's default error handler calls exit() on a protocol error, so any request
// that can legitimately fail — reading pixels back from a drawable that shrank,
// reading a property off a window whose client just died — would take the daemon
// with it. This trap swaps in a handler that records the error instead, turning
// "the daemon dies" into "the call failed".
//
// XSetErrorHandler is process-global and the daemon has several Xlib users
// (eventtap, hotkeys, overlay, accessibility, screen capture, the system
// bridge), each on its own Display. One trap serves all of them, for a reason
// worth stating: two traps with two mutexes would each save and restore the
// handler under its own lock, so an interleaving would leave one caller's
// requests running under the default handler — which exits. Trapped sections
// therefore serialise on this one lock, the handler claims only errors from the
// display it was installed for, and everything else is forwarded to the handler
// it replaced.
//
// Keep trapped sections short: everything else that traps waits behind them.

// neru_x11_error_trap_begin installs the trap for display and takes the lock.
// Every call must be paired with neru_x11_error_trap_end on the same display.
void neru_x11_error_trap_begin(Display *display);

// neru_x11_error_trap_end syncs display so any pending error has reached the
// handler, restores the previous handler, releases the lock, and returns 1 when
// a protocol error arrived from display while the trap was installed.
int neru_x11_error_trap_end(Display *display);

// neru_x11_error_tolerate drops every protocol error from display, not just
// those inside a trapped section, until neru_x11_error_untolerate. It is for a
// display whose traffic can fail at any time through no fault of its own, such
// as the hint-search field's input-method exchange, which carries on with
// another client that may exit mid-search. The handler that does it stays
// installed for the process and hands every other display's errors on, so
// there is still one handler chain. Returns 0 when every slot is taken.
int neru_x11_error_tolerate(Display *display);
void neru_x11_error_untolerate(Display *display);

#endif /* X11_ERROR_TRAP_H */
