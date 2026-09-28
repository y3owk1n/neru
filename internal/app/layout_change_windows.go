//go:build windows

package app

import "github.com/y3owk1n/neru/internal/adapter/platform/windows"

// registerLayoutChangeHandler re-registers global hotkeys when the reference
// keyboard layout changes. RegisterHotKey keeps the virtual key a hotkey
// resolved to when it was registered, and a punctuation key such as ` sits on
// a different virtual key on a US layout than on a UK one, so a switch between
// them would leave the hotkey on the old key.
func (a *App) registerLayoutChangeHandler() {
	windows.SetReferenceLayoutChangeHandler(func() {
		a.logger.Info("Keyboard layout changed; re-registering global hotkeys")

		a.hotkeys.Reregister()
	})
}

// unregisterLayoutChangeHandler stops the watcher so a stale callback cannot
// fire after the App is torn down.
func (a *App) unregisterLayoutChangeHandler() {
	windows.SetReferenceLayoutChangeHandler(nil)
}
