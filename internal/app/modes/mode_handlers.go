package modes

import (
	"context"
	"image"
	"time"

	"go.uber.org/zap"

	hintscomponent "github.com/y3owk1n/neru/internal/app/components/hints"
	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/ui/coordinates"
)

// executeActionAtPoint executes a pending action at the given point and exits the mode.
// When repeat is true and reActivateFunc is provided, the mode is re-activated
// instead of exiting after performing the action.
func (h *Handler) executeActionAtPoint(
	action *string,
	point image.Point,
	repeat bool,
	reActivateFunc func(),
) {
	if action == nil {
		h.logger.Warn("executeActionAtPoint called with nil action")

		return
	}

	h.logger.Info("Executing pending action",
		zap.String("action", *action),
		zap.String("modifiers", h.stickyModifiers().String()),
		zap.Bool("repeat", repeat))

	ctx := context.Background()

	performActionErr := h.actionService.PerformActionAtPoint(
		ctx,
		*action,
		point,
		h.stickyModifiers(),
	)
	if performActionErr != nil {
		h.logger.Error("Failed to perform pending action", zap.Error(performActionErr))
	}

	// Signal that a click was just performed so handleCursorRestoration
	// can insert a settling delay before moving the cursor.
	// Skip move-mouse actions — they don't produce clicks that need settling.
	if performActionErr == nil &&
		*action != "move_mouse" &&
		*action != "move_mouse_relative" {
		h.cursorState.MarkActionPerformed()
	}

	if repeat && reActivateFunc != nil {
		// Wait for the target app to finish processing the click before
		// re-activating (which may move the cursor for grid/recursive-grid).
		// This mirrors the settle delay in handleCursorRestoration and
		// prevents slow apps (Electron, web views) from missing clicks.
		if h.cursorState.WasActionPerformed() {
			time.Sleep(postActionSettleDelay)
		}

		h.logger.Info("Re-activating mode after action (--repeat)")
		reActivateFunc()

		return
	}

	h.exitModeLocked()
}

// moveCursorAndHandleAction moves the cursor to a point and executes any pending action.
func (h *Handler) moveCursorAndHandleAction(
	point image.Point,
	pendingAction *string,
	shouldReActivate bool,
	reActivateFunc func(),
) {
	ctx := context.Background()

	moveCursorErr := h.actionService.MoveCursorToPoint(ctx, point)
	if moveCursorErr != nil {
		h.logger.Error("Failed to move cursor", zap.Error(moveCursorErr))
	}

	if pendingAction != nil {
		h.executeActionAtPoint(pendingAction, point, shouldReActivate, reActivateFunc)

		return
	}

	// No pending action - re-activate mode if requested
	if shouldReActivate && reActivateFunc != nil {
		h.logger.Info("Re-activating mode after cursor movement")
		reActivateFunc()
	}
}

// handleHintsModeKey handles key processing for hints mode.
func (h *Handler) handleHintsModeKey(key string) {
	if h.hints != nil && h.hints.Context != nil && h.hints.Context.SearchActive() {
		h.handleSearchInputKey(key)

		return
	}

	// Route hint-specific keys via domain hints router
	if h.hints.Context.Router() == nil {
		h.logger.Warn("Hints router is nil - ignoring key press until hints initialized")

		return
	}

	hintKeyResult := h.hints.Context.Router().RouteKey(key)

	// Hint input processed by router; if exact match, perform action
	if hintKeyResult.ExactHint() != nil {
		hint := hintKeyResult.ExactHint()
		center := hint.Element().Center()

		h.logger.Info("Found element", zap.String("label", hint.Label()))

		pendingAction := h.hints.Context.PendingAction()
		repeat := h.hints.Context.Repeat()
		cursorFollowSelection := h.hints.Context.CursorFollowSelection()
		filterRoles := h.hints.Context.FilterRoles()
		filterTextContains := h.hints.Context.FilterTextContains()

		h.moveCursorAndHandleAction(
			center,
			pendingAction,
			repeat ||
				pendingAction == nil, // re-activate on repeat, or when no action (existing behavior)
			func() {
				h.activateHintModeInternal(
					nil,
					&cursorFollowSelection,
					filterRoles,
					filterTextContains,
				)
				// Restore repeat and action on the fresh context so subsequent
				// selections continue the repeat cycle.
				// Guard: only restore if re-activation succeeded (mode is still hints).
				if repeat && h.appState.CurrentMode() == domain.ModeHints &&
					h.hints != nil && h.hints.Context != nil {
					h.hints.Context.SetPendingAction(pendingAction)
					h.hints.Context.SetRepeat(true)
					h.hints.Context.SetCursorFollowSelection(cursorFollowSelection)
					h.hints.Context.SetFilterRoles(filterRoles)
					h.hints.Context.SetFilterTextContains(filterTextContains)
				}
			},
		)
	}
}

// handleSearchInputKey routes all keys while hint text search is active.
func (h *Handler) handleSearchInputKey(key string) {
	if h.hints == nil || h.hints.Context == nil {
		return
	}

	ctx := h.hints.Context
	normalizedKey := configpkg.NormalizeKeyForComparison(key)

	switch normalizedKey {
	case configpkg.KeyNameEscape:
		h.cancelHintSearch()

		return
	case configpkg.KeyNameReturn:
		h.confirmHintSearch()

		return
	case configpkg.KeyNameDelete:
		query := ctx.SearchQuery()
		if query != "" {
			ctx.SetSearchQuery(query[:len(query)-1])
			h.applyHintSearchFilter()
		}

		return
	}

	if len(key) != 1 {
		return
	}

	ctx.SetSearchQuery(ctx.SearchQuery() + key)
	h.applyHintSearchFilter()
}

func (h *Handler) applyHintSearchFilter() {
	ctx := h.hints.Context
	sourceHints := ctx.SourceHints()
	if sourceHints == nil {
		return
	}

	filteredHints := sourceHints.FilterByText(ctx.SearchQuery())
	ctx.SetVisibleHints(filteredHints)
	h.drawHintSearchInput()
	h.cycleHintIndex = -1
}

func (h *Handler) confirmHintSearch() {
	if h.hints == nil || h.hints.Context == nil {
		return
	}

	h.hints.Context.SetSearchActive(false)
	h.overlayManager.HideHintSearchInput()
	h.cycleHintIndex = -1
}

func (h *Handler) cancelHintSearch() {
	if h.hints == nil || h.hints.Context == nil {
		return
	}

	ctx := h.hints.Context
	ctx.SetSearchQuery("")
	ctx.SetSearchActive(false)

	if sourceHints := ctx.SourceHints(); sourceHints != nil {
		ctx.SetVisibleHints(sourceHints)
	}

	h.overlayManager.HideHintSearchInput()
	h.cycleHintIndex = -1
}

func (h *Handler) drawHintSearchInput() {
	if h.hints == nil || h.hints.Context == nil {
		return
	}

	ctx := h.hints.Context
	resultCount := 0
	if ctx.Hints() != nil {
		resultCount = ctx.Hints().Count()
	}

	style := hintscomponent.BuildSearchInputStyle(h.config.Hints, h.themeProvider)
	frame := h.searchInputFrame()
	if err := h.overlayManager.DrawHintSearchInput(
		ctx.SearchQuery(),
		resultCount,
		frame,
		style,
	); err != nil {
		h.logger.Error("Failed to draw hint search input", zap.Error(err))
	}
}

func (h *Handler) searchInputFrame() hintscomponent.SearchInputFrame {
	ui := h.config.Hints.SearchInputUI
	screenWidth := h.screenBounds.Dx()
	screenHeight := h.screenBounds.Dy()
	width := ui.Width
	if width <= 0 {
		width = 320
	}
	if screenWidth > 0 && width > screenWidth {
		width = screenWidth
	}

	height := estimatedSearchInputHeight(ui)
	x := ui.XOffset
	y := ui.YOffset

	switch hintscomponent.SearchInputPosition(ui.Position) {
	case hintscomponent.SearchInputTopCenter:
		x = (screenWidth-width)/2 + ui.XOffset
	case hintscomponent.SearchInputTopRight:
		x = screenWidth - width - ui.XOffset
	case hintscomponent.SearchInputCenter:
		x = (screenWidth-width)/2 + ui.XOffset
		y = (screenHeight-height)/2 + ui.YOffset
	case hintscomponent.SearchInputBottomLeft:
		y = screenHeight - height - ui.YOffset
	case hintscomponent.SearchInputBottomCenter:
		x = (screenWidth-width)/2 + ui.XOffset
		y = screenHeight - height - ui.YOffset
	case hintscomponent.SearchInputBottomRight:
		x = screenWidth - width - ui.XOffset
		y = screenHeight - height - ui.YOffset
	case hintscomponent.SearchInputTopLeft:
		fallthrough
	default:
	}

	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if screenWidth > 0 && x+width > screenWidth {
		x = screenWidth - width
	}
	if screenHeight > 0 && y+height > screenHeight {
		y = screenHeight - height
	}

	return hintscomponent.NewSearchInputFrame(image.Point{X: x, Y: y}, width)
}

func estimatedSearchInputHeight(ui configpkg.SearchInputUI) int {
	paddingY := ui.PaddingY
	if paddingY < 0 {
		paddingY = max(5, ui.FontSize/2)
	}

	return ui.FontSize + paddingY*2 + 6
}

// handleGridModeKey handles key processing for grid mode.
func (h *Handler) handleGridModeKey(key string) {
	if h.grid.Router == nil {
		h.logger.Warn("Grid router is nil - ignoring key press until grid router initialized")

		return
	}

	gridKeyResult := h.grid.Router.RouteKey(key)

	if gridKeyResult.Complete() {
		targetPoint := gridKeyResult.TargetPoint()

		// Convert from window-local coordinates to absolute screen coordinates using helper
		absolutePoint := coordinates.ConvertToAbsoluteCoordinates(targetPoint, h.screenBounds)
		h.grid.Context.SetSelectionPoint(absolutePoint)

		h.logger.Info(
			"Grid move mouse",
			zap.Int("x", absolutePoint.X),
			zap.Int("y", absolutePoint.Y),
		)

		repeat := h.grid.Context.Repeat()
		pendingAction := h.grid.Context.PendingAction()
		cursorFollowSelection := h.grid.Context.CursorFollowSelection()

		if pendingAction == nil && !repeat && !cursorFollowSelection {
			h.refreshGridVirtualPointerLocked()

			return
		}

		h.moveCursorAndHandleAction(
			absolutePoint,
			pendingAction,
			repeat, // Re-activate grid mode when --repeat is set
			func() {
				h.activateGridModeWithAction(pendingAction, repeat, &cursorFollowSelection)
			},
		)
	} else if targetPoint := gridKeyResult.TargetPoint(); !targetPoint.Eq(image.Point{}) {
		absolutePoint := coordinates.ConvertToAbsoluteCoordinates(targetPoint, h.screenBounds)
		h.grid.Context.SetSelectionPoint(absolutePoint)

		if !h.grid.Context.CursorFollowSelection() {
			h.refreshGridVirtualPointerLocked()

			return
		}

		moveCursorErr := h.actionService.MoveCursorToPoint(context.Background(), absolutePoint)
		if moveCursorErr != nil {
			h.logger.Error("Failed to move cursor", zap.Error(moveCursorErr))
		}
	}
}
