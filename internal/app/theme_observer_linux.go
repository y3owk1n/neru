//go:build linux

package app

import (
	"time"

	"github.com/godbus/dbus/v5"
	"go.uber.org/zap"
)

const (
	portalSignalBuffer    = 8
	portalSignalInterface = "org.freedesktop.portal.Settings"
	portalSignalMember    = "SettingChanged"
	portalSettingsNS      = "org.freedesktop.appearance"
	portalSettingsKey     = "color-scheme"
	colorSchemeDark       = 1

	pollFallbackInterval = 5 * time.Second
)

// setupThemeObserver subscribes to xdg-desktop-portal SettingChanged
// D-Bus signals and refreshes theme-aware styles when the color scheme
// changes. Falls back to polling if D-Bus is unavailable.
func (a *App) setupThemeObserver() {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		a.logger.Warn("D-Bus unavailable, falling back to polling for theme changes",
			zap.Error(err))
		go a.pollThemeChanges(a.systemPort != nil && a.systemPort.IsDarkMode())

		return
	}

	if err := conn.AddMatchSignal(
		dbus.WithMatchInterface(portalSignalInterface),
		dbus.WithMatchMember(portalSignalMember),
	); err != nil {
		a.logger.Warn("Failed to subscribe to portal theme signals, falling back to polling",
			zap.Error(err))
		conn.Close()
		go a.pollThemeChanges(a.systemPort != nil && a.systemPort.IsDarkMode())

		return
	}

	ch := make(chan *dbus.Signal, portalSignalBuffer)
	conn.Signal(ch)

	go func() {
		defer conn.Close()

		for signal := range ch {
			if len(signal.Body) < 3 {
				continue
			}

			ns, _ := signal.Body[0].(string)
			key, _ := signal.Body[1].(string)
			if ns != portalSettingsNS || key != portalSettingsKey {
				continue
			}

			variant, ok := signal.Body[2].(dbus.Variant)
			if !ok {
				continue
			}

			colorScheme, ok := variant.Value().(uint32)
			if !ok {
				continue
			}

			a.handleThemeChange(colorScheme == colorSchemeDark)
		}
	}()
}

// stopThemeObserver is a no-op on Linux. The D-Bus connection and
// signal goroutine are abandoned on process exit.
func (a *App) stopThemeObserver() {}

// pollThemeChanges periodically checks IsDarkMode and calls
// handleThemeChange when the value transitions. Acts as a fallback
// when the D-Bus portal signal path is unavailable.
func (a *App) pollThemeChanges(lastIsDark bool) {
	ticker := time.NewTicker(pollFallbackInterval)
	defer ticker.Stop()

	for range ticker.C {
		if a.systemPort == nil {
			continue
		}

		currentIsDark := a.systemPort.IsDarkMode()
		if currentIsDark != lastIsDark {
			a.logger.Info("System theme detected change",
				zap.Bool("is_dark", currentIsDark))
			lastIsDark = currentIsDark
			a.handleThemeChange(currentIsDark)
		}
	}
}
