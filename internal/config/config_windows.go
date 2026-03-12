//go:build windows

package config

func applyPlatformDefaults(cfg *Config) {
	// Windows-specific defaults
	cfg.General.ExcludedApps = append(cfg.General.ExcludedApps,
		"explorer.exe",
		"Taskmgr.exe",
	)

	// UIA control type roles for clickable elements
	cfg.Hints.ClickableRoles = append(cfg.Hints.ClickableRoles,
		"Button",
		"CheckBox",
		"RadioButton",
		"Hyperlink",
		"ComboBox",
		"Edit",
		"Slider",
		"TabItem",
		"MenuItem",
		"DataItem",
	)
}
