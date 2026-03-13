//go:build windows

package app

// setupThemeObserver is a no-op on Windows.
func (a *App) setupThemeObserver() {}

// stopThemeObserver is a no-op on Windows.
func (a *App) stopThemeObserver() {}
