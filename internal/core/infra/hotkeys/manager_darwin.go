//go:build darwin

package hotkeys

import (
	"runtime"
	"sync"
	"unsafe"

	"go.uber.org/zap"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
	"github.com/y3owk1n/neru/internal/core/infra/platform/darwin"
)

// HotkeyID represents a unique identifier for a registered hotkey.
type HotkeyID int

// Callback defines the function signature for hotkey event handlers.
type Callback func()

type callbackPair struct {
	press   Callback
	release Callback
	tap     unsafe.Pointer // per-hotkey CGEventTap handle
}

// Manager handles the registration, unregistration, and dispatching of global hotkeys.
// It maintains a mapping of hotkey IDs to their corresponding callback functions.
type Manager struct {
	callbacks map[HotkeyID]callbackPair
	mu        sync.RWMutex
	logger    *zap.Logger
	nextID    HotkeyID
}

// NewManager creates and initializes a new hotkey manager instance.
// The manager is ready to register hotkeys immediately after creation.
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	manager := &Manager{
		callbacks: make(map[HotkeyID]callbackPair),
		logger:    logger.Named("hotkeys"),
		nextID:    1,
	}

	// Ensure C-allocated per-hotkey taps are destroyed if the manager is
	// garbage collected without an explicit UnregisterAll call.
	runtime.SetFinalizer(manager, func(manager *Manager) {
		manager.mu.Lock()
		for _, pair := range manager.callbacks {
			darwin.DestroyHotkeyTap(pair.tap)
		}

		manager.callbacks = nil
		manager.mu.Unlock()
	})

	return manager
}

// Register adds a new global hotkey that will trigger the provided callback when pressed.
// The keyString parameter should follow the format "Cmd+Shift+X" or similar modifier combinations.
// Returns the assigned HotkeyID and an error if registration fails.
func (m *Manager) Register(keyString string, callback Callback) (HotkeyID, error) {
	return m.RegisterWithRelease(keyString, callback, nil)
}

// RegisterWithRelease adds a new global hotkey with press and optional release callbacks.
func (m *Manager) RegisterWithRelease(
	keyString string,
	pressCallback Callback,
	releaseCallback Callback,
) (HotkeyID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Parse key string into key code and modifiers
	keyCode, modifiers, ok := darwin.ParseKeyString(keyString)
	if !ok {
		m.logger.Error("Failed to parse key string", zap.String("key", keyString))

		return 0, derrors.Newf(
			derrors.CodeInvalidInput,
			"failed to parse key string: %s",
			keyString,
		)
	}

	// Generate hotkey ID
	hotkeyID := m.nextID
	m.nextID++

	// Create per-hotkey CGEventTap
	tap := darwin.CreateHotkeyTap(int(hotkeyID), keyCode, modifiers)
	if tap == nil {
		m.logger.Error("Failed to create hotkey tap", zap.String("key", keyString))

		return 0, derrors.Newf(
			derrors.CodeHotkeyRegisterFailed,
			"failed to register hotkey: %s",
			keyString,
		)
	}

	// Store callback and tap handle
	m.callbacks[hotkeyID] = callbackPair{
		press:   pressCallback,
		release: releaseCallback,
		tap:     tap,
	}

	m.logger.Debug("Registered hotkey",
		zap.String("key", keyString),
		zap.Int("key_code", keyCode),
		zap.Int("modifiers", modifiers),
		zap.Int("id", int(hotkeyID)))

	return hotkeyID, nil
}

// Unregister removes a previously registered hotkey by its ID.
// After unregistering, the hotkey will no longer trigger its associated callback.
func (m *Manager) Unregister(hotkeyID HotkeyID) {
	m.logger.Debug("Unregistering hotkey", zap.Int("id", int(hotkeyID)))

	m.mu.Lock()
	defer m.mu.Unlock()

	pair, ok := m.callbacks[hotkeyID]
	if ok {
		darwin.DestroyHotkeyTap(pair.tap)
		delete(m.callbacks, hotkeyID)
	}

	m.logger.Debug("Unregistered hotkey", zap.Int("id", int(hotkeyID)))
}

// UnregisterAll removes all currently registered hotkeys.
// This is typically called during application shutdown to clean up resources.
func (m *Manager) UnregisterAll() {
	m.logger.Debug("Unregistering all hotkeys")

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, pair := range m.callbacks {
		darwin.DestroyHotkeyTap(pair.tap)
	}

	m.callbacks = make(map[HotkeyID]callbackPair)

	m.logger.Debug("Unregistered all hotkeys")
}

// handleCallback processes hotkey events received from the C callback.
// It looks up the appropriate callback function and executes it in a goroutine.
func (m *Manager) handleCallback(hotkeyID HotkeyID, eventKind darwin.HotkeyEventKind) {
	m.logger.Debug("Handling hotkey callback", zap.Int("id", int(hotkeyID)))

	m.mu.RLock()
	callbacks, ok := m.callbacks[hotkeyID]
	m.mu.RUnlock()

	if !ok {
		m.logger.Debug("No callback registered for hotkey", zap.Int("id", int(hotkeyID)))

		return
	}

	switch eventKind {
	case darwin.HotkeyEventReleased:
		if callbacks.release != nil {
			m.logger.Debug("Hotkey released", zap.Int("id", int(hotkeyID)))
			callbacks.release()
		}
	case darwin.HotkeyEventPressed:
		if callbacks.press != nil {
			m.logger.Debug("Hotkey pressed", zap.Int("id", int(hotkeyID)))
			callbacks.press()
		}
	}
}

// Global manager instance for C callbacks.
// This allows the C bridge function to forward events to the appropriate manager instance.
var globalManager *Manager

// SetGlobalManager assigns the global manager instance used by the C callback bridge.
// This should be called once during application initialization with the main hotkey manager.
func SetGlobalManager(manager *Manager) {
	if manager != nil {
		manager.logger.Debug("Setting global hotkey manager")
	} else {
		logger.Get().Named("hotkeys").Debug("Setting global hotkey manager to nil")
	}

	globalManager = manager

	// Set the handler in the darwin package
	if manager != nil {
		darwin.SetHotkeyHandler(func(hotkeyID int, eventKind darwin.HotkeyEventKind) {
			if globalManager != nil {
				globalManager.logger.Debug("Hotkey callback bridge called",
					zap.Int("id", hotkeyID),
					zap.Int("event_kind", int(eventKind)))

				go globalManager.handleCallback(HotkeyID(hotkeyID), eventKind)
			}
		})
	} else {
		darwin.SetHotkeyHandler(nil)
	}
}
