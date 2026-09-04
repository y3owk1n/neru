package app

// Platform observers (theme changes, system sleep/wake) register per-instance
// teardown and hook closures on the App. The shared entry points below consume
// them, so the App struct never references platform types, every platform's
// teardown runs at most once, and a process holding several App instances
// (tests do) cannot tear down or leak another instance's observers.

// stopThemeObserver runs this instance's theme observer teardown, at most
// once. Platforms whose observer stops with the app context leave the field
// nil, making this a no-op.
func (a *App) stopThemeObserver() {
	a.observerMu.Lock()
	stop := a.themeObserverStop
	a.themeObserverStop = nil
	a.observerMu.Unlock()

	if stop != nil {
		stop()
	}
}

// stopSleepObserver runs this instance's sleep observer teardown, at most
// once. Only Linux registers one; see setupSleepObserver.
func (a *App) stopSleepObserver() {
	a.observerMu.Lock()
	stop := a.sleepObserverStop
	a.sleepObserverStop = nil
	a.observerMu.Unlock()

	if stop != nil {
		stop()
	}
}

// schedulePostReloadVerification runs the platform's post-config-reload
// health check, if any. Only Linux registers one: it verifies the evdev
// hotkey listener actually came back after a reload.
func (a *App) schedulePostReloadVerification() {
	a.observerMu.Lock()
	verify := a.postReloadVerify
	a.observerMu.Unlock()

	if verify != nil {
		verify()
	}
}
