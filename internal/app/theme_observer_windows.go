//go:build windows

package app

import (
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform/windows"
)

const themePollInterval = 2 * time.Second

// setupThemeObserver starts polling the Windows registry for theme changes.
// Windows does not fire a notification when AppsUseLightTheme changes, so we
// poll the registry every themePollInterval and fire HandleThemeChange when
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
				a.HandleThemeChange(currentDark)
			}
		}
	}()
}

// No teardown closure is registered on Windows: the poll goroutine is stopped
// by app context cancellation (a.cancel runs before stopThemeObserver in
// Cleanup, so <-a.ctx.Done() fires first).
