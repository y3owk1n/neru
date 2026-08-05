package modes

import (
	"image"
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/geometry"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// activateGridModeWithAction activates grid mode with optional action parameter.
func (h *handlerState) activateGridModeWithAction(activation modecmd.Activation) {
	// Detect refresh before validation so we can do partial cleanup on re-activation.
	isRefresh := h.appState.CurrentMode() == domain.ModeGrid

	actionEnum, activated := h.activateModeBase(
		domain.ModeNameGrid,
		h.config.Grid.Enabled,
		action.TypeMoveMouse,
		"",
	)
	if !activated {
		if isRefresh {
			h.exitMode()
		}

		return
	}

	actionString := domain.ActionString(actionEnum)

	if isRefresh {
		// During refresh (e.g. --repeat re-activation), only stop polling.
		// Mode and event tap are already in the correct state so we avoid the
		// full exit cycle which would hide the overlay, disable the event tap,
		// run cursor restoration, and transition to idle.
		// The overlay is cleared unconditionally below.
		h.stopIndicatorPolling()
	} else {
		h.exitMode()
	}

	// Clear any previous overlay content (e.g., scroll highlights) before drawing grid.
	// Otherwise scroll highlights persist when switching from scroll to grid.
	h.overlayManager.Clear()

	h.appState.SetGridOverlayNeedsRefresh(false)

	gridInstance := h.createGridInstance()
	h.updateGridOverlayConfig()

	// Reset the grid manager state when setting up the grid.
	// Note: Manager is reused across activations (holds grid state) but reset to clear input.
	// Router is recreated each activation (stateless, needs fresh exit keys from config).
	if h.grid.Manager != nil {
		h.grid.Manager.Reset()
	}

	h.initializeGridManager(gridInstance)

	h.grid.Router = domainGrid.NewRouter(h.grid.Manager, h.logger)

	// Draw the grid to populate the overlay
	drawGridErr := h.renderer.DrawGrid(gridInstance, "")
	if drawGridErr != nil {
		h.logger.Error("Failed to draw grid", zap.Error(drawGridErr))

		if isRefresh {
			h.exitMode()
		}

		return
	}

	h.overlayManager.ResizeToActiveScreen()

	// Show the overlay (the grid is already drawn with proper style)
	h.overlayManager.Show()

	applyGridFlags(h.grid.Context, activation, isRefresh)

	h.grid.Context.ClearSelectionPoint()
	h.refreshGridVirtualPointer()

	if activation.Action != nil {
		h.logger.Debug("Grid mode activated with pending action",
			zap.String("action", *activation.Action),
			zap.Bool("repeat", activation.Repeat != nil && *activation.Repeat))
	}

	// Only set mode and enable event tap on initial activation;
	// during refresh these are already in the correct state.
	if !isRefresh {
		h.setMode(domain.ModeGrid, overlay.ModeGrid)
	}

	h.logger.Info("Grid mode activated", zap.String("action", actionString))

	h.startIndicatorPolling(domain.ModeGrid)
}

// createGridInstance creates a new grid with proper bounds and characters.
func (h *handlerState) createGridInstance() *domainGrid.Grid {
	var screenBounds image.Rectangle

	if h.system != nil {
		b, err := h.system.ScreenBounds(h.ctx)
		if err == nil {
			screenBounds = b
		} else if !derrors.IsNotSupported(err) {
			h.logger.Warn("Failed to get screen bounds for grid", zap.Error(err))
		}
	}

	// Store screen bounds for coordinate conversion
	h.setScreenBounds(screenBounds)

	// Normalize normalizedBounds to window-local coordinates using helper function
	normalizedBounds := geometry.NormalizeToLocalCoordinates(screenBounds)

	characters := h.config.Grid.Characters
	if strings.TrimSpace(characters) == "" {
		characters = h.config.Hints.HintCharacters
	}

	gridInstance := domainGrid.NewGridWithLabels(
		characters,
		h.config.Grid.RowLabels,
		h.config.Grid.ColLabels,
		normalizedBounds,
		h.logger,
	)
	h.grid.Context.SetGridInstanceValue(gridInstance)

	return gridInstance
}

// updateGridOverlayConfig updates the grid overlay configuration.
func (h *handlerState) updateGridOverlayConfig() {
	if h.grid.Overlay != nil {
		h.grid.Overlay.SetConfig(h.config.Grid)
	}
}

// initializeGridManager initializes the grid manager with the new grid instance.
// It sets up subgrid configuration, creates the manager with update callbacks for
// overlay rendering and subgrid navigation, and configures the grid router.
func (h *handlerState) initializeGridManager(gridInstance *domainGrid.Grid) {
	const defaultGridCharacters = "asdfghjkl"

	// Defensive check for grid instance
	if gridInstance == nil {
		h.logger.Warn("Grid instance is nil, creating with default bounds")

		var screenBounds image.Rectangle

		if h.system != nil {
			b, err := h.system.ScreenBounds(h.ctx)
			if err == nil {
				screenBounds = b
			} else if !derrors.IsNotSupported(err) {
				h.logger.Warn("Failed to get screen bounds for grid fallback", zap.Error(err))
			}
		}

		bounds := image.Rect(0, 0, screenBounds.Dx(), screenBounds.Dy())
		gridInstance = domainGrid.NewGridWithLabels(
			h.config.Grid.Characters,
			h.config.Grid.RowLabels,
			h.config.Grid.ColLabels,
			bounds,
			h.logger,
		)
	}

	// Configure subgrid keys for 3x3 subgrid navigation within selected cells
	keys := strings.TrimSpace(h.config.Grid.SublayerKeys)
	if keys == "" {
		keys = h.config.Grid.Characters
	}

	// Ensure we have valid keys for subgrid
	if keys == "" {
		h.logger.Warn("No subgrid keys configured, using grid characters as fallback")
		keys = h.config.Grid.Characters
	}

	// Final fallback to default characters if none configured
	if keys == "" {
		keys = defaultGridCharacters

		h.logger.Warn("No characters available for subgrid, using default")
	}

	const (
		subRows = 3
		subCols = 3
	)

	// Create grid manager with callbacks for overlay updates and subgrid navigation
	h.grid.Manager = domainGrid.NewManager(
		gridInstance,
		subRows,
		subCols,
		keys,
		// Update callback: handles grid redrawing and match filtering
		func(forceRedraw bool) {
			// Defensive check for grid manager
			if h.grid.Manager == nil {
				h.logger.Error("Grid manager is nil during update callback")

				return
			}

			input := h.grid.Manager.CurrentInput()

			// Force redraw only when exiting subgrid to restore main grid
			if forceRedraw {
				h.overlayManager.Clear()

				gridErr := h.renderer.DrawGrid(gridInstance, input)
				if gridErr != nil {
					h.logger.Error("Failed to redraw grid", zap.Error(gridErr))

					return
				}

				h.overlayManager.Show()
			}

			// Hide unmatched cells if configured and input exists
			hideUnmatched := h.config.Grid.HideUnmatched && len(input) > 0
			h.renderer.SetHideUnmatched(hideUnmatched)
			h.renderer.UpdateGridMatches(input)
			h.refreshGridVirtualPointer()
		},
		// Subgrid callback: moves cursor and shows subgrid overlay
		func(cell *domainGrid.Cell) {
			// Defensive check for cell
			if cell == nil {
				h.logger.Warn("Attempted to show subgrid for nil cell")

				return
			}

			// Move mouse to center of cell before showing subgrid for better UX
			ctx := h.ctx

			// Convert cell center from window-local to screen-absolute coordinates
			absoluteCenter := geometry.ConvertToAbsoluteCoordinates(
				cell.Center(),
				h.screenBounds,
			)

			if h.grid != nil && h.grid.Context != nil {
				h.grid.Context.SetSelectionPoint(absoluteCenter)

				if !h.grid.Context.CursorFollowSelection() {
					h.renderer.ShowSubgrid(cell)
					h.refreshGridVirtualPointer()

					return
				}
			}

			moveCursorErr := h.actionService.MoveCursorToPoint(ctx, absoluteCenter)
			if moveCursorErr != nil {
				h.logger.Error("Failed to move cursor", zap.Error(moveCursorErr))
			}

			// Draw 3x3 subgrid inside selected cell
			h.renderer.ShowSubgrid(cell)
			h.refreshGridVirtualPointer()
		},
		h.logger,
	)
}

// applyGridFlags writes the flags an activation carries into the context. A refresh
// writes only what it was given; a fresh activation writes every field so
// nothing leaks over from the previous run.
func applyGridFlags(ctx *grid.Context, activation modecmd.Activation, isRefresh bool) {
	if isRefresh {
		if activation.Action != nil {
			ctx.SetPendingAction(activation.Action)
		}

		if activation.OnExit != nil {
			ctx.SetOnExit(activation.OnExit)
		}

		if activation.Modifier != nil {
			ctx.SetPendingModifier(activation.Modifier)
		}

		if activation.Repeat != nil {
			ctx.SetRepeat(*activation.Repeat)
		}

		if activation.CursorFollowSelection != nil {
			ctx.SetCursorFollowSelection(*activation.CursorFollowSelection)
		}

		return
	}

	ctx.SetPendingAction(activation.Action)
	ctx.SetOnExit(activation.OnExit)
	ctx.SetPendingModifier(activation.Modifier)
	ctx.SetRepeat(activation.Repeat != nil && *activation.Repeat)
	ctx.SetCursorFollowSelection(resolveCursorFollowSelection(
		domain.ModeGrid,
		activation.CursorFollowSelection,
	))
}
