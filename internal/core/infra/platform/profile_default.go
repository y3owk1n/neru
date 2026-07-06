//go:build !linux

// internal/core/infra/platform/profile_default.go
// Non-Linux stub for linuxProfileForCurrentBackend so ProfileFor(Linux) compiles
// on every target. It is never called at runtime off Linux.
// It does not define Linux backend profiles.

package platform

func linuxProfileForCurrentBackend() Profile {
	return linuxProfile(DisplayServerUnknown)
}
