// Package hotkey implements the hotkey adapter.
package hotkey

import (
	"context"

	"github.com/y3owk1n/neru/internal/application/ports"
	"go.uber.org/zap"
)

// LegacyHotkeyManager defines the interface for the legacy hotkey manager.
// This is needed because the legacy manager is not exported as a type but used via interface in app.go.
type LegacyHotkeyManager interface {
	Register(key string, callback func()) (int, error)
	Unregister(id int)
	UnregisterAll()
}

// Adapter implements ports.HotkeyPort by wrapping the existing hotkey manager.
type Adapter struct {
	manager       LegacyHotkeyManager
	logger        *zap.Logger
	registeredIDs map[string]int // Track registered hotkey IDs
}

// NewAdapter creates a new hotkey adapter.
func NewAdapter(manager LegacyHotkeyManager, logger *zap.Logger) *Adapter {
	return &Adapter{
		manager:       manager,
		logger:        logger,
		registeredIDs: make(map[string]int),
	}
}

// Register registers a hotkey with a callback.
// The callback signature differs from the legacy manager (func() error vs func()),
// so we wrap it.
func (a *Adapter) Register(_ context.Context, key string, callback func() error) error {
	wrappedCallback := func() {
		err := callback()
		if err != nil {
			a.logger.Error("Hotkey callback error", zap.String("key", key), zap.Error(err))
		}
	}

	id, err := a.manager.Register(key, wrappedCallback)
	if err != nil {
		return err
	}

	a.registeredIDs[key] = id
	return nil
}

// Unregister removes a hotkey registration.
func (a *Adapter) Unregister(_ context.Context, key string) error {
	if id, ok := a.registeredIDs[key]; ok {
		a.manager.Unregister(id)
		delete(a.registeredIDs, key)
	}
	return nil
}

// UnregisterAll removes all hotkey registrations.
func (a *Adapter) UnregisterAll(_ context.Context) error {
	a.manager.UnregisterAll()
	a.registeredIDs = make(map[string]int)
	return nil
}

// IsRegistered reports whether a hotkey is currently registered.
func (a *Adapter) IsRegistered(hotkey string) bool {
	_, ok := a.registeredIDs[hotkey]
	return ok
}

// Ensure Adapter implements ports.HotkeyPort.
var _ ports.HotkeyPort = (*Adapter)(nil)
