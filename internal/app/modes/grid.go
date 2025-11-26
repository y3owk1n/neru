package modes

import (
	"context"
	"image"
	"strings"

	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/infra/bridge"
	"github.com/y3owk1n/neru/internal/ui/coordinates"
	"go.uber.org/zap"
)

// activateGridModeWithAction activates grid mode with optional action parameter.
func (h *Handler) activateGridModeWithAction(action *string) {
	// Validate mode activation
	modeActivationErr := h.validateModeActivation("grid", h.Config.Grid.Enabled)
	if modeActivationErr != nil {
		h.Logger.Warn("Grid mode activation failed", zap.Error(modeActivationErr))

		return
	}

	// Prepare for mode activation (reset scroll, capture cursor)
	h.prepareForModeActivation()

	actionEnum := domain.ActionMoveMouse
	actionString := domain.ActionString(actionEnum)
	h.Logger.Info("Activating grid mode", zap.String("action", actionString))

	h.ExitMode()

	// Always resize overlay to the active screen (where mouse is) before drawing grid.
	// This ensures proper positioning when switching between multiple displays.
	h.OverlayManager.ResizeToActiveScreenSync()
	h.AppState.SetGridOverlayNeedsRefresh(false)

	gridInstance := h.createGridInstance()
	h.updateGridOverlayConfig()

	// Reset the grid manager state when setting up the grid
	if h.Grid.Manager != nil {
		h.Grid.Manager.Reset()
	}

	h.initializeGridManager(gridInstance)
	h.Grid.Router = domainGrid.NewRouter(h.Grid.Manager, h.Logger)

	// Draw the grid to populate the overlay
	drawGridErr := h.Renderer.DrawGrid(gridInstance, "")
	if drawGridErr != nil {
		h.Logger.Error("Failed to draw grid", zap.Error(drawGridErr))

		return
	}

	// Show the overlay (the grid is already drawn with proper style)
	h.OverlayManager.Show()

	// Store pending action if provided
	h.Grid.Context.SetPendingAction(action)

	if action != nil {
		h.Logger.Info("Grid mode activated with pending action", zap.String("action", *action))
	}

	h.SetModeGrid()

	h.Logger.Info("Grid mode activated", zap.String("action", actionString))
	h.Logger.Info("Type a grid label to select a location")
}

// createGridInstance creates a new grid instance with proper bounds and characters.
func (h *Handler) createGridInstance() *domainGrid.Grid {
	screenBounds := bridge.ActiveScreenBounds()

	// Normalize normalizedBounds to window-local coordinates using helper function
	normalizedBounds := coordinates.NormalizeToLocalCoordinates(screenBounds)

	characters := h.Config.Grid.Characters
	if strings.TrimSpace(characters) == "" {
		characters = h.Config.Hints.HintCharacters
	}

	gridInstance := domainGrid.NewGrid(characters, normalizedBounds, h.Logger)
	h.Grid.Context.SetGridInstanceValue(gridInstance)

	return gridInstance
}

// updateGridOverlayConfig updates the grid overlay configuration.
func (h *Handler) updateGridOverlayConfig() {
	(*h.Grid.Context.GridOverlay()).SetConfig(h.Config.Grid)
}

// initializeGridManager initializes the grid manager with the new grid instance.
// It sets up subgrid configuration, creates the manager with update callbacks for
// overlay rendering and subgrid navigation, and configures the grid router.
func (h *Handler) initializeGridManager(gridInstance *domainGrid.Grid) {
	const defaultGridCharacters = "asdfghjkl"

	// Defensive check for grid instance
	if gridInstance == nil {
		h.Logger.Warn("Grid instance is nil, creating with default bounds")

		screenBounds := bridge.ActiveScreenBounds()
		bounds := image.Rect(0, 0, screenBounds.Dx(), screenBounds.Dy())
		gridInstance = domainGrid.NewGrid(h.Config.Grid.Characters, bounds, h.Logger)
	}

	// Configure subgrid keys for 3x3 subgrid navigation within selected cells
	keys := strings.TrimSpace(h.Config.Grid.SublayerKeys)
	if keys == "" {
		keys = h.Config.Grid.Characters
	}

	// Ensure we have valid keys for subgrid
	if keys == "" {
		h.Logger.Warn("No subgrid keys configured, using grid characters as fallback")
		keys = h.Config.Grid.Characters
	}

	// Final fallback to default characters if none configured
	if keys == "" {
		keys = defaultGridCharacters

		h.Logger.Warn("No characters available for subgrid, using default")
	}

	const (
		subRows = 3
		subCols = 3
	)

	// Create grid manager with callbacks for overlay updates and subgrid navigation
	h.Grid.Manager = domainGrid.NewManager(
		gridInstance,
		subRows,
		subCols,
		keys,
		// Update callback: handles grid redrawing and match filtering
		func(forceRedraw bool) {
			// Defensive check for grid manager
			if h.Grid.Manager == nil {
				h.Logger.Error("Grid manager is nil during update callback")

				return
			}

			input := h.Grid.Manager.CurrentInput()

			// Force redraw only when exiting subgrid to restore main grid
			if forceRedraw {
				h.OverlayManager.Clear()

				gridErr := h.Renderer.DrawGrid(gridInstance, input)
				if gridErr != nil {
					h.Logger.Error("Failed to redraw grid", zap.Error(gridErr))

					return
				}

				h.OverlayManager.Show()
			}

			// Hide unmatched cells if configured and input exists
			hideUnmatched := h.Config.Grid.HideUnmatched && len(input) > 0
			h.Renderer.SetHideUnmatched(hideUnmatched)
			h.Renderer.UpdateGridMatches(input)
		},
		// Subgrid callback: moves cursor and shows subgrid overlay
		func(cell *domainGrid.Cell) {
			// Defensive check for cell
			if cell == nil {
				h.Logger.Warn("Attempted to show subgrid for nil cell")

				return
			}

			// Move mouse to center of cell before showing subgrid for better UX
			ctx := context.Background()

			moveCursorErr := h.ActionService.MoveCursorToPoint(ctx, cell.Center())
			if moveCursorErr != nil {
				h.Logger.Error("Failed to move cursor", zap.Error(moveCursorErr))
			}

			// Draw 3x3 subgrid inside selected cell
			h.Renderer.ShowSubgrid(cell)
		},
		h.Logger,
	)
}

// handleGridActionKey handles action keys when in grid action mode.
func (h *Handler) handleGridActionKey(key string) {
	h.handleActionKey(key, "Grid")
}
