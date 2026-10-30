package modes

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/bisect"
	"github.com/y3owk1n/neru/internal/ports"
)

// ResetCurrentMode clears the active mode's accumulated input without exiting
// it. A mode with no input to clear says so in the debug log.
func (h *Handler) ResetCurrentMode() {
	h.mu.Lock()
	defer h.mu.Unlock()

	editor, ok := activeModeEffect[inputEditor](&h.handlerState, extensionInputEditing)
	if !ok {
		return
	}

	h.trackSelection(editor.ResetInput)
}

// BackspaceCurrentMode takes back the active mode's most recent unit of input
// without exiting it. A mode with no input to take back says so in the debug
// log.
func (h *Handler) BackspaceCurrentMode() {
	h.mu.Lock()
	defer h.mu.Unlock()

	editor, ok := activeModeEffect[inputEditor](&h.handlerState, extensionInputEditing)
	if !ok {
		return
	}

	h.trackSelection(editor.Backspace)
}

// MoveCellCurrentMode slides the active mode's selection count cells in dir
// without changing the active layer.
//
// Grid mode moves an open subgrid to the neighboring cell. Recursive-grid
// mode slides the highlighted region at the current depth, crossing into a
// neighboring parent when it runs off the edge of its own. Modes with no cell
// selection ignore it, as does a move that would leave the screen.
func (h *Handler) MoveCellCurrentMode(dir domain.Direction, count int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	navigator, ok := activeModeExtension[cellNavigator](&h.handlerState)
	if !ok {
		return
	}

	h.trackSelection(func() { navigator.MoveCell(dir, count) })
}

// BisectCurrentMode keeps the half or quadrant of the active mode's region
// that cut names, count times over as one press. A mode that does not bisect
// says so in the debug log.
func (h *Handler) BisectCurrentMode(cut bisect.Cut, count int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	cutter, ok := activeModeEffect[bisector](&h.handlerState, extensionBisecting)
	if !ok {
		return
	}

	h.trackSelection(func() { cutter.Bisect(cut, count) })
}

// StartHintSearch activates text filtering for hints mode. confirmOnLabel
// turns on --confirm-on-label for this search alone.
func (h *Handler) StartHintSearch(confirmOnLabel bool) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.startHintSearch(confirmOnLabel)
}

// CycleHint cycles through visible hints in hints mode, selecting the next or previous one.
// When executeAction is true, any pending action is performed on the selected hint
// (used by search confirmation). When false, only the cursor moves (used by the
// cycle_hint IPC action so users can browse results without triggering clicks).
func (h *Handler) CycleHint(ctx context.Context, backward bool, executeAction bool) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.cycleHint(ctx, backward, executeAction)
}

// RefreshHints rescans the open hints session with the flags it was activated
// with, so a strategy or capture-scope cycle stays on the entry in use and the
// filters, action and on-exit steps carry over. It bypasses ActivateMode, which
// would advance the cycle or, with --toggle, exit.
//
// It refuses supplied targets, because there is nothing to scan again and
// relabeling them would look like a refresh that found the same elements.
func (h *Handler) RefreshHints() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.appState.CurrentMode() != domain.ModeHints {
		return derrors.New(derrors.CodeInvalidInput, "refresh_hints requires hints mode")
	}

	if h.hints == nil || h.hints.Context == nil {
		return derrors.New(derrors.CodeActionFailed, "hints component not available")
	}

	if h.hints.Context.TargetsSupplied() {
		return derrors.New(
			derrors.CodeInvalidInput,
			"refresh_hints cannot rescan supplied targets",
		)
	}

	h.activationRefusal = nil
	h.activateHintModeInternal(h.hintSessionActivation())

	// A refresh that cannot draw leaves the mode rather than keep labels
	// pointing at elements that may be gone, and the caller hears why.
	if h.appState.CurrentMode() != domain.ModeHints {
		if h.activationRefusal != nil {
			return h.activationRefusal
		}

		return derrors.New(
			derrors.CodeActionFailed,
			"refresh_hints found no hints, hints mode exited",
		)
	}

	return nil
}

// cycleHint is the body of CycleHint.
//
// Caller must hold h.mu.
func (h *handlerState) cycleHint(ctx context.Context, backward bool, executeAction bool) error {
	if h.appState.CurrentMode() != domain.ModeHints {
		return derrors.New(derrors.CodeInvalidInput, "cycle_hint requires hints mode")
	}

	if h.hints == nil || h.hints.Context == nil {
		return derrors.New(derrors.CodeActionFailed, "hints component not available")
	}

	manager := h.hints.Context.Manager()
	if manager == nil {
		return derrors.New(derrors.CodeActionFailed, "hints manager not available")
	}

	filteredHints := manager.FilteredHints()
	if len(filteredHints) == 0 {
		filteredHints = h.hints.Context.Hints().All()
	}

	if len(filteredHints) == 0 {
		return derrors.New(derrors.CodeActionFailed, "no hints available")
	}

	if h.cycleHintIndex >= len(filteredHints) {
		h.cycleHintIndex = len(filteredHints) - 1
	}

	switch {
	case h.cycleHintIndex < 0:
		h.cycleHintIndex = 0
		if backward {
			h.cycleHintIndex = len(filteredHints) - 1
		}
	default:
		if backward {
			if h.cycleHintIndex > 0 {
				h.cycleHintIndex--
			} else {
				h.cycleHintIndex = len(filteredHints) - 1
			}
		} else {
			if h.cycleHintIndex < len(filteredHints)-1 {
				h.cycleHintIndex++
			} else {
				h.cycleHintIndex = 0
			}
		}
	}

	selectedHint := filteredHints[h.cycleHintIndex]

	center := selectedHint.Element().Center()

	moveErr := h.actionService.MoveCursorToPoint(ctx, center)
	if moveErr != nil {
		return derrors.Wrap(
			moveErr,
			derrors.CodeActionFailed,
			"failed to move cursor during cycle_hint",
		)
	}

	pendingAction := h.hints.Context.PendingAction()

	pendingModifier := h.hints.Context.PendingModifier()
	if pendingAction != nil && executeAction {
		repeat := h.hints.Context.Repeat()
		session := h.hintSessionActivation()

		h.executeActionAtPoint(pendingAction, pendingModifier, center, repeat, func() {
			h.activateHintModeInternal(session)

			// Restore the state so later cycles keep executing the action, but
			// only when repeat was set and the mode is still hints.
			if repeat && h.appState.CurrentMode() == domain.ModeHints &&
				h.hints != nil && h.hints.Context != nil {
				h.hints.Context.SetPendingAction(pendingAction)
				h.hints.Context.SetPendingModifier(pendingModifier)
				h.hints.Context.SetRepeat(true)
				h.hints.Context.SetCursorFollowSelection(*session.CursorFollowSelection)
				h.hints.Context.SetFilterRoles(session.FilterRoles)
				h.hints.Context.SetFilterTextContains(session.FilterTextContains)
				h.hints.Context.SetStartWithSearch(*session.Search)
				h.hints.Context.SetConfirmOnLabel(*session.ConfirmOnLabel)
				h.hints.Context.SetExitOnUnmatched(*session.ExitOnUnmatched)
				h.hints.Context.SetStrategyOverride(*session.Strategy)
				h.hints.Context.SetCaptureScopeOverride(*session.CaptureScope)
				h.hints.Context.SetLabelDirectionOverride(*session.LabelDirection)
				h.hints.Context.SetSplitWord(*session.SplitWord)
			}
		})

		return nil
	}

	h.publishSelect(center, "")

	return nil
}

func (h *handlerState) startHintSearch(confirmOnLabel bool) error {
	if h.appState.CurrentMode() != domain.ModeHints {
		return derrors.New(derrors.CodeInvalidInput, "search_hints requires hints mode")
	}

	if h.hints == nil || h.hints.Context == nil {
		return derrors.New(derrors.CodeActionFailed, "hints component not available")
	}

	if h.hints.Context.SourceHints() == nil {
		return derrors.New(derrors.CodeActionFailed, "hints not available")
	}

	h.stopHintSearchTextInput(true)
	h.hints.Context.SetSearchQuery("")
	h.hints.Context.SetSearchActive(true)
	h.hints.Context.SetSearchConfirmOnLabel(confirmOnLabel)

	session := h.hintSearchSession

	if h.hints.Context.HideOnEmptySearch() {
		// When hide-on-empty-search is active, hide all hints initially.
		// Hints will appear as the user types a query.
		setHintsErr := h.hints.Context.ClearVisibleHints()
		if setHintsErr != nil {
			return setHintsErr
		}
	} else {
		setHintsErr := h.hints.Context.SetVisibleHints(
			h.hints.Context.SourceHints(),
		)
		if setHintsErr != nil {
			return setHintsErr
		}
	}

	h.cycleHintIndex = -1
	h.drawHintSearchInput()

	if h.textInput == nil {
		return nil
	}

	// The IME field sits over the drawn search input, so its placement is
	// asked for rather than derived a second time here.
	bounds := h.hintSearchBounds()
	if bounds.Empty() {
		// The overlay put no box on screen, because it draws none or could not
		// place the anchor it was configured with. Handing the platform's field
		// the keyboard anyway would put an invisible input somewhere the user
		// is not looking. The query keeps arriving through the event tap's key
		// stream instead, which is what every overlay without a search box
		// already relies on.
		//
		// That promise only holds if the tap is on, and this method opened by
		// stopping any live session *without* re-enabling it, on the
		// assumption that the session about to start would want it off. There
		// is no session now, so give the keyboard back or the search has no way
		// left to receive a key.
		h.stopHintSearchTextInput(false)
		h.logger.Debug("Hint search text input skipped: no search input on screen")

		return nil
	}

	textInputFrame := ports.TextInputFrame{
		X:      bounds.Min.X,
		Y:      bounds.Min.Y,
		Width:  bounds.Dx(),
		Height: bounds.Dy(),
		Screen: h.screenBounds,
	}

	started, _ := h.textInput.StartHintSearchSession(
		h.ctx,
		ports.TextInputCallbacks{
			OnQueryChanged: func(query string) {
				h.outer.mu.Lock()
				defer h.outer.mu.Unlock()

				if h.hintSearchSession != session || h.appState.CurrentMode() != domain.ModeHints ||
					h.hints == nil || h.hints.Context == nil {
					return
				}

				if !h.hints.Context.SearchActive() {
					return
				}

				h.hints.Context.SetSearchQuery(query)
				h.applyHintSearchFilter()
			},
			OnConfirm: func() {
				h.outer.mu.Lock()
				defer h.outer.mu.Unlock()

				if h.hintSearchSession != session || h.appState.CurrentMode() != domain.ModeHints {
					return
				}

				h.confirmHintSearch()
			},
			OnCancel: func() {
				h.outer.mu.Lock()
				defer h.outer.mu.Unlock()

				if h.hintSearchSession != session || h.appState.CurrentMode() != domain.ModeHints {
					return
				}

				h.cancelHintSearch()
			},
		},
		textInputFrame,
	)

	if !started {
		// A session this one replaced left the tap off for it, and no field
		// took its place, so the key stream is all that is left.
		h.stopHintSearchTextInput(false)

		return nil
	}

	h.hintSearchTextInputActive = true

	if h.hasEventTap() {
		h.disableEventTap()
		h.hintSearchEventTapDisabled = true
	}

	return nil
}

func (h *handlerState) stopHintSearchTextInput(keepEventTapDisabled bool) {
	if h.hintSearchTextInputActive && h.textInput != nil {
		// Use Background context since this may be called during cleanup,
		// after h.ctx has already been canceled.
		err := h.textInput.StopHintSearchSession(context.Background())
		if err != nil {
			h.logger.Warn("Failed to stop hint search text input", zap.Error(err))
		}
	}

	// Callbacks the stopped session already queued see a new count and stop.
	h.hintSearchSession++
	h.hintSearchTextInputActive = false

	if h.hintSearchEventTapDisabled && h.hasEventTap() &&
		h.appState.CurrentMode() == domain.ModeHints && !keepEventTapDisabled {
		h.enableEventTap()
		h.hintSearchEventTapDisabled = false
	}
}
