//go:build linux

package hotkeys

import (
	"sync"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/infra/eventtap"
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

	// waylandHotkeys honors config keybindings on Wayland via passive evdev
	// reads, since compositors do not expose global hotkeys to clients.
	waylandHotkeys *eventtap.GlobalHotkeyListener
	waylandStarted bool
}

// NewManager creates and initializes a new hotkey manager instance.
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	mgr := &Manager{
		callbacks: make(map[HotkeyID]Callback),
		keys:      make(map[HotkeyID]string),
		logger:    logger.Named("hotkeys"),
		nextID:    1,
		backend:   platformBackend(),
	}

	if isWaylandBackend(mgr.backend) {
		mgr.waylandHotkeys = eventtap.NewGlobalHotkeyListener(logger)
	}

	return mgr
}

func isWaylandBackend(backend platform.LinuxBackend) bool {
	switch backend {
	case platform.BackendWaylandWlroots, platform.BackendWaylandKDE,
		platform.BackendWaylandGNOME, platform.BackendWaylandOther:
		return true
	case platform.BackendUnknown, platform.BackendX11:
		return false
	default:
		return false
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
	case platform.BackendWaylandWlroots, platform.BackendWaylandKDE,
		platform.BackendWaylandGNOME, platform.BackendWaylandOther:
		m.rebuildWaylandBindings()
		m.ensureWaylandStarted()
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

	if isWaylandBackend(m.backend) {
		m.rebuildWaylandBindings()

		if len(m.callbacks) == 0 {
			m.stopWayland()
		}
	}
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

	if isWaylandBackend(m.backend) {
		m.stopWayland()
	}
}

// rebuildWaylandBindings re-syncs the evdev listener with the current set of
// registered hotkeys. Callers must hold m.mu.
func (m *Manager) rebuildWaylandBindings() {
	if m.waylandHotkeys == nil {
		return
	}

	m.waylandHotkeys.ClearBindings()

	for id, key := range m.keys {
		m.waylandHotkeys.SetBinding(key, m.callbacks[id])
	}
}

// ensureWaylandStarted lazily starts the evdev listener on first registration.
// Callers must hold m.mu.
func (m *Manager) ensureWaylandStarted() {
	if m.waylandHotkeys == nil || m.waylandStarted {
		return
	}

	err := m.waylandHotkeys.Start()
	if err != nil {
		m.logger.Warn(
			"Wayland global hotkeys unavailable; grant read access to /dev/input "+
				"(add your user to the `input` group) or bind `neru <mode>` in your compositor instead",
			zap.Error(err),
		)

		return
	}

	m.waylandStarted = true

	m.logger.Info("Wayland global hotkeys enabled via evdev; config keybindings are active")
}

func (m *Manager) stopWayland() {
	if m.waylandHotkeys == nil || !m.waylandStarted {
		return
	}

	m.waylandHotkeys.Stop()
	m.waylandStarted = false
}

// SetGlobalManager assigns the global manager instance (Linux stub).
func SetGlobalManager(_ *Manager) {}

func platformBackend() platform.LinuxBackend {
	return platform.DetectLinuxBackend()
}
