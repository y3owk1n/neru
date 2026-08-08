//go:build windows

package windows

import (
	"github.com/y3owk1n/neru/internal/adapter/platform/fontcache"
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
	return &winFontResolver{cache: fontcache.New(windowsFamilies.Resolve)}
}

type winFontResolver struct {
	cache *fontcache.Resolver
}

// Resolve implements ports.FontResolver.
func (r *winFontResolver) Resolve(family string, bold bool) string {
	_ = bold

	return r.cache.Resolve(family)
}
