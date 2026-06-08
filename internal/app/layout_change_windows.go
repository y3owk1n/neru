//go:build windows

package app

// registerLayoutChangeHandler is a no-op on Windows.
// Windows does not use Carbon hotkeys, so no re-registration is needed.
func (a *App) registerLayoutChangeHandler() {}
