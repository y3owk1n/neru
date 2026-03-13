//go:build linux

package app

// setupThemeObserver is a no-op on Linux.
func (a *App) setupThemeObserver() {}

// stopThemeObserver is a no-op on Linux.
func (a *App) stopThemeObserver() {}
