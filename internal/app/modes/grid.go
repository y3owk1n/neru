package modes

import (
	"image"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/geometry"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
	"github.com/y3owk1n/neru/internal/ports"
)

// activateGridModeWithAction activates grid mode with the flags the activation
// carries.
func (h *handlerState) activateGridModeWithAction(activation modecmd.Activation) {
	// Detect refresh before validation so we can do partial cleanup on re-activation.
	isRefresh := h.appState.CurrentMode() == domain.ModeGrid

	actionEnum, activated := h.activateModeBase(
		domain.ModeNameGrid,
		h.config.Grid.Enabled,
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
		// The frame shown below replaces the overlay in place.
		h.stopIndicatorPolling()
	} else {
		// Coming from another mode, the keyboard is handed over rather than
		// given back. Exit now, release at return if nothing is entered.
		defer h.exitModeForTransition()()
	}

	scope := h.resolveCaptureScope(domain.ModeNameGrid, activation, &h.config.Grid)
	gridInstance := h.createGridInstance(scope)

	// Reset fires the old manager's update callback, which clears hide-unmatched
	// and the match prefix. A refresh skips ClearFrame, so without it a refresh
	// with hide_unmatched on and partial input hides every cell on macOS.
	if h.grid.Manager != nil {
		h.grid.Manager.Reset()
	}

	h.initializeGridManager(gridInstance)

	// The router is rebuilt each activation so it reads fresh exit keys from
	// the config.
	h.grid.Router = domainGrid.NewRouter(h.grid.Manager, h.logger)

	// Hand the grid over as a Frame. Showing the overlay, switching it to grid
	// mode and drawing the cells is one step the overlay owns, and a previous
	// mode's content (scroll highlights, a subgrid) goes with it.
	if !h.showFrame(ports.GridFrame{Grid: gridInstance}, "show grid overlay") {
		if isRefresh {
			h.exitMode()
		} else {
			// Nothing is entering grid mode, so nothing would take the
			// half-shown overlay off the screen later.
			h.clearOverlayFrame()
		}

		return
	}

	applyGridFlags(h.grid.Context, activation, isRefresh)
	h.grid.Context.SetCaptureScope(scope)

	h.grid.Context.ClearSelectionPoint()
	h.refreshGridVirtualPointer()

	// Only set the mode and enable the event tap on the first activation.
	// During a refresh these are already in the correct state. The overlay was
	// switched to grid mode when the Frame was realized.
	if !isRefresh {
		h.enterMode(domain.ModeGrid)
	}

	h.logger.Info("Grid mode activated", activationLogFields(actionString, scope, activation)...)

	h.startIndicatorPolling(domain.ModeGrid)
}

// gridOptionsFor is the configured grid options with the display scale of
// screenBounds, so every grid built for that screen plans in apparent units.
func (h *handlerState) gridOptionsFor(screenBounds image.Rectangle) domainGrid.Options {
	options := h.config.GridOptions()
	options.DisplayScale = h.screenScale(screenBounds)

	return options
}

// createGridInstance creates a new grid over the region scope names, in the
// active screen's own space, with the configured characters.
func (h *handlerState) createGridInstance(scope string) *domainGrid.Grid {
	var screenBounds image.Rectangle

	if h.system != nil {
		b, err := h.system.ScreenBounds(h.ctx)
		if err == nil {
			screenBounds = b
		} else if !derrors.IsNotSupported(err) {
			h.logger.Warn("Failed to get screen bounds for grid", zap.Error(err))
		}
	}

	h.setScreenBounds(screenBounds)

	// The region is global like the screen. The overlay draws in the
	// screen's own space.
	region := h.captureStart(domain.ModeNameGrid, screenBounds, scope).Sub(screenBounds.Min)

	gridInstance := domainGrid.NewGridWithOptions(
		h.gridOptionsFor(screenBounds),
		region,
		h.logger,
	)
	h.grid.Context.SetGridInstanceValue(gridInstance)

	return gridInstance
}

// initializeGridManager builds the grid manager for gridInstance, with the
// update callback that redraws and narrows the grid and the subgrid callback
// that opens a cell.
func (h *handlerState) initializeGridManager(gridInstance *domainGrid.Grid) {
	h.grid.Manager = domainGrid.NewManager(
		gridInstance,
		domain.SubgridDimensions(),
		// The keys the subgrid is drawn with, resolved once by the config
		// (config.ResolveSublayerKeys) so that the set accepted here is the set
		// the overlay draws.
		h.config.Grid.SublayerKeys,
		// Update callback: handles grid redrawing and match filtering
		func(forceRedraw bool) {
			if h.grid.Manager == nil {
				h.logger.Error("Grid manager is nil during update callback")

				return
			}

			input := h.grid.Manager.CurrentInput()

			// Force redraw only when exiting subgrid to restore main grid.
			// The overlay is already up, so this is a redraw and not a
			// transition. Erasing the subgrid it replaces is the overlay's
			// business, and the overlay is the one that knows a subgrid is there.
			if forceRedraw {
				h.redrawFrame(
					ports.GridFrame{Grid: gridInstance, Input: input},
					"restore grid overlay",
				)
			}

			// Narrowing is the per-keystroke path and stays a plain call, so no
			// frame is built and nothing is redrawn (ADR 0003).
			hideUnmatched := h.config.Grid.HideUnmatched && len(input) > 0
			h.setGridHideUnmatched(hideUnmatched)
			h.updateGridMatches(input)
			h.refreshGridVirtualPointer()
		},
		// Subgrid callback: moves cursor and shows subgrid overlay
		func(cell *domainGrid.Cell) {
			if cell == nil {
				h.logger.Debug("Subgrid cell is nil, ignoring")

				return
			}

			// Move the cursor to the cell center before showing the subgrid.
			ctx := h.ctx

			// Convert cell center from window-local to screen-absolute coordinates
			absoluteCenter := geometry.ConvertToAbsoluteCoordinates(
				cell.Center(),
				h.screenBounds,
			)

			if h.grid != nil && h.grid.Context != nil {
				h.grid.Context.SetSelectionPoint(absoluteCenter)

				if !h.grid.Context.CursorFollowSelection() {
					// One call, because the surface is one surface. The
					// pointer just moved onto the cell this subgrid opens
					// inside, and saying so separately repaints the subgrid a
					// second time (#1492).
					h.showGridSubgrid(cell, h.gridPointer())

					return
				}
			}

			moveCursorErr := h.actionService.MoveCursorToPoint(ctx, absoluteCenter)
			if moveCursorErr != nil {
				h.logger.Error("Failed to move cursor", zap.Error(moveCursorErr))
			}

			// Draw 3x3 subgrid inside selected cell. The real cursor is on the
			// selection here, so the pointer this carries stands for nothing
			// and is invisible. The open still says so, rather than leaving
			// whatever the surface last held.
			h.showGridSubgrid(cell, h.gridPointer())
		},
		h.logger,
	)
}

// applyGridFlags writes the flags an activation carries into the context. A refresh
// writes only what it was given. A fresh activation writes every field so
// nothing leaks over from the previous run.
func applyGridFlags(ctx *grid.Context, activation modecmd.Activation, isRefresh bool) {
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
	ctx.SetCursorFollowSelection(
		resolveCursorFollowSelection(activation.CursorFollowSelection),
	)
}
