//go:build darwin

package config

func applyPlatformDefaults(cfg *Config) {
	// macOS-specific defaults
	cfg.Hints.AdditionalMenubarHintsTargets = append(cfg.Hints.AdditionalMenubarHintsTargets,
		"com.apple.TextInputMenuAgent",
		"com.apple.controlcenter",
		"com.apple.systemuiserver",
		"com.y3owk1n.neru",
	)
}
