//go:build windows

package app

// registerLayoutChangeHandler is a no-op on Windows — global hotkeys on
// Windows use RegisterHotKey which re-parses key strings at registration
// time, so no layout-change re-registration is needed.
func (a *App) registerLayoutChangeHandler() {}

// unregisterLayoutChangeHandler is a no-op on Windows.
func (a *App) unregisterLayoutChangeHandler() {}
