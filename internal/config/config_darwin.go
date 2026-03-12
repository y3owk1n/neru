//go:build darwin

package config

func applyPlatformDefaults(cfg *Config) {
	// macOS-specific defaults
	cfg.General.ExcludedApps = append(cfg.General.ExcludedApps,
		"com.apple.loginwindow",
		"com.apple.notificationcenterui",
		"com.apple.controlcenter",
		"com.apple.dock",
		"com.apple.systemuiserver",
		"com.apple.Spotlight",
		"com.apple.finder",
		"com.apple.ScreenSaver.Engine",
	)

	cfg.Hints.AdditionalMenubarHintsTargets = []string{
		"com.apple.TextInputMenuAgent",
		"com.apple.controlcenter",
		"com.apple.systemuiserver",
	}

	cfg.Hints.ClickableRoles = append(cfg.Hints.ClickableRoles,
		"AXButton",
		"AXComboBox",
		"AXCheckBox",
		"AXRadioButton",
		"AXLink",
		"AXPopUpButton",
		"AXTextField",
		"AXSlider",
		"AXTabButton",
		"AXSwitch",
		"AXDisclosureTriangle",
		"AXTextArea",
		"AXMenuItem",
		"AXMenuButton",
		"AXLevelIndicator",
		"AXRadioGroup",
		"AXSegmentedControl",
		"AXSearchField",
		"AXImage",
		"AXStaticText",
	)
}
