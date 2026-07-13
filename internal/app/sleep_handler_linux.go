//go:build linux

package app

import (
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/infra/platform/linux"
)

const (
	sleepSignalBuffer = 8
	login1Interface   = "org.freedesktop.login1.Manager"
	login1Member      = "PrepareForSleep"
	// hibernateReinitDelay is how long after PrepareForSleep(true) we wait
	// before reinitialising. If PrepareForSleep(false) (normal resume) arrives
	// within this window, the reinit is canceled. This handles the systemd
	// issue (https://github.com/systemd/systemd/issues/30666) where
	// PrepareForSleep(false) is not emitted after hibernation.
	hibernateReinitDelay = 10 * time.Second
	// postReloadCheckDelay is how long after a config reload we wait before
	// verifying the hotkey listener started correctly.
	postReloadCheckDelay = 2 * time.Second
)

// Package-level resources for sleep observer cleanup. Stored here (rather than
// on App) so that non-Linux builds never reference the D-Bus types or fields.
var (
	sleepStopChan  chan struct{}
	sleepDBusClose func() error
	sleepWG        sync.WaitGroup
	// hibernateReinitTimer fires after hibernateReinitDelay unless canceled by
	// a matching PrepareForSleep(false) signal.
	hibernateReinitTimer *time.Timer
	hibernateTimerCh     <-chan time.Time
)

// setupSleepObserver subscribes to logind's PrepareForSleep D-Bus signal. On
// wake (signal with body=false) it reinitializes the evdev-based hotkey
// listener and the libei input session, both of which go stale after system
// suspend.
//
// On PrepareForSleep(true) it arms a deferred reinit timer that fires
// hibernateReinitDelay later unless canceled by a matching
// PrepareForSleep(false). This covers the systemd issue
// (https://github.com/systemd/systemd/issues/30666) where the resume signal
// is not emitted after hibernation.
func (a *App) setupSleepObserver() {
	sleepStopChan = make(chan struct{})

	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		a.logger.Warn(
			"D-Bus system bus unavailable, cannot listen for sleep/wake signals; "+
				"evdev hotkey listeners may fail after system suspend",
			zap.Error(err),
		)

		return
	}

	err = conn.AddMatchSignal(
		dbus.WithMatchInterface(login1Interface),
		dbus.WithMatchMember(login1Member),
	)
	if err != nil {
		a.logger.Warn("Failed to subscribe to logind sleep signals", zap.Error(err))

		_ = conn.Close()

		return
	}

	signalCh := make(chan *dbus.Signal, sleepSignalBuffer)
	conn.Signal(signalCh)
	sleepDBusClose = conn.Close

	sleepWG.Go(func() {
		for {
			select {
			case <-sleepStopChan:
				if hibernateReinitTimer != nil {
					hibernateReinitTimer.Stop()
				}

				return
			case <-hibernateTimerCh:
				hibernateTimerCh = nil
				hibernateReinitTimer = nil

				select {
				case <-sleepStopChan:
					return
				default:
				}

				a.handleWakeFromSleep()
			case signal, chOpen := <-signalCh:
				if !chOpen {
					return
				}

				if len(signal.Body) < 1 {
					continue
				}

				preparing, ok := signal.Body[0].(bool)
				if !ok {
					continue
				}

				if preparing {
					if hibernateReinitTimer != nil {
						hibernateReinitTimer.Stop()
					}

					hibernateReinitTimer = time.NewTimer(hibernateReinitDelay)
					hibernateTimerCh = hibernateReinitTimer.C
				} else {
					if hibernateReinitTimer != nil {
						hibernateReinitTimer.Stop()

						hibernateTimerCh = nil
						hibernateReinitTimer = nil
					}

					a.handleWakeFromSleep()
				}
			}
		}
	})
}

// stopSleepObserver shuts down the D-Bus connection and signal goroutine
// by closing the D-Bus connection and signaling the stop channel.
func (a *App) stopSleepObserver() {
	close(sleepStopChan)

	if sleepDBusClose != nil {
		_ = sleepDBusClose()
	}

	sleepWG.Wait()
}

// reinitializeHotkeys tears down and re-registers the global hotkey listener
// without touching the libei/RemoteDesktop session. Use this for health-check
// recovery after stale evdev fds; it avoids triggering a fresh RemoteDesktop
// portal consent prompt.
func (a *App) reinitializeHotkeys() {
	a.logger.Info("Reinitializing hotkey listener (evdev only)")

	a.ExitMode()

	a.hotkeyRegistrationMu.Lock()

	needReregister := a.appState.HotkeysRegistered()
	if needReregister {
		a.stopAllHotkeyRepeats()
		a.hotkeyManager.UnregisterAll()
		a.appState.SetHotkeysRegistered(false)
	}

	a.hotkeyRegistrationMu.Unlock()

	if needReregister {
		a.refreshHotkeysForAppOrCurrent("")
	}

	a.logger.Info("Hotkey listener reinitialized")
}

// handleWakeFromSleep reinitializes all input subsystems after the system
// resumes from suspend. It re-registers the evdev-based hotkey listener and
// resets the libei/RemoteDesktop portal session, both of which go stale when
// the compositor reinitializes during resume.
//
// Called from the logind PrepareForSleep(false) signal handler and the
// deferred hibernation timer. Do NOT call this from post-reload health
// checks — use reinitializeHotkeys instead to avoid triggering a fresh
// RemoteDesktop consent prompt on every recovery.
func (a *App) handleWakeFromSleep() {
	a.logger.Info("Reinitializing input listeners after sleep/wake")

	a.reinitializeHotkeys()

	// Reset the libei/RemoteDesktop session so the next input operation
	// re-establishes the portal connection. The old socket is stale after the
	// compositor reinitialized during resume.
	linux.LibeiReset()

	a.logger.Info("Input listeners reinitialized after sleep/wake")
}

// schedulePostReloadVerification checks the hotkey listener is alive 2s after
// a config reload and reinitialises if not. Catches the "first reload after
// fresh start" lifecycle bug where Start() returns nil but the listener is
// effectively dead.
func (a *App) schedulePostReloadVerification() {
	sleepWG.Go(func() {
		timer := time.NewTimer(postReloadCheckDelay)
		defer timer.Stop()

		select {
		case <-sleepStopChan:
			return
		case <-timer.C:
			a.verifyHotkeyHealth()
		}
	})
}

// verifyHotkeyHealth tests whether the global hotkey listener is alive when it
// should be running, and re-registers hotkeys if not. Uses evdev-only recovery
// (reinitializeHotkeys) rather than the full handleWakeFromSleep so the
// libei/RemoteDesktop session is not torn down unnecessarily.
func (a *App) verifyHotkeyHealth() {
	if !a.appState.HotkeysRegistered() {
		return
	}

	hc, ok := a.hotkeyManager.(interface{ HealthCheck() bool })
	if !ok {
		return
	}

	if !hc.HealthCheck() {
		a.logger.Warn("Hotkey listener not healthy after config reload; reinitialising")
		a.reinitializeHotkeys()
	}
}
