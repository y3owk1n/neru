//go:build darwin

package app

import (
	"github.com/y3owk1n/neru/internal/core/infra/platform/darwin"
)

// setupThemeObserver starts the macOS theme change observer and registers
// a callback that refreshes theme-aware styles (e.g. label_color) when the
// system appearance changes between Light and Dark Mode.
func (a *App) setupThemeObserver() {
	darwin.SetThemeChangeHandler(func(isDark bool) {
		a.handleThemeChange(isDark)
	})
	darwin.StartThemeObserver()
}

// stopThemeObserver stops the macOS theme observer.
func (a *App) stopThemeObserver() {
	darwin.SetThemeChangeHandler(nil)
	darwin.StopThemeObserver()
}
