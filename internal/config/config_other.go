//go:build !darwin && !linux && !windows

package config

// applyPlatformDefaults is a no-op on unsupported platforms. The shared
// semantic clickable roles still load, but resolve to no native role because
// the platform has no accessibility vocabulary (see element.VocabularyForGOOS).
func applyPlatformDefaults(_ *Config) {}
