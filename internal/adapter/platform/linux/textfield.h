#ifndef TEXTFIELD_H
#define TEXTFIELD_H

// Flags the hint-search fields return from their dispatch functions, shared by
// the Wayland and X11 fields so one Go loop serves both.
enum {
	NERU_TEXT_FIELD_CHANGED = 1,  // the query changed, read it from the field
	NERU_TEXT_FIELD_CONFIRM = 2,  // Return outside a composition
	NERU_TEXT_FIELD_CANCEL = 4,   // Escape outside a composition, or focus lost
};

#endif /* TEXTFIELD_H */
