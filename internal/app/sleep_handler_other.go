//go:build !linux

package app

// setupSleepObserver is a no-op on non-Linux platforms. The evdev-based hotkey
// listener and libei input session used on Wayland are Linux-only, and system
// sleep/wake does not require special handling on Darwin or Windows. No
// teardown or post-reload hooks are registered, so the shared entry points in
// observers.go are no-ops too.
func (a *App) setupSleepObserver() {}
