//go:build darwin

package systray

import "github.com/y3owk1n/neru/internal/adapter/systray/icon"

// trayIconFor returns the tray icon bytes and template flag for the given
// enabled state. macOS uses the monochrome template glyphs so the menu bar
// themes them; the platform policy lives here so the tray backends can honor
// whatever bytes they are handed.
func trayIconFor(enabled bool) ([]byte, bool) {
	if enabled {
		return icon.Template, true
	}

	return icon.TemplateDisabled, true
}
