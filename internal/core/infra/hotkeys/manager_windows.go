//go:build windows

package hotkeys

import (
	"sync"

	"go.uber.org/zap"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
	winplatform "github.com/y3owk1n/neru/internal/core/infra/platform/windows"
)

// HotkeyID represents a unique identifier for a registered hotkey.
type HotkeyID int

// Callback defines the function signature for hotkey event handlers.
type Callback func()

// Manager handles the registration, unregistration, and dispatching of global hotkeys.
type Manager struct {
	callbacks map[HotkeyID]Callback
	keys      map[HotkeyID]string
	nativeIDs map[HotkeyID]int
	logger    *zap.Logger
	nextID    HotkeyID
	registry  *winplatform.HotkeyRegistry
	mu        sync.RWMutex
}

// NewManager creates and initializes a new hotkey manager instance.
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	registry, err := winplatform.GlobalHotkeyRegistry()
	if err != nil {
		logger.Warn("failed to initialize Windows hotkey registry", zap.Error(err))
	} else if registry != nil {
		registry.SetHotkeyRegistryLogger(logger)
	}

	return &Manager{
		callbacks: make(map[HotkeyID]Callback),
		keys:      make(map[HotkeyID]string),
		nativeIDs: make(map[HotkeyID]int),
		logger:    logger.Named("hotkeys"),
		nextID:    1,
		registry:  registry,
	}
}

// Register adds a new global hotkey.
func (m *Manager) Register(keyString string, callback Callback) (HotkeyID, error) {
	if m.registry == nil {
		return 0, derrors.New(
			derrors.CodeHotkeyRegisterFailed,
			"Windows hotkey registry is unavailable",
		)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	hotkeyID := m.nextID
	m.nextID++

	nativeID, err := m.registry.Register(keyString, callback)
	if err != nil {
		return 0, derrors.Wrap(err, derrors.CodeHotkeyRegisterFailed, "failed to register hotkey")
	}

	m.callbacks[hotkeyID] = callback
	m.keys[hotkeyID] = keyString
	m.nativeIDs[hotkeyID] = nativeID

	m.logger.Info(
		"global hotkey armed",
		zap.String("key", keyString),
		zap.Int("native_id", nativeID),
	)

	return hotkeyID, nil
}

// Unregister removes a previously registered hotkey by its ID.
func (m *Manager) Unregister(hotkeyID HotkeyID) {
	m.mu.Lock()
	nativeID := m.nativeIDs[hotkeyID]
	delete(m.callbacks, hotkeyID)
	delete(m.keys, hotkeyID)
	delete(m.nativeIDs, hotkeyID)
	m.mu.Unlock()

	if m.registry != nil && nativeID != 0 {
		m.registry.Unregister(nativeID)
	}
}

// UnregisterAll removes all currently registered hotkeys.
func (m *Manager) UnregisterAll() {
	if m.registry != nil {
		m.registry.UnregisterAll()
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.callbacks = make(map[HotkeyID]Callback)
	m.keys = make(map[HotkeyID]string)
	m.nativeIDs = make(map[HotkeyID]int)
}

// SetGlobalManager assigns the global manager instance (no-op on Windows; the
// native hotkey registry dispatches callbacks directly).
func SetGlobalManager(_ *Manager) {}
