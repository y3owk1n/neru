//go:build darwin

package config

func applyPlatformDefaults(cfg *Config) {
	// macOS-specific defaults
	cfg.Hints.AdditionalMenubarHintsTargets = append(cfg.Hints.AdditionalMenubarHintsTargets,
		"com.apple.TextInputMenuAgent",
		"com.apple.controlcenter",
		"com.apple.systemuiserver",
	)

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
		"AXMenuButton",
		"AXMenuItem",
		"AXCell",
		"AXRow",
	)
}
