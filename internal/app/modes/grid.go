package modes

import (
	"context"
	"image"
	"strings"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/features/grid"
	"github.com/y3owk1n/neru/internal/infra/bridge"
	"github.com/y3owk1n/neru/internal/ui/coordinates"
	"go.uber.org/zap"
)

// activateGridModeWithAction activates grid mode with optional action parameter.
func (h *Handler) activateGridModeWithAction(action *string) {
	// Validate mode activation
	err := h.validateModeActivation("grid", h.Config.Grid.Enabled)
	if err != nil {
		h.Logger.Warn("Grid mode activation failed", zap.Error(err))
		return
	}

	// Prepare for mode activation (reset scroll, capture cursor)
	h.prepareForModeActivation()

	actionEnum := domain.ActionMoveMouse
	actionString := domain.GetActionString(actionEnum)
	h.Logger.Info("Activating grid mode", zap.String("action", actionString))

	h.ExitMode()

	// Always resize overlay to the active screen (where mouse is) before drawing grid.
	// This ensures proper positioning when switching between multiple displays.
	h.OverlayManager.ResizeToActiveScreenSync()
	h.State.SetGridOverlayNeedsRefresh(false)

	// Use GridService to show grid
	// Note: We still need to initialize the grid manager for input handling
	// This is a hybrid approach until we fully migrate the grid logic

	// 1. Initialize legacy grid manager (needed for input handling)
	gridInstance := h.createGridInstance()
	h.updateGridOverlayConfig()

	// Reset the grid manager state when setting up the grid
	if h.Grid.Manager != nil {
		h.Grid.Manager.Reset()
	}

	h.initializeGridManager(gridInstance)
	h.Grid.Router = grid.NewRouter(h.Grid.Manager, h.Logger)

	// 2. Show grid using new service
	ctx := context.Background() // TODO: Use proper context
	// We use the grid instance bounds to determine rows/cols if needed,
	// but for now we just show the overlay and let the legacy manager handle drawing
	// via the overlay adapter's SwitchTo("grid") call

	// The adapter's ShowGrid implementation switches mode to "grid"
	// The actual drawing is handled by the legacy overlay which is already set up
	// via h.Renderer.DrawGrid in the legacy code, but here we use the service

	// Wait, the service calls adapter.ShowGrid which calls manager.SwitchTo("grid")
	// But we still need to populate the grid overlay with data

	// Let's call the legacy draw first to populate the overlay
	initErr := h.Renderer.DrawGrid(gridInstance, "")
	if initErr != nil {
		h.Logger.Error("Failed to draw grid", zap.Error(initErr))
		return
	}

	// Then use the service to show it (this is redundant but correct for migration)
	err = h.GridService.ShowGrid(ctx, 0, 0) // Rows/cols ignored by adapter for now
	if err != nil {
		h.Logger.Error("Failed to show grid",
			zap.Error(err),
			zap.String("action", actionString))
		return
	}

	// Store pending action if provided
	h.Grid.Context.SetPendingAction(action)
	if action != nil {
		h.Logger.Info("Grid mode activated with pending action", zap.String("action", *action))
	}

	h.SetModeGrid()

	h.Logger.Info("Grid mode activated", zap.String("action", actionString))
	h.Logger.Info("Type a grid label to select a location")
}

// SetupGrid is deprecated and replaced by GridService.ShowGrid logic
func (h *Handler) SetupGrid() error {
	return nil
}

// createGridInstance creates a new grid instance with proper bounds and characters.
func (h *Handler) createGridInstance() *grid.Grid {
	screenBounds := bridge.GetActiveScreenBounds()

	// Normalize bounds to window-local coordinates using helper function
	bounds := coordinates.NormalizeToLocalCoordinates(screenBounds)

	characters := h.Config.Grid.Characters
	if strings.TrimSpace(characters) == "" {
		characters = h.Config.Hints.HintCharacters
	}
	gridInstance := grid.NewGrid(characters, bounds, h.Logger)
	h.Grid.Context.SetGridInstanceValue(gridInstance)

	return gridInstance
}

// updateGridOverlayConfig updates the grid overlay configuration.
func (h *Handler) updateGridOverlayConfig() {
	(*h.Grid.Context.GridOverlay).UpdateConfig(h.Config.Grid)
}

// initializeGridManager initializes the grid manager with the new grid instance.
func (h *Handler) initializeGridManager(gridInstance *grid.Grid) {
	const defaultGridCharacters = "asdfghjkl"

	// Defensive check for grid instance
	if gridInstance == nil {
		h.Logger.Warn("Grid instance is nil, creating with default bounds")
		screenBounds := bridge.GetActiveScreenBounds()
		bounds := image.Rect(0, 0, screenBounds.Dx(), screenBounds.Dy())
		gridInstance = grid.NewGrid(h.Config.Grid.Characters, bounds, h.Logger)
	}

	// Subgrid configuration and keys (fallback to grid characters): always 3x3
	keys := strings.TrimSpace(h.Config.Grid.SublayerKeys)
	if keys == "" {
		keys = h.Config.Grid.Characters
	}

	// Ensure we have valid keys for subgrid
	if keys == "" {
		h.Logger.Warn("No subgrid keys configured, using grid characters as fallback")
		keys = h.Config.Grid.Characters
	}

	// Final fallback
	if keys == "" {
		keys = defaultGridCharacters
		h.Logger.Warn("No characters available for subgrid, using default")
	}

	const subRows = 3
	const subCols = 3

	h.Grid.Manager = grid.NewManager(
		gridInstance,
		subRows,
		subCols,
		keys,
		func(forceRedraw bool) {
			// Defensive check for grid manager
			if h.Grid.Manager == nil {
				h.Logger.Error("Grid manager is nil during update callback")
				return
			}

			input := h.Grid.Manager.GetInput()

			// special case to handle only when exiting subgrid
			if forceRedraw {
				h.Renderer.Clear()
				gridErr := h.Renderer.DrawGrid(gridInstance, input)
				if gridErr != nil {
					h.Logger.Error("Failed to redraw grid", zap.Error(gridErr))
					return
				}
				h.Renderer.Show()
			}

			// Set hideUnmatched based on whether we have input and the config setting
			hideUnmatched := h.Config.Grid.HideUnmatched && len(input) > 0
			h.Renderer.SetHideUnmatched(hideUnmatched)
			h.Renderer.UpdateGridMatches(input)
		},
		func(cell *grid.Cell) {
			// Defensive check for cell
			if cell == nil {
				h.Logger.Warn("Attempted to show subgrid for nil cell")
				return
			}

			// Move mouse to center of cell before showing subgrid
			ctx := context.Background()
			if err := h.ActionService.MoveCursorToPoint(ctx, cell.Center); err != nil {
				h.Logger.Error("Failed to move cursor", zap.Error(err))
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
