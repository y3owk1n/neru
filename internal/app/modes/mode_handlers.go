package modes

import (
	"image"
	"strings"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"

	hintscomponent "github.com/y3owk1n/neru/internal/app/components/hints"
	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	"github.com/y3owk1n/neru/internal/ui/coordinates"
)

// executeActionAtPoint executes a pending action at the given point and exits the mode.
// When repeat is true and reActivateFunc is provided, the mode is re-activated
// instead of exiting after performing the action.
func (h *Handler) executeActionAtPoint(
	actionStr *string,
	modifierStr *string,
	point image.Point,
	repeat bool,
	reActivateFunc func(),
) {
	if actionStr == nil {
		h.logger.Warn("executeActionAtPoint called with nil action")

		return
	}

	var modifiers action.Modifiers
	if modifierStr != nil {
		var err error

		modifiers, err = action.ParseModifiers(*modifierStr)
		if err != nil {
			h.logger.Error("Failed to parse pending modifier", zap.Error(err))

			return
		}

		if *modifierStr != "" && modifiers == 0 {
			h.logger.Error("Pending modifier was non-empty but parsed to no modifiers")

			return
		}
	}

	modifiers |= h.stickyModifiers()

	h.logger.Debug("Executing pending action",
		zap.String("action", *actionStr),
		zap.String("modifiers", modifiers.String()),
		zap.Bool("repeat", repeat))

	ctx := h.ctx

	// Split comma-separated actions and execute each one sequentially.
	// This enables multi-click sequences like --action left_click,left_click
	// which produce a double-click via the native click-counting layer.
	actions := strings.Split(*actionStr, ",")
	actionPerformed := false
	chainFailed := false

	for actionIdx, a := range actions {
		trimmed := strings.TrimSpace(a)
		if trimmed == "" {
			continue
		}

		// Add a small delay between actions so the OS has time to process
		// each click before the next one arrives. This is required for the
		// native click-counting to correctly detect multi-click sequences.
		if actionIdx > 0 {
			time.Sleep(postActionSettleDelay)
		}

		performErr := h.actionService.PerformActionAtPoint(
			ctx,
			trimmed,
			point,
			modifiers,
		)
		if performErr != nil {
			h.logger.Error("Failed to perform pending action", zap.Error(performErr))

			chainFailed = true

			break
		}

		// Track whether any action was a click (not a move-mouse action)
		// so handleCursorRestoration can insert a settling delay.
		if trimmed != "move_mouse" &&
			trimmed != "move_mouse_relative" {
			actionPerformed = true
		}
	}

	// Signal that a click was just performed so handleCursorRestoration
	// can insert a settling delay before moving the cursor.
	if actionPerformed {
		h.cursorState.MarkActionPerformed()
	}

	if repeat && reActivateFunc != nil && !chainFailed {
		// Wait for the target app to finish processing the click before
		// re-activating (which may move the cursor for grid/recursive-grid).
		// This mirrors the settle delay in handleCursorRestoration and
		// prevents slow apps (Electron, web views) from missing clicks.
		if h.cursorState.WasActionPerformed() {
			time.Sleep(postActionSettleDelay)
		}

		h.logger.Debug("Re-activating mode after action (--repeat)")
		reActivateFunc()

		return
	}

	if chainFailed {
		h.appState.SetModeExitReason(state.ModeExitReasonCancelled)
	} else {
		h.appState.SetModeExitReason(state.ModeExitReasonCompleted)
	}

	h.exitModeLocked()
}

// moveCursorAndHandleAction moves the cursor to a point and executes any pending action.
func (h *Handler) moveCursorAndHandleAction(
	point image.Point,
	pendingAction *string,
	pendingModifier *string,
	shouldReActivate bool,
	reActivateFunc func(),
) {
	ctx := h.ctx

	moveCursorErr := h.actionService.MoveCursorToPoint(ctx, point)
	if moveCursorErr != nil {
		h.logger.Error("Failed to move cursor", zap.Error(moveCursorErr))
	}

	if pendingAction != nil {
		h.executeActionAtPoint(
			pendingAction, pendingModifier, point, shouldReActivate, reActivateFunc,
		)

		return
	}

	// No pending action - re-activate mode if requested
	if shouldReActivate && reActivateFunc != nil {
		h.logger.Debug("Re-activating mode after cursor movement")
		reActivateFunc()
	}
}

// handleHintsModeKey handles key processing for hints mode.
func (h *Handler) handleHintsModeKey(key string) {
	// Route hint-specific keys via domain hints router
	if h.hints.Context.Router() == nil {
		h.logger.Warn("Hints router is nil - ignoring key press until hints initialized")

		return
	}

	hintKeyResult, routeErr := h.hints.Context.Router().RouteKey(key)
	if routeErr != nil {
		h.logger.Error("Hint key routing failed", zap.Error(routeErr))

		return
	}

	// Hint input processed by router; if exact match, perform action
	if hintKeyResult.ExactHint() != nil {
		hint := hintKeyResult.ExactHint()
		center := hint.Element().Center()

		h.logger.Debug("Found element", zap.String("label", hint.Label()))

		pendingAction := h.hints.Context.PendingAction()
		pendingModifier := h.hints.Context.PendingModifier()
		repeat := h.hints.Context.Repeat()
		cursorFollowSelection := h.hints.Context.CursorFollowSelection()
		filterRoles := h.hints.Context.FilterRoles()
		filterTextContains := h.hints.Context.FilterTextContains()
		startWithSearch := h.hints.Context.StartWithSearch()
		strategyOverride := h.hints.Context.StrategyOverride()
		labelDirectionOverride := h.hints.Context.LabelDirectionOverride()
		splitWord := h.hints.Context.SplitWord()

		h.moveCursorAndHandleAction(
			center,
			pendingAction,
			pendingModifier,
			repeat ||
				pendingAction == nil, // re-activate on repeat, or when no action (existing behavior)
			func() {
				h.activateHintModeInternal(
					nil,
					nil,
					&cursorFollowSelection,
					filterRoles,
					filterTextContains,
					&startWithSearch,
					&strategyOverride,
					&labelDirectionOverride,
					&splitWord,
				)
				// Restore repeat, action and modifier on the fresh context so subsequent
				// selections continue the repeat cycle.
				// Guard: only restore if re-activation succeeded (mode is still hints).
				if repeat && h.appState.CurrentMode() == domain.ModeHints &&
					h.hints != nil && h.hints.Context != nil {
					h.hints.Context.SetPendingAction(pendingAction)
					h.hints.Context.SetPendingModifier(pendingModifier)
					h.hints.Context.SetRepeat(true)
					h.hints.Context.SetCursorFollowSelection(cursorFollowSelection)
					h.hints.Context.SetFilterRoles(filterRoles)
					h.hints.Context.SetFilterTextContains(filterTextContains)
					h.hints.Context.SetStartWithSearch(startWithSearch)
					h.hints.Context.SetStrategyOverride(strategyOverride)
					h.hints.Context.SetLabelDirectionOverride(labelDirectionOverride)
					h.hints.Context.SetSplitWord(splitWord)
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
			_, size := utf8.DecodeLastRuneInString(query)
			ctx.SetSearchQuery(query[:len(query)-size])
			h.applyHintSearchFilter()
		}

		return
	case configpkg.KeyNameSpace:
		ctx.SetSearchQuery(ctx.SearchQuery() + " ")
		h.applyHintSearchFilter()

		return
	}

	if utf8.RuneCountInString(key) != 1 {
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

	setHintsErr := ctx.SetVisibleHints(filteredHints)
	if setHintsErr != nil {
		h.logger.Error("Failed to apply hint search filter", zap.Error(setHintsErr))
	}

	h.drawHintSearchInput()
	h.cycleHintIndex = -1
}

func (h *Handler) confirmHintSearch() {
	if h.hints == nil || h.hints.Context == nil {
		return
	}

	h.stopHintSearchTextInputLocked(false)

	ctx := h.hints.Context
	ctx.SetSearchActive(false)
	h.overlayManager.HideHintSearchInput()

	if visibleHints := ctx.Hints(); visibleHints != nil && visibleHints.Count() >= 1 {
		go func() {
			_ = h.CycleHint(h.ctx, false)
		}()
	} else {
		h.cancelHintSearch()
	}

	h.cycleHintIndex = -1
}

func (h *Handler) cancelHintSearch() {
	if h.hints == nil || h.hints.Context == nil {
		return
	}

	h.stopHintSearchTextInputLocked(false)

	ctx := h.hints.Context
	ctx.SetSearchQuery("")
	ctx.SetSearchActive(false)

	if sourceHints := ctx.SourceHints(); sourceHints != nil {
		setHintsErr := ctx.SetVisibleHints(sourceHints)
		if setHintsErr != nil {
			h.logger.Error("Failed to restore hints after search cancel", zap.Error(setHintsErr))
		}
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

	err := h.overlayManager.DrawHintSearchInput(
		ctx.SearchQuery(),
		resultCount,
		frame,
		style,
	)
	if err != nil {
		h.logger.Error("Failed to draw hint search input", zap.Error(err))
	}
}

func (h *Handler) searchInputFrame() hintscomponent.SearchInputFrame {
	searchInputConfig := h.config.Hints.SearchInputUI
	screenWidth := h.screenBounds.Dx()
	screenHeight := h.screenBounds.Dy()

	width := searchInputConfig.Width
	if width <= 0 {
		width = configpkg.DefaultSearchInputWidth
	}

	if screenWidth > 0 && width > screenWidth {
		width = screenWidth
	}

	height := estimatedSearchInputHeight(searchInputConfig)
	xOffset := searchInputConfig.XOffset
	yOffset := searchInputConfig.YOffset

	switch hintscomponent.SearchInputPosition(searchInputConfig.Position) {
	case hintscomponent.SearchInputTopCenter:
		xOffset = (screenWidth-width)/configpkg.DefaultSearchInputCenterDivisor + searchInputConfig.XOffset
	case hintscomponent.SearchInputTopRight:
		xOffset = screenWidth - width - searchInputConfig.XOffset
	case hintscomponent.SearchInputCenter:
		xOffset = (screenWidth-width)/configpkg.DefaultSearchInputCenterDivisor + searchInputConfig.XOffset
		yOffset = (screenHeight-height)/configpkg.DefaultSearchInputCenterDivisor + searchInputConfig.YOffset
	case hintscomponent.SearchInputBottomLeft:
		yOffset = screenHeight - height - searchInputConfig.YOffset
	case hintscomponent.SearchInputBottomCenter:
		xOffset = (screenWidth-width)/configpkg.DefaultSearchInputCenterDivisor + searchInputConfig.XOffset
		yOffset = screenHeight - height - searchInputConfig.YOffset
	case hintscomponent.SearchInputBottomRight:
		xOffset = screenWidth - width - searchInputConfig.XOffset
		yOffset = screenHeight - height - searchInputConfig.YOffset
	case hintscomponent.SearchInputTopLeft:
		fallthrough
	default:
	}

	if xOffset < 0 {
		xOffset = 0
	}

	if yOffset < 0 {
		yOffset = 0
	}

	if screenWidth > 0 && xOffset+width > screenWidth {
		xOffset = screenWidth - width
	}

	if screenHeight > 0 && yOffset+height > screenHeight {
		yOffset = screenHeight - height
	}

	return hintscomponent.NewSearchInputFrame(image.Point{X: xOffset, Y: yOffset}, width)
}

func estimatedSearchInputHeight(searchInputConfig configpkg.SearchInputUI) int {
	paddingY := searchInputConfig.PaddingY
	if paddingY < 0 {
		paddingY = max(
			configpkg.DefaultSearchInputMinPaddingY,
			searchInputConfig.FontSize/configpkg.DefaultSearchInputCenterDivisor,
		)
	}

	return searchInputConfig.FontSize + paddingY*configpkg.DefaultSearchInputPaddingMultiplier + configpkg.DefaultSearchInputHeightPadding
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

		h.logger.Debug(
			"Grid move mouse",
			zap.Int("x", absolutePoint.X),
			zap.Int("y", absolutePoint.Y),
		)

		repeat := h.grid.Context.Repeat()
		pendingAction := h.grid.Context.PendingAction()
		pendingModifier := h.grid.Context.PendingModifier()
		cursorFollowSelection := h.grid.Context.CursorFollowSelection()

		if pendingAction == nil && !repeat && !cursorFollowSelection {
			h.refreshGridVirtualPointerLocked()

			return
		}

		h.moveCursorAndHandleAction(
			absolutePoint,
			pendingAction,
			pendingModifier,
			repeat, // Re-activate grid mode when --repeat is set
			func() {
				h.activateGridModeWithAction(
					pendingAction,
					pendingModifier,
					&repeat,
					&cursorFollowSelection,
				)
			},
		)
	} else if targetPoint := gridKeyResult.TargetPoint(); !targetPoint.Eq(image.Point{}) {
		absolutePoint := coordinates.ConvertToAbsoluteCoordinates(targetPoint, h.screenBounds)
		h.grid.Context.SetSelectionPoint(absolutePoint)

		if !h.grid.Context.CursorFollowSelection() {
			h.refreshGridVirtualPointerLocked()

			return
		}

		moveCursorErr := h.actionService.MoveCursorToPoint(h.ctx, absolutePoint)
		if moveCursorErr != nil {
			h.logger.Error("Failed to move cursor", zap.Error(moveCursorErr))
		}
	}
}
