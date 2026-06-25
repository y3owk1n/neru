//go:build windows

// internal/core/infra/platform/windows/theme.go
// Reads Windows app theme state for overlay color compositing.
// Does not implement ports.SystemPort; system.go delegates dark mode here.

package windows

import (
	"golang.org/x/sys/windows/registry"
)

const (
	// Match config/theme_palette.go defaultThemeLightSurface / defaultThemeDarkSurface.
	themeSurfaceLight uint32 = 0xEEF2FF
	themeSurfaceDark  uint32 = 0x0A1338
)

// AppsUseDarkTheme reports whether Windows app dark mode is active.
func AppsUseDarkTheme() bool {
	key, err := registry.OpenKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return false
	}
	defer func() { _ = key.Close() }()

	value, _, err := key.GetIntegerValue("AppsUseLightTheme")
	if err != nil {
		return false
	}

	return value == 0
}

// ThemeSurfaceRGB returns the opaque RGB surface used to approximate semi-transparent
// theme colors on the color-key GDI overlay (mirrors cairo compositing on Linux/macOS).
func ThemeSurfaceRGB() uint32 {
	if AppsUseDarkTheme() {
		return themeSurfaceDark
	}

	return themeSurfaceLight
}
