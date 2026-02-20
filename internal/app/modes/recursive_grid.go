package modes

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/app/components"
	componentrecursivegrid "github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/recursivegrid"
	"github.com/y3owk1n/neru/internal/core/infra/bridge"
	"github.com/y3owk1n/neru/internal/ui/coordinates"
	"go.uber.org/zap"
)

// activateRecursiveGridModeWithAction activates recursive-grid mode with optional action parameter.
func (h *Handler) activateRecursiveGridModeWithAction(actionStr *string) {
	actionEnum, ok := h.activateModeBase(
		domain.ModeNameRecursiveGrid,
		h.config.RecursiveGrid.Enabled,
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

	// Initialize recursive-grid manager
	h.initializeRecursiveGridManager(normalizedBounds)

	// Move cursor to center of initial grid
	if h.recursiveGrid.Manager != nil {
		center := h.recursiveGrid.Manager.CurrentGrid().CurrentCenter()
		absoluteCenter := coordinates.ConvertToAbsoluteCoordinates(center, h.screenBounds)

		err := h.actionService.MoveCursorToPoint(context.Background(), absoluteCenter)
		if err != nil {
			h.logger.Warn("Failed to move cursor to initial center", zap.Error(err))
		}
	}

	// Draw initial recursive-grid
	h.updateRecursiveGridOverlay()

	h.overlayManager.Show()

	// Store pending action if provided
	if h.recursiveGrid.Context != nil {
		h.recursiveGrid.Context.SetPendingAction(actionStr)
	}

	if actionStr != nil {
		h.logger.Info(
			"Recursive-grid mode activated with pending action",
			zap.String("action", *actionStr),
		)
	}

	h.SetModeRecursiveGrid()

	h.logger.Info("Recursive-grid mode activated", zap.String("action", actionString))
	h.logger.Info("Press u/i/j/k to select cells, backspace to backtrack, escape to exit")
}

// initializeRecursiveGridManager initializes the recursive-grid manager.
func (h *Handler) initializeRecursiveGridManager(screenBounds image.Rectangle) {
	exitKeys := h.config.General.ModeExitKeys
	if len(exitKeys) == 0 {
		exitKeys = DefaultModeExitKeys()
	}

	// Ensure recursiveGrid component is initialized
	if h.recursiveGrid == nil {
		h.recursiveGrid = &components.RecursiveGridComponent{
			Context: &componentrecursivegrid.Context{},
			Style:   componentrecursivegrid.BuildStyle(h.config.RecursiveGrid),
		}
	}

	h.recursiveGrid.Manager = recursivegrid.NewManagerWithConfig(
		screenBounds,
		h.config.RecursiveGrid.Keys,
		h.config.RecursiveGrid.ResetKey,
		exitKeys,
		h.config.RecursiveGrid.MinSize,
		h.config.RecursiveGrid.MaxDepth,
		h.config.RecursiveGrid.GridCols,
		h.config.RecursiveGrid.GridRows,
		// Update callback
		func() {
			h.updateRecursiveGridOverlay()
		},
		// Complete callback
		func(point image.Point) {
			h.logger.Info("Recursive-grid selection complete",
				zap.Int("x", point.X),
				zap.Int("y", point.Y))
		},
		h.logger,
	)
}

// handleRecursiveGridKey handles key processing for recursive-grid mode.
func (h *Handler) handleRecursiveGridKey(key string) {
	ctx := context.Background()

	// Handle direct action keys first
	if h.actionService.IsDirectActionKey(key) {
		_, err := h.actionService.HandleDirectActionKey(ctx, key)
		if err != nil {
			h.logger.Error("Failed to handle direct action key", zap.Error(err))
		}

		return
	}

	if h.recursiveGrid == nil || h.recursiveGrid.Manager == nil {
		h.logger.Warn("Recursive-grid manager is nil - ignoring key press")

		return
	}

	// Process the key through the manager
	center, completed, shouldExit := h.recursiveGrid.Manager.HandleInput(key)

	if shouldExit {
		h.ExitMode()

		return
	}

	if completed {
		// Selection is complete - move cursor and execute pending action if any
		absoluteCenter := coordinates.ConvertToAbsoluteCoordinates(center, h.screenBounds)

		h.moveCursorAndHandleAction(
			absoluteCenter,
			h.recursiveGrid.Context.PendingAction(),
			false, // Recursive-grid mode doesn't re-activate after cursor movement
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

// updateRecursiveGridOverlay refreshes the visual overlay.
func (h *Handler) updateRecursiveGridOverlay() {
	if h.recursiveGrid == nil || h.recursiveGrid.Manager == nil {
		return
	}

	manager := h.recursiveGrid.Manager

	err := h.renderer.DrawRecursiveGrid(
		manager.CurrentBounds(),
		manager.CurrentDepth(),
		manager.Keys(),
		manager.GridCols(),
		manager.GridRows(),
	)
	if err != nil {
		h.logger.Debug("Failed to draw recursive-grid overlay", zap.Error(err))
	}
}

// cleanupRecursiveGridMode handles cleanup for recursive-grid mode.
func (h *Handler) cleanupRecursiveGridMode() {
	if h.recursiveGrid != nil && h.recursiveGrid.Manager != nil {
		h.recursiveGrid.Manager.Reset()
	}

	h.clearAndHideOverlay()
}
