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
func (eventTap *EventTap) Enable() {
	eventTap.logger.Debug("Enabling event tap")
	if eventTap.handle != nil {
		C.enableEventTap(eventTap.handle)
		eventTap.logger.Debug("Event tap enabled")
	} else {
		eventTap.logger.Warn("Cannot enable nil event tap")
	}
}

// SetHotkeys configures which hotkey combinations should be intercepted by the event tap.
// Hotkeys that are not configured will pass through to the system normally.
func (eventTap *EventTap) SetHotkeys(hotkeys []string) {
	eventTap.logger.Debug("Setting event tap hotkeys", zap.Int("count", len(hotkeys)))

	if eventTap.handle == nil {
		eventTap.logger.Warn("Cannot set hotkeys on nil event tap")

		return
	}

	// Convert Go string slice to C array
	cHotkeys := make([]*C.char, len(hotkeys))
	for index, hotkey := range hotkeys {
		if hotkey != "" {
			cHotkeys[index] = C.CString(hotkey)

			defer C.free(unsafe.Pointer(cHotkeys[index])) //nolint:nlreturn

			eventTap.logger.Debug("Adding hotkey", zap.String("hotkey", hotkey))
		} else {
			cHotkeys[index] = nil
		}
	}

	// Pass pointer to first element and length
	cHotkeysPtr := (**C.char)(nil)
	if len(cHotkeys) > 0 {
		cHotkeysPtr = &cHotkeys[0]
	}

	C.setEventTapHotkeys(eventTap.handle, cHotkeysPtr, C.int(len(cHotkeys)))
	eventTap.logger.Debug("Event tap hotkeys set")
}

// Disable deactivates the event tap, stopping keyboard event capture.
func (eventTap *EventTap) Disable() {
	eventTap.logger.Debug("Disabling event tap")
	if eventTap.handle != nil {
		C.disableEventTap(eventTap.handle)
		eventTap.logger.Debug("Event tap disabled")
	} else {
		eventTap.logger.Warn("Cannot disable nil event tap")
	}
}

// Destroy cleans up the event tap resources and releases system hooks.
// This method ensures proper cleanup by disabling the tap first and clearing references.
func (eventTap *EventTap) Destroy() {
	eventTap.logger.Debug("Destroying event tap")
	if eventTap.handle != nil {
		// Disable first to prevent any pending callbacks
		eventTap.Disable()

		// Destroy the tap
		C.destroyEventTap(eventTap.handle)
		eventTap.handle = nil

		// Clear callback to prevent any lingering references
		eventTap.callback = nil

		// Clear global reference if this is the global event tap
		globalEventTapMu.Lock()
		if globalEventTap == eventTap {
			globalEventTap = nil
		}
		globalEventTapMu.Unlock()

		eventTap.logger.Debug("Event tap destroyed")
	} else {
		eventTap.logger.Warn("Cannot destroy nil event tap")
	}
}

// handleCallback processes key press events received from the C event tap bridge.
// It forwards the key information to the registered callback function if one exists.
func (eventTap *EventTap) handleCallback(key string) {
	eventTap.logger.Debug("Key pressed", zap.String("key", key))

	if eventTap.callback != nil {
		eventTap.logger.Debug("Calling event tap callback")
		eventTap.callback(key)
	} else {
		eventTap.logger.Debug("No callback registered for event tap")
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
