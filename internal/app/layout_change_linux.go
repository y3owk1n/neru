//go:build linux

package app

// registerLayoutChangeHandler is a no-op on Linux — global hotkeys on Linux
// use platform-specific backends that re-parse key strings each time, so no
// layout-change re-registration is needed.
func (a *App) registerLayoutChangeHandler() {}

// unregisterLayoutChangeHandler is a no-op on Linux.
func (a *App) unregisterLayoutChangeHandler() {}
