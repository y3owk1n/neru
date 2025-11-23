package modes

import (
	"context"
	"time"

	"github.com/y3owk1n/neru/internal/application/ports"
	"github.com/y3owk1n/neru/internal/domain"
	domainHint "github.com/y3owk1n/neru/internal/domain/hint"
	"github.com/y3owk1n/neru/internal/features/hints"
	infra "github.com/y3owk1n/neru/internal/infra/accessibility"
	"go.uber.org/zap"
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
	h.Logger.Warn("Unknown mode", zap.String("mode", domain.GetModeString(mode)))
}

// activateHintModeWithAction activates hint mode with optional action parameter.
func (h *Handler) activateHintModeWithAction(action *string) {
	h.activateHintModeInternal(false, action)
}

// activateHintModeInternal activates hint mode with option to preserve action mode state and optional action.
func (h *Handler) activateHintModeInternal(preserveActionMode bool, action *string) {
	// Validate mode activation
	err := h.validateModeActivation("hints", h.Config.Hints.Enabled)
	if err != nil {
		return
	}

	// Prepare for mode activation (reset scroll, capture cursor)
	h.prepareForModeActivation()

	actionEnum := domain.ActionMoveMouse
	actionString := domain.GetActionString(actionEnum)
	h.Logger.Info("Activating hint mode", zap.String("action", actionString))

	if !preserveActionMode {
		// Skip cursor restoration when transitioning within hint mode
		if h.State.CurrentMode() == domain.ModeHints {
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
	h.State.SetHintOverlayNeedsRefresh(false)

	// Use new HintService to show hints
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := ports.DefaultElementFilter()

	// Populate filter with configuration
	filter.IncludeMenubar = h.Config.Hints.IncludeMenubarHints
	filter.AdditionalMenubarTargets = h.Config.Hints.AdditionalMenubarHintsTargets
	filter.IncludeDock = h.Config.Hints.IncludeDockHints
	filter.IncludeNotificationCenter = h.Config.Hints.IncludeNCHints

	// Get hints from service
	domainHints, err := h.HintService.ShowHints(ctx, filter)
	if err != nil {
		h.Logger.Error("Failed to show hints", zap.Error(err), zap.String("action", actionString))
		return
	}

	if len(domainHints) == 0 {
		h.Logger.Warn("No hints generated for action", zap.String("action", actionString))
		return
	}

	// Create domain hint collection
	hintCollection := domainHint.NewCollection(domainHints)

	// Initialize domain manager with overlay update callback
	if h.Hints.Context.Manager == nil {
		manager := domainHint.NewManager(h.Logger)
		// Set callback to update overlay when hints are filtered
		manager.SetUpdateCallback(func(filteredHints []*domainHint.Hint) {
			if h.Hints.Overlay == nil {
				return
			}
			// Convert domain hints to overlay hints for rendering
			overlayHints := make([]*hints.Hint, len(filteredHints))
			for i, dh := range filteredHints {
				overlayHints[i] = &hints.Hint{
					Label:         dh.Label(),
					Position:      dh.Position(),
					Size:          dh.Element().Bounds().Size(),
					MatchedPrefix: dh.MatchedPrefix(),
				}
			}
			err := h.Hints.Overlay.DrawHintsWithStyle(overlayHints, h.Hints.Style)
			if err != nil {
				h.Logger.Error("Failed to update hints overlay", zap.Error(err))
			}
		})
		h.Hints.Context.SetManager(manager)
	}

	// Initialize domain router
	if h.Hints.Context.Router == nil {
		h.Hints.Context.SetRouter(domainHint.NewRouter(h.Hints.Context.Manager, h.Logger))
	}

	// Set hints in context (this also updates the manager)
	h.Hints.Context.SetHints(hintCollection)

	// Store pending action if provided
	h.Hints.Context.SetPendingAction(action)
	if action != nil {
		h.Logger.Info("Hints mode activated with pending action", zap.String("action", *action))
	}

	h.SetModeHints()
}

// SetupHints is deprecated and replaced by HintService.ShowHints.
func (h *Handler) SetupHints(_ []*infra.TreeNode) error {
	return nil
}

// handleHintsActionKey handles action keys when in hints action mode.
func (h *Handler) handleHintsActionKey(key string) {
	h.handleActionKey(key, "Hints")
}
