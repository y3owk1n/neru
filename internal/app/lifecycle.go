package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/getlantern/systray"
	"github.com/y3owk1n/neru/internal/core/domain"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
	domainHint "github.com/y3owk1n/neru/internal/core/domain/hint"
	"github.com/y3owk1n/neru/internal/core/infra/bridge"
	"github.com/y3owk1n/neru/internal/core/infra/electron"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
	"github.com/y3owk1n/neru/internal/ui/coordinates"
	"go.uber.org/zap"
)

const (
	// MaxExecDisplayLength is the maximum length for executable display names.
	MaxExecDisplayLength = 30
	// SystrayQuitTimeout is the timeout for systray quit.
	SystrayQuitTimeout = 10 * time.Second
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
)

// Run starts the main application loop and initializes all subsystems.
func (a *App) Run() error {
	a.logger.Info("Starting Neru")

	err := a.ipcServer.Start(context.Background())
	if err != nil {
		a.logger.Error("Failed to start IPC server", zap.Error(err))

		return err
	}

	a.logger.Info("IPC server started")

	a.appWatcher.Start()
	a.logger.Info("App watcher started")

	a.refreshHotkeysForAppOrCurrent("")
	a.logger.Info("Hotkeys initialized")

	a.setupAppWatcherCallbacks()

	a.logger.Info("Neru is running")

	if a.config.Grid.EnableGC {
		ctx, cancel := context.WithCancel(context.Background())
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

// adaptiveGC performs garbage collection based on current memory pressure.
// Returns true if high memory pressure is detected.
func (a *App) adaptiveGC() bool {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	heapAllocMB := memStats.HeapAlloc / BytesPerMB
	highThresholdMB := uint64(HighMemoryThreshold / BytesPerMB)
	lowThresholdMB := uint64(LowMemoryThreshold / BytesPerMB)

	// Use hysteresis to prevent GC oscillation
	if heapAllocMB >= highThresholdMB {
		a.gcAggressiveMode = true
	}

	if a.gcAggressiveMode {
		a.logger.Debug("Running GC due to high memory pressure",
			zap.Uint64("heap_alloc_mb", heapAllocMB),
			zap.Uint64("next_gc_mb", memStats.NextGC/BytesPerMB),
			zap.Bool("aggressive_mode", true))
		runtime.GC()

		// Exit aggressive mode if memory drops below low threshold
		if heapAllocMB < lowThresholdMB {
			a.gcAggressiveMode = false
			a.logger.Debug("Exiting aggressive GC mode",
				zap.Uint64("heap_alloc_mb", heapAllocMB))
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
}

// handleScreenParametersChange responds to display configuration changes by updating overlays.
func (a *App) handleScreenParametersChange() {
	if a.appState.ScreenChangeProcessing() {
		return
	}

	a.appState.SetScreenChangeProcessing(true)

	defer func() { a.appState.SetScreenChangeProcessing(false) }()

	a.logger.Info("Screen parameters changed; adjusting overlays")

	ctx := context.Background()

	gridResized := a.handleGridScreenChange()
	hintResized := a.handleHintScreenChange(ctx)
	scrollResized := a.handleScrollScreenChange(ctx)

	// Final resize only if no handler already resized the overlay
	if !gridResized && !hintResized && !scrollResized {
		if a.overlayManager != nil {
			a.overlayManager.ResizeToActiveScreen()
		}
	}
}

// handleGridScreenChange handles grid overlay updates when screen parameters change.
// Returns true if the overlay was resized.
func (a *App) handleGridScreenChange() bool {
	if !a.config.Grid.Enabled || a.gridComponent.Context == nil ||
		a.gridComponent.Context.GridOverlay() == nil {
		return false
	}

	if a.appState.CurrentMode() != domain.ModeGrid {
		a.appState.SetGridOverlayNeedsRefresh(true)

		return false
	}

	// Grid mode is active - resize the existing overlay window to match new screen bounds
	if a.overlayManager == nil {
		a.logger.Warn("overlay manager unavailable; skipping grid refresh")

		return false
	}

	a.overlayManager.ResizeToActiveScreen()

	// Regenerate the grid with updated screen bounds and redraw with proper styling
	screenBounds := bridge.ActiveScreenBounds()
	normalizedBounds := coordinates.NormalizeToLocalCoordinates(screenBounds)

	characters := a.config.Grid.Characters
	if strings.TrimSpace(characters) == "" {
		characters = a.config.Hints.HintCharacters
	}

	gridInstance := domainGrid.NewGridWithLabels(
		characters,
		a.config.Grid.RowLabels,
		a.config.Grid.ColLabels,
		normalizedBounds,
		a.logger,
	)
	a.gridComponent.Context.SetGridInstanceValue(gridInstance)

	currentInput := ""
	if a.gridComponent.Manager != nil {
		currentInput = a.gridComponent.Manager.CurrentInput()
	}

	drawGridErr := a.renderer.DrawGrid(gridInstance, currentInput)
	if drawGridErr != nil {
		a.logger.Error("Failed to refresh grid after screen change", zap.Error(drawGridErr))

		return true
	}

	a.overlayManager.Show()
	a.logger.Info("Grid overlay resized and regenerated for new screen bounds")

	return true
}

// handleHintScreenChange handles hint overlay updates when screen parameters change.
// Returns true if the overlay was resized.
func (a *App) handleHintScreenChange(ctx context.Context) bool {
	if !a.config.Hints.Enabled || a.hintsComponent.Overlay == nil {
		return false
	}

	if a.appState.CurrentMode() != domain.ModeHints {
		a.appState.SetHintOverlayNeedsRefresh(true)

		return false
	}

	if a.overlayManager != nil {
		a.overlayManager.ResizeToActiveScreen()
	}

	domainHints, showHintsErr := a.hintService.ShowHints(ctx)
	if showHintsErr != nil {
		a.logger.Error("Failed to refresh hints after screen change", zap.Error(showHintsErr))

		return true
	}

	if len(domainHints) > 0 {
		hintCollection := domainHint.NewCollection(domainHints)
		a.hintsComponent.Context.SetHints(hintCollection)
	}

	a.logger.Info("Hint overlay resized and regenerated for new screen bounds")

	return true
}

// handleScrollScreenChange handles scroll overlay updates when screen parameters change.
// Returns true if the overlay was resized.
func (a *App) handleScrollScreenChange(ctx context.Context) bool {
	if a.scrollComponent.Context == nil || !a.scrollComponent.Context.IsActive() {
		return false
	}

	if a.overlayManager != nil {
		a.overlayManager.ResizeToActiveScreen()
	}

	showScrollOverlayErr := a.scrollService.Show(ctx)
	if showScrollOverlayErr != nil {
		a.logger.Error(
			"Failed to refresh scroll overlay after screen change",
			zap.Error(showScrollOverlayErr),
		)

		return true
	}

	a.logger.Info("Scroll overlay resized and regenerated for new screen bounds")

	return true
}

// handleAppActivation responds to application activation events.
func (a *App) handleAppActivation(bundleID string) {
	if a.appState.CurrentMode() == domain.ModeIdle {
		go a.refreshHotkeysForAppOrCurrent(bundleID)
	} else {
		// Defer hotkey refresh to avoid re-entry during active modes
		a.appState.SetHotkeyRefreshPending(true)
	}

	if a.config.Hints.Enabled {
		if a.config.Hints.AdditionalAXSupport.Enable {
			a.handleAdditionalAccessibility(bundleID)
		}
	}
}

// handleAdditionalAccessibility configures accessibility support for Electron/Chromium/Firefox applications.
func (a *App) handleAdditionalAccessibility(bundleID string) {
	config := a.config.Hints.AdditionalAXSupport

	if electron.ShouldEnableElectronSupport(bundleID, config.AdditionalElectronBundles) {
		electron.EnsureElectronAccessibility(bundleID, a.logger)
	}

	if electron.ShouldEnableChromiumSupport(bundleID, config.AdditionalChromiumBundles) {
		electron.EnsureChromiumAccessibility(bundleID, a.logger)
	}

	if electron.ShouldEnableFirefoxSupport(bundleID, config.AdditionalFirefoxBundles) {
		electron.EnsureFirefoxAccessibility(bundleID, a.logger)
	}
}

// printStartupInfo displays startup information including registered hotkeys.
func (a *App) printStartupInfo() {
	a.logger.Info("✓ Neru is running")

	for key, value := range a.config.Hotkeys.Bindings {
		mode := value
		if parts := strings.Split(value, " "); len(parts) > 0 {
			mode = parts[0]
		}

		if mode == domain.ModeString(domain.ModeHints) && !a.config.Hints.Enabled {
			continue
		}

		if mode == domain.ModeString(domain.ModeGrid) && !a.config.Grid.Enabled {
			continue
		}

		toShow := value
		if strings.HasPrefix(value, "exec") {
			runes := []rune(value)
			if len(runes) > MaxExecDisplayLength {
				toShow = string(runes[:30]) + "..."
			}
		}

		a.logger.Info(fmt.Sprintf("  %s: %s", key, toShow))
	}
}

// waitForShutdown waits for shutdown signals and handles graceful termination.
func (a *App) waitForShutdown() error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sigChan:
		// OS signal received
	case <-a.stopChan:
		// Programmatic stop requested
	}

	a.logger.Info("Received shutdown signal, starting graceful shutdown...")
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
		timer.Stop() // Stop timer immediately on success
		a.logger.Info("Graceful shutdown completed")

		signal.Stop(sigChan)

		return nil
	case <-sigChan:
		timer.Stop() // Stop timer on second signal
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

// Cleanup cleans up resources.
func (a *App) Cleanup() {
	a.logger.Info("Cleaning up")

	// Cancel background GC if running
	if a.gcCancel != nil {
		a.gcCancel()
	}

	a.ExitMode()

	// Stop IPC server first to prevent new requests
	if a.ipcServer != nil {
		stopServerErr := a.ipcServer.Stop(context.Background())
		if stopServerErr != nil {
			a.logger.Error("Failed to stop IPC server", zap.Error(stopServerErr))
		}
	}

	if a.hotkeyManager != nil {
		a.hotkeyManager.UnregisterAll()
	}

	if a.overlayManager != nil {
		a.overlayManager.Destroy()
	}

	if a.eventTap != nil {
		a.eventTap.Destroy()
	}

	// Sync and close logger
	loggerSyncErr := logger.Sync()
	if loggerSyncErr != nil {
		// Ignore "inappropriate ioctl for device" error which occurs when syncing stdout/stderr
		if !strings.Contains(loggerSyncErr.Error(), "inappropriate ioctl for device") {
			a.logger.Error("Failed to sync logger", zap.Error(loggerSyncErr))
		}
	}

	a.appWatcher.Stop()

	loggerCloseErr := logger.Close()
	if loggerCloseErr != nil {
		// Can't log this since logger is being closed
		fmt.Fprintf(os.Stderr, "Warning: failed to close logger: %v\n", loggerCloseErr)
	}
}
