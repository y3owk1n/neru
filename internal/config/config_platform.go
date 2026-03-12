package config

// DefaultConfig returns the default application configuration with sensible defaults for the current platform.
func DefaultConfig() *Config {
	cfg := commonDefaultConfig()
	applyPlatformDefaults(cfg)

	return cfg
}
