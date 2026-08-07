//go:build windows

package windows

import (
	"strings"
	"sync"

	"github.com/y3owk1n/neru/internal/adapter/platform/fontgeneric"
	"github.com/y3owk1n/neru/internal/ports"
)

const (
	defaultWindowsSans  = "Segoe UI"
	defaultWindowsMono  = "Consolas"
	defaultWindowsSerif = "Cambria"
)

// windowsFamilies is what the generic aliases mean on Windows.
var windowsFamilies = fontgeneric.Families{
	Sans:  defaultWindowsSans,
	Serif: defaultWindowsSerif,
	Mono:  defaultWindowsMono,
}

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

	resolved := windowsFamilies.Resolve(family)

	r.mu.Lock()
	r.cache[key] = resolved
	r.mu.Unlock()

	return resolved
}
