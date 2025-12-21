package modes

import (
	"context"
	"time"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/bridge"
	"go.uber.org/zap"
)

const (
	// ValidationTimeout is the timeout for validation checks.
	ValidationTimeout = 2 * time.Second
)

// validateModeActivation performs common validation checks before mode activation.
// Returns an error if the mode cannot be activated.
func (h *Handler) validateModeActivation(modeName string, modeEnabled bool) error {
	// Check for secure input mode first - this is a macOS security feature
	// that blocks keyboard events when password fields are focused
	if bridge.IsSecureInputEnabled() {
		h.logger.Warn("Secure input is enabled, blocking mode activation",
			zap.String("mode", modeName))

		// Show notification to inform the user
		bridge.ShowSecureInputNotification()

		return derrors.New(
			derrors.CodeSecureInputEnabled,
			"secure input is enabled - a password field may be focused",
		)
	}

	if !h.appState.IsEnabled() {
		h.logger.Warn("Neru is disabled, ignoring mode activation",
			zap.String("mode", modeName))

		return derrors.New(derrors.CodeInvalidInput, "neru is disabled")
	}

	if !modeEnabled {
		h.logger.Warn("Mode disabled by config, ignoring activation",
			zap.String("mode", modeName))

		return derrors.Newf(derrors.CodeInvalidInput, "mode %s is disabled", modeName)
	}

	// Check if focused app is excluded
	// Use a short timeout context for this check
	context, cancel := context.WithTimeout(context.Background(), ValidationTimeout)
	defer cancel()

	isExcluded, isExcludedErr := h.actionService.IsFocusedAppExcluded(context)
	if isExcludedErr != nil {
		h.logger.Warn("Failed to check if app is excluded", zap.Error(isExcludedErr))
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
	if h.scroll.Context.IsActive() {
		// Reset scroll context to ensure clean transition
		h.scroll.Context.SetIsActive(false)
		h.scroll.Context.SetLastKey("")
		// Also reset the skip restore flag since we're transitioning from scroll mode
		h.cursorState.Reset()
	}
}
