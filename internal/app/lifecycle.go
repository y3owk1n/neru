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

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/infra/electron"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
	"github.com/y3owk1n/neru/internal/core/infra/platform"
	"github.com/y3owk1n/neru/internal/core/infra/systray"
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

	err := a.ipcServer.Start(a.ctx)
	if err != nil {
		a.logger.Error("Failed to start IPC server", zap.Error(err))

		return err
	}

	a.logger.Info("IPC server started")

	a.appWatcher.Start()
	a.logger.Info("App watcher started")

	a.refreshHotkeysForAppOrCurrent("")
	a.logger.Info("Hotkeys initialized")

	a.setupSleepObserver()

	a.setupAppWatcherCallbacks()

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

// setupAppWatcherCallbacks configures callbacks for application watcher events.
func (a *App) setupAppWatcherCallbacks() {
	a.appWatcher.OnActivate(func(_, bundleID string) {
		a.handleAppActivation(bundleID)
	})

	// Watch for display parameter changes (monitor unplug/plug, resolution changes)
	a.appWatcher.OnScreenParametersChanged(func() {
		a.handleScreenParametersChange()
	})

	// Watch for Mission Control activated events
	a.appWatcher.OnMissionControlActivated(func() {
		cfg := a.configSnapshot()
		if len(cfg.Hints.OnMissionControlActivated) > 0 && cfg.Hints.DetectMissionControl {
			a.logger.Info("Mission Control activated: executing actions",
				zap.Int("action_count", len(cfg.Hints.OnMissionControlActivated)))
			a.dispatchHotkeyActionsAsync(
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
			a.dispatchHotkeyActionsAsync(
				"mission_control_deactivated",
				cfg.Hints.OnMissionControlDeactivated,
			)
		}
	})

	// Watch for macOS theme changes (Dark Mode / Light Mode) to update
	// theme-aware label colors without requiring restart.
	a.setupThemeObserver()

	// Gate Mission Control detection at all levels using config
	a.appWatcher.SetMCDetection(a.configSnapshot().Hints.DetectMissionControl)
}

// handleScreenParametersChange responds to display configuration changes by updating overlays.
func (a *App) handleScreenParametersChange() {
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
func (a *App) processScreenChange() {
	// Snapshot the mode once so every decision in this pass uses a consistent value.
	// Without the snapshot, a concurrent mode transition could cause the idle check,
	// the grid handler, and the hint handler to each see a different mode.
	// Each handler's Refresh* method re-checks the mode under h.mu to guard
	// against a concurrent ExitMode between the snapshot and the actual work.
	currentMode := a.appState.CurrentMode()

	// Only log and adjust overlays if we are in an active mode.
	// In Idle mode, we just want to update the needs-refresh flags (handled by sub-handlers)
	// but avoid showing the overlay window which happens in ResizeToActiveScreen.
	isIdle := currentMode == domain.ModeIdle
	if !isIdle {
		a.logger.Debug("Screen parameters changed; adjusting overlays")
	}

	ctx := a.ctx

	cfg := a.configSnapshot()

	gridResized := a.handleGridScreenChange(cfg, currentMode)
	hintResized := a.handleHintScreenChange(ctx, cfg, currentMode)
	recursiveGridResized := a.handleRecursiveGridScreenChange(cfg, currentMode)

	// Final resize only if no handler already resized the overlay AND we are not idle.
	// Resizing the overlay when idle would cause it to become visible, which we want to avoid.
	if !gridResized && !hintResized && !recursiveGridResized && !isIdle {
		if a.overlayManager != nil {
			a.overlayManager.ResizeToActiveScreen()
		}
	}
}

// handleGridScreenChange handles grid overlay updates when screen parameters change.
// Returns true if the overlay was resized.
func (a *App) handleGridScreenChange(cfg *config.Config, currentMode domain.Mode) bool {
	if !cfg.Grid.Enabled || a.gridComponent.Overlay == nil {
		return false
	}

	if currentMode != domain.ModeGrid {
		a.appState.SetGridOverlayNeedsRefresh(true)

		return false
	}

	// Grid mode is active - resize the existing overlay window to match new screen bounds
	if a.overlayManager == nil {
		a.logger.Warn("overlay manager unavailable; skipping grid refresh")

		return false
	}

	a.overlayManager.ResizeToActiveScreen()

	// Delegate to the modes handler which holds the grid manager state and
	// can regenerate the grid with new screen bounds under the mutex.
	// RefreshGridForScreenChange re-checks the mode under h.mu to guard
	// against a concurrent mode exit (TOCTOU).
	if !a.modes.RefreshGridForScreenChange() {
		// Mode was exited concurrently or draw failed — don't show the overlay.
		a.logger.Debug(
			"Grid screen-change refresh skipped (mode exited or draw failed); skipping show",
		)

		return true
	}

	a.overlayManager.Show()
	a.logger.Debug("Grid overlay resized and regenerated for new screen bounds")

	return true
}

// handleHintScreenChange handles hint overlay updates when screen parameters change.
// Returns true if the overlay was resized.
func (a *App) handleHintScreenChange(
	ctx context.Context,
	cfg *config.Config,
	currentMode domain.Mode,
) bool {
	if !cfg.Hints.Enabled || a.hintsComponent.Overlay == nil {
		return false
	}

	if currentMode != domain.ModeHints {
		a.appState.SetHintOverlayNeedsRefresh(true)

		return false
	}

	if a.overlayManager != nil {
		a.overlayManager.ResizeToActiveScreen()
	}

	// RefreshHintsForScreenChange re-checks the mode under h.mu to guard
	// against a concurrent mode exit (TOCTOU).
	if !a.modes.RefreshHintsForScreenChange(ctx, a.hintService) {
		a.logger.Debug("Hint mode exited during screen change; skipping show")

		return true
	}

	a.logger.Debug("Hint overlay resized and regenerated for new screen bounds")

	return true
}

// handleRecursiveGridScreenChange handles recursive-grid overlay updates when screen parameters change.
// Returns true if the overlay was resized.
func (a *App) handleRecursiveGridScreenChange(cfg *config.Config, currentMode domain.Mode) bool {
	if !cfg.RecursiveGrid.Enabled || a.recursiveGridComponent == nil ||
		a.recursiveGridComponent.Overlay == nil {
		return false
	}

	if currentMode != domain.ModeRecursiveGrid {
		a.appState.SetRecursiveGridOverlayNeedsRefresh(true)

		return false
	}

	if a.overlayManager == nil {
		a.logger.Warn("overlay manager unavailable; skipping recursive-grid refresh")

		return false
	}

	a.overlayManager.ResizeToActiveScreen()

	// Delegate to the modes handler which holds the recursive-grid manager
	// state and can reinitialize it with new screen bounds under the mutex.
	// RefreshRecursiveGridForScreenChange re-checks the mode under h.mu to
	// guard against a concurrent mode exit (TOCTOU between the snapshot in
	// processScreenChange and the actual work here).
	if !a.modes.RefreshRecursiveGridForScreenChange() {
		// Mode was exited concurrently — don't show the overlay.
		a.logger.Debug("Recursive-grid mode exited during screen change; skipping show")

		return true
	}

	a.overlayManager.Show()
	a.logger.Debug("Recursive-grid overlay resized and regenerated for new screen bounds")

	return true
}

// handleAppActivation responds to application activation events.
func (a *App) handleAppActivation(bundleID string) {
	cfg := a.configSnapshot()

	if a.appState.CurrentMode() == domain.ModeIdle {
		go a.refreshHotkeysForAppOrCurrent(bundleID)
	} else {
		// Defer hotkey refresh to avoid re-entry during active modes
		a.appState.SetHotkeyRefreshPending(true)
	}

	if cfg.Hints.Enabled {
		a.handleAdditionalAccessibility(bundleID, cfg)
	}
}

// handleAdditionalAccessibility wakes the focused application's accessibility
// tree so hints can read it. AXManualAccessibility is set on every focused app.
// The window-moving AXEnhancedUserInterface is set only for Chromium/Firefox
// browsers, and only when web-content hint support is enabled.
func (a *App) handleAdditionalAccessibility(bundleID string, cfg *config.Config) {
	axCfg := cfg.Hints.AdditionalAXSupport

	useEnhanced := axCfg.Enable &&
		(electron.ShouldEnableChromiumSupport(bundleID, axCfg.AdditionalChromiumBundles) ||
			electron.ShouldEnableFirefoxSupport(bundleID, axCfg.AdditionalFirefoxBundles))

	go electron.EnsureAppAccessibility(bundleID, useEnhanced, a.logger)
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

		if actionsReferenceDisabledMode(actions, cfg) {
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
		// systray.Quit() again would dispatch to a dead run loop and
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
		systray.Quit()
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
			a.hotkeyRegistrationMu.Lock()
			a.stopAllHotkeyRepeats()
			a.hotkeyManager.UnregisterAll()
			a.appState.SetHotkeysRegistered(false)
			a.hotkeyRegistrationMu.Unlock()
		}

		if a.overlayManager != nil {
			a.overlayManager.Destroy()
		}
		// Cleanup screen share state subscription
		if a.screenShareSubscriptionID != 0 {
			a.appState.OffScreenShareStateChanged(a.screenShareSubscriptionID)
			a.screenShareSubscriptionID = 0
		}

		if a.eventTap != nil {
			a.eventTap.Destroy()
		}
		// Sync and close logger
		loggerSyncErr := logger.Sync()
		if loggerSyncErr != nil {
			a.logger.Error("Failed to sync logger", zap.Error(loggerSyncErr))
		}

		a.appWatcher.Stop()

		loggerCloseErr := logger.Close()
		if loggerCloseErr != nil {
			// Can't log this since logger is being closed
			fmt.Fprintf(os.Stderr, "Warning: failed to close logger: %v\n", loggerCloseErr)
		}
	})
}
