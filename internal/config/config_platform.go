package config

// DefaultConfig returns the default application configuration with sensible defaults for the current platform.
func DefaultConfig() *Config {
	cfg := newDefaultConfig()
	applyPlatformDefaults(cfg)
	cfg.ResolveThemeDefaults()

	return cfg
}

func defaultConfigForDecoding() *Config {
	cfg := newDefaultConfig()
	applyPlatformDefaults(cfg)

	return cfg
}
