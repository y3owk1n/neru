//go:build linux

package app

import (
	"context"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform/linux"
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

	// themeBusDialTimeout caps the session-bus dial this makes. The dial itself
	// cannot be canceled, so without a cap a bus that accepts a connection and
	// then stops answering holds daemon startup — this runs on the goroutine
	// that starts it — rather than merely costing theme changes. Giving up is
	// cheap: polling picks the theme up anyway, a few seconds later.
	themeBusDialTimeout = 2 * time.Second
)

// setupThemeObserver subscribes to xdg-desktop-portal SettingChanged
// D-Bus signals and refreshes theme-aware styles when the color scheme
// changes. Falls back to polling if D-Bus is unavailable.
//
// A session bus that never answers counts as unavailable rather than as a
// reason to wait: the dial is bounded (linux.ConnectSessionBus), and a dial
// that outlasts its budget takes the same polling fallback as a dial that
// failed outright — the daemon is still starting up on this goroutine, and
// polling notices a theme change a few seconds later either way.
//
// All teardown state is captured per App instance in the stop closure —
// package-level state here would be shared across App instances, and a
// process that creates more than one (tests do) would close another
// instance's channel and leak its goroutine.
func (a *App) setupThemeObserver() {
	stopChan := make(chan struct{})
	waitGroup := &sync.WaitGroup{}

	// Assigned once the D-Bus signal path is established; the stop closure
	// reads it at call time, so the early polling fallbacks leave it nil.
	var dbusClose func() error

	a.themeObserverStop = func() {
		close(stopChan)

		if dbusClose != nil {
			_ = dbusClose()
		}

		waitGroup.Wait()
	}

	dialCtx, cancelDial := context.WithTimeout(a.ctx, themeBusDialTimeout)
	defer cancelDial()

	conn, err := linux.ConnectSessionBus(dialCtx)
	if err != nil {
		a.logger.Warn("D-Bus unavailable, falling back to polling for theme changes",
			zap.Error(err))

		isDark := a.systemPort != nil && a.systemPort.IsDarkMode()

		waitGroup.Go(func() { a.pollThemeChanges(stopChan, isDark) })

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

		isDark := a.systemPort != nil && a.systemPort.IsDarkMode()

		waitGroup.Go(func() { a.pollThemeChanges(stopChan, isDark) })

		return
	}

	signalCh := make(chan *dbus.Signal, portalSignalBuffer)
	conn.Signal(signalCh)
	dbusClose = conn.Close

	waitGroup.Go(func() {
		for {
			select {
			case <-stopChan:
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

				a.HandleThemeChange(colorScheme == colorSchemeDark)
			}
		}
	})
}

// pollThemeChanges periodically checks IsDarkMode and calls
// HandleThemeChange when the value transitions. Acts as a fallback
// when the D-Bus portal signal path is unavailable.
func (a *App) pollThemeChanges(stopChan <-chan struct{}, lastIsDark bool) {
	ticker := time.NewTicker(pollFallbackInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stopChan:
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
				a.HandleThemeChange(currentIsDark)
			}
		}
	}
}
