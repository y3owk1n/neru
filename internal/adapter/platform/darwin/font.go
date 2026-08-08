//go:build darwin

package darwin

import (
	"github.com/y3owk1n/neru/internal/adapter/platform/fontcache"
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
	return &nsFontResolver{cache: fontcache.New(darwinFamilies.Resolve)}
}

// nsFontResolver implements ports.FontResolver for macOS. Generic
// aliases are translated to concrete macOS families; everything else
// is passed through trimmed to the C layer, which already performs the
// full NSFont + NSFontManager resolution chain.
type nsFontResolver struct {
	cache *fontcache.Resolver
}

// Resolve implements ports.FontResolver.
func (r *nsFontResolver) Resolve(family string, bold bool) string {
	_ = bold // weight is enforced at the C layer

	return r.cache.Resolve(family)
}
