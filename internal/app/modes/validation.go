package modes

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/ports"
)

const (
	// ValidationTimeout is the timeout for validation checks.
	ValidationTimeout = 2 * time.Second
	// cursorSyncTimeout is the timeout for cursor sync before mode activation.
	cursorSyncTimeout = 150 * time.Millisecond
)

// validateModeActivation performs common validation checks before mode activation.
// Returns an error if the mode cannot be activated.
// If bundleID is non-empty, it is used directly for exclusion check (skips AX call).
func (h *handlerState) validateModeActivation(
	bundleID string,
	modeName string,
	modeEnabled bool,
) error {
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
func (h *handlerState) prepareForModeActivation() {
	h.resetScrollContext()
	h.syncCursorPositionForModeActivation()
}

// resetScrollContext resets scroll-related state to ensure clean mode transitions.
func (h *handlerState) resetScrollContext() {
	if h.scroll.Context.IsActive() {
		// Atomically reset scroll context to ensure clean transition
		h.scroll.Context.Reset()
		// Also reset the skip restore flag since we're transitioning from scroll mode
		h.cursorState.Reset()
	}
}

// syncCursorPositionForModeActivation refreshes the adapter's cached cursor
// position when the platform keeps one. Adapters that do not implement
// ports.CursorSynchronizer are already authoritative, so there is nothing to do.
func (h *handlerState) syncCursorPositionForModeActivation() {
	h.syncCursorPosition(h.ctx)
}

// syncCursorPosition refreshes the platform's cursor cache within
// cursorSyncTimeout. A failure is a warning, not a debug line: everything that
// selects a monitor from the cursor (ScreenBounds on Wayland) now reads a
// stale position, which puts the overlay on the wrong display (#1279) — and
// that has to be answerable from a default log.
//
// Called from two lock contexts: under h.mu (activation, via
// prepareForModeActivation) and under only moveMonitorMu (MoveMonitor). That
// is safe because it touches nothing a reload or activation mutates — h.system
// and h.logger are construction-time-only — and it must stay that way: reading
// h.config or any mode component here is a data race with the unlocked caller.
func (h *handlerState) syncCursorPosition(ctx context.Context) {
	syncer, ok := h.system.(ports.CursorSynchronizer)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, cursorSyncTimeout)
	defer cancel()

	err := syncer.SyncCursorPosition(ctx)
	if err != nil {
		h.logger.Warn(
			"Failed to sync cursor position; monitor selection may be stale",
			zap.Error(err),
		)
	}
}
