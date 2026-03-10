package systray

import _ "embed"

// trayIcon is the PNG icon displayed in the macOS menu bar when Neru is running.
// Should be 44×44px (22pt @2x retina) monochrome PNG with transparent background.
// It is used as a template icon so macOS automatically adapts it
// to the current menu bar appearance (light/dark).
//
//go:embed resources/tray-icon.png
var trayIcon []byte

// trayIconDisabled is the PNG icon displayed in the macOS menu bar when Neru is paused.
// Same size and format requirements as trayIcon.
//
//go:embed resources/tray-icon-disabled.png
var trayIconDisabled []byte
