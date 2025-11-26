package modes

import (
	"context"
	"time"

	derrors "github.com/y3owk1n/neru/internal/errors"
	"go.uber.org/zap"
)

const (
	// ValidationTimeout is the timeout for validation checks.
	ValidationTimeout = 2 * time.Second
)

// validateModeActivation performs common validation checks before mode activation.
// Returns an error if the mode cannot be activated.
func (h *Handler) validateModeActivation(modeName string, modeEnabled bool) error {
	if !h.AppState.IsEnabled() {
		h.Logger.Warn("Neru is disabled, ignoring mode activation",
			zap.String("mode", modeName))

		return derrors.New(derrors.CodeInvalidInput, "neru is disabled")
	}

	if !modeEnabled {
		h.Logger.Warn("Mode disabled by config, ignoring activation",
			zap.String("mode", modeName))

		return derrors.Newf(derrors.CodeInvalidInput, "mode %s is disabled", modeName)
	}

	// Check if focused app is excluded
	// Use a short timeout context for this check
	context, cancel := context.WithTimeout(context.Background(), ValidationTimeout)
	defer cancel()

	isExcluded, isExcludedErr := h.ActionService.IsFocusedAppExcluded(context)
	if isExcludedErr != nil {
		h.Logger.Warn("Failed to check if app is excluded", zap.Error(isExcludedErr))
	} else if isExcluded {
		return derrors.New(derrors.CodeInvalidInput, "focused app is excluded")
	}

	return nil
}

// prepareForModeActivation performs common preparation steps before activating a mode.
// This includes resetting scroll context and capturing the initial cursor position.
func (h *Handler) prepareForModeActivation() {
	h.resetScrollContext()
	h.CaptureInitialCursorPosition()
}

// resetScrollContext resets scroll-related state to ensure clean mode transitions.
func (h *Handler) resetScrollContext() {
	if h.Scroll.Context.IsActive() {
		// Reset scroll context to ensure clean transition
		h.Scroll.Context.SetIsActive(false)
		h.Scroll.Context.SetLastKey("")
		// Also reset the skip restore flag since we're transitioning from scroll mode
		h.CursorState.Reset()
	}
}
