//go:build darwin

package app

import (
	"github.com/y3owk1n/neru/internal/adapter/platform/darwin"
)

// setupThemeObserver starts the macOS theme change observer and registers
// a callback that refreshes theme-aware styles (e.g. label_color) when the
// system appearance changes between Light and Dark Mode.
//
// Teardown nils the handler first so an in-flight KVO callback (between the
// async dispatch and the actual observer removal) is a no-op.
func (a *App) setupThemeObserver() {
	darwin.SetThemeChangeHandler(func(isDark bool) {
		a.HandleThemeChange(isDark)
	})
	darwin.StartThemeObserver()

	a.observerMu.Lock()
	defer a.observerMu.Unlock()

	a.themeObserverStop = func() {
		darwin.SetThemeChangeHandler(nil)
		darwin.StopThemeObserver()
	}
}
