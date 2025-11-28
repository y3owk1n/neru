package hotkeys

/*
#cgo CFLAGS: -x objective-c
#include "../bridge/hotkeys.h"
#include <stdlib.h>

extern void hotkeyCallbackBridge(int hotkeyId, void* userData);
*/
import "C"

import (
	"sync"
	"unsafe"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
	"go.uber.org/zap"
)

// HotkeyID represents a unique identifier for a registered hotkey.
type HotkeyID int

// Callback defines the function signature for hotkey event handlers.
type Callback func()

// Manager handles the registration, unregistration, and dispatching of global hotkeys.
// It maintains a mapping of hotkey IDs to their corresponding callback functions.
type Manager struct {
	callbacks map[HotkeyID]Callback
	mu        sync.RWMutex
	logger    *zap.Logger
	nextID    HotkeyID
}

// NewManager creates and initializes a new hotkey manager instance.
// The manager is ready to register hotkeys immediately after creation.
func NewManager(logger *zap.Logger) *Manager {
	manager := &Manager{
		callbacks: make(map[HotkeyID]Callback),
		logger:    logger,
		nextID:    1,
	}

	return manager
}

// Register adds a new global hotkey that will trigger the provided callback when pressed.
// The keyString parameter should follow the format "Cmd+Shift+X" or similar modifier combinations.
// Returns the assigned HotkeyID and an error if registration fails.
func (m *Manager) Register(keyString string, callback Callback) (HotkeyID, error) {
	m.logger.Debug("Registering hotkey", zap.String("key", keyString))

	m.mu.Lock()
	defer m.mu.Unlock()

	// Parse key string
	var keyCode, modifiers C.int
	cKeyString := C.CString(keyString)

	defer C.free(unsafe.Pointer(cKeyString)) //nolint:nlreturn

	result := C.parseKeyString(cKeyString, &keyCode, &modifiers)
	if result == 0 {
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

	m.logger.Debug("Parsed key string",
		zap.String("key", keyString),
		zap.Int("key_code", int(keyCode)),
		zap.Int("modifiers", int(modifiers)),
		zap.Int("id", int(hotkeyID)))

	// Register hotkey
	success := C.registerHotkey(keyCode, modifiers, C.int(hotkeyID),
		C.HotkeyCallback(C.hotkeyCallbackBridge), nil)

	if success == 0 {
		m.logger.Error("Failed to register hotkey", zap.String("key", keyString))

		return 0, derrors.Newf(
			derrors.CodeHotkeyRegisterFailed,
			"failed to register hotkey: %s",
			keyString,
		)
	}

	// Store callback
	m.callbacks[hotkeyID] = callback

	m.logger.Info("Registered hotkey",
		zap.String("key", keyString),
		zap.Int("id", int(hotkeyID)))

	return hotkeyID, nil
}

// Unregister removes a previously registered hotkey by its ID.
// After unregistering, the hotkey will no longer trigger its associated callback.
func (m *Manager) Unregister(hotkeyID HotkeyID) {
	m.logger.Debug("Unregistering hotkey", zap.Int("id", int(hotkeyID)))

	m.mu.Lock()
	defer m.mu.Unlock()

	C.unregisterHotkey(C.int(hotkeyID))
	delete(m.callbacks, hotkeyID)

	m.logger.Info("Unregistered hotkey", zap.Int("id", int(hotkeyID)))
}

// UnregisterAll removes all currently registered hotkeys.
// This is typically called during application shutdown to clean up resources.
func (m *Manager) UnregisterAll() {
	m.logger.Debug("Unregistering all hotkeys")

	m.mu.Lock()
	defer m.mu.Unlock()

	C.unregisterAllHotkeys()
	m.callbacks = make(map[HotkeyID]Callback)

	m.logger.Info("Unregistered all hotkeys")
}

// handleCallback processes hotkey events received from the C callback bridge.
// It looks up the appropriate callback function and executes it in a goroutine.
func (m *Manager) handleCallback(hotkeyID HotkeyID) {
	m.logger.Debug("Handling hotkey callback", zap.Int("id", int(hotkeyID)))

	m.mu.RLock()
	callback, ok := m.callbacks[hotkeyID]
	m.mu.RUnlock()

	if ok && callback != nil {
		m.logger.Debug("Hotkey pressed", zap.Int("id", int(hotkeyID)))
		callback()
	} else {
		m.logger.Debug("No callback registered for hotkey", zap.Int("id", int(hotkeyID)))
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
		// This would be unusual but let's log it
		logger.Get().Info("Setting global hotkey manager to nil")
	}
	globalManager = manager
}

// hotkeyCallbackBridge serves as the C-to-Go callback bridge for hotkey events.
// It forwards hotkey events to the global manager's handleCallback method.
//
//export hotkeyCallbackBridge
func hotkeyCallbackBridge(hotkeyID C.int, _ unsafe.Pointer) {
	if globalManager != nil {
		globalManager.logger.Debug("Hotkey callback bridge called", zap.Int("id", int(hotkeyID)))
		go globalManager.handleCallback(HotkeyID(hotkeyID))
	}
}
