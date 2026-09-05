//go:build windows

package windows

import (
	"sync"

	"go.uber.org/zap"

	winplatform "github.com/y3owk1n/neru/internal/adapter/platform/windows"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/ports"
)

// Manager handles the registration, unregistration, and dispatching of global hotkeys.
type Manager struct {
	callbacks map[ports.HotkeyID]ports.HotkeyCallback
	keys      map[ports.HotkeyID]string
	nativeIDs map[ports.HotkeyID]int
	logger    *zap.Logger
	nextID    ports.HotkeyID
	registry  *winplatform.HotkeyRegistry
	mu        sync.RWMutex
}

// NewManager creates and creates a new hotkey manager instance.
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
		callbacks: make(map[ports.HotkeyID]ports.HotkeyCallback),
		keys:      make(map[ports.HotkeyID]string),
		nativeIDs: make(map[ports.HotkeyID]int),
		logger:    logger.Named("hotkeys"),
		nextID:    1,
		registry:  registry,
	}
}

// Register adds a new global hotkey that fires callback once per press.
func (m *Manager) Register(
	keyString string,
	callback ports.HotkeyCallback,
) (ports.HotkeyID, error) {
	return m.RegisterWithRelease(keyString, callback, nil)
}

// RegisterHotKey reports the press only; the registry finds the release by
// polling the key's state while it is held (HotkeyRegistry.RegisterWithRelease).
// The binder reaches this by type assertion, so a signature drift would
// silently return every hotkey to press-only instead of failing to compile.
var _ ports.HotkeyReleaseRegistrar = (*Manager)(nil)

// RegisterWithRelease adds a new global hotkey with press and optional release
// callbacks. A held chord is one press and one release, however long it is held.
func (m *Manager) RegisterWithRelease(
	keyString string,
	pressCallback ports.HotkeyCallback,
	releaseCallback ports.HotkeyCallback,
) (ports.HotkeyID, error) {
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

	nativeID, err := m.registry.RegisterWithRelease(keyString, pressCallback, releaseCallback)
	if err != nil {
		return 0, derrors.Wrap(err, derrors.CodeHotkeyRegisterFailed, "failed to register hotkey")
	}

	m.callbacks[hotkeyID] = pressCallback
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
func (m *Manager) Unregister(hotkeyID ports.HotkeyID) {
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

	m.callbacks = make(map[ports.HotkeyID]ports.HotkeyCallback)
	m.keys = make(map[ports.HotkeyID]string)
	m.nativeIDs = make(map[ports.HotkeyID]int)
}

// SetGlobalManager assigns the global manager instance (no-op on Windows; the
// native hotkey registry dispatches callbacks directly).
func SetGlobalManager(_ *Manager) {}
