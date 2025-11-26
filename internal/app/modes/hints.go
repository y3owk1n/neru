package modes

import (
	"context"
	"time"

	"github.com/y3owk1n/neru/internal/application/ports"
	"github.com/y3owk1n/neru/internal/domain"
	domainHint "github.com/y3owk1n/neru/internal/domain/hint"
	"github.com/y3owk1n/neru/internal/features/hints"
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

	if mode == domain.ModeHints {
		h.activateHintModeWithAction(action)

		return
	}

	if mode == domain.ModeGrid {
		h.activateGridModeWithAction(action)

		return
	}

	h.Logger.Warn("Unknown mode", zap.String("mode", domain.ModeString(mode)))
}

// activateHintModeWithAction activates hint mode with optional action parameter.
func (h *Handler) activateHintModeWithAction(action *string) {
	h.activateHintModeInternal(false, action)
}

// activateHintModeInternal activates hint mode with option to preserve action mode state and optional action.
func (h *Handler) activateHintModeInternal(preserveActionMode bool, action *string) {
	// Validate mode activation
	domainHintsErr := h.validateModeActivation("hints", h.Config.Hints.Enabled)
	if domainHintsErr != nil {
		h.Logger.Warn("Hint mode activation failed", zap.Error(domainHintsErr))

		return
	}

	// Prepare for mode activation (reset scroll, capture cursor)
	h.prepareForModeActivation()

	actionEnum := domain.ActionMoveMouse
	actionString := domain.ActionString(actionEnum)
	h.Logger.Info("Activating hint mode", zap.String("action", actionString))

	if !preserveActionMode {
		// Skip cursor restoration when transitioning within hint mode
		if h.AppState.CurrentMode() == domain.ModeHints {
			h.performModeSpecificCleanup()
			h.performCommonCleanup()
			// Skip cursor restoration
		} else {
			h.ExitMode()
		}
	}

	if actionString == domain.UnknownAction {
		h.Logger.Warn("Unknown action string, ignoring")

		return
	}

	// Always resize overlay to the active screen (where mouse is) before collecting elements.
	// This ensures proper positioning when switching between multiple displays.
	h.OverlayManager.ResizeToActiveScreenSync()
	h.AppState.SetHintOverlayNeedsRefresh(false)

	// Use new HintService to show hints
	context, cancel := context.WithTimeout(context.Background(), HintTimeout)
	defer cancel()

	filter := ports.DefaultElementFilter()

	// Populate filter with configuration
	filter.IncludeMenubar = h.Config.Hints.IncludeMenubarHints
	filter.AdditionalMenubarTargets = h.Config.Hints.AdditionalMenubarHintsTargets
	filter.IncludeDock = h.Config.Hints.IncludeDockHints
	filter.IncludeNotificationCenter = h.Config.Hints.IncludeNCHints

	// Get hints from service
	domainHints, domainHintsErr := h.HintService.ShowHints(context, filter)
	if domainHintsErr != nil {
		h.Logger.Error(
			"Failed to show hints",
			zap.Error(domainHintsErr),
			zap.String("action", actionString),
		)

		return
	}

	if len(domainHints) == 0 {
		h.Logger.Warn("No hints generated for action", zap.String("action", actionString))

		return
	}

	// Create domain hint collection
	hintCollection := domainHint.NewCollection(domainHints)

	// Initialize domain manager with overlay update callback
	if h.Hints.Context.Manager() == nil {
		manager := domainHint.NewManager(h.Logger)
		// Set callback to update overlay when hints are filtered
		manager.SetUpdateCallback(func(filteredHints []*domainHint.Interface) {
			if h.Hints.Overlay == nil {
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

			drawHintsErr := h.Hints.Overlay.DrawHintsWithStyle(overlayHints, h.Hints.Style)
			if drawHintsErr != nil {
				h.Logger.Error("Failed to update hints overlay", zap.Error(drawHintsErr))
			}
		})
		h.Hints.Context.SetManager(manager)
	}

	// Initialize domain router
	if h.Hints.Context.Router() == nil {
		h.Hints.Context.SetRouter(domainHint.NewRouter(h.Hints.Context.Manager(), h.Logger))
	}

	// Set hints in context (this also updates the manager)
	h.Hints.Context.SetHints(hintCollection)

	// Store pending action if provided
	h.Hints.Context.SetPendingAction(action)

	if action != nil {
		h.Logger.Info("Hints mode activated with pending action", zap.String("action", *action))
	}

	h.SetModeHints()
	h.Logger.Info("Hints mode activated")
}

// handleHintsActionKey handles action keys when in hints action mode.
func (h *Handler) handleHintsActionKey(key string) {
	h.handleActionKey(key, "Hints")
}
