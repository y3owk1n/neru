//go:build !darwin && !linux && !windows

package config

// applyPlatformDefaults is a no-op on unsupported platforms.
// ClickableRoles and ExcludedApps will be empty; users must configure them manually.
func applyPlatformDefaults(_ *Config) {}
