package app

import (
	"context"

	"go.uber.org/zap"
)

// EnableEventTap enables the event tap.
func (a *App) EnableEventTap() { a.enableEventTap() }

// DisableEventTap disables the event tap.
func (a *App) DisableEventTap() { a.disableEventTap() }

// enableEventTap / disableEventTap back the exported wrappers above.
// The mode handler no longer routes through here — it holds
// ports.EventTapPort directly (see modes/eventtap.go).

func (a *App) enableEventTap() {
	if a.eventTap != nil {
		err := a.eventTap.Enable(a.ctx)
		if err != nil {
			if a.logger != nil {
				a.logger.Error("Failed to enable event tap", zap.Error(err))
			}
		}
	}
}

func (a *App) disableEventTap() {
	if a.eventTap != nil {
		// Use Background context since this may be called during cleanup,
		// after a.ctx has already been canceled.
		err := a.eventTap.Disable(context.Background())
		if err != nil {
			if a.logger != nil {
				a.logger.Error("Failed to disable event tap", zap.Error(err))
			}
		}
	}
}
