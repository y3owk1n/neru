package hotkeys

import (
	"context"

	"github.com/y3owk1n/neru/internal/core/ports"
	"go.uber.org/zap"
)

// InfraManager defines the interface for the infrastructure hotkey manager.
// This interface allows the adapter to interact with the low-level hotkey implementation.
type InfraManager interface {
	Register(key string, callback func()) (int, error)
	Unregister(id int)
	UnregisterAll()
}

// Adapter implements ports.HotkeyPort by wrapping the existing hotkey manager.
type Adapter struct {
	manager       InfraManager
	logger        *zap.Logger
	registeredIDs map[string]int // Track registered hotkey IDs
}

// NewAdapter creates a new hotkey adapter.
func NewAdapter(manager InfraManager, logger *zap.Logger) *Adapter {
	return &Adapter{
		manager:       manager,
		logger:        logger,
		registeredIDs: make(map[string]int),
	}
}

// Register registers a hotkey with a callback.
// The callback signature differs from the infrastructure manager (func() error vs func()),
// so we wrap it.
func (a *Adapter) Register(_ context.Context, key string, callback func() error) error {
	wrappedCallback := func() {
		err := callback()
		if err != nil {
			a.logger.Error("Hotkey callback error", zap.String("key", key), zap.Error(err))
		}
	}

	registeredID, registeredIDErr := a.manager.Register(key, wrappedCallback)
	if registeredIDErr != nil {
		return registeredIDErr
	}

	a.registeredIDs[key] = registeredID

	return nil
}

// Unregister removes a hotkey registration.
func (a *Adapter) Unregister(_ context.Context, key string) error {
	registeredID, ok := a.registeredIDs[key]

	if ok {
		a.manager.Unregister(registeredID)
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
