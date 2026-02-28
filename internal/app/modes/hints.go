package modes

import (
	"context"
	"image"
	"time"

	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	domainHint "github.com/y3owk1n/neru/internal/core/domain/hint"
	"github.com/y3owk1n/neru/internal/core/infra/bridge"
	"go.uber.org/zap"
)

const (
	// HintTimeout is the timeout for hint operations.
	HintTimeout = 5 * time.Second
)

// ActivateMode activates a mode with a given action (for hints mode).
func (h *Handler) ActivateMode(mode domain.Mode) {
	h.ActivateModeWithAction(mode, nil)
}

// ActivateModeWithAction activates a mode with an optional action parameter.
func (h *Handler) ActivateModeWithAction(mode domain.Mode, action *string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if mode == domain.ModeIdle {
		h.exitModeLocked()

		return
	}

	modeImpl, exists := h.modes[mode]
	if !exists {
		h.logger.Warn("Unknown mode", zap.String("mode", domain.ModeString(mode)))

		return
	}

	modeImpl.Activate(action)
}

// activateHintModeWithAction activates hint mode with optional action parameter.
func (h *Handler) activateHintModeWithAction(action *string) {
	h.activateHintModeInternal(false, action)
}

// activateHintModeInternal activates hint mode with option to preserve action mode state and optional action.
// It handles mode validation, overlay positioning, element collection, hint generation,
// and UI setup for hint-based navigation.
// NOTE: preserveActionMode is always passed as false by current callers but retained for potential future use.
// The unparam linter is suppressed because while all current calls pass false, removing this parameter
// would be a breaking change if future callers need the preserve behavior.
//
//nolint:unparam
func (h *Handler) activateHintModeInternal(preserveActionMode bool, actionStr *string) {
	actionEnum, ok := h.activateModeBase(
		domain.ModeNameHints,
		h.config.Hints.Enabled,
		action.TypeMoveMouse,
	)
	if !ok {
		return
	}

	actionString := domain.ActionString(actionEnum)

	if !preserveActionMode {
		// Handle mode transitions: if already in hints mode, do partial cleanup to preserve state;
		// otherwise exit completely to reset all state
		if h.appState.CurrentMode() == domain.ModeHints {
			// During refresh, only clear overlay and stop polling but do NOT change mode
			// or disable event tap. The success path will call SetModeHints() to restore state.
			// This prevents leaving the app in idle mode with event tap disabled if hint
			// generation fails.
			h.overlayManager.Clear()
			h.stopModeIndicatorPolling()

			// Stop any pending refresh timer to prevent stale re-activation
			if h.refreshHintsTimer != nil {
				h.refreshHintsTimer.Stop()
				h.refreshHintsTimer = nil
			}
		} else {
			h.exitModeLocked()
		}
	}

	if actionString == domain.UnknownAction {
		h.logger.Warn("Unknown action string, ignoring")

		return
	}

	// Always resize overlay to the active screen (where mouse is) before collecting elements.
	// This ensures proper positioning when switching between multiple displays.
	activeScreenBounds := bridge.ActiveScreenBounds()
	h.screenBounds = activeScreenBounds
	h.overlayManager.ResizeToActiveScreen()

	// Clear any previous overlay content (e.g., scroll highlights) before drawing hints.
	// This prevents scroll highlights from persisting when switching from scroll mode to hints mode.
	h.overlayManager.Clear()
	h.appState.SetHintOverlayNeedsRefresh(false)

	// Use new HintService to show hints
	ctx, cancel := context.WithTimeout(context.Background(), HintTimeout)
	defer cancel()

	// Get hints from service
	domainHints, domainHintsErr := h.hintService.ShowHints(ctx)
	if domainHintsErr != nil {
		h.logger.Error(
			"Failed to show hints",
			zap.Error(domainHintsErr),
			zap.String("action", actionString),
		)

		return
	}

	// Filter hints to only those on the active screen for multi-monitor support,
	// and deduplicate by position so that downstream code (overlay incremental
	// updates, Objective-C NeruDrawIncrementHints) can safely use position as a
	// unique key without silently dropping entries.
	filteredHints := make([]*domainHint.Interface, 0, len(domainHints))
	seenPositions := make(map[image.Point]struct{}, len(domainHints))

	for _, hint := range domainHints {
		hintBounds := hint.Element().Bounds()
		hintCenter := image.Point{
			X: hintBounds.Min.X + hintBounds.Dx()/2,
			Y: hintBounds.Min.Y + hintBounds.Dy()/2,
		}

		// Include hint if its center is within the active screen bounds
		if !hintCenter.In(activeScreenBounds) {
			continue
		}
		// Skip duplicate positions — two hints at the same pixel would
		// visually overlap and confuse the incremental update logic.
		if _, exists := seenPositions[hintCenter]; exists {
			continue
		}

		seenPositions[hintCenter] = struct{}{}

		filteredHints = append(filteredHints, hint)
	}

	h.logger.Debug("Filtered hints by screen",
		zap.Int("total_hints", len(domainHints)),
		zap.Int("filtered_hints", len(filteredHints)),
		zap.String("screen_bounds", activeScreenBounds.String()))

	domainHints = filteredHints

	if len(domainHints) == 0 {
		h.logger.Warn("No hints generated for action", zap.String("action", actionString))

		return
	}

	// Create domain hint collection from generated hints
	hintCollection := domainHint.NewCollection(domainHints)

	// Initialize hint manager and router if not already set up
	// Note: Manager is created once and reused across activations (holds mutable state).
	// Router is recreated each activation (stateless, needs fresh exit keys from config).
	if h.hints.Context.Manager() == nil {
		manager := domainHint.NewManager(h.logger)
		// Set callback to update overlay when hints are filtered
		manager.SetUpdateCallback(func(filteredHints []*domainHint.Interface) {
			if h.hints.Overlay == nil {
				return
			}
			// Convert domain hints to overlay hints for rendering
			overlayHints := make([]*hints.Hint, len(filteredHints))
			for index, hint := range filteredHints {
				// Convert screen-absolute coordinates to overlay-local coordinates
				localPos := image.Point{
					X: hint.Position().X - h.screenBounds.Min.X,
					Y: hint.Position().Y - h.screenBounds.Min.Y,
				}
				overlayHints[index] = hints.NewHint(
					hint.Label(),
					localPos,
					hint.Element().Bounds().Size(),
					hint.MatchedPrefix(),
				)
			}

			drawHintsErr := h.overlayManager.DrawHintsWithStyle(overlayHints, h.hints.Style)
			if drawHintsErr != nil {
				h.logger.Error("Failed to update hints overlay", zap.Error(drawHintsErr))
			}
		})
		h.hints.Context.SetManager(manager)
	}

	// Initialize domain router for hint navigation
	exitKeys := h.config.General.ModeExitKeys
	if len(exitKeys) == 0 {
		exitKeys = DefaultModeExitKeys()
	}

	h.hints.Context.SetRouter(
		domainHint.NewRouterWithExitKeys(h.hints.Context.Manager(), h.logger, exitKeys),
	)

	h.hints.Context.SetHints(hintCollection)

	// Store pending action if provided
	h.hints.Context.SetPendingAction(actionStr)

	if actionStr != nil {
		h.logger.Info("Hints mode activated with pending action", zap.String("action", *actionStr))
	}

	h.SetModeHints()
	h.logger.Info("Hints mode activated")

	h.startModeIndicatorPolling(domain.ModeHints)
}
