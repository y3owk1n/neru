//go:build windows

package hotkeys

import (
	"sync"

	"go.uber.org/zap"
)

// HotkeyID represents a unique identifier for a registered hotkey.
type HotkeyID int

// Callback defines the function signature for hotkey event handlers.
type Callback func()

// Manager handles the registration, unregistration, and dispatching of global hotkeys (Windows stub).
type Manager struct {
	callbacks map[HotkeyID]Callback
	mu        sync.RWMutex
	logger    *zap.Logger
	nextID    HotkeyID
}

// NewManager creates and initializes a new hotkey manager instance (Windows stub).
func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		callbacks: make(map[HotkeyID]Callback),
		logger:    logger,
		nextID:    1,
	}
}

// Register adds a new global hotkey (Windows stub).
func (m *Manager) Register(keyString string, callback Callback) (HotkeyID, error) {
	return 0, nil
}

// Unregister removes a previously registered hotkey by its ID (Windows stub).
func (m *Manager) Unregister(hotkeyID HotkeyID) {}

// UnregisterAll removes all currently registered hotkeys (Windows stub).
func (m *Manager) UnregisterAll() {}

// SetGlobalManager sets the global hotkey manager (Windows stub).
func SetGlobalManager(manager *Manager) {}
