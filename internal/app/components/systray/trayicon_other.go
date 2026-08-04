//go:build !darwin

package systray

import "github.com/y3owk1n/neru/internal/adapter/systray/icon"

// trayIconFor returns the tray icon bytes and template flag for the given
// enabled state. Windows and Linux tray hosts render icon bytes literally, so
// they get the colored brand tile — the macOS template glyph is
// white-on-transparent and would be invisible there. Neither platform has a
// distinct paused variant yet, so both states share the brand tile.
func trayIconFor(enabled bool) ([]byte, bool) {
	return icon.Brand, false
}
