#include "font_fallback.h"

#include <fontconfig/fontconfig.h>
#include <pthread.h>
#include <stdio.h>
#include <string.h>

// A draw asks again for the same few texts, such as the word under vim's
// cursor, so the last answers are kept. Sorting every font takes milliseconds.
#define NERU_FONT_CACHE_SIZE 32
#define NERU_FONT_KEY_SIZE 512
#define NERU_FONT_FAMILY_SIZE 256

typedef struct {
	char key[NERU_FONT_KEY_SIZE];
	char family[NERU_FONT_FAMILY_SIZE];
} NeruFontCacheEntry;

static pthread_mutex_t neru_font_cache_mutex = PTHREAD_MUTEX_INITIALIZER;
static NeruFontCacheEntry neru_font_cache[NERU_FONT_CACHE_SIZE];
static int neru_font_cache_next;

static int neru_font_is_ascii(const char *text) {
	for (const unsigned char *at = (const unsigned char *)text; *at; at++) {
		if (*at >= 0x80)
			return 0;
	}

	return 1;
}

// neru_font_covering finds the first font, in fontconfig's order for family,
// whose characters include every one of text's. Returns 1 and writes its
// family to out when there is one.
static int neru_font_covering(const char *family, const char *text, char *out, size_t out_len) {
	FcCharSet *need = FcCharSetCreate();
	if (need == NULL)
		return 0;

	const FcChar8 *at = (const FcChar8 *)text;
	int left = (int)strlen(text);
	while (left > 0) {
		FcChar32 character = 0;
		int used = FcUtf8ToUcs4(at, &character, left);
		if (used <= 0)
			break;

		if (character > ' ')
			FcCharSetAddChar(need, character);

		at += used;
		left -= used;
	}

	FcPattern *pattern = FcPatternCreate();
	int found = 0;

	if (pattern != NULL && FcPatternAddString(pattern, FC_FAMILY, (const FcChar8 *)family) &&
	    FcPatternAddCharSet(pattern, FC_CHARSET, need)) {
		FcConfigSubstitute(NULL, pattern, FcMatchPattern);
		FcDefaultSubstitute(pattern);

		FcResult result;
		FcFontSet *fonts = FcFontSort(NULL, pattern, FcFalse, NULL, &result);

		for (int i = 0; fonts != NULL && i < fonts->nfont && !found; i++) {
			FcCharSet *has = NULL;
			FcChar8 *name = NULL;
			if (FcPatternGetCharSet(fonts->fonts[i], FC_CHARSET, 0, &has) == FcResultMatch &&
			    FcCharSetIsSubset(need, has) &&
			    FcPatternGetString(fonts->fonts[i], FC_FAMILY, 0, &name) == FcResultMatch) {
				snprintf(out, out_len, "%s", (const char *)name);
				found = 1;
			}
		}

		if (fonts != NULL)
			FcFontSetDestroy(fonts);
	}

	if (pattern != NULL)
		FcPatternDestroy(pattern);
	FcCharSetDestroy(need);

	return found;
}

void neru_font_family_for_text(const char *family, const char *text, char *out, size_t out_len) {
	if (family == NULL || family[0] == '\0')
		family = "sans-serif";

	snprintf(out, out_len, "%s", family);

	if (text == NULL || neru_font_is_ascii(text))
		return;

	char key[NERU_FONT_KEY_SIZE];
	if ((size_t)snprintf(key, sizeof(key), "%s\n%s", family, text) >= sizeof(key))
		return;

	pthread_mutex_lock(&neru_font_cache_mutex);

	for (int i = 0; i < NERU_FONT_CACHE_SIZE; i++) {
		if (strcmp(neru_font_cache[i].key, key) == 0) {
			snprintf(out, out_len, "%s", neru_font_cache[i].family);
			pthread_mutex_unlock(&neru_font_cache_mutex);

			return;
		}
	}

	char covering[NERU_FONT_FAMILY_SIZE];
	if (FcInit() && neru_font_covering(family, text, covering, sizeof(covering)))
		snprintf(out, out_len, "%s", covering);

	NeruFontCacheEntry *entry = &neru_font_cache[neru_font_cache_next];
	snprintf(entry->key, sizeof(entry->key), "%s", key);
	snprintf(entry->family, sizeof(entry->family), "%s", out);
	neru_font_cache_next = (neru_font_cache_next + 1) % NERU_FONT_CACHE_SIZE;

	pthread_mutex_unlock(&neru_font_cache_mutex);
}
