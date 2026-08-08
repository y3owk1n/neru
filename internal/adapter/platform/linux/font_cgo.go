//go:build linux && cgo

package linux

/*
#include <fontconfig/fontconfig.h>
#include <stdlib.h>
#include <string.h>

// Reports whether fontconfig knows a font whose family is the given name,
// compared the way fontconfig compares family names (ignoring case and
// blanks). Returns 1 when the family is installed, 0 when it is not, and -1
// when fontconfig cannot be consulted at all.
//
// This lists fonts rather than matching one: FcFontMatch always answers with
// something, so it can say what would render but never whether the family
// asked for is the family that would.
static int fc_family_installed(const char *family) {
	if (!family || !*family) {
		return 0;
	}
	if (!FcInit()) {
		return -1;
	}

	FcPattern *pat = FcPatternCreate();
	if (!pat) {
		return -1;
	}

	if (!FcPatternAddString(pat, FC_FAMILY, (const FcChar8 *)family)) {
		FcPatternDestroy(pat);

		return -1;
	}

	FcObjectSet *props = FcObjectSetBuild(FC_FAMILY, (char *)NULL);
	if (!props) {
		FcPatternDestroy(pat);

		return -1;
	}

	FcFontSet *fonts = FcFontList(NULL, pat, props);
	FcObjectSetDestroy(props);
	FcPatternDestroy(pat);

	if (!fonts) {
		return -1;
	}

	int installed = fonts->nfont > 0 ? 1 : 0;
	FcFontSetDestroy(fonts);

	return installed;
}

// Returns the family name fontconfig would render for the given input, or NULL
// if it cannot be resolved. The returned string is a heap allocation; the
// caller must free it.
static char *fc_substitute_family(const char *family) {
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
	"unsafe"

	"github.com/y3owk1n/neru/internal/adapter/platform/fontcache"
	"github.com/y3owk1n/neru/internal/ports"
)

// NewFontResolver returns a fontconfig-backed ports.FontResolver.
// Each family is resolved on first use and cached for the lifetime of the
// process.
func NewFontResolver() ports.FontResolver {
	resolver := &fontconfigResolver{}
	resolver.cache = fontcache.New(resolver.resolve)

	return resolver
}

// fontconfigResolver implements ports.FontResolver using libfontconfig.
// It maps generic aliases to known-good installed families and asks
// fontconfig whether a user-supplied family is installed, falling back to
// this platform's generic family when it is not.
type fontconfigResolver struct {
	cache *fontcache.Resolver
}

// Resolve implements ports.FontResolver.
func (r *fontconfigResolver) Resolve(family string) string {
	return r.cache.Resolve(family)
}

// resolve performs the actual lookup. It maps the input to a concrete family
// and asks fontconfig whether that family is installed: if it is, that is the
// answer — the name as the caller wrote it, which is what ports.FontResolver
// promises and what macOS and Windows return. If it is not, the answer is this
// platform's generic family for it, handed to fontconfig so that a machine
// without the DejaVu baseline still gets a family it has.
//
// The question deliberately is not "what would fontconfig render for this
// name": fontconfig always matches something, so asking that way answers a
// missing "Arial" with whatever it substitutes and leaves the caller unable to
// tell a family it has from one it does not. Substitution still happens where
// it belongs — Pango and Cairo do it when the text is drawn.
func (r *fontconfigResolver) resolve(family string) string {
	mapped := linuxFamilies.Resolve(family)

	presence := lookUpFamily(mapped)

	if presence == familyMissing {
		return substituteFamily(defaultForMapped(mapped))
	}

	// familyPresent, or familyUnknown: nothing can be verified on a machine
	// whose fontconfig cannot be consulted at all, so that case behaves like
	// the non-CGO build — map the generics, pass the rest through as written.
	return mapped
}

// familyPresence is what fontconfig can say about a family. An unusable
// fontconfig is not the same answer as a family that is missing, so the two
// are not one boolean.
type familyPresence int

const (
	// familyUnknown means fontconfig could not be consulted at all.
	familyUnknown familyPresence = iota
	// familyMissing means fontconfig has no font of that family.
	familyMissing
	// familyPresent means fontconfig has the family installed.
	familyPresent
)

// lookUpFamily asks fontconfig whether it has the given family.
func lookUpFamily(family string) familyPresence {
	cFamily := C.CString(family)
	defer C.free(unsafe.Pointer(cFamily))

	switch C.fc_family_installed(cFamily) {
	case 1:
		return familyPresent
	case 0:
		return familyMissing
	default:
		return familyUnknown
	}
}

// substituteFamily returns the family fontconfig would render for the given
// one, and the family itself when fontconfig has nothing to say. It is asked
// only about the hardcoded baselines, so what it substitutes for a machine
// without DejaVu is that machine's own generic.
func substituteFamily(family string) string {
	cFamily := C.CString(family)
	defer C.free(unsafe.Pointer(cFamily))

	matched := C.fc_substitute_family(cFamily)
	if matched == nil {
		return family
	}

	defer C.free(unsafe.Pointer(matched))

	if got := C.GoString(matched); got != "" {
		return got
	}

	return family
}
