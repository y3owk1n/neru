//go:build !linux

package platform

// Non-Linux stub for linuxProfileForCurrentBackend so ProfileFor(Linux) compiles
// on every target. It is never called at runtime off Linux.
// It does not define Linux backend profiles.
func linuxProfileForCurrentBackend() Profile {
	return linuxProfile(DisplayServerUnknown)
}
