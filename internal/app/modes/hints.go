package modes

import (
	"context"
	"image"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
	domainHint "github.com/y3owk1n/neru/internal/domain/hint"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
	"github.com/y3owk1n/neru/internal/ports"
)

// debugElapsed logs the duration since start with the given message.
func debugElapsed(logger *zap.Logger, start time.Time, msg string, fields ...zap.Field) {
	logger.Debug(msg, append(fields, zap.Duration("elapsed", time.Since(start)))...)
}

const (
	// HintTimeout is the timeout for hint operations.
	HintTimeout = 5 * time.Second
)

// filterHintsForScreen returns only the hints whose element center falls within
// screenBounds, and deduplicates by position so that downstream code (overlay
// incremental updates, Objective-C NeruDrawIncrementHints) can safely use
// position as a unique key without silently dropping entries.
func filterHintsForScreen(
	allHints []*domainHint.Interface,
	screenBounds image.Rectangle,
) []*domainHint.Interface {
	filtered := make([]*domainHint.Interface, 0, len(allHints))

	seenPositions := make(map[image.Point]struct{}, len(allHints))
	for _, hint := range allHints {
		hintBounds := hint.Element().Bounds()

		hintCenter := image.Point{
			X: hintBounds.Min.X + hintBounds.Dx()/2,
			Y: hintBounds.Min.Y + hintBounds.Dy()/2,
		}
		if !hintCenter.In(screenBounds) {
			continue
		}

		if _, exists := seenPositions[hintCenter]; exists {
			continue
		}

		seenPositions[hintCenter] = struct{}{}

		filtered = append(filtered, hint)
	}

	return filtered
}

// activateHintModeWithAction activates hint mode with optional action parameter.
func (h *handlerState) activateHintModeWithAction(activation modecmd.Activation) {
	h.activateHintModeInternal(activation)

	// Repeat is stored after activation, by which point the context exists.
	if activation.Repeat != nil && *activation.Repeat && h.hints != nil && h.hints.Context != nil {
		h.hints.Context.SetRepeat(true)
	}
}

// activateHintModeInternal activates hint mode with an optional action.
// It handles mode validation, overlay positioning, element collection, hint
// generation, and UI setup for hint-based navigation.
func (h *handlerState) activateHintModeInternal(activation modecmd.Activation) {
	// Detect refresh before validation so we can clean up on failure
	isRefresh := h.appState.CurrentMode() == domain.ModeHints

	// Reset cycle index on refresh since the hint list is regenerated
	if isRefresh {
		h.cycleHintIndex = -1
	}

	// On refresh, properly escape the active IME and clear search state first.
	// Otherwise the IME is left orphaned during screen or space transitions,
	// where the OS moves focus to the frontmost app.
	if isRefresh && h.hints != nil && h.hints.Context != nil && h.hints.Context.SearchActive() {
		h.cancelHintSearch()
	}

	// Defer bundle ID fetch until after validation (secure input check) to avoid
	// unnecessary AX calls when a password field is focused.
	actionEnum, activated := h.activateModeBase(
		domain.ModeNameHints,
		h.config.Hints.Enabled,
		action.TypeMoveMouse,
		"",
	)
	if !activated {
		h.abandonHintActivation(isRefresh)

		return
	}

	actionString := domain.ActionString(actionEnum)

	if isRefresh {
		// Keep the mode, event tap, and overlay in place for an in-place refresh, so
		// the existing labels stay visible until the fresh scan draws the new set over
		// them. Only indicator polling stops. Skipping SetModeHints on the success path
		// avoids leaving the app idle with the event tap disabled if hint generation
		// fails.
		h.stopIndicatorPolling()
	} else {
		// Coming from another mode, the keyboard is handed over rather than
		// given back: exit now, release at return if nothing is entered. Hints
		// has more ways to give up than any other mode — every
		// abandonHintActivation below, plus the permission dialog that suspends
		// the activation — and the deferred release covers all of them.
		defer h.exitModeForTransition()()
	}

	if actionString == domain.UnknownAction {
		h.logger.Warn("Unknown action string, ignoring")

		h.abandonHintActivation(isRefresh)

		return
	}

	// Always resize overlay to the active screen (where mouse is) before collecting elements.
	// Otherwise the overlay lands on the display the mouse just left.
	var activeScreenBounds image.Rectangle

	if h.system != nil {
		b, err := h.system.ScreenBounds(h.ctx)
		if err == nil {
			activeScreenBounds = b
		} else if !derrors.IsNotSupported(err) {
			h.logger.Warn("Failed to get screen bounds for hints", zap.Error(err))
		}
	}

	h.setScreenBounds(activeScreenBounds)
	// On a fresh activation, take whatever frame is on screen off it (e.g.
	// scroll highlights) before drawing hints. Entering from another mode has
	// already done this through exitMode above; entering from idle has not,
	// and that is the case this covers. A refresh keeps its frame so the
	// existing labels persist until the redraw draws the new set over them.
	if !isRefresh {
		h.clearOverlayFrame()
	}

	if h.hints != nil && h.hints.Context != nil {
		applyHintFlags(h.hints.Context, activation, isRefresh)
	}

	// Fetch bundle ID for hint generation. Validation already passed (secure input check,
	// exclusion check), so this is the only call. Use a dedicated short timeout so slow
	// AX doesn't erode the hint generation budget.
	bundleCtx, bundleCancel := context.WithTimeout(h.ctx, 1*time.Second)
	bundleID, bundleIDErr := h.actionService.FocusedAppBundleID(bundleCtx)

	bundleCancel()

	if bundleIDErr != nil {
		h.logger.Debug("Failed to get focused app bundle ID for hint generation",
			zap.Error(bundleIDErr))
	}

	// Get hints from service. Drawing is intentionally deferred until after
	// active-screen filtering so activation performs one full overlay render.
	ctx, cancel := context.WithTimeout(h.ctx, HintTimeout)
	defer cancel()

	activationStart := time.Now()

	overrides := h.resolveHintOverrides(activation)

	strategy := h.config.Hints.StrategyForApp(bundleID)
	if overrides.strategy != "" {
		strategy = overrides.strategy
	}

	if h.needsScreenCapturePermission(strategy) {
		// The permission request blocks on a modal dialog and h.mu must not be
		// held across it (see the SystemPort contract). The activation is
		// suspended here and re-entered through the outer locked surface once
		// the user decides.
		h.requestScreenCapturePermissionAndResume(activation)

		return
	}

	domainHints, domainHintsErr := h.hintService.GenerateHints(
		ctx,
		activation.FilterRoles,
		activation.FilterTextContains,
		bundleID,
		overrides.strategy,
		overrides.captureScope,
		overrides.labelDirection,
		overrides.splitWord,
	)
	if domainHintsErr != nil {
		h.logger.Error(
			"Failed to show hints",
			zap.Error(domainHintsErr),
			zap.String("action", actionString),
		)

		h.abandonHintActivation(isRefresh)

		return
	}

	debugElapsed(h.logger, activationStart, "GenerateHints completed",
		zap.Int("total_hints", len(domainHints)))

	filteredHints := filterHintsForScreen(domainHints, activeScreenBounds)

	debugElapsed(h.logger, activationStart, "FilterHintsForScreen completed",
		zap.Int("after_filter", len(filteredHints)),
		zap.Int("before_filter", len(domainHints)))

	h.logger.Debug("Filtered hints by screen",
		zap.Int("total_hints", len(domainHints)),
		zap.Int("filtered_hints", len(filteredHints)),
		zap.String("screen_bounds", activeScreenBounds.String()))

	domainHints = filteredHints

	if len(domainHints) == 0 {
		h.logger.Warn("No hints generated for action", zap.String("action", actionString))

		h.abandonHintActivation(isRefresh)

		return
	}

	// Create domain hint collection from generated hints
	hintCollection := domainHint.NewCollection(domainHints)

	// Initialize hint manager and router if not already set up
	// Note: Manager is created once and reused across activations (holds mutable state).
	// Router is recreated each activation (stateless, needs fresh exit keys from config).
	if h.hints.Context.Manager() == nil {
		manager := domainHint.NewManager(h.logger, &h.outer.mu)
		manager.SetUpdateCallback(h.drawHints)
		h.hints.Context.SetManager(manager)
	}

	// Only set mode and enable event tap on initial activation;
	// during refresh these are already in the correct state. The overlay is
	// not switched here: hints hands over a Frame, and switching the overlay
	// belongs to the one place that realizes it.
	if !isRefresh {
		h.enterMode(domain.ModeHints)
	} else {
		// During a refresh (e.g., after Cmd+Tab passthrough) the focused app
		// may have changed. Re-sync the modifier passthrough blacklist so
		// app-specific hotkey overrides for the new app are correctly
		// intercepted instead of being passed through to macOS.
		h.syncModifierPassthrough(domain.ModeHints)
	}

	h.hints.Context.SetRouter(domainHint.NewRouter(h.hints.Context.Manager(), h.logger))

	debugElapsed(h.logger, activationStart, "Manager.SetHints completed")

	// Every activation puts the hints Frame on screen again, fresh or
	// in-place: the refresh path runs after a passthrough or a space change,
	// where the overlay has to be re-shown on the display the user is now on.
	//
	// This sits immediately before SetHints on purpose. Clearing the flag says
	// "the next draw performs the window sequence", so it may only be cleared
	// where the hint manager's pending debounced updates are invalidated in the
	// same locked section — SetHints below bumps the update generation, and on
	// the teardown path Manager.Clear does. Clearing it any earlier would let a
	// debounce timer that fired in between show and switch an overlay this
	// activation may still abandon.
	h.hintsFrameOnScreen = false

	setHintsErr := h.hints.Context.SetHints(hintCollection)
	if setHintsErr != nil {
		h.logger.Error("Failed to set hints in manager", zap.Error(setHintsErr))
		h.exitMode()

		return
	}

	fields := []zap.Field{
		zap.Duration("elapsed", time.Since(activationStart)),
		zap.Int("hint_count", len(domainHints)),
		zap.String("strategy", strategy),
	}
	if activation.Action != nil {
		fields = append(fields, zap.String("action", *activation.Action))
	}

	h.logger.Info("Hints mode activated", fields...)

	if activation.Search != nil && *activation.Search {
		err := h.startHintSearch()
		if err != nil {
			h.logger.Error("Failed to start hint search on activation", zap.Error(err))
		}
	}

	h.startIndicatorPolling(domain.ModeHints)
}

// needsScreenCapturePermission reports whether a screen-capture strategy is
// blocked on the screen-recording permission. Vision and contour both read the
// window's pixels, so both pay the gate. Axtree never touches the screen.
func (h *handlerState) needsScreenCapturePermission(strategy string) bool {
	if strategy != domain.StrategyVision && strategy != domain.StrategyContour {
		return false
	}

	return h.system != nil && !h.system.CheckScreenCapturePermission(h.ctx)
}

// requestScreenCapturePermissionAndResume shows the blocking permission dialog
// on its own goroutine and re-enters the handler through the outer locked
// surface once the user decides. The mode-session token captured here lets the
// resume detect any state change that happened while the dialog was open and
// drop the stale consent.
func (h *handlerState) requestScreenCapturePermissionAndResume(
	activation modecmd.Activation,
) {
	session := h.modeSession
	system := h.system
	ctx := h.ctx
	outer := h.outer

	go func() {
		consent := system.RequestScreenCapturePermission(ctx)
		outer.resumeHintActivationAfterPermission(consent, session, activation)
	}()
}

// resumeHintActivationAfterPermission re-enters the handler once the
// screen-capture permission dialog is answered. A granted consent re-runs the
// activation from the top, which re-reads the screen bounds, focused app and
// strategy that may have changed while the dialog was open; a consent whose
// mode session moved on is dropped.
func (h *Handler) resumeHintActivationAfterPermission(
	consent ports.ScreenCaptureConsent,
	session uint64,
	activation modecmd.Activation,
) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.ctx.Err() != nil || h.modeSession != session {
		h.logger.Debug(
			"Dropping screen-capture consent: state changed while the permission dialog was open",
		)

		return
	}

	switch consent {
	case ports.ScreenCaptureQuit:
		h.shutdown()
	case ports.ScreenCaptureCanceled:
		// Canceled is the platform's answer for "the user said no" *and* for a
		// permission gate that could not be reached at all — a stopped
		// xdg-desktop-portal on KDE, say — because the consent vocabulary has no
		// third word for it. Leaving the mode without a line would make the two
		// indistinguishable from a hint activation that silently did nothing.
		h.logger.Info("Screen capture was not permitted; leaving hints mode")
		h.exitMode()
	case ports.ScreenCaptureGranted:
		// A Granted consent implies the check now passes (the SystemPort
		// contract). If it does not, re-running the activation would show the
		// dialog again in an unbounded loop, so drop the consent instead.
		if h.system != nil && !h.system.CheckScreenCapturePermission(h.ctx) {
			h.logger.Warn(
				"Screen-capture consent granted but the permission check still fails; not retrying",
			)

			return
		}

		h.activateHintModeInternal(activation)
	}
}
