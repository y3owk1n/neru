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
func (h *Handler) activateRecursiveGridModeWithAction(actionStr *string, repeat bool) {
	// Detect refresh before validation so we can do partial cleanup on re-activation.
	isRefresh := h.appState.CurrentMode() == domain.ModeRecursiveGrid

	actionEnum, ok := h.activateModeBase(
		domain.ModeNameRecursiveGrid,
		h.config.RecursiveGrid.Enabled,
		action.TypeMoveMouse,
	)
	if !ok {
		if isRefresh {
			h.exitModeLocked()
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
		h.exitModeLocked()
	}

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

	// Store pending action and repeat flag if provided
	if h.recursiveGrid.Context != nil {
		h.recursiveGrid.Context.SetPendingAction(actionStr)
		h.recursiveGrid.Context.SetRepeat(repeat)
	}

	if actionStr != nil {
		h.logger.Info(
			"Recursive-grid mode activated with pending action",
			zap.String("action", *actionStr),
			zap.Bool("repeat", repeat),
		)
	}

	// Only set mode and enable event tap on initial activation;
	// during refresh these are already in the correct state.
	if !isRefresh {
		h.setModeLocked(domain.ModeRecursiveGrid, overlay.ModeRecursiveGrid)
	}

	h.logger.Info("Recursive-grid mode activated", zap.String("action", actionString))
	h.logger.Info("Press u/i/j/k to select cells, backspace to backtrack, escape to exit")

	h.startIndicatorPolling(domain.ModeRecursiveGrid)
}

// initializeRecursiveGridManager initializes the recursive-grid manager.
func (h *Handler) initializeRecursiveGridManager(screenBounds image.Rectangle) {
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

	if h.recursiveGrid == nil || h.recursiveGrid.Manager == nil {
		h.logger.Warn("Recursive-grid manager is nil - ignoring key press")

		return
	}

	// Process the key through the manager
	center, completed := h.recursiveGrid.Manager.HandleInput(key)

	if completed {
		// Selection is complete - move cursor and execute pending action if any
		absoluteCenter := coordinates.ConvertToAbsoluteCoordinates(center, h.screenBounds)

		repeat := h.recursiveGrid.Context.Repeat()
		pendingAction := h.recursiveGrid.Context.PendingAction()

		h.moveCursorAndHandleAction(
			absoluteCenter,
			pendingAction,
			repeat, // Re-activate recursive-grid mode when --repeat is set
			func() { h.activateRecursiveGridModeWithAction(pendingAction, repeat) },
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
	if h.recursiveGrid != nil {
		h.recursiveGrid.Context.Reset()

		if h.recursiveGrid.Manager != nil {
			h.recursiveGrid.Manager.Reset()
		}
	}

	h.clearAndHideOverlay()
}
