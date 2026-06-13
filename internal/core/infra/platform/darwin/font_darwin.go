//go:build darwin

package darwin

import (
	"strings"
	"sync"

	"github.com/y3owk1n/neru/internal/core/ports"
)

const (
	// defaultDarwinSans is the macOS baseline sans-serif family used for
	// empty input and for generic "Sans" / "Sans Serif" aliases.
	defaultDarwinSans = "Helvetica Neue"
	// defaultDarwinMono is the macOS baseline monospace family.
	defaultDarwinMono = "Menlo"
	// defaultDarwinSerif is the macOS baseline serif family.
	defaultDarwinSerif = "Times New Roman"
)

// NewFontResolver returns a macOS-backed ports.FontResolver. The Go-side
// resolver maps generic aliases to known macOS families. User-supplied
// names are returned unchanged so the existing C/Objective-C layer
// (which already does PostScript and family lookups via NSFontManager)
// can verify and weight-resolve them.
func NewFontResolver() ports.FontResolver {
	return &nsFontResolver{
		cache: make(map[string]string),
	}
}

// nsFontResolver implements ports.FontResolver for macOS. Generic
// aliases are translated to concrete macOS families; everything else
// is passed through to the C layer, which already performs the full
// NSFont + NSFontManager resolution chain.
type nsFontResolver struct {
	mu    sync.RWMutex
	cache map[string]string
}

// Resolve implements ports.FontResolver.
func (r *nsFontResolver) Resolve(family string, bold bool) string {
	_ = bold // weight is enforced at the C layer

	key := strings.ToLower(strings.TrimSpace(family))

	r.mu.RLock()

	if cached, ok := r.cache[key]; ok {
		r.mu.RUnlock()

		return cached
	}

	r.mu.RUnlock()

	resolved := mapDarwinGenericAlias(family)

	r.mu.Lock()
	r.cache[key] = resolved
	r.mu.Unlock()

	return resolved
}

// mapDarwinGenericAlias translates fontconfig-style generic names (and
// empty input) to a concrete macOS family. Case-, whitespace-, and
// separator-insensitive (spaces, hyphens, and underscores all
// normalize to the same key). Non-generic names are returned unchanged
// so the C layer can verify them via NSFontManager.
func mapDarwinGenericAlias(family string) string {
	normalized := strings.NewReplacer(" ", "", "-", "", "_", "").Replace(
		strings.ToLower(strings.TrimSpace(family)),
	)
	switch normalized {
	case "", "sans", "sansserif":
		return defaultDarwinSans
	case "serif":
		return defaultDarwinSerif
	case "mono", "monospace":
		return defaultDarwinMono
	default:
		return family
	}
}
