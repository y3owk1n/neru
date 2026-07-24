package modes

import (
	"context"
	"time"

	"go.uber.org/zap"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

const (
	// ValidationTimeout is the timeout for validation checks.
	ValidationTimeout = 2 * time.Second
)

type cursorSyncer interface {
	SyncCursorPosition(ctx context.Context) error
}

// validateModeActivation performs common validation checks before mode activation.
// Returns an error if the mode cannot be activated.
// If bundleID is non-empty, it is used directly for exclusion check (skips AX call).
func (h *Handler) validateModeActivation(bundleID string, modeName string, modeEnabled bool) error {
	// Check for secure input mode first - this is a macOS security feature
	// that blocks keyboard events when password fields are focused.
	// On non-macOS platforms IsSecureInputEnabled always returns false.
	if h.system != nil && h.system.IsSecureInputEnabled() {
		h.logger.Warn("Secure input is enabled, blocking mode activation",
			zap.String("mode", modeName))

		// Show notification to inform the user
		h.system.ShowSecureInputNotification()

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
	if bundleID != "" {
		if h.actionService.IsAppExcluded(h.ctx, bundleID) {
			return derrors.New(derrors.CodeInvalidInput, "focused app is excluded")
		}
	} else {
		ctx, cancel := context.WithTimeout(h.ctx, ValidationTimeout)
		defer cancel()

		isExcluded, isExcludedErr := h.actionService.IsFocusedAppExcluded(ctx)
		if isExcludedErr != nil {
			h.logger.Warn("Failed to check if app is excluded", zap.Error(isExcludedErr))
		} else if isExcluded {
			return derrors.New(derrors.CodeInvalidInput, "focused app is excluded")
		}
	}

	return nil
}

// prepareForModeActivation performs common preparation steps before activating a mode.
// This includes resetting scroll state and syncing any platform cursor cache.
func (h *Handler) prepareForModeActivation() {
	h.resetScrollContext()
	h.syncCursorPositionForModeActivation()
}

// resetScrollContext resets scroll-related state to ensure clean mode transitions.
func (h *Handler) resetScrollContext() {
	if h.scroll.Context.IsActive() {
		// Atomically reset scroll context to ensure clean transition
		h.scroll.Context.Reset()
		// Also reset the skip restore flag since we're transitioning from scroll mode
		h.cursorState.Reset()
	}
}

func (h *Handler) syncCursorPositionForModeActivation() {
	syncer, ok := h.system.(cursorSyncer)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(h.ctx, 150*time.Millisecond)
	defer cancel()

	if err := syncer.SyncCursorPosition(ctx); err != nil {
		h.logger.Debug("Failed to sync cursor position for mode activation", zap.Error(err))
	}
}
