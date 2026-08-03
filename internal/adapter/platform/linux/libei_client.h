#ifndef LIBEI_CLIENT_H
#define LIBEI_CLIENT_H

// libei input-injection client for Wayland compositors that do not implement
// zwlr_virtual_pointer_v1 (notably KWin/KDE Plasma). Input is delivered through
// the org.freedesktop.portal.RemoteDesktop portal: liboeffis runs the portal
// session and hands back an EIS socket, then libei emits pointer/button/scroll/
// keyboard events on it. Screen enumeration and overlays still go through the
// wlroots client; this wrapper only covers input.

typedef struct NeruEiClient NeruEiClient;

// Establish a RemoteDesktop portal session and a libei sender context, blocking
// until the absolute-pointer device is ready or timeout_ms elapses. The portal
// shows a one-time consent dialog the user must approve. Returns NULL on
// denial, timeout, or any setup failure.
NeruEiClient *neru_ei_connect(int timeout_ms);

// Tear down the libei context and portal session.
void neru_ei_disconnect(NeruEiClient *c);

// Move the absolute pointer to global compositor coordinates (logical pixels).
// Returns 1 on success, 0 otherwise.
int neru_ei_move_abs(NeruEiClient *c, int x, int y);

// Press (pressed != 0) or release a pointer button. The button code is an
// evdev code (e.g. 0x110 for BTN_LEFT). Returns 1 on success, 0 otherwise.
int neru_ei_button(NeruEiClient *c, int button, int pressed);

// Emit a scroll event. axis: 0 = vertical, 1 = horizontal. delta is the scroll
// distance in logical pixels (positive = down/right). Returns 1 on success.
int neru_ei_scroll(NeruEiClient *c, int axis, int delta);

// Press or release a keyboard key (evdev keycode). Returns 1 on success, 0 when
// no keyboard device is available on the granted session.
int neru_ei_key(NeruEiClient *c, int keycode, int pressed);

// Whether the granted session exposes a keyboard device.
int neru_ei_has_keyboard(NeruEiClient *c);

#endif /* LIBEI_CLIENT_H */
