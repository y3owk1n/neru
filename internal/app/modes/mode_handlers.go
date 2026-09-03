package modes

import (
	"image"
	"strings"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"

	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/geometry"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
	"github.com/y3owk1n/neru/internal/domain/state"
	"github.com/y3owk1n/neru/internal/ports"
)

// executeActionAtPoint executes a pending action at the given point and exits the mode.
// When repeat is true and reActivateFunc is provided, the mode is re-activated
// instead of exiting after performing the action.
func (h *handlerState) executeActionAtPoint(
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

	// Capture the mode's --on-exit action before exitMode clears the
	// context. It only runs when the action chain was fulfilled and the mode
	// idled through this action path — never on manual escape or a switch.
	onExit := h.currentModeOnExit()

	h.exitMode()

	if !chainFailed {
		h.runOnExit(onExit)
	}
}

// currentModeOnExit returns the --on-exit steps configured for the currently
// active action mode, or nil when none are set or the mode has no context.
//
// A mode that reports no exit steps at all — scroll, the monitor picker, idle —
// takes no pending --action either, so there is nothing here it could have been
// asked to run and its silence is the same nothing as an empty list.
func (h *handlerState) currentModeOnExit() []string {
	reporter, ok := activeModeExtension[exitStepReporter](h)
	if !ok {
		return nil
	}

	return reporter.ExitSteps()
}

// runOnExit dispatches the mode's --on-exit steps after the pending action was
// fulfilled. They reuse the hotkey action grammar ("action ...", "exec ...",
// mode names) and run asynchronously: a step may route back through IPC into
// ActivateMode, which acquires h.mu — held by the
// executeActionAtPoint caller.
func (h *handlerState) runOnExit(onExit []string) {
	if len(onExit) == 0 || h.executeActionSequence == nil {
		return
	}

	steps := append([]string(nil), onExit...)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				h.logger.Error("panic in on-exit handler",
					zap.Any("recover", r),
					zap.Int("steps", len(steps)))
			}
		}()

		h.executeActionSequence("on-exit", steps)
	}()
}

// moveCursorAndHandleAction moves the cursor to a point and executes any pending action.
func (h *handlerState) moveCursorAndHandleAction(
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
func (h *handlerState) handleHintsModeKey(key string) {
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
		captureScopeOverride := h.hints.Context.CaptureScopeOverride()
		labelDirectionOverride := h.hints.Context.LabelDirectionOverride()
		splitWord := h.hints.Context.SplitWord()

		h.moveCursorAndHandleAction(
			center,
			pendingAction,
			pendingModifier,
			repeat ||
				pendingAction == nil, // re-activate on repeat, or when no action (existing behavior)
			func() {
				h.activateHintModeInternal(modecmd.Activation{
					Mode:                  domain.ModeHints,
					CursorFollowSelection: &cursorFollowSelection,
					FilterRoles:           filterRoles,
					FilterTextContains:    filterTextContains,
					Search:                &startWithSearch,
					Strategy:              &strategyOverride,
					CaptureScope:          &captureScopeOverride,
					LabelDirection:        &labelDirectionOverride,
					SplitWord:             &splitWord,
					// OnExit is left nil to preserve the stored steps across
					// re-activation.
				})
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
					h.hints.Context.SetCaptureScopeOverride(captureScopeOverride)
					h.hints.Context.SetLabelDirectionOverride(labelDirectionOverride)
					h.hints.Context.SetSplitWord(splitWord)
				}
			},
		)
	}
}

// handleSearchInputKey routes all keys while hint text search is active.
func (h *handlerState) handleSearchInputKey(key string) {
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

func (h *handlerState) applyHintSearchFilter() {
	ctx := h.hints.Context

	sourceHints := ctx.SourceHints()
	if sourceHints == nil {
		return
	}

	// When HideOnEmptySearch is active and the query is empty, hide all hints.
	// This lets the user see nothing until they type a search query.
	if ctx.HideOnEmptySearch() && ctx.SearchQuery() == "" {
		setHintsErr := ctx.ClearVisibleHints()
		if setHintsErr != nil {
			h.logger.Error("Failed to clear hints for empty search", zap.Error(setHintsErr))
		}

		h.drawHintSearchInput()
		h.cycleHintIndex = -1

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

func (h *handlerState) confirmHintSearch() {
	if h.hints == nil || h.hints.Context == nil {
		return
	}

	h.stopHintSearchTextInput(false)

	ctx := h.hints.Context
	ctx.SetSearchActive(false)
	h.hideHintSearchInput()

	visibleHints := ctx.Hints()
	if visibleHints != nil && visibleHints.Count() >= 1 {
		// When a pending action is configured and more than one hint matches
		// the search query, just close the search overlay without executing
		// the action. This lets the user type the exact hint label to select
		// an element instead of blindly acting on the first match.
		if ctx.PendingAction() != nil && visibleHints.Count() > 1 {
			h.cycleHintIndex = -1

			return
		}

		go func() {
			_ = h.outer.CycleHint(h.ctx, false, true)
		}()
	} else {
		h.cancelHintSearch()
	}

	h.cycleHintIndex = -1
}

func (h *handlerState) cancelHintSearch() {
	if h.hints == nil || h.hints.Context == nil {
		return
	}

	h.stopHintSearchTextInput(false)

	ctx := h.hints.Context
	ctx.SetSearchQuery("")
	ctx.SetSearchActive(false)

	if sourceHints := ctx.SourceHints(); sourceHints != nil {
		setHintsErr := ctx.SetVisibleHints(sourceHints)
		if setHintsErr != nil {
			h.logger.Error("Failed to restore hints after search cancel", zap.Error(setHintsErr))
		}
	}

	h.hideHintSearchInput()
	h.cycleHintIndex = -1
}

func (h *handlerState) drawHintSearchInput() {
	if h.hints == nil || h.hints.Context == nil || h.overlayPort == nil {
		return
	}

	ctx := h.hints.Context

	resultCount := 0
	if ctx.Hints() != nil {
		resultCount = ctx.Hints().Count()
	}

	// Where the input sits is resolved from configuration by the overlay; the
	// handler says what is in it and which screen it is on, and nothing else.
	err := h.overlayPort.DrawHintSearch(ports.HintSearch{
		Screen:      h.screenBounds,
		Query:       ctx.SearchQuery(),
		ResultCount: resultCount,
	})
	if err != nil {
		if derrors.IsNotSupported(err) {
			// Either the backend draws no search input or it could not place
			// the configured anchor; both mean nothing is on screen, and the
			// overlay says which it was.
			h.logger.Debug("Hint search input not drawn")

			return
		}

		h.logger.Error("Failed to draw hint search input", zap.Error(err))
	}
}

// hideHintSearchInput takes the search input off the screen, leaving the hint
// labels behind it where they are.
func (h *handlerState) hideHintSearchInput() {
	if h.overlayPort == nil {
		return
	}

	h.overlayPort.HideHintSearch()
}

// hintSearchBounds reports where the overlay places the search input on the
// active screen, so the platform's IME field can be put over it.
func (h *handlerState) hintSearchBounds() image.Rectangle {
	if h.overlayPort == nil {
		return image.Rectangle{}
	}

	return h.overlayPort.HintSearchBounds(h.screenBounds)
}

// handleGridModeKey handles key processing for grid mode.
func (h *handlerState) handleGridModeKey(key string) {
	if h.grid.Router == nil {
		h.logger.Warn("Grid router is nil - ignoring key press until grid router initialized")

		return
	}

	gridKeyResult := h.grid.Router.RouteKey(key)

	if gridKeyResult.Complete() {
		targetPoint := gridKeyResult.TargetPoint()

		// Convert from window-local coordinates to absolute screen coordinates using helper
		absolutePoint := geometry.ConvertToAbsoluteCoordinates(targetPoint, h.screenBounds)
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
			h.refreshGridVirtualPointer()

			return
		}

		h.moveCursorAndHandleAction(
			absolutePoint,
			pendingAction,
			pendingModifier,
			repeat, // Re-activate grid mode when --repeat is set
			func() {
				h.activateGridModeWithAction(modecmd.Activation{
					Mode:                  domain.ModeGrid,
					Action:                pendingAction,
					Modifier:              pendingModifier,
					Repeat:                &repeat,
					CursorFollowSelection: &cursorFollowSelection,
					// OnExit stays nil to preserve the stored steps.
				})
			},
		)
	} else if targetPoint := gridKeyResult.TargetPoint(); !targetPoint.Eq(image.Point{}) {
		absolutePoint := geometry.ConvertToAbsoluteCoordinates(targetPoint, h.screenBounds)
		h.grid.Context.SetSelectionPoint(absolutePoint)

		if !h.grid.Context.CursorFollowSelection() {
			h.refreshGridVirtualPointer()

			return
		}

		moveCursorErr := h.actionService.MoveCursorToPoint(h.ctx, absolutePoint)
		if moveCursorErr != nil {
			h.logger.Error("Failed to move cursor", zap.Error(moveCursorErr))
		}
	}
}
