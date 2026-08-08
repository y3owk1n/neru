package modes

import (
	"image"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	componentrecursivegrid "github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/geometry"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
	"github.com/y3owk1n/neru/internal/domain/recursivegrid"
	"github.com/y3owk1n/neru/internal/ports"
)

// activateRecursiveGridModeWithAction activates recursive-grid mode with optional action parameter
// and optional zoom-to-depth. When zoomToDepth is set, the mode will automatically drill down to
// the specified depth at the current cursor position before awaiting user input.
func (h *handlerState) activateRecursiveGridModeWithAction(activation modecmd.Activation) {
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
	if isRefresh && activation.CursorFollowSelection == nil && h.recursiveGrid.Context != nil {
		cursorShouldFollow = h.recursiveGrid.Context.CursorFollowSelection()
	} else {
		cursorShouldFollow = resolveCursorFollowSelection(
			domain.ModeRecursiveGrid,
			activation.CursorFollowSelection,
		)
	}

	// Auto-zoom to depth if requested.
	// This reads the cursor position *before* we potentially move it to
	// the grid center below, so zoom uses the user's actual cursor location.
	// A refresh is excluded: --zoom-to-depth drills down from where the cursor
	// was when the mode was entered, and re-applying it on a repeat
	// re-activation or a screen change would drag the user back down a level
	// they had already climbed out of.
	if activation.ZoomToDepth != nil && *activation.ZoomToDepth > 0 &&
		h.recursiveGrid.Manager != nil &&
		!isRefresh {
		cursorPos, posErr := h.actionService.CursorPosition(h.ctx)
		if posErr == nil {
			localCursorPos := geometry.ConvertToLocalCoordinates(cursorPos, h.screenBounds)
			h.recursiveGrid.Manager.ZoomToPoint(localCursorPos, *activation.ZoomToDepth)
		} else {
			h.logger.Warn("Failed to get cursor position for zoom", zap.Error(posErr))
		}
	}

	// Move cursor to center of initial grid.
	// When zoom-to-depth is active, skip this — the zoom completion handler
	// (or partial-zoom handler below) positions the cursor from the user's
	// actual cursor position rather than the grid center.
	isZoomRequested := activation.ZoomToDepth != nil && *activation.ZoomToDepth > 0 && !isRefresh
	if !isZoomRequested {
		h.selectRecursiveGridCenter(cursorShouldFollow, "Failed to move cursor to initial center")
	}

	// Draw initial recursive-grid
	if h.recursiveGrid.Context != nil {
		applyRecursiveGridFlags(
			h.recursiveGrid.Context,
			activation,
			isRefresh,
			cursorShouldFollow,
		)
	}

	// When zoom-to-depth completed (or clamped), update the selection point
	// to the zoomed position and let the user refine in interactive mode.
	// The pending action fires on the next manual cell selection rather than
	// executing immediately, which is consistent with normal grid behavior.
	if isZoomRequested {
		h.selectRecursiveGridCenter(cursorShouldFollow, "Failed to move cursor after zoom")
	}

	// Hand the first recursive-grid over as a Frame: the overlay comes up,
	// switches to recursive-grid mode and draws the region in one step.
	//
	// A backend with no surface for it logs and the mode carries on, which is
	// what this mode has always done: unlike grid it never abandoned an
	// activation over a draw, and a refactor is not the place to start.
	h.showFrame(h.recursiveGridFrame(), "show recursive-grid overlay")

	if activation.Action != nil {
		h.logger.Debug(
			"Recursive-grid mode activated with pending action",
			zap.String("action", *activation.Action),
			zap.Bool("repeat", activation.Repeat != nil && *activation.Repeat),
		)
	}

	// Only set mode and enable event tap on initial activation;
	// during refresh these are already in the correct state. The overlay was
	// switched to recursive-grid mode when the Frame was realized.
	if !isRefresh {
		h.enterMode(domain.ModeRecursiveGrid)
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
		domain.GridDimensions{
			Rows: h.config.RecursiveGrid.GridRows,
			Cols: h.config.RecursiveGrid.GridCols,
		},
		depthLayouts,
		depthKeys,
		recursivegrid.SelectionCallbacks{
			OnUpdate: func(center image.Point) {
				absoluteCenter := geometry.ConvertToAbsoluteCoordinates(center, h.screenBounds)
				if h.recursiveGrid != nil && h.recursiveGrid.Context != nil {
					h.recursiveGrid.Context.SetSelectionPoint(absoluteCenter)
				}

				h.updateRecursiveGridOverlay()
			},
			OnComplete: func(point image.Point) {
				h.logger.Debug("Recursive-grid selection complete",
					zap.Int("x", point.X),
					zap.Int("y", point.Y))
			},
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
				h.activateRecursiveGridModeWithAction(modecmd.Activation{
					Mode:                  domain.ModeRecursiveGrid,
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

// updateRecursiveGridOverlay redraws the recursive grid on the overlay it is
// already on. Every keystroke in the mode lands here: the surface is repainted
// whole, so the frame it hands over is what a keystroke costs.
func (h *handlerState) updateRecursiveGridOverlay() {
	if h.recursiveGrid == nil || h.recursiveGrid.Manager == nil {
		return
	}

	h.redrawFrame(h.recursiveGridFrame(), "update recursive-grid overlay")
}

// recursiveGridFrame describes what should be on screen for the recursive grid
// as it stands: the region zoomed into, the keys dividing it, a preview of the
// next depth, and the pointer riding the same surface.
//
// Building it in one place is what took ten positional parameters out of the
// app layer; nothing here names a style or a render model.
//
// It is also where the manager's separate row and column counts become the one
// domain.GridDimensions the draw path carries (#1313). It is the last
// conversion before the overlay: from here through the port, the adapter, the
// backends and the cgo helpers the shape travels whole, so writing either pair
// under each other's names here would transpose every cell on every backend and
// nothing further down could tell.
// TestRecursiveGridFrame_NonSquareGridKeepsRowsAndColumnsApart stands over it.
//
// Upstream of here the pair still exists, in recursivegrid.DepthLayout — which
// #1313 left alone deliberately, because a transposition there is caught by the
// non-square Divide tests rather than only on screen.
func (h *handlerState) recursiveGridFrame() ports.RecursiveGridFrame {
	if h.recursiveGrid == nil || h.recursiveGrid.Manager == nil {
		return ports.RecursiveGridFrame{}
	}

	manager := h.recursiveGrid.Manager
	currentDepth := manager.CurrentDepth()

	// For sub-key preview: resolve what the *next* depth's layout and keys
	// will be so each cell shows a preview of what pressing that key will produce.
	// If the grid can no longer be divided (max depth or min size reached),
	// skip the preview entirely — those keys are unreachable.
	var next ports.RecursiveGridLayout

	if manager.CanDivide() {
		nextDepth := currentDepth + 1
		layout := manager.CurrentGrid().LayoutForDepth(nextDepth)
		next = ports.RecursiveGridLayout{
			Keys:       manager.KeysForDepth(nextDepth),
			Dimensions: domain.GridDimensions{Rows: layout.GridRows, Cols: layout.GridCols},
		}
	}

	return ports.RecursiveGridFrame{
		Bounds: manager.CurrentBounds(),
		Depth:  currentDepth,
		Layout: ports.RecursiveGridLayout{
			Keys:       manager.Keys(),
			Dimensions: domain.GridDimensions{Rows: manager.GridRows(), Cols: manager.GridCols()},
		},
		NextLayout: next,
		Pointer:    h.recursiveGridPointer(),
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
		h.updateGridPointer(domain.ModeRecursiveGrid, ports.GridPointer{})
	}

	// Stop the indicator poller before common cleanup takes the frame off the
	// screen: a tick landing after the clear would put an indicator back on it.
	h.stopIndicatorPolling()
}

// applyRecursiveGridFlags writes the flags an activation carries into the context.
// A refresh writes only what it was given; a fresh activation writes every
// field so nothing leaks over from the previous run.
func applyRecursiveGridFlags(
	ctx *componentrecursivegrid.Context,
	activation modecmd.Activation,
	isRefresh bool,
	cursorShouldFollow bool,
) {
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
