//go:build darwin

package app

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/infra/platform/darwin"
)

// registerLayoutChangeHandler registers a Go-level callback that fires after
// keyboard layout maps are rebuilt at runtime (e.g. switching US → Dvorak).
//
// Global hotkeys use per-hotkey CGEventTaps that store raw keycodes at
// registration time. When the layout changes, the keycodes for key names
// like "Space" or "H" may change. This handler unregisters all global
// hotkeys and re-registers them with the updated keycodes.
func (a *App) registerLayoutChangeHandler() {
	darwin.SetKeymapLayoutChangeHandler(func() {
		a.logger.Info("Keyboard layout changed; re-registering global hotkeys")

		a.hotkeyRegistrationMu.Lock()
		defer a.hotkeyRegistrationMu.Unlock()

		if !a.appState.HotkeysRegistered() {
			a.logger.Debug("Hotkeys not currently registered; skipping re-registration")

			return
		}

		// Unregister all existing CGEventTap hotkeys
		a.stopAllHotkeyRepeats()
		a.hotkeyManager.UnregisterAll()
		a.appState.SetHotkeysRegistered(false)

		// Re-register with updated keycodes from the new layout, querying
		// the currently focused app so [[app_configs]] overrides match
		// the app the user is actually in.
		bundleID, err := a.actionService.FocusedAppBundleID(a.ctx)
		if err != nil {
			bundleID = ""
		}

		a.currentHotkeyBundleID = bundleID
		a.registerHotkeys(bundleID)
		a.appState.SetHotkeysRegistered(true)

		a.logger.Info("Global hotkeys re-registered for new keyboard layout")
	})

	a.logger.Debug("Registered keyboard layout change handler for hotkey re-registration",
		zap.Bool("hotkeys_registered", a.appState.HotkeysRegistered()))
}

// unregisterLayoutChangeHandler clears the keyboard layout change handler
// so a stale callback cannot fire after the App is torn down.
func (a *App) unregisterLayoutChangeHandler() {
	darwin.SetKeymapLayoutChangeHandler(nil)
}
