package app

import (
	"context"

	"go.uber.org/zap"
)

// EnableEventTap enables the event tap.
func (a *App) EnableEventTap() { a.enableEventTap() }

// DisableEventTap disables the event tap.
func (a *App) DisableEventTap() { a.disableEventTap() }

// Helper methods for event tap control (used by callbacks)

func (a *App) enableEventTap() {
	if a.eventTap != nil {
		err := a.eventTap.Enable(context.Background())
		if err != nil {
			a.logger.Error("Failed to enable event tap", zap.Error(err))
		}
	}
}

func (a *App) disableEventTap() {
	if a.eventTap != nil {
		err := a.eventTap.Disable(context.Background())
		if err != nil {
			a.logger.Error("Failed to disable event tap", zap.Error(err))
		}
	}
}
