package modes

import (
	"context"
	"image"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	componentrecursivegrid "github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/recursivegrid"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/ui/coordinates"
	"github.com/y3owk1n/neru/internal/ui/overlay"
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

	h.exitModeLocked()
	h.overlayManager.Clear()

	h.appState.SetRecursiveGridOverlayNeedsRefresh(false)

	// Get screen bounds
	var screenBounds image.Rectangle

	if h.system != nil {
		b, err := h.system.ScreenBounds(context.Background())
		if err == nil {
			screenBounds = b
		} else if !derrors.IsNotSupported(err) {
			h.logger.Warn("Failed to get screen bounds for recursive grid", zap.Error(err))
		}
	}

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

	h.setModeLocked(domain.ModeRecursiveGrid, overlay.ModeRecursiveGrid)

	h.logger.Info("Recursive-grid mode activated", zap.String("action", actionString))
	h.logger.Info("Press u/i/j/k to select cells, backspace to backtrack, escape to exit")

	h.startIndicatorPolling(domain.ModeRecursiveGrid)
}

// initializeRecursiveGridManager initializes the recursive-grid manager.
func (h *Handler) initializeRecursiveGridManager(screenBounds image.Rectangle) {
	exitKeys := h.config.ResolvedExitKeys("recursive_grid")

	// Ensure recursiveGrid component is initialized
	if h.recursiveGrid == nil {
		h.recursiveGrid = &components.RecursiveGridComponent{
			Context: &componentrecursivegrid.Context{},
		}
	}

	// Build per-depth layout and key overrides from config layers
	depthLayouts := make(map[int]recursivegrid.DepthLayout, len(h.config.RecursiveGrid.Layers))
	depthKeys := make(map[int]string, len(h.config.RecursiveGrid.Layers))

	for _, layer := range h.config.RecursiveGrid.Layers {
		depthLayouts[layer.Depth] = recursivegrid.DepthLayout{
			GridCols: layer.GridCols,
			GridRows: layer.GridRows,
		}
		depthKeys[layer.Depth] = layer.Keys
	}

	h.recursiveGrid.Manager = recursivegrid.NewManagerWithLayers(
		screenBounds,
		h.config.RecursiveGrid.Keys,
		h.config.RecursiveGrid.ResetKey,
		h.config.RecursiveGrid.BackspaceKey,
		exitKeys,
		h.config.RecursiveGrid.MinSizeWidth,
		h.config.RecursiveGrid.MinSizeHeight,
		h.config.RecursiveGrid.MaxDepth,
		h.config.RecursiveGrid.GridCols,
		h.config.RecursiveGrid.GridRows,
		depthLayouts,
		depthKeys,
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
	actionName, wasHandled, err := h.actionService.HandleDirectActionKey(
		ctx,
		key,
		h.stickyModifiers(),
	)
	if wasHandled {
		if err != nil {
			h.logger.Error("Failed to handle direct action key", zap.Error(err))

			return
		}

		if h.shouldAutoExit(h.config.RecursiveGrid.AutoExitActions, actionName) {
			if !h.actionService.IsMoveMouseKey(key) {
				h.cursorState.MarkActionPerformed()
			}

			h.exitModeLocked()
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
		h.exitModeLocked()

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
	currentDepth := manager.CurrentDepth()

	// For sub-key preview: resolve what the *next* depth's layout and keys
	// will be so each cell shows a preview of what pressing that key will produce.
	// If the grid can no longer be divided (max depth or min size reached),
	// skip the preview entirely — those keys are unreachable.
	var (
		nextKeys           string
		nextCols, nextRows int
	)

	if manager.CanDivide() {
		nextDepth := currentDepth + 1
		nextKeys = manager.KeysForDepth(nextDepth)
		nextLayout := manager.CurrentGrid().LayoutForDepth(nextDepth)
		nextCols = nextLayout.GridCols
		nextRows = nextLayout.GridRows
	}

	err := h.renderer.DrawRecursiveGrid(
		manager.CurrentBounds(),
		currentDepth,
		manager.Keys(),
		manager.GridCols(),
		manager.GridRows(),
		nextKeys,
		nextCols,
		nextRows,
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
