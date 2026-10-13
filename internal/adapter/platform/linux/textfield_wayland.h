#ifndef TEXTFIELD_WAYLAND_H
#define TEXTFIELD_WAYLAND_H

// The Wayland hint-search field: a transparent layer-shell surface over the
// drawn search box that takes the keyboard while search is open. Input-method
// text arrives through text-input-v3, and plain keys go through xkb with
// compose, so dead keys work. It runs on its own Wayland connection, driven by
// the caller: poll the two descriptors, then call the matching function.
//
// The query is screen input. Nothing here logs or stores it beyond the field.

typedef struct NeruTextField NeruTextField;

// Flags the dispatch functions return.
enum {
	NERU_TEXT_FIELD_CHANGED = 1,  // the query changed, read it with neru_text_field_query
	NERU_TEXT_FIELD_CONFIRM = 2,  // Return outside a composition
	NERU_TEXT_FIELD_CANCEL = 4,   // Escape outside a composition
};

// neru_text_field_open places a width x height field at x, y in global
// coordinates. It returns NULL when there is no Wayland display, the
// compositor offers no layer shell or keyboard, or it does not answer within
// NERU_TEXT_FIELD_OPEN_TIMEOUT_MS.
NeruTextField *neru_text_field_open(int x, int y, int width, int height);

// neru_text_field_display_fd is readable when the compositor sent events.
int neru_text_field_display_fd(NeruTextField *field);

// neru_text_field_repeat_fd is readable when a held key should repeat.
int neru_text_field_repeat_fd(NeruTextField *field);

// neru_text_field_dispatch reads and handles the compositor's events. It
// returns NERU_TEXT_FIELD_* flags, or -1 when the connection failed.
int neru_text_field_dispatch(NeruTextField *field);

// neru_text_field_repeat handles a repeat tick and returns the flags.
int neru_text_field_repeat(NeruTextField *field);

// neru_text_field_query is the committed text followed by any composition in
// progress. It stays valid until the next call on the field.
const char *neru_text_field_query(NeruTextField *field);

void neru_text_field_close(NeruTextField *field);

#endif /* TEXTFIELD_WAYLAND_H */
