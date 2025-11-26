package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/getlantern/systray"
	"github.com/y3owk1n/neru/internal/application/ports"
	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	domainHint "github.com/y3owk1n/neru/internal/domain/hint"
	"github.com/y3owk1n/neru/internal/infra/bridge"
	"github.com/y3owk1n/neru/internal/infra/electron"
	"github.com/y3owk1n/neru/internal/infra/logger"
	"github.com/y3owk1n/neru/internal/ui/coordinates"
	"go.uber.org/zap"
)

const (
	// MaxExecDisplayLength is the maximum length to display for exec commands.
	MaxExecDisplayLength = 30
	// SystrayQuitTimeout is the timeout for systray quit.
	SystrayQuitTimeout = 10 * time.Second
)

// Run starts the main application loop and initializes all subsystems.
func (a *App) Run() error {
	a.logger.Info("Starting Neru")

	a.ipcServer.Start()
	a.logger.Info("IPC server started")

	a.appWatcher.Start()
	a.logger.Info("App watcher started")

	a.refreshHotkeysForAppOrCurrent("")
	a.logger.Info("Hotkeys initialized")

	a.setupAppWatcherCallbacks()

	a.logger.Info("Neru is running")
	a.printStartupInfo()

	return a.waitForShutdown()
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

	if a.overlayManager != nil {
		a.overlayManager.ResizeToActiveScreenSync()
	}

	// Handle grid overlay
	if a.config.Grid.Enabled && a.gridComponent.Context != nil &&
		a.gridComponent.Context.GridOverlay() != nil {
		// If grid mode is not active, mark for refresh on next activation
		if a.appState.CurrentMode() != domain.ModeGrid {
			a.appState.SetGridOverlayNeedsRefresh(true)
		} else {
			// Grid mode is active - resize the existing overlay window to match new screen bounds
			// Resize overlay window to current active screen (where mouse is)
			if a.overlayManager != nil {
				a.overlayManager.ResizeToActiveScreenSync()
			}

			// Regenerate the grid with updated screen bounds and redraw with proper styling
			// Use the renderer which has the configured style, instead of going through
			// the service layer which would lose styling information
			screenBounds := bridge.ActiveScreenBounds()
			normalizedBounds := coordinates.NormalizeToLocalCoordinates(screenBounds)

			characters := a.config.Grid.Characters
			if strings.TrimSpace(characters) == "" {
				characters = a.config.Hints.HintCharacters
			}

			// Create new grid instance with updated bounds
			gridInstance := domainGrid.NewGrid(characters, normalizedBounds, a.logger)
			a.gridComponent.Context.SetGridInstanceValue(gridInstance)

			// Get current input state from grid manager if it exists
			currentInput := ""
			if a.gridComponent.Manager != nil {
				currentInput = a.gridComponent.Manager.CurrentInput()
			}

			// Redraw grid using renderer which preserves the configured style
			drawGridErr := a.renderer.DrawGrid(gridInstance, currentInput)
			if drawGridErr != nil {
				a.logger.Error("Failed to refresh grid after screen change", zap.Error(drawGridErr))

				return
			}

			// Show the overlay
			a.overlayManager.Show()

			a.logger.Info("Grid overlay resized and regenerated for new screen bounds")
		}
	}

	// Handle hint overlay
	if a.config.Hints.Enabled && a.hintsComponent.Overlay != nil {
		// If hints mode is not active, mark for refresh on next activation
		if a.appState.CurrentMode() != domain.ModeHints {
			a.appState.SetHintOverlayNeedsRefresh(true)
		} else {
			// Hints mode is active - resize the overlay and regenerate hints
			if a.overlayManager != nil {
				a.overlayManager.ResizeToActiveScreenSync()
			}

			// Regenerate hints with updated screen bounds
			// Use the hint service which will collect elements with new bounds and apply proper styling
			ctx := context.Background()

			filter := ports.DefaultElementFilter()
			filter.IncludeMenubar = a.config.Hints.IncludeMenubarHints
			filter.AdditionalMenubarTargets = a.config.Hints.AdditionalMenubarHintsTargets
			filter.IncludeDock = a.config.Hints.IncludeDockHints
			filter.IncludeNotificationCenter = a.config.Hints.IncludeNCHints

			// Regenerate hints using the service which preserves styling
			domainHints, showHintsErr := a.hintService.ShowHints(ctx, filter)
			if showHintsErr != nil {
				a.logger.Error("Failed to refresh hints after screen change", zap.Error(showHintsErr))

				return
			}

			// Update hints context with new hints
			if len(domainHints) > 0 {
				hintCollection := domainHint.NewCollection(domainHints)
				a.hintsComponent.Context.SetHints(hintCollection)
			}

			a.logger.Info("Hint overlay resized and regenerated for new screen bounds")
		}
	}

	// Handle scroll overlay
	if a.scrollComponent.Context != nil && a.scrollComponent.Context.IsActive() {
		// Scroll mode is active - resize the overlay and redraw highlight
		if a.overlayManager != nil {
			a.overlayManager.ResizeToActiveScreenSync()
		}

		// Redraw scroll highlight with updated screen bounds
		ctx := context.Background()

		showScrollOverlayErr := a.scrollService.ShowScrollOverlay(ctx)
		if showScrollOverlayErr != nil {
			a.logger.Error(
				"Failed to refresh scroll overlay after screen change",
				zap.Error(showScrollOverlayErr),
			)

			return
		}

		a.logger.Info("Scroll overlay resized and regenerated for new screen bounds")
	}

	// Final resize for any other overlay state
	if a.overlayManager != nil {
		a.overlayManager.ResizeToActiveScreenSync()
	}
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
		electron.EnsureElectronAccessibility(bundleID)
	}

	if electron.ShouldEnableChromiumSupport(bundleID, config.AdditionalChromiumBundles) {
		electron.EnsureChromiumAccessibility(bundleID)
	}

	if electron.ShouldEnableFirefoxSupport(bundleID, config.AdditionalFirefoxBundles) {
		electron.EnsureFirefoxAccessibility(bundleID)
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

	<-sigChan
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

// Cleanup cleans up resources.
func (a *App) Cleanup() {
	a.logger.Info("Cleaning up")

	a.ExitMode()

	// Stop IPC server first to prevent new requests
	if a.ipcServer != nil {
		stopServerErr := a.ipcServer.Stop()
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
