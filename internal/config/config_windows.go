//go:build windows

package config

func applyPlatformDefaults(cfg *Config) {
	// Windows-specific defaults
	cfg.General.ExcludedApps = append(cfg.General.ExcludedApps,
		"explorer.exe",
		"Taskmgr.exe",
	)
}
