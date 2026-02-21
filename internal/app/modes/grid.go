package modes

import (
	"context"
	"image"
	"strings"

	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
	"github.com/y3owk1n/neru/internal/core/infra/bridge"
	"github.com/y3owk1n/neru/internal/ui/coordinates"
	"go.uber.org/zap"
)

// activateGridModeWithAction activates grid mode with optional action parameter.
func (h *Handler) activateGridModeWithAction(actionStr *string) {
	actionEnum, ok := h.activateModeBase(
		domain.ModeNameGrid,
		h.config.Grid.Enabled,
		action.TypeMoveMouse,
	)
	if !ok {
		return
	}

	actionString := domain.ActionString(actionEnum)

	h.ExitMode()
	// Clear any previous overlay content (e.g., scroll highlights) before drawing grid.
	// This prevents scroll highlights from persisting when switching from scroll mode to grid mode.
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

	exitKeys := h.config.General.ModeExitKeys
	if len(exitKeys) == 0 {
		exitKeys = DefaultModeExitKeys()
	}

	h.grid.Router = domainGrid.NewRouterWithExitKeys(h.grid.Manager, h.logger, exitKeys)

	// Draw the grid to populate the overlay
	drawGridErr := h.renderer.DrawGrid(gridInstance, "")
	if drawGridErr != nil {
		h.logger.Error("Failed to draw grid", zap.Error(drawGridErr))

		return
	}

	// Show the overlay (the grid is already drawn with proper style)
	h.overlayManager.Show()

	// Store pending action if provided
	h.grid.Context.SetPendingAction(actionStr)

	if actionStr != nil {
		h.logger.Info("Grid mode activated with pending action", zap.String("action", *actionStr))
	}

	h.SetModeGrid()

	h.logger.Info("Grid mode activated", zap.String("action", actionString))
	h.logger.Info("Type a grid label to select a location")

	h.startModeIndicatorPolling(domain.ModeGrid)
}

// createGridInstance creates a new grid instance with proper bounds and characters.
func (h *Handler) createGridInstance() *domainGrid.Grid {
	screenBounds := bridge.ActiveScreenBounds()

	// Store screen bounds for coordinate conversion
	h.screenBounds = screenBounds

	// Normalize normalizedBounds to window-local coordinates using helper function
	normalizedBounds := coordinates.NormalizeToLocalCoordinates(screenBounds)

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
func (h *Handler) updateGridOverlayConfig() {
	h.grid.Overlay.SetConfig(h.config.Grid)
}

// initializeGridManager initializes the grid manager with the new grid instance.
// It sets up subgrid configuration, creates the manager with update callbacks for
// overlay rendering and subgrid navigation, and configures the grid router.
func (h *Handler) initializeGridManager(gridInstance *domainGrid.Grid) {
	const defaultGridCharacters = "asdfghjkl"

	// Defensive check for grid instance
	if gridInstance == nil {
		h.logger.Warn("Grid instance is nil, creating with default bounds")

		screenBounds := bridge.ActiveScreenBounds()
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
		h.config.Grid.ResetKey,
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
		},
		// Subgrid callback: moves cursor and shows subgrid overlay
		func(cell *domainGrid.Cell) {
			// Defensive check for cell
			if cell == nil {
				h.logger.Warn("Attempted to show subgrid for nil cell")

				return
			}

			// Move mouse to center of cell before showing subgrid for better UX
			ctx := context.Background()

			// Convert cell center from window-local to screen-absolute coordinates
			absoluteCenter := coordinates.ConvertToAbsoluteCoordinates(
				cell.Center(),
				h.screenBounds,
			)

			moveCursorErr := h.actionService.MoveCursorToPoint(ctx, absoluteCenter)
			if moveCursorErr != nil {
				h.logger.Error("Failed to move cursor", zap.Error(moveCursorErr))
			}

			// Draw 3x3 subgrid inside selected cell
			h.renderer.ShowSubgrid(cell)
		},
		h.logger,
	)
}
