package app

import (
	"context"

	"go.uber.org/zap"
)

// ActivateMode activates the specified mode.
func (a *App) ActivateMode(mode Mode) {
	a.modes.ActivateMode(mode)
}

// CaptureInitialCursorPosition captures the initial cursor position.
func (a *App) CaptureInitialCursorPosition() { a.modes.CaptureInitialCursorPosition() }

// IsFocusedAppExcluded checks if the focused app is excluded.
func (a *App) IsFocusedAppExcluded() bool {
	// Use ActionService to check exclusion
	ctx := context.Background()

	excluded, excludedErr := a.actionService.IsFocusedAppExcluded(ctx)
	if excludedErr != nil {
		a.logger.Warn("Failed to check exclusion", zap.Error(excludedErr))

		return false
	}

	return excluded
}

// ExitMode exits the current mode.
func (a *App) ExitMode() { a.modes.ExitMode() }

// SetModeHints sets the mode to hints.
// SetModeHints switches the application to hints mode.
func (a *App) SetModeHints() { a.modes.SetModeHints() }

// SetModeGrid switches the application to grid mode.
func (a *App) SetModeGrid() { a.modes.SetModeGrid() }

// SetModeRecursiveGrid switches the application to recursive-grid mode.
func (a *App) SetModeRecursiveGrid() { a.modes.SetModeRecursiveGrid() }

// SetModeScroll switches the application to scroll mode.
func (a *App) SetModeScroll() { a.modes.SetModeScroll() }

// SetModeIdle switches the application to idle mode.
func (a *App) SetModeIdle() { a.modes.SetModeIdle() }

// HandleKeyPress delegates key press handling to the mode handler.
func (a *App) HandleKeyPress(key string) {
	a.modes.HandleKeyPress(key)
}
