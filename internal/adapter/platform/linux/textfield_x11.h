#ifndef TEXTFIELD_X11_H
#define TEXTFIELD_X11_H

#include "textfield.h"

// The X11 hint-search field: a one-pixel override-redirect window at the drawn
// search box that holds the input focus while search is open. Keys go through
// an XIM input context, so an input method named by XMODIFIERS (IBus, fcitx)
// and the locale's compose rules for dead keys both reach the query. It runs on
// its own X connection, driven by one goroutine: prepare it, focus it, then
// poll the descriptor and call neru_x11_text_field_dispatch.
//
// Opening the input method and its context waits on the input-method server
// with no timeout, so prepare and close must not run where a stall matters.
// Errors on the field's display are dropped (neru_x11_error_tolerate), since an
// input-method server can exit mid-search. A lost X connection still ends the
// process through Xlib's I/O error handler, as it does for every Xlib user.
//
// The query is keyboard input. Nothing here logs or stores it beyond the field.

typedef struct NeruX11TextField NeruX11TextField;

// neru_x11_text_field_prepare opens the input method, then creates and maps
// the window at the bottom-left corner of the search box, given in global
// coordinates. It returns NULL when there is no X display, no slot is left to
// tolerate its errors, or the window does not map. The window does not take
// the focus yet.
NeruX11TextField *neru_x11_text_field_prepare(int x, int y, int width, int height);

// neru_x11_text_field_focus gives the window the input focus and reports
// whether it took.
int neru_x11_text_field_focus(NeruX11TextField *field);

// neru_x11_text_field_fd is readable when the server sent events.
int neru_x11_text_field_fd(NeruX11TextField *field);

// neru_x11_text_field_dispatch handles the events received so far, including
// any Xlib already read into its queue, and returns NERU_TEXT_FIELD_* flags. It
// does not block.
int neru_x11_text_field_dispatch(NeruX11TextField *field);

// neru_x11_text_field_query is the committed text. It stays valid until the
// next call on the field.
const char *neru_x11_text_field_query(NeruX11TextField *field);

// neru_x11_text_field_close gives the input focus back to the window that had
// it, if the field still holds it, and frees everything.
void neru_x11_text_field_close(NeruX11TextField *field);

#endif /* TEXTFIELD_X11_H */
