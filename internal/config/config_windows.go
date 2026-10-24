//go:build windows

package config

// applyPlatformDefaults is a no-op on Windows. Its defaults are entries of the
// per-platform options, such as the windows entry of general.shell.
func applyPlatformDefaults(_ *Config) {}
