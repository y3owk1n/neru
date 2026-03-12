//go:build linux

package config

func applyPlatformDefaults(cfg *Config) {
	// Linux-specific defaults
	cfg.General.ExcludedApps = append(cfg.General.ExcludedApps,
		"org.gnome.Shell",
		"org.kde.kwin",
		"org.kde.plasmashell",
	)

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
