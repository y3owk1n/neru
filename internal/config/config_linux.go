//go:build linux

package config

func applyPlatformDefaults(cfg *Config) {
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
