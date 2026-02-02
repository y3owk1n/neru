package modes

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/app/components"
	componentquadgrid "github.com/y3owk1n/neru/internal/app/components/quadgrid"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/quadgrid"
	"github.com/y3owk1n/neru/internal/core/infra/bridge"
	"github.com/y3owk1n/neru/internal/ui/coordinates"
	"go.uber.org/zap"
)

// activateQuadGridModeWithAction activates quad-grid mode with optional action parameter.
func (h *Handler) activateQuadGridModeWithAction(actionStr *string) {
	actionEnum, ok := h.activateModeBase(
		domain.ModeNameQuadGrid,
		h.config.QuadGrid.Enabled,
		action.TypeMoveMouse,
	)
	if !ok {
		return
	}

	actionString := domain.ActionString(actionEnum)

	h.ExitMode()
	h.overlayManager.Clear()

	// Get screen bounds
	screenBounds := bridge.ActiveScreenBounds()
	h.screenBounds = screenBounds
	normalizedBounds := coordinates.NormalizeToLocalCoordinates(screenBounds)

	// Initialize quad-grid manager
	h.initializeQuadGridManager(normalizedBounds)

	// Move cursor to center of initial grid
	if h.quadGrid.Manager != nil {
		center := h.quadGrid.Manager.CurrentGrid().CurrentCenter()
		absoluteCenter := coordinates.ConvertToAbsoluteCoordinates(center, h.screenBounds)

		err := h.actionService.MoveCursorToPoint(context.Background(), absoluteCenter)
		if err != nil {
			h.logger.Warn("Failed to move cursor to initial center", zap.Error(err))
		}
	}

	// Draw initial quad-grid
	h.updateQuadGridOverlay()

	h.overlayManager.Show()

	// Store pending action if provided
	if h.quadGrid.Context != nil {
		h.quadGrid.Context.SetPendingAction(actionStr)
	}

	if actionStr != nil {
		h.logger.Info(
			"Quad-grid mode activated with pending action",
			zap.String("action", *actionStr),
		)
	}

	h.SetModeQuadGrid()

	h.logger.Info("Quad-grid mode activated", zap.String("action", actionString))
	h.logger.Info("Press u/i/j/k to select quadrants, backspace to backtrack, escape to exit")
}

// initializeQuadGridManager initializes the quad-grid manager.
func (h *Handler) initializeQuadGridManager(screenBounds image.Rectangle) {
	exitKeys := h.config.General.ModeExitKeys
	if len(exitKeys) == 0 {
		exitKeys = DefaultModeExitKeys()
	}

	// Ensure quadGrid component is initialized
	if h.quadGrid == nil {
		h.quadGrid = &components.QuadGridComponent{
			Context: &componentquadgrid.Context{},
			Style:   componentquadgrid.BuildStyle(h.config.QuadGrid),
		}
	}

	h.quadGrid.Manager = quadgrid.NewManagerWithConfig(
		screenBounds,
		h.config.QuadGrid.Keys,
		h.config.QuadGrid.ResetKey,
		exitKeys,
		h.config.QuadGrid.MinSize,
		h.config.QuadGrid.MaxDepth,
		// Update callback
		func() {
			h.updateQuadGridOverlay()
		},
		// Complete callback
		func(point image.Point) {
			h.logger.Info("Quad-grid selection complete",
				zap.Int("x", point.X),
				zap.Int("y", point.Y))
		},
		h.logger,
	)
}

// handleQuadGridKey handles key processing for quad-grid mode.
func (h *Handler) handleQuadGridKey(key string) {
	ctx := context.Background()

	// Handle direct action keys first
	if h.actionService.IsDirectActionKey(key) {
		_, err := h.actionService.HandleDirectActionKey(ctx, key)
		if err != nil {
			h.logger.Error("Failed to handle direct action key", zap.Error(err))
		}

		return
	}

	if h.quadGrid.Manager == nil {
		h.logger.Warn("Quad-grid manager is nil - ignoring key press")

		return
	}

	// Process the key through the manager
	center, completed, shouldExit := h.quadGrid.Manager.HandleInput(key)

	if shouldExit {
		h.ExitMode()

		return
	}

	if completed {
		// Selection is complete - move cursor and execute pending action if any
		absoluteCenter := coordinates.ConvertToAbsoluteCoordinates(center, h.screenBounds)

		h.moveCursorAndHandleAction(
			absoluteCenter,
			h.quadGrid.Context.PendingAction(),
			false, // Quad-grid mode doesn't re-activate after cursor movement
			nil,
		)
	} else if !center.Eq(image.Point{}) {
		// Move cursor to the center point for preview
		absoluteCenter := coordinates.ConvertToAbsoluteCoordinates(center, h.screenBounds)

		moveErr := h.actionService.MoveCursorToPoint(ctx, absoluteCenter)
		if moveErr != nil {
			h.logger.Error("Failed to move cursor", zap.Error(moveErr))
		}
	}
}

// updateQuadGridOverlay refreshes the visual overlay.
func (h *Handler) updateQuadGridOverlay() {
	if h.quadGrid == nil || h.quadGrid.Manager == nil {
		return
	}

	manager := h.quadGrid.Manager

	err := h.renderer.DrawQuadGrid(
		manager.CurrentBounds(),
		manager.CurrentDepth(),
		manager.Keys(),
	)
	if err != nil {
		h.logger.Debug("Failed to draw quad-grid overlay", zap.Error(err))
	}
}

// cleanupQuadGridMode handles cleanup for quad-grid mode.
func (h *Handler) cleanupQuadGridMode() {
	if h.quadGrid != nil && h.quadGrid.Manager != nil {
		h.quadGrid.Manager.Reset()
	}

	h.clearAndHideOverlay()
}
