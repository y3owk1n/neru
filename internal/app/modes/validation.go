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
	// Secure input is checked first. It is a macOS security feature that
	// blocks keyboard events while a password field is focused. On other
	// platforms IsSecureInputEnabled always returns false.
	if h.system != nil && h.system.IsSecureInputEnabled() {
		h.system.ShowSecureInputNotification()

		return derrors.New(
			derrors.CodeSecureInputEnabled,
			"secure input is enabled - a password field may be focused",
		)
	}

	if !h.appState.IsEnabled() {
		return derrors.New(derrors.CodeInvalidInput, "neru is disabled")
	}

	if !modeEnabled {
		return derrors.Newf(derrors.CodeInvalidInput, "mode %s is disabled", modeName)
	}

	if bundleID == "" {
		ctx, cancel := context.WithTimeout(h.ctx, ValidationTimeout)
		defer cancel()

		focused, focusedErr := h.actionService.FocusedAppBundleID(ctx)
		if focusedErr != nil {
			h.logger.Warn("Failed to check if app is excluded", zap.Error(focusedErr))
		}

		bundleID = focused
	}

	// This reads the live config, the same list the hotkey binder checks, so a
	// reload that changes excluded_apps takes effect here too.
	if h.config != nil && h.config.IsAppExcluded(bundleID) {
		return derrors.New(derrors.CodeInvalidInput, "focused app is excluded")
	}

	return nil
}

// prepareForModeActivation performs common preparation steps before activating a mode.
// This includes resetting scroll state and syncing any platform cursor cache.
func (h *handlerState) prepareForModeActivation() {
	h.resetScrollContext()
	h.syncCursorPositionForModeActivation()
}

// resetScrollContext resets the state scroll mode leaves behind.
func (h *handlerState) resetScrollContext() {
	if h.scroll.Context.IsActive() {
		h.scroll.Context.Reset()
		// Leaving scroll mode also clears its skip-restore flag.
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
// cursorSyncTimeout. A failure is a warning, not a debug line, because
// everything that selects a monitor from the cursor (ScreenBounds on Wayland)
// then reads a stale position. That puts the overlay on the wrong display
// (#1279), which has to be answerable from a default log.
//
// Called from two lock contexts: under h.mu (activation, via
// prepareForModeActivation) and under only moveMonitorMu (MoveMonitor,
// MoveMonitorByName and the monitor_select move). That is safe because it
// touches nothing a reload or activation mutates, since h.system and h.logger
// are set only at construction, and it must stay that way. Reading h.config
// or any mode component here is a data race with the unlocked caller.
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
