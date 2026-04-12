//go:build linux

package hotkeys

import (
	"sync"

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
	keys      map[HotkeyID]string
	logger    *zap.Logger
	nextID    HotkeyID
	backend   platform.LinuxBackend
	mu        sync.RWMutex
}

// NewManager creates and initializes a new hotkey manager instance.
func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		callbacks: make(map[HotkeyID]Callback),
		keys:      make(map[HotkeyID]string),
		logger:    logger,
		nextID:    1,
		backend:   platformBackend(),
	}
}

// Register adds a new global hotkey (Linux stub).
func (m *Manager) Register(keyString string, callback Callback) (HotkeyID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	hotkeyID := m.nextID
	m.nextID++
	m.callbacks[hotkeyID] = callback
	m.keys[hotkeyID] = keyString

	switch m.backend {
	case platform.BackendX11:
		err := m.registerX11Hotkey(hotkeyID, keyString)
		if err != nil {
			delete(m.callbacks, hotkeyID)
			delete(m.keys, hotkeyID)

			return 0, err
		}
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
	case platform.BackendUnknown:
		m.logger.Debug(
			"Registering hotkey in Linux manager",
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
func (m *Manager) Unregister(hotkeyID HotkeyID) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.backend == platform.BackendX11 {
		m.unregisterX11Hotkey(hotkeyID)
	}

	delete(m.callbacks, hotkeyID)
	delete(m.keys, hotkeyID)
}

// UnregisterAll removes all currently registered hotkeys (Linux stub).
func (m *Manager) UnregisterAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.backend == platform.BackendX11 {
		m.unregisterAllX11Hotkeys()
	}

	m.callbacks = make(map[HotkeyID]Callback)
	m.keys = make(map[HotkeyID]string)
}

// SetGlobalManager assigns the global manager instance (Linux stub).
func SetGlobalManager(_ *Manager) {}

func platformBackend() platform.LinuxBackend {
	return platform.DetectLinuxBackend()
}
