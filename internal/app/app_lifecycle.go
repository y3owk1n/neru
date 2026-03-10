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
			if a.logger != nil {
				a.logger.Error("Failed to enable event tap", zap.Error(err))
			}
		}
	}
}

func (a *App) disableEventTap() {
	if a.eventTap != nil {
		err := a.eventTap.Disable(context.Background())
		if err != nil {
			if a.logger != nil {
				a.logger.Error("Failed to disable event tap", zap.Error(err))
			}
		}
	}
}

// setEventTapModifierPassthrough configures whether unbound modifier shortcuts
// should pass through to macOS and which ones remain blacklisted.
func (a *App) setEventTapModifierPassthrough(enabled bool, blacklist []string) {
	if a.eventTap != nil {
		a.eventTap.SetModifierPassthrough(enabled, blacklist)
	}
}

// setEventTapInterceptedModifierKeys updates the modifier shortcuts the active
// mode still wants Neru to consume.
func (a *App) setEventTapInterceptedModifierKeys(keys []string) {
	if a.eventTap != nil {
		a.eventTap.SetInterceptedModifierKeys(keys)
	}
}
