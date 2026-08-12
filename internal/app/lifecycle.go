package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"runtime/metrics"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/adapter/logger"
	"github.com/y3owk1n/neru/internal/adapter/platform"
	"github.com/y3owk1n/neru/internal/app/keybinding"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
)

const (
	// SystrayQuitTimeout is the timeout for systray quit.
	SystrayQuitTimeout = 10 * time.Second
	// StopTimeout is the timeout for IPC server stop during cleanup.
	StopTimeout = 5 * time.Second
	// GCTickerInterval is the interval for garbage collection.
	GCTickerInterval = 5 * time.Minute

	// HighMemoryThreshold is the heap allocation threshold for triggering GC (100MB).
	HighMemoryThreshold = 100 * 1024 * 1024 // 100MB

	// LowMemoryThreshold is the normal heap allocation threshold (50MB).
	LowMemoryThreshold = 50 * 1024 * 1024 // 50MB

	// HighMemoryGCInterval is the GC interval when memory pressure is high.
	HighMemoryGCInterval = 1 * time.Minute // GC every 1 minute when high memory

	// LowMemoryGCInterval is the GC interval when memory pressure is normal.
	LowMemoryGCInterval = 5 * time.Minute // GC every 5 minutes when low memory

	// BytesPerMB is the number of bytes in a megabyte.
	BytesPerMB = 1024 * 1024

	// metricHeapObjects is the runtime/metrics name for live heap object bytes (equivalent to MemStats.HeapAlloc).
	metricHeapObjects = "/memory/classes/heap/objects:bytes"

	// metricGCGoal is the runtime/metrics name for the GC heap goal (equivalent to MemStats.NextGC).
	metricGCGoal = "/gc/heap/goal:bytes"
)

// Run starts the main application loop and initializes all subsystems.
func (a *App) Run() error {
	cfg := a.configSnapshot()
	a.logger.Info("Starting Neru",
		zap.String("version", ipc.BuildVersion()),
		zap.String("platform", string(platform.CurrentOS())),
		zap.String("config_path", a.ConfigPath),
		zap.String("log_level", cfg.Logging.LogLevel),
		zap.Bool("file_logging", !cfg.Logging.DisableFileLogging))

	// Immediately after the identity line and before anything can fail: a build
	// outside the parity boundary says so once, here, rather than one stubbed
	// keystroke at a time.
	announceBuildBoundary(a.logger, platform.CurrentOS(), cgoEnabled)

	err := a.ipcServer.Start(a.ctx)
	if err != nil {
		a.logger.Error("Failed to start IPC server", zap.Error(err))

		return err
	}

	a.logger.Info("IPC server started")

	// These three are ordered, not merely sequential, and the order is starting
	// the watcher last: an activation it delivers reaches both of the things
	// above it, and neither absorbs one until it has run. Registering after the
	// watcher started meant a dropped activation, which nothing retries — ADR
	// 0005 has the consequence, #1348 was the bug, and
	// TestSimulation_TheAppHearsFocusChangesFromTheMomentItWatchesForThem pins
	// it. Registering the hotkeys after it would hand the same activation a
	// binder holding nothing, and the refresh here would then re-register the
	// global table over the per-app one it had just installed
	// (keybinding/binder.go).
	//
	// Only the registrations belong ahead of this; nothing that can block does.
	// The observers below reach a session bus on Linux, and putting a bus round
	// trip in front of the watcher and the hotkeys would delay the daemon
	// accepting its first chord to hurry up something nothing is waiting on.
	a.registerAppWatcherCallbacks()

	a.hotkeys.RefreshFor("")
	a.logger.Info("Hotkeys initialized")

	a.appWatcher.Start()
	a.logger.Info("App watcher started")

	// Watch for theme changes (Dark Mode / Light Mode) so theme-aware label
	// colors follow without a restart.
	a.setupThemeObserver()

	// Gate Mission Control detection at all levels using config. It stays after
	// the hotkeys are live because what it arms can run an action sequence, and
	// it reads the configuration now rather than the snapshot taken at the top
	// of Run: the IPC server is already accepting a reload by this point, and
	// nothing applies this setting again afterwards.
	a.appWatcher.SetMCDetection(a.configSnapshot().Hints.DetectMissionControl)

	a.setupSleepObserver()

	if cfg.Grid.EnableGC {
		ctx, cancel := context.WithCancel(a.ctx)
		a.gcCancel = cancel

		go func() {
			a.logger.Debug("Starting adaptive GC based on memory pressure")

			currentInterval := LowMemoryGCInterval

			ticker := time.NewTicker(currentInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					highMemory := a.adaptiveGC()
					// Adjust interval based on memory pressure
					newInterval := LowMemoryGCInterval
					if highMemory {
						newInterval = HighMemoryGCInterval
					}

					if newInterval != currentInterval {
						ticker.Reset(newInterval)
						currentInterval = newInterval
						a.logger.Debug("Adjusted GC interval",
							zap.Duration("new_interval", newInterval),
							zap.Bool("high_memory", highMemory))
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	a.printStartupInfo()

	return a.waitForShutdown()
}

// readMemMetrics reads HeapAlloc and NextGC equivalents via runtime/metrics,
// which does not require a full stop-the-world pause unlike runtime.ReadMemStats.
// Returns (0, 0) if the metrics are unavailable or have an unexpected kind.
func readMemMetrics() (uint64, uint64) {
	samples := []metrics.Sample{
		{Name: metricHeapObjects},
		{Name: metricGCGoal},
	}
	metrics.Read(samples)

	var heapAlloc, nextGC uint64

	if samples[0].Value.Kind() == metrics.KindUint64 {
		heapAlloc = samples[0].Value.Uint64()
	}

	if samples[1].Value.Kind() == metrics.KindUint64 {
		nextGC = samples[1].Value.Uint64()
	}

	return heapAlloc, nextGC
}

// adaptiveGC performs garbage collection based on current memory pressure.
// Returns true if high memory pressure is detected.
//
// It uses runtime/metrics to read only HeapAlloc and NextGC equivalents,
// avoiding the full stop-the-world pause that runtime.ReadMemStats incurs.
func (a *App) adaptiveGC() bool {
	heapAlloc, nextGC := readMemMetrics()

	heapAllocMB := heapAlloc / BytesPerMB
	highThresholdMB := uint64(HighMemoryThreshold / BytesPerMB)
	lowThresholdMB := uint64(LowMemoryThreshold / BytesPerMB)

	// Use hysteresis to prevent GC oscillation
	if heapAllocMB >= highThresholdMB {
		a.gcAggressiveMode = true
	}

	if a.gcAggressiveMode {
		a.logger.Debug("Running GC due to high memory pressure",
			zap.Uint64("heap_alloc_mb", heapAllocMB),
			zap.Uint64("next_gc_mb", nextGC/BytesPerMB),
			zap.Bool("aggressive_mode", true))
		runtime.GC()

		// Re-read metrics after GC to get the actual post-GC heap size
		postGCHeapAlloc, _ := readMemMetrics()
		postGCHeapAllocMB := postGCHeapAlloc / BytesPerMB

		// Exit aggressive mode if memory drops below low threshold
		if postGCHeapAllocMB < lowThresholdMB {
			a.gcAggressiveMode = false
			a.logger.Debug("Exiting aggressive GC mode",
				zap.Uint64("heap_alloc_mb", postGCHeapAllocMB))
		}

		return true
	} else {
		a.logger.Debug("Skipping GC - memory usage normal",
			zap.Uint64("heap_alloc_mb", heapAllocMB),
			zap.Bool("aggressive_mode", false))

		return false
	}
}

// registerAppWatcherCallbacks registers the handlers for application watcher
// events. It only registers — everything here has to be in place before the
// watcher starts, so nothing that can block belongs in it (see Run).
func (a *App) registerAppWatcherCallbacks() {
	a.appWatcher.OnActivate(func(_, bundleID string) {
		a.handleAppActivation(bundleID)
	})

	// Watch for display parameter changes (monitor unplug/plug, resolution changes)
	a.appWatcher.OnScreenParametersChanged(func() {
		a.HandleScreenParametersChange()
	})

	// Watch for Mission Control activated events
	a.appWatcher.OnMissionControlActivated(func() {
		cfg := a.configSnapshot()
		if len(cfg.Hints.OnMissionControlActivated) > 0 && cfg.Hints.DetectMissionControl {
			a.logger.Info("Mission Control activated: executing actions",
				zap.Int("action_count", len(cfg.Hints.OnMissionControlActivated)))
			a.hotkeys.RunActions(
				"mission_control_activated",
				cfg.Hints.OnMissionControlActivated,
			)
		}
	})

	// Watch for Mission Control deactivated events
	a.appWatcher.OnMissionControlDeactivated(func() {
		cfg := a.configSnapshot()
		if len(cfg.Hints.OnMissionControlDeactivated) > 0 && cfg.Hints.DetectMissionControl {
			a.logger.Info("Mission Control deactivated: executing actions",
				zap.Int("action_count", len(cfg.Hints.OnMissionControlDeactivated)))
			a.hotkeys.RunActions(
				"mission_control_deactivated",
				cfg.Hints.OnMissionControlDeactivated,
			)
		}
	})
}

// HandleScreenParametersChange is the app's entry point for a display
// configuration change — a monitor plugged in or unplugged, a resolution
// change, a laptop waking to a different arrangement: the platform screen
// observers call it, and the simulation harness drives it the same way.
//
// It responds by putting whichever mode is on screen back onto the display as
// it now is, and it is re-entrant by design: an event arriving while one is
// being handled is coalesced into a single retry rather than processed
// concurrently.
func (a *App) HandleScreenParametersChange() {
	if !a.appState.TrySetScreenChangeProcessing() {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			a.logger.Error("panic during screen change processing", zap.Any("recovered", r))
			// Force-clear both flags so future screen-change events are not
			// permanently blocked.
			a.appState.ResetScreenChangeProcessing()
		}
	}()

	for {
		a.processScreenChange()
		// If another screen-change event arrived while we were processing,
		// loop to handle it so no display configuration update is lost.
		// FinishScreenChangeProcessing keeps the processing flag set when
		// a retry is pending, so no re-acquisition is needed and no other
		// goroutine can enter the critical section.
		if !a.appState.FinishScreenChangeProcessing() {
			return
		}
	}
}

// processScreenChange performs the actual screen-change handling logic.
//
// Which mode is on screen, whether its feature is switched on, and what putting
// it back onto the new display means all belong to the package that owns modes:
// this hands the change over and then answers for the overlay it owns. One call
// under one lock hold replaces the unlocked mode snapshot this used to take and
// the three near-identical per-mode functions it fed.
//
// The resize afterwards is a fallback for a mode that rebuilt nothing — scroll,
// the monitor picker, a mode switched off in configuration. It is deliberately
// skipped when the handler reports the overlay needs nothing, because resizing
// the overlay is what brings it up: doing it with nothing open would show an
// overlay the user never asked for on every dock, undock and wake.
func (a *App) processScreenChange() {
	ctx := a.ctx

	overlayNeedsResize := a.modes.RefreshActiveModeForScreenChange(ctx)
	if !overlayNeedsResize || a.overlayPort == nil {
		return
	}

	refreshErr := a.overlayPort.Refresh(ctx)
	if refreshErr != nil && !derrors.IsNotSupported(refreshErr) {
		a.logger.Warn("Failed to resize the overlay after a screen change",
			zap.Error(refreshErr))
	}
}

// handleAppActivation responds to application activation events.
func (a *App) handleAppActivation(bundleID string) {
	cfg := a.configSnapshot()

	// Tell the mode handler which application is focused now, so the keymap can
	// be settled against it. This is a lock-free write and must stay one: the
	// watcher calls this inline and, on macOS, on the main queue (ADR 0005).
	a.modes.PublishFocusedApp(bundleID)

	// The keymap settles from that publication on the next read, but the event
	// tap has to be told before the next key arrives rather than because one
	// did: its blacklist decides whether a chord the newly focused application
	// binds reaches Neru at all. So this one is pushed — on a goroutine, since
	// the handler takes h.mu and this callback runs on the main queue on macOS
	// (ADR 0005). It reads the cell published just above, so racing activations
	// converge on the last one announced.
	go a.modes.RefreshPassthroughForFocusedAppChange()

	if a.appState.CurrentMode() == domain.ModeIdle {
		go a.hotkeys.RefreshFor(bundleID)
	} else {
		// Defer hotkey refresh to avoid re-entry during active modes
		a.appState.SetHotkeyRefreshPending(true)
	}

	if cfg.Hints.Enabled {
		a.handleAdditionalAccessibility(bundleID)
	}
}

// handleAdditionalAccessibility waits for the app's accessibility tree to become
// ready (needed by Electron/Chromium/Firefox apps which initialize asynchronously).
func (a *App) handleAdditionalAccessibility(bundleID string) {
	go func() {
		const maxRetries = 5
		for range maxRetries {
			ready, err := a.accessibility.PrimeApplication(a.ctx, bundleID)
			if err != nil {
				a.logger.Debug("Accessibility priming failed", zap.Error(err))

				return
			}

			if ready {
				return
			}
		}
	}()
}

// printStartupInfo displays startup information including registered hotkeys.
func (a *App) printStartupInfo() {
	a.logger.Info("✓ Neru is running")

	cfg := a.configSnapshot()

	registeredBindings := 0
	for _, actions := range cfg.Hotkeys.Bindings {
		if len(actions) == 0 {
			continue
		}

		if keybinding.ActionsReferenceDisabledMode(actions, cfg) {
			continue
		}

		registeredBindings++
	}

	a.logger.Debug("Configured hotkey bindings", zap.Int("count", registeredBindings))
}

// waitForShutdown waits for shutdown signals and handles graceful termination.
func (a *App) waitForShutdown() error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	programmatic := false

	select {
	case <-sigChan:
		// OS signal received
	case <-a.stopChan:
		// Programmatic stop requested (e.g. systray quit on Darwin).
		// The systray event loop has already exited, so calling
		// platformQuit() again would dispatch to a dead run loop and
		// hang until the timeout fires.
		programmatic = true
	}

	a.logger.Info("Received shutdown signal, starting graceful shutdown...")

	if programmatic {
		signal.Stop(sigChan)
		a.logger.Info("Graceful shutdown completed")

		return nil
	}

	a.logger.Info("\n⚠️  Shutting down gracefully... (press Ctrl+C again to force quit)")

	done := make(chan struct{})

	go func() {
		// platformQuit is the build-tagged dispatch (wiring_platform_*.go);
		// calling infra's systray.Quit directly from here bypassed it.
		platformQuit()
		close(done)
	}()

	// Use timer instead of time.After to prevent memory leaks
	timer := time.NewTimer(SystrayQuitTimeout)

	select {
	case <-done:
		timer.Stop()
		a.logger.Info("Graceful shutdown completed")

		signal.Stop(sigChan)

		return nil
	case <-sigChan:
		a.logger.Warn("Received second signal, forcing shutdown")
		a.logger.Info("⚠️  Force quitting...")
		os.Exit(1)
	case <-timer.C:
		a.logger.Error("Shutdown timeout exceeded, forcing shutdown")
		a.logger.Info("⚠️  Shutdown timeout, force quitting...")
		os.Exit(1)
	}

	return nil
}

// Stop gracefully stops the application.
func (a *App) Stop() {
	a.stopOnce.Do(func() {
		if a.stopChan != nil {
			close(a.stopChan)
		}
	})
}

// Quit triggers a graceful shutdown of the application.
func (a *App) Quit() {
	a.Stop()
	platformQuit()
}

// Cleanup cleans up resources. It is safe to call multiple times; only the
// first invocation performs the actual teardown.
func (a *App) Cleanup() {
	a.cleanupOnce.Do(func() {
		a.logger.Debug("Cleaning up")
		// Cancel root context to signal shutdown to all operations
		if a.cancel != nil {
			a.cancel()
		}
		// Cancel background GC if running
		if a.gcCancel != nil {
			a.gcCancel()
		}

		a.ExitMode()

		if a.modes != nil {
			a.modes.ShowSystemCursor()
		}
		// Stop theme observer: nil the handler first so any in-flight KVO callback
		// (between the async dispatch and actual observer removal) is a no-op.
		a.stopThemeObserver()
		a.stopSleepObserver()
		// Stop IPC server first to prevent new requests.
		// Use a fresh context instead of a.ctx since the root context was
		// canceled above; a canceled context would cause Stop() to fail
		// immediately before it can complete graceful teardown.
		if a.ipcServer != nil {
			stopCtx, stopCancel := context.WithTimeout(context.Background(), StopTimeout)

			stopServerErr := a.ipcServer.Stop(stopCtx)

			stopCancel()

			if stopServerErr != nil {
				a.logger.Error("Failed to stop IPC server", zap.Error(stopServerErr))
			}
		}

		// Clear layout-change callback first so a stale closure can't
		// re-register hotkeys after teardown.
		a.unregisterLayoutChangeHandler()

		if a.hotkeyManager != nil {
			a.hotkeys.Unregister()
		}

		if a.overlayPort != nil {
			a.overlayPort.Destroy()
		}

		if a.screenShareSubscriptionID != 0 {
			a.appState.OffScreenShareStateChanged(a.screenShareSubscriptionID)
			a.screenShareSubscriptionID = 0
		}

		if a.eventTap != nil {
			a.eventTap.Destroy()
		}
		// Close the accessibility client to release platform resources
		// (e.g. AT-SPI D-Bus connection and a11y status on Linux).
		if a.axClient != nil {
			closeErr := a.axClient.Close()
			if closeErr != nil {
				a.logger.Error("Failed to close accessibility client", zap.Error(closeErr))
			}
		}
		// Sync and close logger
		loggerSyncErr := logger.Sync()
		if loggerSyncErr != nil {
			a.logger.Error("Failed to sync logger", zap.Error(loggerSyncErr))
		}

		a.appWatcher.Stop()

		loggerCloseErr := logger.Close()
		if loggerCloseErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close logger: %v\n", loggerCloseErr)
		}
	})
}
