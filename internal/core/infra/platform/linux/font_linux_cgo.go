//go:build linux && cgo

package linux

/*
#include <fontconfig/fontconfig.h>
#include <stdlib.h>
#include <string.h>
#include <strings.h>

// Returns the family name that fontconfig matches for the given input,
// or NULL if it cannot be resolved. The returned string is a heap
// allocation; the caller must free it.
static char *fc_match_family(const char *family) {
	if (!family || !*family) {
		return NULL;
	}
	if (!FcInit()) {
		return NULL;
	}

	FcPattern *pat = FcNameParse((const FcChar8 *)family);
	if (!pat) {
		return NULL;
	}

	FcConfigSubstitute(NULL, pat, FcMatchPattern);
	FcDefaultSubstitute(pat);

	FcResult result = FcResultNoMatch;
	FcPattern *match = FcFontMatch(NULL, pat, &result);
	FcPatternDestroy(pat);

	if (!match || result != FcResultMatch) {
		if (match) {
			FcPatternDestroy(match);
		}

		return NULL;
	}

	FcChar8 *matched_family = NULL;
	char *out = NULL;
	if (FcPatternGetString(match, FC_FAMILY, 0, &matched_family) == FcResultMatch
		&& matched_family) {
		out = strdup((const char *)matched_family);
	}
	FcPatternDestroy(match);

	return out;
}
*/
import "C"

import (
	"strings"
	"sync"
	"unsafe"

	"github.com/y3owk1n/neru/internal/core/ports"
)

// NewFontResolver returns a fontconfig-backed ports.FontResolver.
// Each (family) tuple is resolved on first use and cached for the
// lifetime of the process.
func NewFontResolver() ports.FontResolver {
	return &fontconfigResolver{
		cache: make(map[string]string),
	}
}

// fontconfigResolver implements ports.FontResolver using libfontconfig.
// It maps generic aliases to known-good installed families, asks
// fontconfig to verify user-supplied names, and falls back to a hardcoded
// default when fontconfig cannot resolve a family at all.
type fontconfigResolver struct {
	mu    sync.RWMutex
	cache map[string]string
}

// Resolve implements ports.FontResolver.
func (r *fontconfigResolver) Resolve(family string, bold bool) string {
	_ = bold // reserved for future weight-specific resolution

	key := strings.ToLower(strings.TrimSpace(family))

	r.mu.RLock()
	if cached, ok := r.cache[key]; ok {
		r.mu.RUnlock()

		return cached
	}
	r.mu.RUnlock()

	resolved := r.resolve(family)

	r.mu.Lock()
	r.cache[key] = resolved
	r.mu.Unlock()

	return resolved
}

// resolve performs the actual lookup. It first maps the input to a
// concrete family, then asks fontconfig to match it. If fontconfig
// cannot match the family at all, it falls back to a hardcoded default.
func (r *fontconfigResolver) resolve(family string) string {
	mapped := mapGenericAlias(family)

	cFamily := C.CString(mapped)
	defer C.free(unsafe.Pointer(cFamily)) //nolint:nlreturn

	matched := C.fc_match_family(cFamily)
	if matched != nil {
		defer C.free(unsafe.Pointer(matched)) //nolint:nlreturn

		got := C.GoString(matched)
		if got != "" {
			return got
		}
	}

	return defaultForMapped(mapped)
}
