package modes

import (
	"context"
	"image"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	domainHint "github.com/y3owk1n/neru/internal/core/domain/hint"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/platform"
	"github.com/y3owk1n/neru/internal/ui/overlay"
)

// debugElapsed logs the duration since start with the given message.
func debugElapsed(logger *zap.Logger, start time.Time, msg string, fields ...zap.Field) {
	logger.Debug(msg, append(fields, zap.Duration("elapsed", time.Since(start)))...)
}

// currentHintStyleLocked resolves theme-aware hint overlay colors from the live
// config, matching search-input and mode-indicator draw paths. Caller must
// hold h.mu.
func (h *Handler) currentHintStyleLocked() hints.StyleMode {
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
	Repeat                *bool
	CursorFollowSelection *bool
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
		h.exitModeLocked()

		return
	}

	if mode == domain.ModeIdle {
		h.exitModeLocked()

		return
	}

	modeImpl, exists := h.modes[mode]
	if !exists {
		h.logger.Warn("Unknown mode", zap.String("mode", domain.ModeString(mode)))

		return
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
func (h *Handler) activateHintModeWithAction(
	action *string,
	modifier *string,
	repeat *bool,
	cursorFollowSelection *bool,
	filterRoles []string,
	filterTextContains []string,
	search *bool,
	hideOnEmptySearch *bool,
	strategy *string,
	labelDirection *string,
	splitWord *bool,
) {
	h.activateHintModeInternal(
		action,
		modifier,
		cursorFollowSelection,
		filterRoles,
		filterTextContains,
		search,
		hideOnEmptySearch,
		strategy,
		labelDirection,
		splitWord,
	)

	// Store repeat flag after activation so the context is already initialized.
	if repeat != nil && *repeat && h.hints != nil && h.hints.Context != nil {
		h.hints.Context.SetRepeat(true)
	}
}

// activateHintModeInternal activates hint mode with optional action.
// It handles mode validation, overlay positioning, element collection, hint generation,
// and UI setup for hint-based navigation.

func (h *Handler) activateHintModeInternal(
	actionStr *string,
	modifier *string,
	cursorFollowSelection *bool,
	filterRoles []string,
	filterTextContains []string,
	search *bool,
	hideOnEmptySearch *bool,
	strategyOverride *string,
	labelDirectionOverride *string,
	splitWordOverride *bool,
) {
	// Detect refresh before validation so we can clean up on failure
	isRefresh := h.appState.CurrentMode() == domain.ModeHints

	// Reset cycle index on refresh since the hint list is regenerated
	if isRefresh {
		h.cycleHintIndex = -1
	}

	// On refresh, properly escape the active IME and clear search state first.
	// This prevents the IME from becoming orphaned/unfocused during screen/space
	// transitions where the OS moves focus to the frontmost app.
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
		// If validation fails during a refresh (e.g., secure input activated,
		// focused app became excluded), exit cleanly instead of leaving stale
		// hints on the overlay.
		if isRefresh {
			h.exitModeLocked()
		}

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
		h.exitModeLocked()
	}

	if actionString == domain.UnknownAction {
		h.logger.Warn("Unknown action string, ignoring")

		if isRefresh {
			h.exitModeLocked()
		}

		return
	}

	// Always resize overlay to the active screen (where mouse is) before collecting elements.
	// This ensures proper positioning when switching between multiple displays.
	var activeScreenBounds image.Rectangle

	if h.system != nil {
		b, err := h.system.ScreenBounds(h.ctx)
		if err == nil {
			activeScreenBounds = b
		} else if !derrors.IsNotSupported(err) {
			h.logger.Warn("Failed to get screen bounds for hints", zap.Error(err))
		}
	}

	h.screenBounds = activeScreenBounds
	// On a fresh activation, clear leftover overlay content (e.g. scroll highlights)
	// before drawing hints. A refresh keeps its overlay so the existing labels persist
	// until the redraw draws the new set over them.
	if !isRefresh {
		h.overlayManager.Clear()
	}

	h.appState.SetHintOverlayNeedsRefresh(false)

	if h.hints != nil && h.hints.Context != nil {
		if isRefresh {
			// On refresh preserve existing context flags for any field not
			// explicitly provided. This prevents configured action strings
			// (e.g. space change → MC callback → "hints" with no args) from
			// overwriting the user's custom --action flag.
			if actionStr != nil {
				h.hints.Context.SetPendingAction(actionStr)
			}

			if modifier != nil {
				h.hints.Context.SetPendingModifier(modifier)
			}

			if cursorFollowSelection != nil {
				h.hints.Context.SetCursorFollowSelection(*cursorFollowSelection)
			}

			if filterRoles != nil {
				h.hints.Context.SetFilterRoles(filterRoles)
			}

			if filterTextContains != nil {
				h.hints.Context.SetFilterTextContains(filterTextContains)
			}

			if search != nil {
				h.hints.Context.SetStartWithSearch(*search)
			}

			if hideOnEmptySearch != nil {
				h.hints.Context.SetHideOnEmptySearch(*hideOnEmptySearch)
			}

			if strategyOverride != nil {
				h.hints.Context.SetStrategyOverride(*strategyOverride)
			}

			if labelDirectionOverride != nil {
				h.hints.Context.SetLabelDirectionOverride(*labelDirectionOverride)
			}

			if splitWordOverride != nil {
				h.hints.Context.SetSplitWord(*splitWordOverride)
			}
		} else {
			h.hints.Context.SetPendingAction(actionStr)
			h.hints.Context.SetPendingModifier(modifier)
			h.hints.Context.SetRepeat(false)
			h.hints.Context.SetCursorFollowSelection(resolveCursorFollowSelection(
				domain.ModeHints,
				cursorFollowSelection,
			))
			h.hints.Context.SetFilterRoles(filterRoles)
			h.hints.Context.SetFilterTextContains(filterTextContains)
			h.hints.Context.SetStartWithSearch(search != nil && *search)
			h.hints.Context.SetHideOnEmptySearch(hideOnEmptySearch != nil && *hideOnEmptySearch)

			if strategyOverride != nil {
				h.hints.Context.SetStrategyOverride(*strategyOverride)
			} else {
				h.hints.Context.SetStrategyOverride("")
			}

			if labelDirectionOverride != nil {
				h.hints.Context.SetLabelDirectionOverride(*labelDirectionOverride)
			} else {
				h.hints.Context.SetLabelDirectionOverride("")
			}

			if splitWordOverride != nil {
				h.hints.Context.SetSplitWord(*splitWordOverride)
			} else {
				h.hints.Context.SetSplitWord(false)
			}
		}
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

	strategyVal := ""
	if h.hints != nil && h.hints.Context != nil {
		strategyVal = h.hints.Context.StrategyOverride()
	} else if strategyOverride != nil {
		strategyVal = *strategyOverride
	}

	strategy := h.config.Hints.StrategyForApp(bundleID)
	if strategyVal != "" {
		strategy = strategyVal
	}

	labelDirectionVal := ""
	if h.hints != nil && h.hints.Context != nil {
		labelDirectionVal = h.hints.Context.LabelDirectionOverride()
	} else if labelDirectionOverride != nil {
		labelDirectionVal = *labelDirectionOverride
	}

	splitWordVal := false
	if h.hints != nil && h.hints.Context != nil {
		splitWordVal = h.hints.Context.SplitWord()
	} else if splitWordOverride != nil {
		splitWordVal = *splitWordOverride
	}

	var permissionOk bool

	activeScreenBounds, bundleID, strategy, permissionOk = h.ensureScreenCapturePermissionsLocked(
		activeScreenBounds,
		bundleID,
		strategy,
		strategyVal,
	)
	if !permissionOk {
		// On a refresh, exit through exitModeLocked so the stale overlay is cleared,
		// matching the other refresh abort paths.
		if isRefresh {
			h.exitModeLocked()
		}

		return
	}

	domainHints, domainHintsErr := h.hintService.GenerateHints(
		ctx,
		filterRoles,
		filterTextContains,
		bundleID,
		strategyVal,
		labelDirectionVal,
		splitWordVal,
	)
	if domainHintsErr != nil {
		h.logger.Error(
			"Failed to show hints",
			zap.Error(domainHintsErr),
			zap.String("action", actionString),
		)

		if isRefresh {
			h.exitModeLocked()
		}

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

		if isRefresh {
			h.exitModeLocked()
		}

		return
	}

	// Create domain hint collection from generated hints
	hintCollection := domainHint.NewCollection(domainHints)

	// Initialize hint manager and router if not already set up
	// Note: Manager is created once and reused across activations (holds mutable state).
	// Router is recreated each activation (stateless, needs fresh exit keys from config).
	if h.hints.Context.Manager() == nil {
		manager := domainHint.NewManager(h.logger, &h.mu)
		// Set callback to update overlay when hints are filtered
		manager.SetUpdateCallback(func(filteredHints []*domainHint.Interface) {
			// Caller must hold h.mu. Synchronous call sites (SetHints, Reset,
			// HandleInput) already hold it. The async debouncedUpdate timer
			// acquires it via the external mutex set below.
			if h.hints.Overlay == nil {
				return
			}

			screenBounds := h.screenBounds

			// Convert domain hints to overlay hints for rendering
			overlayHints := make([]*hints.Hint, len(filteredHints))
			for index, hint := range filteredHints {
				// Convert screen-absolute coordinates to overlay-local coordinates
				localPos := image.Point{
					X: hint.Position().X - screenBounds.Min.X,
					Y: hint.Position().Y - screenBounds.Min.Y,
				}
				overlayHints[index] = hints.NewHint(
					hint.Label(),
					localPos,
					hint.Element().Bounds().Size(),
					hint.MatchedPrefix(),
				)
			}

			drawHintsErr := h.overlayManager.DrawHintsWithStyle(
				overlayHints,
				h.currentHintStyleLocked(),
			)
			if drawHintsErr != nil {
				h.logger.Error("Failed to update hints overlay", zap.Error(drawHintsErr))
			}
		})
		h.hints.Context.SetManager(manager)
	}

	// Only set mode and enable event tap on initial activation;
	// during refresh these are already in the correct state.
	if !isRefresh {
		h.setModeLocked(domain.ModeHints, overlay.ModeHints)
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
		h.exitModeLocked()

		return
	}

	h.overlayManager.ResizeToActiveScreen()
	h.overlayManager.Show()

	fields := []zap.Field{
		zap.Duration("elapsed", time.Since(activationStart)),
		zap.Int("hint_count", len(domainHints)),
		zap.String("strategy", strategy),
	}
	if actionStr != nil {
		fields = append(fields, zap.String("action", *actionStr))
	}

	h.logger.Info("Hints mode activated", fields...)

	if search != nil && *search {
		err := h.startHintSearchLocked()
		if err != nil {
			h.logger.Error("Failed to start hint search on activation", zap.Error(err))
		}
	}

	h.startIndicatorPolling(domain.ModeHints)
}

// ensureScreenCapturePermissionsLocked checks and requests screen capture permissions.
// It releases h.mu during the modal prompt to avoid blocking other threads.
// Returns the updated activeScreenBounds, bundleID, strategy, and whether it is safe to proceed.
func (h *Handler) ensureScreenCapturePermissionsLocked(
	activeScreenBounds image.Rectangle,
	bundleID string,
	strategy string,
	strategyVal string,
) (image.Rectangle, string, string, bool) {
	if strategy != config.StrategyVision {
		return activeScreenBounds, bundleID, strategy, true
	}

	if platform.CheckScreenCapturePermissions() {
		return activeScreenBounds, bundleID, strategy, true
	}

	session := h.modeSession
	h.mu.Unlock()

	choice := platform.ShowScreenCapturePermissionAlert()

	h.mu.Lock()

	// Check if state changed while we were unlocked.
	if h.ctx.Err() != nil || h.modeSession != session {
		h.logger.Debug(
			"Aborting hint mode activation: state changed or context canceled while waiting for permission dialog",
		)

		return activeScreenBounds, bundleID, strategy, false
	}

	if choice == platform.ScreenCapturePermissionStartupQuit {
		h.shutdown()

		return activeScreenBounds, bundleID, strategy, false
	}

	if choice == platform.ScreenCapturePermissionStartupCancel {
		h.exitModeLocked()

		return activeScreenBounds, bundleID, strategy, false
	}

	// Re-read screen bounds under the lock in case they changed while the modal was open.
	if h.system != nil {
		b, err := h.system.ScreenBounds(h.ctx)
		if err == nil {
			activeScreenBounds = b
			h.screenBounds = activeScreenBounds
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
