package modes

import (
	"context"
	"time"

	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/core/domain"
	domainHint "github.com/y3owk1n/neru/internal/core/domain/hint"
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
	if mode == domain.ModeIdle {
		h.ExitMode()

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
func (h *Handler) activateHintModeInternal(preserveActionMode bool, action *string) {
	actionEnum, ok := h.activateModeBase(
		domain.ModeNameHints,
		h.config.Hints.Enabled,
		domain.ActionMoveMouse,
	)
	if !ok {
		return
	}

	actionString := domain.ActionString(actionEnum)

	if !preserveActionMode {
		// Handle mode transitions: if already in hints mode, do partial cleanup to preserve state;
		// otherwise exit completely to reset all state
		if h.appState.CurrentMode() == domain.ModeHints {
			h.performModeSpecificCleanup()
			h.performCommonCleanup()
			// Skip cursor restoration to maintain position during hint mode transitions
		} else {
			h.ExitMode()
		}
	}

	if actionString == domain.UnknownAction {
		h.logger.Warn("Unknown action string, ignoring")

		return
	}

	// Always resize overlay to the active screen (where mouse is) before collecting elements.
	// This ensures proper positioning when switching between multiple displays.
	h.overlayManager.ResizeToActiveScreenSync()
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

	if len(domainHints) == 0 {
		h.logger.Warn("No hints generated for action", zap.String("action", actionString))

		return
	}

	// Create domain hint collection from generated hints
	hintCollection := domainHint.NewCollection(domainHints)

	// Initialize hint manager and router if not already set up
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
				overlayHints[index] = hints.NewHint(
					hint.Label(),
					hint.Position(),
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
	if h.hints.Context.Router() == nil {
		h.hints.Context.SetRouter(domainHint.NewRouter(h.hints.Context.Manager(), h.logger))
	}

	h.hints.Context.SetHints(hintCollection)

	// Store pending action if provided
	h.hints.Context.SetPendingAction(action)

	if action != nil {
		h.logger.Info("Hints mode activated with pending action", zap.String("action", *action))
	}

	h.SetModeHints()
	h.logger.Info("Hints mode activated")
}

// handleHintsActionKey handles action keys when in hints action mode.
func (h *Handler) handleHintsActionKey(key string) {
	h.handleActionKey(key, "Hints")
}
