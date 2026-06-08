//go:build linux

package app

// registerLayoutChangeHandler is a no-op on Linux.
// Linux does not use Carbon hotkeys, so no re-registration is needed.
func (a *App) registerLayoutChangeHandler() {}

// unregisterLayoutChangeHandler is a no-op on Linux.
func (a *App) unregisterLayoutChangeHandler() {}
