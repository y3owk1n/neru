//go:build !linux

package app

// setupSleepObserver is a no-op on non-Linux platforms. The evdev-based hotkey
// listener and libei input session used on Wayland are Linux-only, and system
// sleep/wake does not require special handling on Darwin or Windows.
func (a *App) setupSleepObserver() {}

// stopSleepObserver is a no-op on non-Linux platforms.
func (a *App) stopSleepObserver() {}

// schedulePostReloadVerification is a no-op on non-Linux platforms. The evdev
// hotkey listener health check is Linux-only.
func (a *App) schedulePostReloadVerification() {}
