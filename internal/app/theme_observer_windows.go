//go:build windows

package app

// setupThemeObserver is a no-op on Windows.
func (a *App) setupThemeObserver() {}

// stopThemeObserver is a no-op on Windows.
func (a *App) stopThemeObserver() {}

// Unused import and method markers for cross-platform code.
// handleThemeChange is defined in theme.go and called from darwin/linux
// observer files. This reference silences the unused linter on Windows.
var _ = (*App).handleThemeChange
