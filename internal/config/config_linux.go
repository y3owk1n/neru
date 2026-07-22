//go:build linux

package config

func applyPlatformDefaults(cfg *Config) {
	// Clear default global hotkeys to avoid collisions with terminal and
	// application shortcuts (e.g. Ctrl+Shift+C / copy, Ctrl+Shift+V / paste).
	// Users can enable global hotkeys in their config, or bind `neru <mode>`
	// in their desktop environment / window manager.
	cfg.Hotkeys.Bindings = make(map[string][]string)

	// Linux-specific defaults
	cfg.Hints.ClickableRoles = append(cfg.Hints.ClickableRoles,
		"push button",
		"check box",
		"radio button",
		"link",
		"combo box",
		"text",
		"slider",
		"tab",
		"menu item",
		"image",
		"static",
	)
}
