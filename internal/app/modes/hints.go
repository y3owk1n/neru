package modes

import (
	"context"
	"image"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
	domainHint "github.com/y3owk1n/neru/internal/domain/hint"
	"github.com/y3owk1n/neru/internal/ports"
)

// debugElapsed logs the duration since start with the given message.
func debugElapsed(logger *zap.Logger, start time.Time, msg string, fields ...zap.Field) {
	logger.Debug(msg, append(fields, zap.Duration("elapsed", time.Since(start)))...)
}

// currentHintStyle resolves theme-aware hint overlay colors from the live
// config, matching search-input and mode-indicator draw paths. Caller must
// hold h.mu.
func (h *handlerState) currentHintStyle() hints.StyleMode {
	style := hints.BuildStyle(h.config.Hints, h.themeProvider)
	if h.hints != nil {
		h.hints.Style = style
	}

	return style
}

// ModeActivationOptions configures a mode activation request.
type ModeActivationOptions struct {
	Action                *string
	Modifier              *string
	OnExit                []string
	Repeat                *bool
	CursorFollowSelection *bool
	ZoomToDepth           *int
	FilterRoles           []string
	FilterTextContains    []string
	Search                *bool
	HideOnEmptySearch     *bool
	Strategy              *string
	LabelDirection        *string
	Toggle                *bool
	SplitWord             *bool
}

const (
	// HintTimeout is the timeout for hint operations.
	HintTimeout = 5 * time.Second
)

// ActivateMode activates a mode with a given action (for hints mode).
func (h *Handler) ActivateMode(mode domain.Mode) {
	h.ActivateModeWithOptions(mode, ModeActivationOptions{})
}

// ActivateModeWithAction activates a mode with an optional action parameter.
func (h *Handler) ActivateModeWithAction(mode domain.Mode, action *string) {
	h.ActivateModeWithOptions(mode, ModeActivationOptions{Action: action})
}

// ActivateModeWithOptions activates a mode with an optional action and repeat flag.
// When repeat is true the mode re-activates after performing the pending action.
func (h *Handler) ActivateModeWithOptions(mode domain.Mode, opts ModeActivationOptions) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Toggle: if the mode is already active and --toggle was specified,
	// exit to idle instead of re-activating
	if opts.Toggle != nil && *opts.Toggle && h.appState.CurrentMode() == mode {
		h.exitMode()

		return
	}

	if mode == domain.ModeIdle {
		h.exitMode()

		return
	}

	modeImpl, exists := h.modes[mode]
	if !exists {
		h.logger.Warn("Unknown mode", zap.String("mode", domain.ModeString(mode)))

		return
	}

	// Normalize --on-exit for external (re-)activations. This method is the sole
	// entry point for user-driven activations (IPC, hotkeys, systray); internal
	// refreshes (repeat re-activation, space/screen change, cycle) bypass it and
	// call the activate* helpers directly with a nil onExit to preserve the
	// stored steps. An omitted --on-exit on a fresh external command must
	// clear any steps left over from a prior activation of the same mode
	// rather than inheriting them, so a later completed action does not run a
	// stale command. A nil slice reaching those helpers means "preserve"; the
	// non-nil empty slice substituted here means "clear", and it is a no-op at
	// dispatch time.
	if opts.OnExit == nil {
		opts.OnExit = []string{}
	}

	modeImpl.Activate(opts)
}

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
func (h *handlerState) activateHintModeWithAction(opts ModeActivationOptions) {
	h.activateHintModeInternal(opts)

	// Repeat is stored after activation, by which point the context exists.
	if opts.Repeat != nil && *opts.Repeat && h.hints != nil && h.hints.Context != nil {
		h.hints.Context.SetRepeat(true)
	}
}

// activateHintModeInternal activates hint mode with an optional action.
// It handles mode validation, overlay positioning, element collection, hint
// generation, and UI setup for hint-based navigation.
func (h *handlerState) activateHintModeInternal(opts ModeActivationOptions) {
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
		h.exitMode()
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
	// On a fresh activation, clear leftover overlay content (e.g. scroll highlights)
	// before drawing hints. A refresh keeps its overlay so the existing labels persist
	// until the redraw draws the new set over them.
	if !isRefresh {
		h.overlayManager.Clear()
	}

	h.appState.SetHintOverlayNeedsRefresh(false)

	if h.hints != nil && h.hints.Context != nil {
		applyHintOptions(h.hints.Context, opts, isRefresh)
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

	overrides := h.resolveHintOverrides(opts)

	strategy := h.config.Hints.StrategyForApp(bundleID)
	if overrides.strategy != "" {
		strategy = overrides.strategy
	}

	var permissionOk bool

	activeScreenBounds, bundleID, strategy, permissionOk = h.ensureScreenCapturePermissions(
		activeScreenBounds,
		bundleID,
		strategy,
		overrides.strategy,
	)
	if !permissionOk {
		h.abandonHintActivation(isRefresh)

		return
	}

	domainHints, domainHintsErr := h.hintService.GenerateHints(
		ctx,
		opts.FilterRoles,
		opts.FilterTextContains,
		bundleID,
		overrides.strategy,
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
	// during refresh these are already in the correct state.
	if !isRefresh {
		h.setMode(domain.ModeHints, overlay.ModeHints)
	} else {
		// During a refresh (e.g., after Cmd+Tab passthrough) the focused app
		// may have changed. Re-sync the modifier passthrough blacklist so
		// app-specific hotkey overrides for the new app are correctly
		// intercepted instead of being passed through to macOS.
		h.syncModifierPassthrough(domain.ModeHints)
	}

	h.hints.Context.SetRouter(domainHint.NewRouter(h.hints.Context.Manager(), h.logger))

	debugElapsed(h.logger, activationStart, "Manager.SetHints completed")

	setHintsErr := h.hints.Context.SetHints(hintCollection)
	if setHintsErr != nil {
		h.logger.Error("Failed to set hints in manager", zap.Error(setHintsErr))
		h.exitMode()

		return
	}

	h.overlayManager.ResizeToActiveScreen()
	h.overlayManager.Show()

	fields := []zap.Field{
		zap.Duration("elapsed", time.Since(activationStart)),
		zap.Int("hint_count", len(domainHints)),
		zap.String("strategy", strategy),
	}
	if opts.Action != nil {
		fields = append(fields, zap.String("action", *opts.Action))
	}

	h.logger.Info("Hints mode activated", fields...)

	if opts.Search != nil && *opts.Search {
		err := h.startHintSearch()
		if err != nil {
			h.logger.Error("Failed to start hint search on activation", zap.Error(err))
		}
	}

	h.startIndicatorPolling(domain.ModeHints)
}

// ensureScreenCapturePermissions checks and requests screen capture permissions.
// It releases h.mu during the modal prompt to avoid blocking other threads.
// Returns the updated activeScreenBounds, bundleID, strategy, and whether it is safe to proceed.
func (h *handlerState) ensureScreenCapturePermissions(
	activeScreenBounds image.Rectangle,
	bundleID string,
	strategy string,
	strategyVal string,
) (image.Rectangle, string, string, bool) {
	if strategy != domain.StrategyVision {
		return activeScreenBounds, bundleID, strategy, true
	}

	if h.system == nil || h.system.CheckScreenCapturePermission(h.ctx) {
		return activeScreenBounds, bundleID, strategy, true
	}

	session := h.modeSession

	// Sanctioned mid-flight unlock: the permission request blocks on a modal
	// dialog and the lock must not be held across it (see the SystemPort
	// contract). The mode-session token below detects any state change that
	// happened while unlocked, and the caller bails when it did.
	h.outer.mu.Unlock()

	consent := h.system.RequestScreenCapturePermission(h.ctx)

	h.outer.mu.Lock()

	// Check if state changed while we were unlocked.
	if h.ctx.Err() != nil || h.modeSession != session {
		h.logger.Debug(
			"Aborting hint mode activation: state changed or context canceled while waiting for permission dialog",
		)

		return activeScreenBounds, bundleID, strategy, false
	}

	if consent == ports.ScreenCaptureQuit {
		h.shutdown()

		return activeScreenBounds, bundleID, strategy, false
	}

	if consent == ports.ScreenCaptureCanceled {
		h.exitMode()

		return activeScreenBounds, bundleID, strategy, false
	}

	// Re-read screen bounds under the lock in case they changed while the modal was open.
	if h.system != nil {
		b, err := h.system.ScreenBounds(h.ctx)
		if err == nil {
			activeScreenBounds = b
			h.setScreenBounds(activeScreenBounds)
		}
	}

	// Re-fetch bundle ID under the lock since the focused app might have changed while the modal was open.
	bundleCtx, bundleCancel := context.WithTimeout(h.ctx, 1*time.Second)
	newBundleID, bundleIDErr := h.actionService.FocusedAppBundleID(bundleCtx)

	bundleCancel()

	if bundleIDErr == nil {
		bundleID = newBundleID
	} else {
		h.logger.Debug("Failed to re-fetch focused app bundle ID for hint generation",
			zap.Error(bundleIDErr))
	}

	// Re-evaluate strategy in case the focused app changed.
	strategy = h.config.Hints.StrategyForApp(bundleID)
	if strategyVal != "" {
		strategy = strategyVal
	}

	return activeScreenBounds, bundleID, strategy, true
}
