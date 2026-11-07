package modes

import (
	"image"
	"strings"
	"time"

	"go.uber.org/zap"

	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
	"github.com/y3owk1n/neru/internal/domain/state"
)

// routeSelectWithLabelKey types a hint character held with a select_with
// modifier set into the label, ahead of the bindings, and reports whether it
// did. Validation refuses a binding for any such chord, so no binding is
// skipped. A modified key that is not a hint character, or whose modifiers
// have no entry, is left to the bindings as before.
//
// Caller must hold h.mu.
func (h *handlerState) routeSelectWithLabelKey(key string) bool {
	if h.appState.CurrentMode() != domain.ModeHints ||
		h.hints == nil || h.hints.Context == nil || h.config == nil {
		return false
	}

	// The cheap checks come first, so a bare label key, the common case, never
	// settles here. handleHotkey settles once per keystroke and that is the
	// one settle a keystroke may cost (keymap.go).
	normalized := configpkg.NormalizeKeyForComparison(key)

	idx := strings.LastIndex(normalized, "+")
	if idx <= 0 || idx == len(normalized)-1 {
		return false
	}

	modifiers, base := normalized[:idx], normalized[idx+1:]
	if len(base) != 1 || !strings.Contains(strings.ToLower(h.config.Hints.HintCharacters), base) {
		return false
	}

	h.settleKeymaps()

	if _, ok := h.selectWith.Lookup(modifiers); !ok {
		return false
	}

	h.handleHintsLabelKey(base, modifiers)

	return true
}

// selectWithPlan is what finishing a select_with selection needs from the
// session it was made in, captured before the entry's steps run, since a step
// may leave the session.
type selectWithPlan struct {
	session         uint64
	activation      modecmd.Activation
	pendingAction   *string
	pendingModifier *string
	repeat          bool
	onExit          []string
	env             []string
}

// selectWithEntry moves the cursor to the selected hint and runs entry's
// steps there in place of the session's --action, then finishes the session
// the way that action would have.
//
// Caller must hold h.mu.
func (h *handlerState) selectWithEntry(point image.Point, entry configpkg.Binding) {
	h.logger.Debug("Hint selected with modifiers",
		zap.String("entry", entry.Key),
		zap.Int("step_count", len(entry.Steps)))

	h.runSelectionSteps(point, entry.Key, entry.Steps, "")
}

// copyTextSelection runs a copy_text session's action on the selected hint.
// It goes through the steps path rather than the click path so the clipboard
// is written by the copy_text action off h.mu, where a slow clipboard tool
// cannot hold up key handling.
//
// Caller must hold h.mu.
func (h *handlerState) copyTextSelection(point image.Point) {
	h.runSelectionSteps(
		point,
		string(action.NameCopyText),
		[]string{action.PrefixAction + " " + string(action.NameCopyText)},
		string(action.NameCopyText),
	)
}

// runSelectionSteps moves the cursor to the selected hint, runs steps there,
// then finishes the session the way its --action would have. source names
// the steps in logs and to the modifier suppression a mode-entering step
// triggers, and actionName is what the select event reports the selection ran.
//
// The steps run on a goroutine, as a binding's do, because a step may reach
// the handler through IPC and h.mu is held here. Finishing re-enters through
// the outer locked surface, guarded by the mode-session token. Until it does,
// label keys are dropped (selectWithPending), since the hints on screen still
// belong to this selection and a second one would move the cursor under the
// steps. Bindings, Escape among them, still answer.
//
// Caller must hold h.mu.
func (h *handlerState) runSelectionSteps(
	point image.Point,
	source string,
	steps []string,
	actionName string,
) {
	moveErr := h.actionService.MoveCursorToPoint(h.ctx, point)
	if moveErr != nil {
		h.logger.Error("Failed to move cursor", zap.Error(moveErr))
	}

	selection := h.publishSelect(point, actionName)

	plan := selectWithPlan{
		session:         h.modeSession,
		activation:      h.hintSessionActivation(),
		pendingAction:   h.hints.Context.PendingAction(),
		pendingModifier: h.hints.Context.PendingModifier(),
		repeat:          h.hints.Context.Repeat(),
		onExit:          h.currentModeOnExit(),
		env:             selection.Env(),
	}

	if h.executeActionSequence == nil {
		h.finishSelectWith(plan, false)

		return
	}

	h.selectWithInFlight = true
	h.selectWithSession = plan.session

	steps = append([]string(nil), steps...)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				h.logger.Error("Panic in selection steps",
					zap.Any("recover", r),
					zap.String("source", source))
			}
		}()

		h.executeActionSequence(source, steps, plan.env)
		h.outer.finishSelectWithAfterSteps(plan)
	}()
}

// finishSelectWithAfterSteps finishes a select_with selection once its steps
// have run.
func (h *Handler) finishSelectWithAfterSteps(plan selectWithPlan) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.ctx.Err() != nil {
		return
	}

	h.finishSelectWith(plan, true)
}

// selectWithPending reports whether a select_with selection made in the
// current session is still running its steps.
//
// Caller must hold h.mu.
func (h *handlerState) selectWithPending() bool {
	return h.selectWithInFlight && h.selectWithSession == h.modeSession
}

// finishSelectWith ends a select_with selection as the session's --action
// would have ended it. Hints reopen under --repeat or when the session has no
// --action, and otherwise exit and run --on-exit.
//
// A step can end the session itself. Leaving to idle finishes it, so
// --on-exit still runs, while entering another mode hands the keyboard on and
// runs nothing. Only a single change of session, to idle, counts as that exit.
// More than one means the session ended and something else opened and closed
// since, which is no longer this selection's to finish. An escape pressed
// while the steps run is indistinguishable from an "idle" step and counts as
// one.
//
// stepsRan says the entry's steps ran, which is when the target app needs a
// moment to take their clicks before a rescan.
//
// Caller must hold h.mu.
func (h *handlerState) finishSelectWith(plan selectWithPlan, stepsRan bool) {
	// Only this selection's own guard is cleared. A later session may have a
	// selection of its own running.
	if h.selectWithSession == plan.session {
		h.selectWithInFlight = false
	}

	if h.modeSession != plan.session {
		if h.modeSession == plan.session+1 && h.appState.CurrentMode() == domain.ModeIdle {
			h.runOnExit(plan.onExit, plan.env)
		}

		return
	}

	if plan.repeat || plan.pendingAction == nil {
		// Give the target app time to take the steps' clicks before the
		// rescan, as a --repeat selection does after its action.
		if stepsRan {
			time.Sleep(postActionSettleDelay)
		}

		h.reactivateHintSession(
			plan.activation,
			plan.pendingAction,
			plan.pendingModifier,
			plan.repeat,
		)

		return
	}

	h.appState.SetModeExitReason(state.ModeExitReasonCompleted)
	h.exitMode()
	h.runOnExit(plan.onExit, plan.env)
}

// selectWithChords returns every chord a select_with modifier set makes with a
// hint character, for the event tap to consume rather than pass through.
// Shift-only sets are left out, since a Shift chord is never passed through.
//
// Caller must hold h.mu, with the keymaps settled.
func (h *handlerState) selectWithChords() []string {
	if h.selectWith.Len() == 0 || h.config == nil {
		return nil
	}

	hintCharacters := strings.ToLower(h.config.Hints.HintCharacters)
	chords := make([]string, 0, h.selectWith.Len()*len(hintCharacters))

	for _, modifiers := range h.selectWith.ModifierSets() {
		for _, char := range hintCharacters {
			chord := modifiers + "+" + string(char)
			if configpkg.HasPassthroughModifier(chord) {
				chords = append(chords, chord)
			}
		}
	}

	return chords
}

// rememberSelectedHint records the hint the user just selected, for copy_text.
//
// Caller must hold h.mu.
func (h *handlerState) rememberSelectedHint(text string) {
	h.hintSelected = true
	h.selectedHintText = text
}

// SelectedHintText returns the text of the hint most recently selected, for
// copy_text. It is refused when no hint has been selected since a mode last
// opened, and fails when the hint has no text, such as one found by contour
// or segment.
func (h *Handler) SelectedHintText() (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.hintSelected {
		return "", derrors.New(derrors.CodeInvalidInput, "copy_text needs a selected hint")
	}

	if h.selectedHintText == "" {
		return "", derrors.New(derrors.CodeActionFailed, "the selected hint has no text")
	}

	return h.selectedHintText, nil
}

// isCopyTextAction reports whether a session's --action is copy_text.
func isCopyTextAction(pendingAction *string) bool {
	return pendingAction != nil && action.IsCopyTextAction(strings.TrimSpace(*pendingAction))
}
