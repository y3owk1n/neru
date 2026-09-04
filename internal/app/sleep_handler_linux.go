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
	// hibernateReinitDelay is how long after PrepareForSleep(true) we wait
	// before reinitialising. If PrepareForSleep(false) (normal resume) arrives
	// within this window, the reinit is canceled. This handles the systemd
	// issue (https://github.com/systemd/systemd/issues/30666) where
	// PrepareForSleep(false) is not emitted after hibernation.
	hibernateReinitDelay = 10 * time.Second
	// postReloadCheckDelay is how long after a config reload we wait before
	// verifying the hotkey listener started correctly.
	postReloadCheckDelay = 2 * time.Second
	// hotkeyReinitRetries is the default number of hotkey reinitialization retries.
	hotkeyReinitRetries = 5
	// hotkeyReinitDelay is the delay between retry attempts during normal reinit.
	hotkeyReinitDelay = 500 * time.Millisecond
	// hotkeySleepRetries is the number of hotkey reinitialization retries after
	// sleep/wake, allowing extra time for evdev devices to settle.
	hotkeySleepRetries = 10
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
//
// All teardown state is captured per App instance in the closures registered
// on the App — package-level state here would be shared across instances, and
// a process that creates more than one (tests do) would close another
// instance's channel and leak its goroutine. The teardown and post-reload
// hooks are registered before the D-Bus attempt so the post-reload health
// check keeps working when the bus is unavailable.
func (a *App) setupSleepObserver() {
	stopChan := make(chan struct{})
	waitGroup := &sync.WaitGroup{}

	// Assigned once the D-Bus signal path is established; the stop closure
	// reads it at call time, so the degraded no-bus path leaves it nil.
	var dbusClose func() error

	// Under observerMu: the IPC server is already live, so a reload can be
	// reading postReloadVerify while Run is still in here.
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
		// hibernateReinitTimer fires after hibernateReinitDelay unless
		// canceled by a matching PrepareForSleep(false) signal. Only this
		// goroutine touches it.
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

// reinitializeHotkeys tears down and re-registers the global hotkey listener
// without touching the libei/RemoteDesktop session. Use this for health-check
// recovery after stale evdev fds; it avoids triggering a fresh RemoteDesktop
// portal consent prompt.
func (a *App) reinitializeHotkeys() {
	a.reinitializeHotkeysWithParams(hotkeyReinitRetries, hotkeyReinitDelay)
}

// reinitializeHotkeysAfterSleep is used on sleep/wake resume. Some systems
// can take several seconds for evdev input devices to settle after
// resume, so this uses a longer retry window (10 retries x 1s = 10s).
func (a *App) reinitializeHotkeysAfterSleep() {
	a.reinitializeHotkeysWithParams(hotkeySleepRetries, 1*time.Second)
}

func (a *App) reinitializeHotkeysWithParams(maxRetries int, retryDelay time.Duration) {
	a.logger.Info("Reinitializing hotkey listener (evdev only)")

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

	a.reinitializeHotkeysAfterSleep()

	a.logger.Info("Input listeners reinitialized after sleep/wake")
}

// verifyHotkeyHealth tests whether the global hotkey listener is alive when it
// should be running, and re-registers hotkeys if not. Uses evdev-only recovery
// (reinitializeHotkeys) rather than the full handleWakeFromSleep so the
// libei/RemoteDesktop session is not torn down unnecessarily.
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
