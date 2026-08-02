package ports

import "sync"

// FontResolver resolves a user-supplied font family to a real, installed font
// family on the host system. Implementations handle generic aliases
// (e.g. "Sans", "Monospace"), verify family availability, and apply
// platform-specific fallback chains. Results are cached internally.
type FontResolver interface {
	// Resolve returns a family name guaranteed to be usable on this platform.
	//
	//   - empty input maps to a sensible default sans-serif family
	//   - generic aliases ("Sans", "Sans Serif", "Serif", "Monospace", ...) are
	//     mapped to a known-good installed family
	//   - other family names are returned unchanged when installed; missing
	//     families fall back to the resolved generic family
	//   - bold is reserved for future weight-specific resolution and is
	//     currently ignored by all implementations
	Resolve(family string, bold bool) string
}

var (
	fontResolverMu sync.RWMutex
	fontResolver   FontResolver = noopFontResolver{}
)

// SetFontResolver installs the process-wide FontResolver. Call once during
// infrastructure initialization. Passing nil restores the no-op default.
func SetFontResolver(resolver FontResolver) {
	fontResolverMu.Lock()
	defer fontResolverMu.Unlock()

	if resolver == nil {
		fontResolver = noopFontResolver{}

		return
	}

	fontResolver = resolver
}

// ResolveFont is a convenience wrapper around the active FontResolver. Returns
// the input unchanged when no resolver has been installed (e.g. in tests).
func ResolveFont(family string, bold bool) string {
	fontResolverMu.RLock()

	r := fontResolver

	fontResolverMu.RUnlock()

	return r.Resolve(family, bold)
}

// noopFontResolver is the default resolver. It returns the input unchanged so
// that tests and unsupported platforms behave like a transparent passthrough.
type noopFontResolver struct{}

// Resolve returns the input family unchanged.
func (noopFontResolver) Resolve(family string, _ bool) string { return family }
