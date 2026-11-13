#ifndef FONT_FALLBACK_H
#define FONT_FALLBACK_H

#include <stddef.h>

// neru_font_family_for_text writes to out the family cairo should draw text
// in: family itself when the font fontconfig picks for it has every character
// of text, otherwise the family of the first font that does. cairo's toy text
// API, which both overlays draw with, never falls back to another font, so a
// family without Chinese draws a missing-glyph box for each Chinese character.
// ASCII text and a text no font covers keep family. Safe from any thread.
void neru_font_family_for_text(const char *family, const char *text, char *out, size_t out_len);

#endif /* FONT_FALLBACK_H */
