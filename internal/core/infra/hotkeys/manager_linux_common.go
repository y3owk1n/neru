//go:build linux

package hotkeys

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/infra/platform"
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
	backend   platform.LinuxBackend
}

// NewManager creates and initializes a new hotkey manager instance.
func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		callbacks: make(map[HotkeyID]Callback),
		logger:    logger,
		nextID:    1,
		backend:   platformBackend(),
	}
}

// Register adds a new global hotkey (Linux stub).
func (m *Manager) Register(keyString string, callback Callback) (HotkeyID, error) {
	hotkeyID := m.nextID
	m.nextID++
	m.callbacks[hotkeyID] = callback

	switch m.backend {
	case platform.BackendWaylandWlroots:
		m.logger.Info(
			"Running on Wayland: global hotkeys are unavailable inside Neru. Bind `neru trigger <mode>` in your compositor config instead.",
			zap.String("key", keyString),
		)
	case platform.BackendWaylandGNOME, platform.BackendWaylandKDE, platform.BackendWaylandOther:
		m.logger.Info(
			"Linux hotkey registration stored, but native Wayland global hotkeys are not implemented for this compositor.",
			zap.String("key", keyString),
			zap.String("backend", m.backend.String()),
		)
	default:
		m.logger.Debug(
			"Registering hotkey in Linux manager",
			zap.String("key", keyString),
			zap.String("backend", m.backend.String()),
		)
	}

	return hotkeyID, nil
}

// Unregister removes a previously registered hotkey by its ID (Linux stub).
func (m *Manager) Unregister(id HotkeyID) {
	delete(m.callbacks, id)
}

// UnregisterAll removes all currently registered hotkeys (Linux stub).
func (m *Manager) UnregisterAll() {
	m.callbacks = make(map[HotkeyID]Callback)
}

// SetGlobalManager assigns the global manager instance (Linux stub).
func SetGlobalManager(_ *Manager) {}

func platformBackend() platform.LinuxBackend {
	return platform.DetectLinuxBackend()
}
