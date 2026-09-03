//go:build !darwin

package systray

import "github.com/y3owk1n/neru/internal/adapter/systray/icon"

// trayIconFor returns the tray icon bytes and template flag for the given
// enabled state. Windows and Linux tray hosts render icon bytes literally, so
// they get the colored brand tile — the macOS template glyph is
// white-on-transparent and would be invisible there. Paused is the same tile
// desaturated and flattened towards grey, since a host that renders bytes
// literally cannot restyle an icon for us the way a template image lets macOS
// restyle its own.
func trayIconFor(enabled bool) ([]byte, bool) {
	if enabled {
		return icon.Brand, false
	}

	return icon.BrandPaused(), false
}
