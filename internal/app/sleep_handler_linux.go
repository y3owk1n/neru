//go:build linux

package app

import (
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/ports"
)

const (
	sleepSignalBuffer = 8
	login1Interface   = "org.freedesktop.login1.Manager"
	login1Member      = "PrepareForSleep"
	// hibernateReinitDelay is how long after PrepareForSleep(true) a reinit
	// waits, canceled by a PrepareForSleep(false) in time (see
	// setupSleepObserver).
	hibernateReinitDelay = 10 * time.Second
	// postReloadCheckDelay is how long after a config reload we wait before
	// verifying the hotkey listener started correctly.
	postReloadCheckDelay = 2 * time.Second
	// hotkeyReinitRetries is the number of retries during normal reinit.
	hotkeyReinitRetries = 5
	// hotkeyReinitDelay is the delay between retry attempts during normal reinit.
	hotkeyReinitDelay = 500 * time.Millisecond
	// hotkeySleepRetries is the number of hotkey reinitialization retries after
	// sleep/wake, allowing extra time for evdev devices to settle.
	hotkeySleepRetries = 10
)

// setupSleepObserver subscribes to logind's PrepareForSleep D-Bus signal. On
// wake (body=false) it reinitializes the evdev hotkey listener and the libei
// input session, both of which go stale across suspend. PrepareForSleep(true)
// arms a reinit after hibernateReinitDelay unless a PrepareForSleep(false)
// cancels it, because systemd does not emit the resume signal after
// hibernation (https://github.com/systemd/systemd/issues/30666).
//
// Teardown state lives in closures on the App, never in package state, which
// a second App instance (tests) would share. The teardown and post-reload
// hooks register before the D-Bus attempt so the post-reload health check
// works without a bus.
func (a *App) setupSleepObserver() {
	stopChan := make(chan struct{})
	waitGroup := &sync.WaitGroup{}

	// Assigned once the D-Bus signal path is established. The stop closure
	// reads it at call time, so the degraded no-bus path leaves it nil.
	var dbusClose func() error

	// Under observerMu, because the live IPC server's reload can read
	// postReloadVerify concurrently.
	a.observerMu.Lock()

	a.sleepObserverStop = func() {
		close(stopChan)

		if dbusClose != nil {
			_ = dbusClose()
		}

		waitGroup.Wait()
	}

	a.postReloadVerify = func() {
		waitGroup.Go(func() {
			timer := time.NewTimer(postReloadCheckDelay)
			defer timer.Stop()

			select {
			case <-stopChan:
				return
			case <-timer.C:
				a.verifyHotkeyHealth()
			}
		})
	}

	a.observerMu.Unlock()

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
	dbusClose = conn.Close

	waitGroup.Go(func() {
		// hibernateReinitTimer is touched only by this goroutine.
		var (
			hibernateReinitTimer *time.Timer
			hibernateTimerCh     <-chan time.Time
		)

		for {
			select {
			case <-stopChan:
				if hibernateReinitTimer != nil {
					hibernateReinitTimer.Stop()
				}

				return
			case <-hibernateTimerCh:
				hibernateTimerCh = nil
				hibernateReinitTimer = nil

				select {
				case <-stopChan:
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

// reinitializeHotkeys re-registers the global hotkey listener without touching
// the libei/RemoteDesktop session, so health-check recovery from stale evdev
// fds does not trigger a fresh portal consent prompt.
func (a *App) reinitializeHotkeys() {
	a.reinitializeHotkeysWithParams(hotkeyReinitRetries, hotkeyReinitDelay)
}

// reinitializeHotkeysAfterSleep uses a longer retry window (10 retries x 1s),
// since evdev devices can take seconds to settle after resume.
func (a *App) reinitializeHotkeysAfterSleep() {
	a.reinitializeHotkeysWithParams(hotkeySleepRetries, 1*time.Second)
}

func (a *App) reinitializeHotkeysWithParams(maxRetries int, retryDelay time.Duration) {
	a.ExitMode()

	needReregister := a.appState.HotkeysRegistered()
	if needReregister {
		a.hotkeys.Unregister()
	}

	if needReregister {
		healthy := false

		for attempt := 1; attempt <= maxRetries; attempt++ {
			a.hotkeys.RefreshFor("")

			hc, ok := a.hotkeyManager.(ports.HotkeyHealthReporter)
			if !ok || hc.HealthCheck() {
				healthy = true

				break
			}

			if attempt < maxRetries {
				a.logger.Debug(
					"Hotkey listener not healthy after reinitialization attempt; retrying",
					zap.Int("attempt", attempt),
					zap.Int("max_retries", maxRetries),
				)
				a.hotkeys.Unregister()
				time.Sleep(retryDelay)
			}
		}

		if !healthy {
			a.logger.Warn(
				"Hotkey listener failed to recover after max reinitialization attempts",
				zap.Int("max_retries", maxRetries),
			)

			return
		}
	}

	a.logger.Debug("Hotkey listener reinitialized")
}

// handleWakeFromSleep reinitializes all input subsystems after resume, both the
// evdev hotkey listener and the libei/RemoteDesktop session. It is called from
// the PrepareForSleep(false) handler and the hibernation timer. Never call it
// from post-reload health checks, because resetting the session prompts for
// RemoteDesktop consent again. Use reinitializeHotkeys there.
func (a *App) handleWakeFromSleep() {
	a.reinitializeHotkeysAfterSleep()

	a.logger.Info("Input listeners reinitialized after sleep/wake")
}

// verifyHotkeyHealth re-registers hotkeys if the listener should be running and
// is not, using reinitializeHotkeys so the libei session is left alone.
func (a *App) verifyHotkeyHealth() {
	if !a.appState.HotkeysRegistered() {
		return
	}

	hc, ok := a.hotkeyManager.(ports.HotkeyHealthReporter)
	if !ok {
		return
	}

	if !hc.HealthCheck() {
		a.logger.Warn("Hotkey listener not healthy after config reload; reinitialising")
		a.reinitializeHotkeys()
	}
}
