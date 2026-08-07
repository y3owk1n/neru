package config

// DefaultConfig returns the default application configuration with sensible defaults for the current platform.
func DefaultConfig() *Config {
	cfg := newDefaultConfig()
	applyPlatformDefaults(cfg)
	cfg.ResolveDerived()

	return cfg
}

// DefaultConfigForDecoding returns the defaults used as the TOML decode
// target: platform-adjusted, without theme resolution or launcher hotkeys.
//
// Leaving the derived values alone is what makes it a decode target. Resolved
// grid labels here would read as labels the user configured, so the file's own
// grid.characters would decode over the characters and leave the labels behind.
func DefaultConfigForDecoding() *Config {
	cfg := newDefaultConfig()
	applyPlatformDefaults(cfg)

	return cfg
}

// PlatformDefaultConfig returns the platform-adjusted defaults without
// resolving theme-dependent colors. The loader merges the user's file on top
// of this and resolves themes afterwards.
func PlatformDefaultConfig() *Config {
	cfg := newDefaultConfig()
	applyPlatformDefaults(cfg)

	return cfg
}
