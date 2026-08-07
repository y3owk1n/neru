//go:build darwin

package darwin

import (
	"strings"
	"sync"

	"github.com/y3owk1n/neru/internal/adapter/platform/fontgeneric"
	"github.com/y3owk1n/neru/internal/ports"
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

// darwinFamilies is what the generic aliases mean on macOS.
var darwinFamilies = fontgeneric.Families{
	Sans:  defaultDarwinSans,
	Serif: defaultDarwinSerif,
	Mono:  defaultDarwinMono,
}

// NewFontResolver returns a macOS-backed ports.FontResolver. The Go-side
// resolver maps generic aliases (platform/fontgeneric) to known macOS
// families. A family somebody named is passed on as written, trimmed, so the
// existing C/Objective-C layer (which already does PostScript and family
// lookups via NSFontManager) can verify and weight-resolve it.
func NewFontResolver() ports.FontResolver {
	return &nsFontResolver{
		cache: make(map[string]string),
	}
}

// nsFontResolver implements ports.FontResolver for macOS. Generic
// aliases are translated to concrete macOS families; everything else
// is passed through trimmed to the C layer, which already performs the
// full NSFont + NSFontManager resolution chain.
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

	resolved := darwinFamilies.Resolve(family)

	r.mu.Lock()
	r.cache[key] = resolved
	r.mu.Unlock()

	return resolved
}
