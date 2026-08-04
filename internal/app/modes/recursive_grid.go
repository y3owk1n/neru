package modes

import (
	"image"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	componentrecursivegrid "github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/geometry"
	"github.com/y3owk1n/neru/internal/domain/recursivegrid"
)

// activateRecursiveGridModeWithAction activates recursive-grid mode with optional action parameter
// and optional zoom-to-depth. When zoomToDepth is set, the mode will automatically drill down to
// the specified depth at the current cursor position before awaiting user input.
func (h *handlerState) activateRecursiveGridModeWithAction(opts ModeActivationOptions) {
	// Detect refresh before validation so we can do partial cleanup on re-activation.
	isRefresh := h.appState.CurrentMode() == domain.ModeRecursiveGrid

	actionEnum, activated := h.activateModeBase(
		domain.ModeNameRecursiveGrid,
		h.config.RecursiveGrid.Enabled,
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

	h.overlayManager.Clear()

	h.appState.SetRecursiveGridOverlayNeedsRefresh(false)

	// Get screen bounds
	var screenBounds image.Rectangle

	if h.system != nil {
		b, err := h.system.ScreenBounds(h.ctx)
		if err == nil {
			screenBounds = b
		} else if !derrors.IsNotSupported(err) {
			h.logger.Warn("Failed to get screen bounds for recursive grid", zap.Error(err))
		}
	}

	h.setScreenBounds(screenBounds)
	normalizedBounds := geometry.NormalizeToLocalCoordinates(screenBounds)

	h.initializeRecursiveGridManager(normalizedBounds)

	var cursorShouldFollow bool
	if isRefresh && opts.CursorFollowSelection == nil && h.recursiveGrid.Context != nil {
		cursorShouldFollow = h.recursiveGrid.Context.CursorFollowSelection()
	} else {
		cursorShouldFollow = resolveCursorFollowSelection(
			domain.ModeRecursiveGrid,
			opts.CursorFollowSelection,
		)
	}

	// Auto-zoom to depth if requested.
	// This reads the cursor position *before* we potentially move it to
	// the grid center below, so zoom uses the user's actual cursor location.
	if opts.ZoomToDepth != nil && *opts.ZoomToDepth > 0 && h.recursiveGrid.Manager != nil &&
		!isRefresh {
		cursorPos, posErr := h.actionService.CursorPosition(h.ctx)
		if posErr == nil {
			localCursorPos := geometry.ConvertToLocalCoordinates(cursorPos, h.screenBounds)
			h.recursiveGrid.Manager.ZoomToPoint(localCursorPos, *opts.ZoomToDepth)
		} else {
			h.logger.Warn("Failed to get cursor position for zoom", zap.Error(posErr))
		}
	}

	// Move cursor to center of initial grid.
	// When zoom-to-depth is active, skip this — the zoom completion handler
	// (or partial-zoom handler below) positions the cursor from the user's
	// actual cursor position rather than the grid center.
	isZoomRequested := opts.ZoomToDepth != nil && *opts.ZoomToDepth > 0 && !isRefresh
	if !isZoomRequested {
		h.selectRecursiveGridCenter(cursorShouldFollow, "Failed to move cursor to initial center")
	}

	// Draw initial recursive-grid
	if h.recursiveGrid.Context != nil {
		applyRecursiveGridOptions(h.recursiveGrid.Context, opts, isRefresh, cursorShouldFollow)
	}

	// When zoom-to-depth completed (or clamped), update the selection point
	// to the zoomed position and let the user refine in interactive mode.
	// The pending action fires on the next manual cell selection rather than
	// executing immediately, which is consistent with normal grid behavior.
	if isZoomRequested {
		h.selectRecursiveGridCenter(cursorShouldFollow, "Failed to move cursor after zoom")
	}

	// Draw initial recursive-grid
	h.updateRecursiveGridOverlay()

	h.overlayManager.ResizeToActiveScreen()
	h.overlayManager.Show()

	if opts.Action != nil {
		h.logger.Debug(
			"Recursive-grid mode activated with pending action",
			zap.String("action", *opts.Action),
			zap.Bool("repeat", opts.Repeat != nil && *opts.Repeat),
		)
	}

	// Only set mode and enable event tap on initial activation;
	// during refresh these are already in the correct state.
	if !isRefresh {
		h.setMode(domain.ModeRecursiveGrid, overlay.ModeRecursiveGrid)
	}

	h.logger.Info("Recursive-grid mode activated", zap.String("action", actionString))

	h.startIndicatorPolling(domain.ModeRecursiveGrid)
}

// initializeRecursiveGridManager initializes the recursive-grid manager.
func (h *handlerState) initializeRecursiveGridManager(screenBounds image.Rectangle) {
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
			absoluteCenter := geometry.ConvertToAbsoluteCoordinates(center, h.screenBounds)
			if h.recursiveGrid != nil && h.recursiveGrid.Context != nil {
				h.recursiveGrid.Context.SetSelectionPoint(absoluteCenter)
			}

			h.updateRecursiveGridOverlay()
		},
		// Complete callback
		func(point image.Point) {
			h.logger.Debug("Recursive-grid selection complete",
				zap.Int("x", point.X),
				zap.Int("y", point.Y))
		},
		h.logger,
	)
}

// handleRecursiveGridKey handles key processing for recursive-grid mode.
func (h *handlerState) handleRecursiveGridKey(key string) {
	ctx := h.ctx

	if h.recursiveGrid == nil || h.recursiveGrid.Manager == nil {
		h.logger.Warn("Recursive-grid manager is nil - ignoring key press")

		return
	}

	// Process the key through the manager
	center, completed := h.recursiveGrid.Manager.HandleInput(key)

	if completed {
		// Selection is complete - always remember the final target, but only
		// move immediately when tracking is enabled or an action needs to commit.
		absoluteCenter := geometry.ConvertToAbsoluteCoordinates(center, h.screenBounds)
		h.recursiveGrid.Context.SetSelectionPoint(absoluteCenter)

		repeat := h.recursiveGrid.Context.Repeat()
		pendingAction := h.recursiveGrid.Context.PendingAction()
		pendingModifier := h.recursiveGrid.Context.PendingModifier()
		cursorFollowSelection := h.recursiveGrid.Context.CursorFollowSelection()

		if pendingAction == nil && !repeat && !cursorFollowSelection {
			h.refreshRecursiveGridVirtualPointer()

			return
		}

		h.moveCursorAndHandleAction(
			absoluteCenter,
			pendingAction,
			pendingModifier,
			repeat, // Re-activate recursive-grid mode when --repeat is set
			func() {
				h.activateRecursiveGridModeWithAction(ModeActivationOptions{
					Action:                pendingAction,
					Modifier:              pendingModifier,
					Repeat:                &repeat,
					CursorFollowSelection: &cursorFollowSelection,
					// Zoom is not re-applied on repeat; OnExit stays nil to
					// preserve the stored steps.
				})
			},
		)
	} else if !center.Eq(image.Point{}) {
		// Move cursor to the center point for preview
		absoluteCenter := geometry.ConvertToAbsoluteCoordinates(center, h.screenBounds)
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
func (h *handlerState) updateRecursiveGridOverlay() {
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
		h.currentRecursiveGridVirtualPointerState(),
	)
	if err != nil {
		h.logger.Debug("Failed to draw recursive-grid overlay", zap.Error(err))
	}
}

// cleanupRecursiveGridMode handles cleanup for recursive-grid mode.
func (h *handlerState) cleanupRecursiveGridMode() {
	if h.recursiveGrid != nil {
		h.recursiveGrid.Context.Reset()

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

// applyRecursiveGridOptions writes an activation's options into the context.
// A refresh writes only what it was given; a fresh activation writes every
// field so nothing leaks over from the previous run.
func applyRecursiveGridOptions(
	ctx *componentrecursivegrid.Context,
	opts ModeActivationOptions,
	isRefresh bool,
	cursorShouldFollow bool,
) {
	if isRefresh {
		if opts.Action != nil {
			ctx.SetPendingAction(opts.Action)
		}

		if opts.OnExit != nil {
			ctx.SetOnExit(opts.OnExit)
		}

		if opts.Modifier != nil {
			ctx.SetPendingModifier(opts.Modifier)
		}

		if opts.Repeat != nil {
			ctx.SetRepeat(*opts.Repeat)
		}

		if opts.CursorFollowSelection != nil {
			ctx.SetCursorFollowSelection(*opts.CursorFollowSelection)
		}

		return
	}

	ctx.SetPendingAction(opts.Action)
	ctx.SetOnExit(opts.OnExit)
	ctx.SetPendingModifier(opts.Modifier)
	ctx.SetRepeat(opts.Repeat != nil && *opts.Repeat)
	ctx.SetCursorFollowSelection(cursorShouldFollow)
}

// selectRecursiveGridCenter marks the current grid's center as the selection
// and moves the cursor there when it follows the selection.
func (h *handlerState) selectRecursiveGridCenter(cursorShouldFollow bool, moveFailMsg string) {
	if h.recursiveGrid.Manager == nil {
		return
	}

	center := h.recursiveGrid.Manager.CurrentGrid().CurrentCenter()

	absoluteCenter := geometry.ConvertToAbsoluteCoordinates(center, h.screenBounds)
	if h.recursiveGrid.Context != nil {
		h.recursiveGrid.Context.SetSelectionPoint(absoluteCenter)
	}

	if cursorShouldFollow {
		err := h.actionService.MoveCursorToPoint(h.ctx, absoluteCenter)
		if err != nil {
			h.logger.Warn(moveFailMsg, zap.Error(err))
		}
	}
}
