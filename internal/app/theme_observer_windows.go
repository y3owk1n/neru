//go:build windows

package app

import (
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/infra/platform/windows"
)

const themePollInterval = 2 * time.Second

// setupThemeObserver starts polling the Windows registry for theme changes.
// Windows does not fire a notification when AppsUseLightTheme changes, so we
// poll the registry every themePollInterval and fire handleThemeChange when
// the dark/light state flips. The goroutine exits when the app context is
// canceled, so no separate stop mechanism is needed.
func (a *App) setupThemeObserver() {
	wasDark := windows.AppsUseDarkTheme()

	go func() {
		ticker := time.NewTicker(themePollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-a.ctx.Done():
				return
			case <-ticker.C:
			}

			currentDark := windows.AppsUseDarkTheme()
			if currentDark != wasDark {
				wasDark = currentDark
				a.logger.Debug("Windows theme change detected via registry poll",
					zap.Bool("is_dark", currentDark))
				a.handleThemeChange(currentDark)
			}
		}
	}()
}

// stopThemeObserver is a no-op on Windows. The poll goroutine is stopped by
// app context cancellation (a.cancel is called before stopThemeObserver in
// Cleanup, so <-a.ctx.Done() fires before this runs).
func (a *App) stopThemeObserver() {}
