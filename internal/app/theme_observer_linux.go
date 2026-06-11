//go:build linux

package app

import (
	"sync"
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
	minSignalBodyLength  = 3
)

// Package-level resources for theme observer cleanup. Stored here (rather than
// on App) so that non-Linux builds never reference the D-Bus types or fields.
var (
	themeStopChan  chan struct{}
	themeDBusClose func() error
	themeWG        sync.WaitGroup
)

// setupThemeObserver subscribes to xdg-desktop-portal SettingChanged
// D-Bus signals and refreshes theme-aware styles when the color scheme
// changes. Falls back to polling if D-Bus is unavailable.
func (a *App) setupThemeObserver() {
	themeStopChan = make(chan struct{})

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		a.logger.Warn("D-Bus unavailable, falling back to polling for theme changes",
			zap.Error(err))

		themeWG.Add(1)
		go a.pollThemeChanges(a.systemPort != nil && a.systemPort.IsDarkMode())

		return
	}

	err = conn.AddMatchSignal(
		dbus.WithMatchInterface(portalSignalInterface),
		dbus.WithMatchMember(portalSignalMember),
	)
	if err != nil {
		a.logger.Warn("Failed to subscribe to portal theme signals, falling back to polling",
			zap.Error(err))

		_ = conn.Close()

		themeWG.Add(1)
		go a.pollThemeChanges(a.systemPort != nil && a.systemPort.IsDarkMode())

		return
	}

	signalCh := make(chan *dbus.Signal, portalSignalBuffer)
	conn.Signal(signalCh)
	themeDBusClose = conn.Close

	themeWG.Add(1)
	go func() {
		defer themeWG.Done()

		for {
			select {
			case <-themeStopChan:
				return
			case signal, ok := <-signalCh:
				if !ok {
					return
				}

				if len(signal.Body) < minSignalBodyLength {
					continue
				}

				ns, _ := signal.Body[0].(string)

				key, _ := signal.Body[1].(string)
				if ns != portalSettingsNS || key != portalSettingsKey {
					continue
				}

				variant, parsedOK := signal.Body[2].(dbus.Variant)
				if !parsedOK {
					continue
				}

				colorScheme, csOK := variant.Value().(uint32)
				if !csOK {
					continue
				}

				a.handleThemeChange(colorScheme == colorSchemeDark)
			}
		}
	}()
}

// stopThemeObserver shuts down the D-Bus connection and signal goroutine
// by closing the D-Bus connection and signaling the stop channel, which
// also terminates the polling fallback if it is running.
func (a *App) stopThemeObserver() {
	close(themeStopChan)

	if themeDBusClose != nil {
		_ = themeDBusClose()
	}

	themeWG.Wait()
}

// pollThemeChanges periodically checks IsDarkMode and calls
// handleThemeChange when the value transitions. Acts as a fallback
// when the D-Bus portal signal path is unavailable.
func (a *App) pollThemeChanges(lastIsDark bool) {
	defer themeWG.Done()

	ticker := time.NewTicker(pollFallbackInterval)
	defer ticker.Stop()

	for {
		select {
		case <-themeStopChan:
			return
		case <-ticker.C:
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
}
