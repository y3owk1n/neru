#ifndef TEXTFIELD_X11_H
#define TEXTFIELD_X11_H

#include "textfield.h"

// The X11 hint-search field: a one-pixel override-redirect window at the drawn
// search box that holds the input focus while search is open. Keys go through
// an XIM input context, so an input method named by XMODIFIERS (IBus, fcitx)
// and the locale's compose rules for dead keys both reach the query. It runs on
// its own X connection, driven by the caller: poll the descriptor, then call
// neru_x11_text_field_dispatch.
//
// The query is keyboard input. Nothing here logs or stores it beyond the field.

typedef struct NeruX11TextField NeruX11TextField;

// neru_x11_text_field_open places the field at the search box, whose top-left
// corner is x, y and whose height is height, in global coordinates, and gives
// it the input focus. It returns NULL when there is no X display, or when the
// window is not mapped and focused within NERU_X11_TEXT_FIELD_OPEN_TIMEOUT_MS.
NeruX11TextField *neru_x11_text_field_open(int x, int y, int width, int height);

// neru_x11_text_field_fd is readable when the server sent events.
int neru_x11_text_field_fd(NeruX11TextField *field);

// neru_x11_text_field_dispatch handles the pending events and returns
// NERU_TEXT_FIELD_* flags, or -1 when the connection failed.
int neru_x11_text_field_dispatch(NeruX11TextField *field);

// neru_x11_text_field_query is the committed text. It stays valid until the
// next call on the field.
const char *neru_x11_text_field_query(NeruX11TextField *field);

// neru_x11_text_field_close gives the input focus back to the window that had
// it, if the field still holds it, and frees everything.
void neru_x11_text_field_close(NeruX11TextField *field);

#endif /* TEXTFIELD_X11_H */
