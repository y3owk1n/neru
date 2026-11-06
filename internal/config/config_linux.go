//go:build linux

package config

func applyPlatformDefaults(cfg *Config) {
	// No default global hotkeys, to avoid colliding with terminal shortcuts
	// (e.g. Ctrl+Shift+C). Users enable them in config or bind `neru <mode>` in
	// their desktop environment or window manager.
	cfg.Hotkeys.Bindings = make(map[string][]string)
}
