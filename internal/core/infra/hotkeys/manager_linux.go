//go:build linux

package hotkeys

import (
	"go.uber.org/zap"
)

// HotkeyID represents a unique identifier for a registered hotkey.
type HotkeyID int

// Callback defines the function signature for hotkey event handlers.
type Callback func()

// Manager handles the registration, unregistration, and dispatching of global hotkeys.
type Manager struct {
	callbacks map[HotkeyID]Callback
	logger    *zap.Logger
	nextID    HotkeyID
}

// NewManager creates and initializes a new hotkey manager instance.
func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		callbacks: make(map[HotkeyID]Callback),
		logger:    logger,
		nextID:    1,
	}
}

// Register adds a new global hotkey (Linux stub).
func (m *Manager) Register(keyString string, _ Callback) (HotkeyID, error) {
	m.logger.Debug("Registering hotkey (Linux stub)", zap.String("key", keyString))

	return 0, nil
}

// Unregister removes a previously registered hotkey by its ID (Linux stub).
func (m *Manager) Unregister(_ HotkeyID) {}

// UnregisterAll removes all currently registered hotkeys (Linux stub).
func (m *Manager) UnregisterAll() {}

// SetGlobalManager assigns the global manager instance (Linux stub).
func SetGlobalManager(_ *Manager) {}
