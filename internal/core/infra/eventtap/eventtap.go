package eventtap

/*
#cgo CFLAGS: -x objective-c
#include "../bridge/eventtap.h"
#include <stdlib.h>

extern void eventTapCallbackBridge(char* key, void* userData);
*/
import "C"

import (
	"sync"
	"unsafe"

	"go.uber.org/zap"
)

// Callback defines the function signature for handling key press events.
type Callback func(key string)

// EventTap represents a keyboard event interceptor that captures global key presses.
type EventTap struct {
	handle   C.EventTap
	callback Callback
	logger   *zap.Logger
}

// NewEventTap initializes a new event tap for capturing global keyboard events.
// Returns nil if the event tap cannot be created, typically due to missing Accessibility permissions.
func NewEventTap(callback Callback, logger *zap.Logger) *EventTap {
	eventTap := &EventTap{
		callback: callback,
		logger:   logger,
	}

	eventTap.handle = C.createEventTap(C.EventTapCallback(C.eventTapCallbackBridge), nil)
	if eventTap.handle == nil {
		logger.Error("Failed to create event tap - check Accessibility permissions")

		return nil
	}

	// Store in global variable for callbacks with mutex protection
	globalEventTapMu.Lock()
	globalEventTap = eventTap
	globalEventTapMu.Unlock()

	return eventTap
}

// Enable activates the event tap to start capturing keyboard events.
func (et *EventTap) Enable() {
	et.logger.Debug("Enabling event tap")
	if et.handle != nil {
		C.enableEventTap(et.handle)
		et.logger.Debug("Event tap enabled")
	} else {
		et.logger.Warn("Cannot enable nil event tap")
	}
}

// SetHotkeys configures which hotkey combinations should be intercepted by the event tap.
// Hotkeys that are not configured will pass through to the system normally.
func (et *EventTap) SetHotkeys(hotkeys []string) {
	et.logger.Debug("Setting event tap hotkeys", zap.Int("count", len(hotkeys)))

	if et.handle == nil {
		et.logger.Warn("Cannot set hotkeys on nil event tap")

		return
	}

	// Convert Go string slice to C array
	cHotkeys := make([]*C.char, len(hotkeys))
	for index, hotkey := range hotkeys {
		if hotkey != "" {
			cHotkeys[index] = C.CString(hotkey)

			defer C.free(unsafe.Pointer(cHotkeys[index])) //nolint:nlreturn

			et.logger.Debug("Adding hotkey", zap.String("hotkey", hotkey))
		} else {
			cHotkeys[index] = nil
		}
	}

	// Pass pointer to first element and length
	cHotkeysPtr := (**C.char)(nil)
	if len(cHotkeys) > 0 {
		cHotkeysPtr = &cHotkeys[0]
	}

	C.setEventTapHotkeys(et.handle, cHotkeysPtr, C.int(len(cHotkeys)))
	et.logger.Debug("Event tap hotkeys set")
}

// Disable deactivates the event tap, stopping keyboard event capture.
func (et *EventTap) Disable() {
	et.logger.Debug("Disabling event tap")
	if et.handle != nil {
		C.disableEventTap(et.handle)
		et.logger.Debug("Event tap disabled")
	} else {
		et.logger.Warn("Cannot disable nil event tap")
	}
}

// Destroy cleans up the event tap resources and releases system hooks.
// This method ensures proper cleanup by disabling the tap first and clearing references.
func (et *EventTap) Destroy() {
	et.logger.Debug("Destroying event tap")
	if et.handle != nil {
		// Disable first to prevent any pending callbacks
		et.Disable()

		// Destroy the tap
		C.destroyEventTap(et.handle)
		et.handle = nil

		// Clear callback to prevent any lingering references
		et.callback = nil

		// Clear global reference if this is the global event tap
		globalEventTapMu.Lock()
		if globalEventTap == et {
			globalEventTap = nil
		}
		globalEventTapMu.Unlock()

		et.logger.Debug("Event tap destroyed")
	} else {
		et.logger.Warn("Cannot destroy nil event tap")
	}
}

// handleCallback processes key press events received from the C event tap bridge.
// It forwards the key information to the registered callback function if one exists.
func (et *EventTap) handleCallback(key string) {
	et.logger.Debug("Key pressed", zap.String("key", key))

	if et.callback != nil {
		et.callback(key)
	}
}

// Global event tap instance for C callbacks with thread safety.
// This is used to route events from the C bridge function back to the Go EventTap instance.
var (
	globalEventTap   *EventTap
	globalEventTapMu sync.RWMutex
)

// eventTapCallbackBridge serves as the C-to-Go callback bridge for event tap notifications.
// It safely retrieves the global EventTap instance and forwards the key event to handleCallback.
//
//export eventTapCallbackBridge
func eventTapCallbackBridge(key *C.char, _ unsafe.Pointer) {
	globalEventTapMu.RLock()
	eventTap := globalEventTap
	globalEventTapMu.RUnlock()

	if eventTap != nil && key != nil {
		goKey := C.GoString(key)
		eventTap.handleCallback(goKey)
	}
}
