package modes

import (
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/domain"
)

const (
	// postActionSettleDelay is the time to wait after a click action completes
	// before moving the cursor for restoration/centering. This gives the target
	// application time to finish processing the mouseUp event. Without this
	// delay, cursor restoration can race with click processing in slow apps
	// (Electron, web views) causing missed clicks.
	postActionSettleDelay = 75 * time.Millisecond
)

// ExitMode exits the current mode. Safe to call from any goroutine.
func (h *Handler) ExitMode() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.exitMode()
}

// exitMode exits the current mode and releases the keyboard with it. Caller
// must hold h.mu.
func (h *handlerState) exitMode() {
	h.exitCurrentMode(false)
}

// exitModeForTransition exits the current mode on the way into another one,
// leaving the keyboard capture up across the handover. Caller must hold h.mu.
//
// Releasing it is what an exit *to idle* means, and doing it on a mode-to-mode
// transition opens a window with nobody holding the keyboard: everything typed
// between the release and the next mode's re-grab goes to the focused
// application, and the activation in between is not fast — it queries the
// screen, builds the mode's domain state and draws the overlay. On macOS the
// window is a dropped keystroke or two, which is why scroll mode has skipped
// the disable since it was first reported there (see startInteractiveScroll,
// which keeps a partial cleanup of its own for reasons of its own). On Linux it
// is worse in both directions, which is how it was reported: the keys land in
// the focused app as literal text, and the re-grab that follows costs a keymap
// rebuild plus a wait for every physically-held key to be released before the
// evdev grab is even attempted, so a user still mashing keys keeps extending
// the window they are being dropped into.
//
// Keeping the capture up closes it, and buys buffering with it — but only from
// the moment the activation owns h.mu. An activation holds the lock from start
// to finish and the tap's dispatcher delivers through HandleKeyPress, which
// takes that same lock, so a key the tap reads *after* the activation is under
// way waits on the lock and lands in the mode coming up instead of in the
// application behind it. A key that reaches the handler *before* it does is a
// different thing and is not fixed here: the old mode is still the active one
// and the key is correctly its, which for scroll means dropped, because
// handleGenericScrollKey does nothing. Nothing orders those two — the hotkey
// dispatch is a goroutine and so is the tap's dispatcher, and they contend for
// h.mu unordered — so "the key I pressed right after the chord" is buffered or
// eaten by the mode it was actually pressed in, depending on which won. Closing
// *that* window means ordering the activation ahead of the dispatcher, which
// this does not attempt.
//
// The caller's half of the contract is the release. The keyboard is held with
// the app briefly idle, so this returns the release rather than leaving it to
// be remembered, and the only correct way to call it is
//
//	defer h.exitModeForTransition()()
//
// which runs the exit now and the release at return. The double call is the
// point: an activation that gives up after this must not leave idle holding the
// keyboard, and pairing the two by hand is a thing a new call site can half-do.
// `internal/architecture/mode_transition_release_test.go` fails on any other
// form, because getting it wrong is silent to everything else (ADR 0011).
func (h *handlerState) exitModeForTransition() func() {
	h.exitCurrentMode(true)

	return h.releaseKeyboardIfNoModeEntered
}

// exitCurrentMode is the body of both exits; keepEventTap is what separates
// them. Caller must hold h.mu.
func (h *handlerState) exitCurrentMode(keepEventTap bool) {
	if h.appState.CurrentMode() == domain.ModeIdle {
		return
	}

	h.logger.Debug("Exiting current mode", zap.String("mode", h.CurrModeString()))

	h.performModeSpecificCleanup()
	h.performCommonCleanup(keepEventTap)
	h.handleCursorRestoration()
}

// releaseKeyboardIfNoModeEntered puts down the capture exitModeForTransition
// kept, when the activation that kept it never entered a mode. It is what that
// method returns, and it runs deferred. Caller must hold h.mu.
//
// It is deferred rather than written into each abandon path because there are
// several per mode — a refused draw, an empty hint scan, a permission dialog
// that suspends the activation — and one that forgot would leave the daemon
// idle with the keyboard grabbed, which is every key the user presses going
// nowhere. Idle plus a live capture is the state that must not survive an
// activation returning, whichever way it returns.
//
// What this costs on the abandon path is worth stating, because it is a
// deliberate choice between two wrong answers. Keys the user typed while the
// activation was running were captured, and the mode was idle throughout, where
// nothing is passed through (syncModifierPassthrough, passthrough.go) — so they
// are sitting in the tap's dispatch queue with no mode to deliver them to.
// Disable drops them: it bumps the dispatch epoch and drains the channel
// (adapter/eventtap/linux/tap.go), by design, so a stale keystream cannot be
// read by whatever comes next. They are therefore *discarded* rather than
// reaching the focused application, which is the opposite of what this code
// did before. That is the better of the two: leaking them into the focused
// application as text is the bug this whole change exists to stop, and a user
// who typed into an activation that failed would otherwise get the failure
// *and* the mess. It matters more on Linux than it reads, because the empty
// hint scan is a common abandon there. Delivering them instead would mean
// re-injecting through the virtual keyboard, which is a lot of machinery aimed
// at the case where the user has already been told nothing happened.
func (h *handlerState) releaseKeyboardIfNoModeEntered() {
	if !h.eventTapEnabled() || h.appState.CurrentMode() != domain.ModeIdle {
		return
	}

	h.logger.Debug("Mode transition abandoned; releasing the keyboard it kept")

	h.disableEventTap()
}

// performModeSpecificCleanup handles mode-specific cleanup logic.
func (h *handlerState) performModeSpecificCleanup() {
	mode, exists := h.modes[h.appState.CurrentMode()]
	if !exists {
		h.cleanupDefaultMode()

		return
	}

	mode.Exit()
}

// cleanupHintsMode handles cleanup for hints mode.
func (h *handlerState) cleanupHintsMode() {
	h.stopHintSearchTextInput(false)

	resetErr := h.hints.Context.Reset()
	if resetErr != nil {
		h.logger.Error("Failed to reset hints context", zap.Error(resetErr))
	}

	h.cycleHintIndex = -1

	// Stop the indicator poller before common cleanup takes the frame off the
	// screen: a tick landing after the clear would put an indicator back on it.
	h.stopIndicatorPolling()
}

// cleanupDefaultMode handles cleanup for a mode with no implementation
// registered. There is no domain state to reset, and the frame on screen is
// taken off by the common cleanup that follows — the same way it is for every
// mode that does have one.
func (h *handlerState) cleanupDefaultMode() {}

// cleanupGridMode handles cleanup for grid mode.
func (h *handlerState) cleanupGridMode() {
	// Only reset the base context fields (pendingAction, repeat).
	// Do NOT call h.grid.Context.Reset() because it nils out
	// gridInstance (a **domainGrid.Grid pointer-to-pointer that is
	// wired once during component setup in component_factory.go).
	// Nilling it causes a nil-pointer dereference on re-activation
	// when SetGridInstanceValue dereferences the pointer.
	h.grid.Context.SetPendingAction(nil)
	h.grid.Context.SetOnExit(nil)
	h.grid.Context.SetRepeat(false)

	if h.grid.Manager != nil {
		h.grid.Manager.ResetSilent()
	}

	// What the overlay still holds from this session — the hide-unmatched flag,
	// the match prefix and the pointer stand-in — is dropped by the frame clear
	// that follows in performCommonCleanup, and dropped there on purpose
	// (#1492): resetting them from here repainted the grid, twice, to throw it
	// away a moment later. ClearFrame owns the leaving half of every surface,
	// and these are the last things a caller had to take off one itself.

	// Stop the indicator poller before common cleanup takes the frame off the
	// screen: a tick landing after the clear would put an indicator back on it.
	h.stopIndicatorPolling()
}

// performCommonCleanup handles common cleanup logic for all modes. keepEventTap
// leaves the keyboard capture up for a caller that is handing it to the next
// mode rather than giving it back — see exitModeForTransition.
func (h *handlerState) performCommonCleanup(keepEventTap bool) {
	h.stopIndicatorPolling()
	h.stopHeldRepeat()
	h.clearOverlayFrame()

	// Stop any pending hints refresh timer to prevent re-activation after exit
	if h.refreshHintsTimer != nil {
		h.refreshHintsTimer.Stop()
		h.refreshHintsTimer = nil
	}

	h.hotkeyLastKey = ""
	h.hotkeyLastKeyTime = 0

	// Release synthetic sticky modifiers before disabling the Wayland event tap.
	// The evdev-backed tap restores overlay keyboard focus during shutdown; if
	// we post modifier key-up events after that handoff, the overlay can receive
	// the release instead of the target app and leave the app "stuck" with the
	// modifier still logically held.
	h.clearStickyModifiers()

	if !keepEventTap && h.hasEventTap() {
		h.disableEventTap()
	}

	h.releaseHeldButtons()

	h.setAppMode(domain.ModeIdle)

	// Do NOT reset suppressedModifiers here — SuppressModifiersForHotkey was
	// called synchronously by the hotkey dispatch path (dispatchModeAwareHeldHotkey
	// or dispatchModeAwareHotkeyAsync) before the mode switch, and the modifier
	// UP events arrive after the user releases the chord. Clearing
	// suppressedModifiers here causes the modifier DOWN events (for the next
	// chord press) to be handled as normal modifier taps instead of suppressed,
	// which schedules a debounce timer that fires and toggles the sticky modifier
	// when it shouldn't.
	//
	// expireSuppressedModifiersIfNeeded handles cleanup after the 2-second
	// activationModifierSuppressionWindow expires.
	// h.suppressedModifiers = 0
	// h.suppressedUntil = time.Time{}
	// The overlay is already idle: clearOverlayFrame above returned it there,
	// because taking the frame off screen and leaving the mode behind are one
	// step and not two a caller has to remember.
	h.logger.Debug("Mode transition complete",
		zap.String("to", "idle"))

	// If a hotkey refresh was deferred while in an active mode, perform it now
	if h.appState.HotkeyRefreshPending() {
		h.appState.SetHotkeyRefreshPending(false)

		if h.refreshHotkeys != nil {
			go h.refreshHotkeys()
		}
	}
}

// handleCursorRestoration finalizes transient cursor and scroll state on mode exit.
func (h *handlerState) handleCursorRestoration() {
	h.cursorState.Reset()

	// Always reset scroll context to ensure proper state cleanup when switching modes.
	h.scroll.Context.Reset()
}

// releaseHeldButtons releases any mouse button left down by an interrupted
// drag. It routes through the action service rather than reaching into the
// accessibility infra package directly, and swallows the error because every
// caller is already on a cleanup path where there is nothing left to abort.
func (h *handlerState) releaseHeldButtons() {
	if h.actionService == nil {
		return
	}

	err := h.actionService.ReleaseHeldButtons(h.ctx)
	if err != nil {
		h.logger.Debug("Failed to release held mouse buttons", zap.Error(err))
	}
}
