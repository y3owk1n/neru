package modes

import (
	"context"
	"fmt"
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
func (h *Handler) activateRecursiveGridModeWithAction(
	actionStr *string,
	repeat bool,
	cursorFollowSelection *bool,
	training bool,
) {
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

	if training && !h.config.RecursiveGrid.Training.Enabled {
		h.logger.Warn("Recursive-grid training activation failed",
			zap.String("reason", "training disabled in config"))

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
	if h.renderer != nil {
		// Grid mode uses a shared native overlay flag to hide unmatched cells.
		// Reset it here so recursive-grid and training never inherit that state.
		h.renderer.SetHideUnmatched(false)
	}

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

	cursorShouldFollow := false
	if !training {
		cursorShouldFollow = resolveCursorFollowSelection(
			domain.ModeRecursiveGrid,
			cursorFollowSelection,
		)

		// Move cursor to center of initial grid
		if h.recursiveGrid.Manager != nil {
			center := h.recursiveGrid.Manager.CurrentGrid().CurrentCenter()

			absoluteCenter := coordinates.ConvertToAbsoluteCoordinates(center, h.screenBounds)
			if h.recursiveGrid.Context != nil {
				h.recursiveGrid.Context.SetSelectionPoint(absoluteCenter)
			}

			if cursorShouldFollow {
				err := h.actionService.MoveCursorToPoint(context.Background(), absoluteCenter)
				if err != nil {
					h.logger.Warn("Failed to move cursor to initial center", zap.Error(err))
				}
			}
		}
	}

	// Draw initial recursive-grid
	// Store pending action and repeat flag if provided
	if h.recursiveGrid.Context != nil {
		if training {
			h.recursiveGrid.Context.SetPendingAction(nil)
			h.recursiveGrid.Context.SetRepeat(false)
			h.recursiveGrid.Context.ClearSelectionPoint()
		} else {
			h.recursiveGrid.Context.SetPendingAction(actionStr)
			h.recursiveGrid.Context.SetRepeat(repeat)
		}

		h.recursiveGrid.Context.SetCursorFollowSelection(cursorShouldFollow)
	}
	h.initializeRecursiveGridTrainingSession(training)

	// Draw initial recursive-grid
	h.updateRecursiveGridOverlay()

	h.overlayManager.Show()

	if actionStr != nil {
		h.logger.Info(
			"Recursive-grid mode activated with pending action",
			zap.String("action", *actionStr),
			zap.Bool("repeat", repeat),
		)
	}

	if training {
		learned, total := h.recursiveGridTrainingProgress()
		h.logger.Info("Recursive-grid training activated",
			zap.Int("learned", learned),
			zap.Int("total", total))
	}

	// Only set mode and enable event tap on initial activation;
	// during refresh these are already in the correct state.
	if !isRefresh {
		h.setModeLocked(domain.ModeRecursiveGrid, overlay.ModeRecursiveGrid)
	}

	h.logger.Info("Recursive-grid mode activated", zap.String("action", actionString))
	if training {
		h.logger.Info("Press the highlighted top-level key, then the highlighted second-depth key; space/backspace restarts, escape exits")
	} else {
		h.logger.Info("Press u/i/j/k to select cells, backspace to backtrack, escape to exit")
	}

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
		func(center image.Point) {
			absoluteCenter := coordinates.ConvertToAbsoluteCoordinates(center, h.screenBounds)
			if h.recursiveGrid != nil && h.recursiveGrid.Context != nil {
				h.recursiveGrid.Context.SetSelectionPoint(absoluteCenter)
			}

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

	if h.recursiveGrid.Training != nil && h.recursiveGrid.Training.Active() {
		result := h.recursiveGrid.Training.HandleKey(key)
		switch result {
		case componentrecursivegrid.TrainingResultIgnored:
			return
		case componentrecursivegrid.TrainingResultAdvanceDepth:
			center, completed := h.recursiveGrid.Manager.HandleInput(key)
			if completed {
				h.logger.Warn("Recursive-grid training could not enter second depth; keeping top-level view")
			} else if h.recursiveGrid.Context != nil {
				absoluteCenter := coordinates.ConvertToAbsoluteCoordinates(center, h.screenBounds)
				h.recursiveGrid.Context.SetSelectionPoint(absoluteCenter)
			}
			h.updateRecursiveGridOverlay()

			return
		case componentrecursivegrid.TrainingResultCorrect:
			if h.recursiveGrid.Manager.CurrentDepth() > 0 &&
				h.recursiveGrid.Manager.Backtrack() &&
				h.recursiveGrid.Context != nil {
				absoluteCenter := coordinates.ConvertToAbsoluteCoordinates(
					h.recursiveGrid.Manager.CurrentCenter(),
					h.screenBounds,
				)
				h.recursiveGrid.Context.SetSelectionPoint(absoluteCenter)
			}

			learned, total := h.recursiveGridTrainingProgress()
			h.logger.Info("Recursive-grid training progress",
				zap.Int("learned", learned),
				zap.Int("total", total))
			h.updateRecursiveGridOverlay()

			return
		case componentrecursivegrid.TrainingResultWrong:
			h.updateRecursiveGridOverlay()

			return
		case componentrecursivegrid.TrainingResultCompleted:
			if h.recursiveGrid.Manager.CurrentDepth() > 0 {
				h.recursiveGrid.Manager.Backtrack()
			}

			learned, total := h.recursiveGridTrainingProgress()
			h.updateRecursiveGridOverlay()
			if h.system != nil {
				h.system.ShowNotification(
					"Neru",
					fmt.Sprintf("Recursive-grid training complete (%d/%d)", learned, total),
				)
			}
			h.logger.Info("Recursive-grid training complete",
				zap.Int("learned", learned),
				zap.Int("total", total))
			h.exitModeLocked()

			return
		}
	}

	// Process the key through the manager
	center, completed := h.recursiveGrid.Manager.HandleInput(key)

	if completed {
		// Selection is complete - always remember the final target, but only
		// move immediately when tracking is enabled or an action needs to commit.
		absoluteCenter := coordinates.ConvertToAbsoluteCoordinates(center, h.screenBounds)
		h.recursiveGrid.Context.SetSelectionPoint(absoluteCenter)

		repeat := h.recursiveGrid.Context.Repeat()
		pendingAction := h.recursiveGrid.Context.PendingAction()
		cursorFollowSelection := h.recursiveGrid.Context.CursorFollowSelection()

		if pendingAction == nil && !repeat && !cursorFollowSelection {
			h.refreshRecursiveGridVirtualPointerLocked()

			return
		}

		h.moveCursorAndHandleAction(
			absoluteCenter,
			pendingAction,
			repeat, // Re-activate recursive-grid mode when --repeat is set
			func() {
				h.activateRecursiveGridModeWithAction(
					pendingAction,
					repeat,
					&cursorFollowSelection,
					false,
				)
			},
		)
	} else if !center.Eq(image.Point{}) {
		// Move cursor to the center point for preview
		absoluteCenter := coordinates.ConvertToAbsoluteCoordinates(center, h.screenBounds)
		h.recursiveGrid.Context.SetSelectionPoint(absoluteCenter)

		if !h.recursiveGrid.Context.CursorFollowSelection() {
			return
		}

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

	keys := manager.Keys()
	matchedIndex := -1
	if session := h.recursiveGrid.Training; session != nil && session.Active() {
		keys = session.OverlayKeys()
		matchedIndex = session.TargetIndex()
		nextKeys = ""
		nextCols = 0
		nextRows = 0
	}

	err := h.renderer.DrawRecursiveGrid(
		manager.CurrentBounds(),
		currentDepth,
		keys,
		manager.GridCols(),
		manager.GridRows(),
		nextKeys,
		nextCols,
		nextRows,
		matchedIndex,
		h.currentRecursiveGridVirtualPointerState(),
	)
	if err != nil {
		h.logger.Debug("Failed to draw recursive-grid overlay", zap.Error(err))
	}
}

// cleanupRecursiveGridMode handles cleanup for recursive-grid mode.
func (h *Handler) cleanupRecursiveGridMode() {
	if h.recursiveGrid != nil {
		h.recursiveGrid.Context.Reset()
		h.recursiveGrid.Training = nil

		if h.recursiveGrid.Manager != nil {
			h.recursiveGrid.Manager.Reset()
		}

		// Explicitly hide the virtual pointer before clearing the overlay.
		// NeruClearOverlay also resets cursorIndicatorVisible, but we do this
		// explicitly so the pointer cleanup does not silently depend on the
		// overlay clear implementation.
		if h.recursiveGrid.Overlay != nil {
			h.recursiveGrid.Overlay.HideVirtualPointer()
		}
	}

	h.clearAndHideOverlay()
}

func (h *Handler) initializeRecursiveGridTrainingSession(training bool) {
	if h.recursiveGrid == nil {
		return
	}

	if !training {
		h.recursiveGrid.Training = nil

		return
	}

	secondDepthTrainingActive := h.recursiveGrid.Manager.CanDivide()

	h.recursiveGrid.Training = componentrecursivegrid.NewTrainingSession(
		h.recursiveGrid.Manager.Keys(),
		h.recursiveGrid.Manager.KeysForDepth(1),
		secondDepthTrainingActive,
		h.config.RecursiveGrid.Training.HitsToHide,
		h.config.RecursiveGrid.Training.PenaltyOnError,
	)
}

func (h *Handler) recursiveGridTrainingProgress() (int, int) {
	if h.recursiveGrid == nil || h.recursiveGrid.Training == nil {
		return 0, 0
	}

	return h.recursiveGrid.Training.Progress()
}
