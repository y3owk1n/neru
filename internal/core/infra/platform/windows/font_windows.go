//go:build windows

package windows

import (
	"strings"
	"sync"

	"github.com/y3owk1n/neru/internal/core/ports"
)

const (
	defaultWindowsSans  = "Segoe UI"
	defaultWindowsMono  = "Consolas"
	defaultWindowsSerif = "Cambria"
)

// NewFontResolver returns a Windows-backed ports.FontResolver.
func NewFontResolver() ports.FontResolver {
	return &winFontResolver{
		cache: make(map[string]string),
	}
}

type winFontResolver struct {
	mu    sync.RWMutex
	cache map[string]string
}

// Resolve implements ports.FontResolver.
func (r *winFontResolver) Resolve(family string, bold bool) string {
	_ = bold

	key := strings.ToLower(strings.TrimSpace(family))

	r.mu.RLock()
	if cached, ok := r.cache[key]; ok {
		r.mu.RUnlock()
		return cached
	}
	r.mu.RUnlock()

	resolved := mapWindowsGenericAlias(family)

	r.mu.Lock()
	r.cache[key] = resolved
	r.mu.Unlock()

	return resolved
}

// mapWindowsGenericAlias translates generic font names and empty input to
// concrete Windows font families. Non-generic names pass through unchanged.
func mapWindowsGenericAlias(family string) string {
	normalized := strings.NewReplacer(" ", "", "-", "", "_", "").Replace(
		strings.ToLower(strings.TrimSpace(family)),
	)
	switch normalized {
	case "", "sans", "sansserif", "sans-serif":
		return defaultWindowsSans
	case "serif":
		return defaultWindowsSerif
	case "mono", "monospace":
		return defaultWindowsMono
	default:
		return family
	}
}
