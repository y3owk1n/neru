package modes

import (
	"context"
	"fmt"
	"image"

	"github.com/y3owk1n/neru/internal/application/ports"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/features/hints"
	infra "github.com/y3owk1n/neru/internal/infra/accessibility"
	"github.com/y3owk1n/neru/internal/infra/bridge"
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
	ctx := context.Background() // TODO: Use proper context with timeout
	filter := ports.DefaultElementFilter()

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

	// Convert domain hints to legacy hints for input handling
	// We need to do this reverse conversion because the input handler still uses legacy types
	// This is temporary until we migrate the input handling logic
	legacyHintsList := make([]*hints.Hint, len(domainHints))
	for i, dh := range domainHints {
		legacyHintsList[i] = &hints.Hint{
			Label:         dh.Label(),
			Position:      dh.Position(),
			MatchedPrefix: dh.MatchedPrefix(),
			// We don't have the legacy TreeNode here, but input handler might need it
			// For now, we'll leave it nil and see if it breaks anything
			// The overlay adapter already handled the display part
		}
	}

	hintCollection := hints.NewHintCollection(legacyHintsList)
	h.Hints.Manager.SetHints(hintCollection)

	h.Hints.Context.SetSelectedHint(nil)

	// Store pending action if provided
	h.Hints.Context.SetPendingAction(action)
	if action != nil {
		h.Logger.Info("Hints mode activated with pending action", zap.String("action", *action))
	}

	h.SetModeHints()
}

// SetupHints is deprecated and replaced by HintService.ShowHints
func (h *Handler) SetupHints(elements []*infra.TreeNode) error {
	return nil
}

// generateAndNormalizeHints generates hints and normalizes their positions.
func (h *Handler) generateAndNormalizeHints(elements []*infra.TreeNode) ([]*hints.Hint, error) {
	// Get active screen bounds to calculate offset for normalization
	screenBounds := bridge.GetActiveScreenBounds()
	screenOffsetX := screenBounds.Min.X
	screenOffsetY := screenBounds.Min.Y

	hintList, err := h.Hints.Generator.Generate(elements)
	if err != nil {
		return nil, fmt.Errorf("failed to generate hints: %w", err)
	}

	// Check if we have any hints
	if len(hintList) == 0 {
		h.Logger.Warn("No hints generated",
			zap.Int("elements_count", len(elements)),
			zap.Any("screen_bounds", screenBounds))
		return hintList, nil
	}

	// Normalize hint positions to window-local coordinates.
	// The overlay window is positioned at the screen origin, but the view uses local coordinates.
	for _, hint := range hintList {
		pos := hint.GetPosition()
		hint.Position.X = pos.X - screenOffsetX
		hint.Position.Y = pos.Y - screenOffsetY
	}

	localBounds := image.Rect(0, 0, screenBounds.Dx(), screenBounds.Dy())
	// Pre-allocate with estimated capacity (typically 70-90% of hints are visible)
	filtered := make([]*hints.Hint, 0, len(hintList)*9/10)
	for _, h := range hintList {
		if h.IsVisible(localBounds) {
			filtered = append(filtered, h)
		}
	}

	h.Logger.Debug("Hints generated and normalized",
		zap.Int("generated", len(hintList)),
		zap.Int("visible", len(filtered)),
		zap.Int("elements", len(elements)))

	return filtered, nil
}

// handleHintsActionKey handles action keys when in hints action mode.
func (h *Handler) handleHintsActionKey(key string) {
	h.handleActionKey(key, "Hints")
}
