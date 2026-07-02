//go:build darwin

package eventtap

/*
#cgo CFLAGS: -x objective-c
#include "../platform/darwin/eventtap.h"
#include <stdlib.h>

extern void eventTapCallbackBridge(char* key, void* userData);
extern void eventTapPassthroughBridge(void* userData);
*/
import "C"

import (
	"sync"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/infra/platform/darwin"
)

// Callback defines the function signature for handling key press events.
type Callback func(key string)

// PassthroughCallback is invoked when a modifier shortcut passes through to macOS.
type PassthroughCallback func()

type callbackEventKind uint8

const (
	callbackEventKey callbackEventKind = iota
	callbackEventPassthrough
)

type callbackEvent struct {
	kind                callbackEventKind
	key                 string
	passthroughCallback PassthroughCallback
}

// unboundedQueue is an infinite-capacity event queue.
// Producers always succeed (non-blocking); the consumer drains via next().
type unboundedQueue struct {
	mu    sync.Mutex
	cond  *sync.Cond
	queue []callbackEvent
	done  bool
}

func newUnboundedQueue() *unboundedQueue {
	q := &unboundedQueue{}
	q.cond = sync.NewCond(&q.mu)

	return q
}

func (q *unboundedQueue) push(event callbackEvent) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.done {
		return
	}
	q.queue = append(q.queue, event)
	q.cond.Signal()
}

func (q *unboundedQueue) next() (callbackEvent, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.queue) == 0 && !q.done {
		q.cond.Wait()
	}
	if len(q.queue) > 0 {
		event := q.queue[0]
		q.queue[0] = callbackEvent{} // allow GC
		q.queue = q.queue[1:]

		return event, true
	}

	return callbackEvent{}, false
}

func (q *unboundedQueue) close() {
	q.mu.Lock()
	q.done = true
	q.cond.Broadcast()
	q.mu.Unlock()
}

// EventTap represents a keyboard event interceptor that captures global key presses.
type EventTap struct {
	handle C.EventTap
	logger *zap.Logger

	callbackMu          sync.RWMutex
	callback            Callback
	passthroughCallback PassthroughCallback

	queue        *unboundedQueue
	stopDispatch chan struct{}
	stopOnce     sync.Once
	dispatchWg   sync.WaitGroup
}

// NewEventTap initializes a new event tap for capturing global keyboard events.
// Returns nil if the event tap cannot be created, typically due to missing Accessibility permissions.
func NewEventTap(callback Callback, logger *zap.Logger) *EventTap {
	if logger == nil {
		logger = zap.NewNop()
	}

	logger = logger.Named("eventtap")

	eventTap := &EventTap{
		callback:     callback,
		logger:       logger,
		queue:        newUnboundedQueue(),
		stopDispatch: make(chan struct{}),
	}

	eventTap.handle = C.NeruCreateEventTap(C.EventTapCallback(C.eventTapCallbackBridge), nil)
	if eventTap.handle == nil {
		logger.Error("Failed to create event tap - check Accessibility permissions")

		return nil
	}

	eventTap.startDispatcher()

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
		C.NeruEnableEventTap(et.handle)
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
		} else {
			cHotkeys[index] = nil
		}
	}

	// Pass pointer to first element and length
	cHotkeysPtr := (**C.char)(nil)
	if len(cHotkeys) > 0 {
		cHotkeysPtr = &cHotkeys[0]
	}

	C.NeruSetEventTapHotkeys(et.handle, cHotkeysPtr, C.int(len(cHotkeys)))
	et.logger.Debug("Event tap hotkeys set")
}

// SetModifierPassthrough configures whether unbound modifier shortcuts should
// pass through to macOS and which shortcuts remain blacklisted.
func (et *EventTap) SetModifierPassthrough(enabled bool, blacklist []string) {
	et.logger.Debug(
		"Setting event tap modifier passthrough",
		zap.Bool("enabled", enabled),
		zap.Int("blacklist_count", len(blacklist)),
	)

	if et.handle == nil {
		et.logger.Warn("Cannot set modifier passthrough on nil event tap")

		return
	}

	cKeys := make([]*C.char, len(blacklist))
	for index, key := range blacklist {
		if key != "" {
			cKeys[index] = C.CString(key)

			defer C.free(unsafe.Pointer(cKeys[index])) //nolint:nlreturn
		} else {
			cKeys[index] = nil
		}
	}

	cKeysPtr := (**C.char)(nil)
	if len(cKeys) > 0 {
		cKeysPtr = &cKeys[0]
	}

	enabledValue := 0
	if enabled {
		enabledValue = 1
	}

	C.NeruSetEventTapModifierPassthrough(
		et.handle,
		C.int(enabledValue),
		cKeysPtr,
		C.int(len(cKeys)),
	)
	et.logger.Debug("Event tap modifier passthrough set")
}

// SetInterceptedModifierKeys configures modifier shortcuts that the active mode
// still wants Neru to consume.
func (et *EventTap) SetInterceptedModifierKeys(keys []string) {
	et.logger.Debug("Setting intercepted modifier keys", zap.Int("count", len(keys)))

	if et.handle == nil {
		et.logger.Warn("Cannot set intercepted modifier keys on nil event tap")

		return
	}

	cKeys := make([]*C.char, len(keys))
	for index, key := range keys {
		if key != "" {
			cKeys[index] = C.CString(key)

			defer C.free(unsafe.Pointer(cKeys[index])) //nolint:nlreturn
		} else {
			cKeys[index] = nil
		}
	}

	cKeysPtr := (**C.char)(nil)
	if len(cKeys) > 0 {
		cKeysPtr = &cKeys[0]
	}

	C.NeruSetEventTapInterceptedModifierKeys(et.handle, cKeysPtr, C.int(len(cKeys)))
	et.logger.Debug("Intercepted modifier keys set")
}

// SetPassthroughCallback registers a function to call when a modifier shortcut
// passes through to macOS. Pass nil to clear the callback.
func (et *EventTap) SetPassthroughCallback(callback PassthroughCallback) {
	if et.handle == nil {
		et.logger.Warn("Cannot set passthrough callback on nil event tap")

		return
	}

	et.callbackMu.Lock()
	defer et.callbackMu.Unlock()

	et.passthroughCallback = callback

	if callback != nil {
		C.NeruSetEventTapPassthroughCallback(
			et.handle,
			C.EventTapPassthroughCallback(C.eventTapPassthroughBridge),
		)
	} else {
		C.NeruSetEventTapPassthroughCallback(et.handle, nil)
	}
}

// SetStickyModifierToggle enables or disables sticky modifier toggle detection.
// When enabled, modifier key events (Shift, Cmd, Alt, Ctrl) are detected and
// callback is invoked with "__modifier_<name>_down/up" strings.
func (et *EventTap) SetStickyModifierToggle(enabled bool) {
	if et.handle == nil {
		et.logger.Warn("Cannot set sticky modifier toggle on nil event tap")

		return
	}

	cEnabled := C.int(0)
	if enabled {
		cEnabled = C.int(1)
	}

	C.NeruSetEventTapStickyModifierToggle(et.handle, cEnabled)
}

// SetKeyboardLayout configures the reference keyboard layout used by key translation.
// Returns false when an explicit layout ID is provided but cannot be resolved.
func (et *EventTap) SetKeyboardLayout(layoutID string) bool {
	return darwin.SetReferenceKeyboardLayout(layoutID)
}

// PostModifierEvent simulates a physical modifier key press or release.
// modifier must be one of "cmd", "shift", "alt", "ctrl".
func (et *EventTap) PostModifierEvent(modifier string, isDown bool) {
	cModifier := C.CString(modifier)
	defer C.free(unsafe.Pointer(cModifier)) //nolint:nlreturn

	cDown := C.int(0)
	if isDown {
		cDown = C.int(1)
	}

	C.NeruPostEventTapModifierEvent(cModifier, cDown)
}

// Disable deactivates the event tap, stopping keyboard event capture.
func (et *EventTap) Disable() {
	et.logger.Debug("Disabling event tap")
	if et.handle != nil {
		C.NeruDisableEventTap(et.handle)
		et.logger.Debug("Event tap disabled")
	} else {
		et.logger.Warn("Cannot disable nil event tap")
	}
}

// Destroy cleans up the event tap resources and releases system hooks.
// This method ensures proper cleanup by disabling the tap first and clearing references.
func (et *EventTap) Destroy() {
	et.logger.Debug("Destroying event tap")

	// Clear global reference first so no new C callbacks enqueue keys while
	// teardown is in progress.
	globalEventTapMu.Lock()
	if globalEventTap == et {
		globalEventTap = nil
	}
	globalEventTapMu.Unlock()

	et.stopDispatcher()

	if et.handle == nil {
		et.logger.Warn("Cannot destroy nil event tap")

		return
	}

	// Disable first to prevent any pending callbacks
	et.Disable()

	// Destroy the tap
	C.NeruDestroyEventTap(et.handle)
	et.handle = nil

	// Clear callbacks to prevent any lingering references
	et.callbackMu.Lock()
	defer et.callbackMu.Unlock()

	et.callback = nil
	et.passthroughCallback = nil

	et.logger.Debug("Event tap destroyed")
}

// handleKeyCallback processes key press events received from the C event tap darwin.
// It forwards the key information to the registered callback function if one exists.
func (et *EventTap) handleKeyCallback(key string) {
	et.callbackMu.RLock()
	callback := et.callback
	et.callbackMu.RUnlock()

	if callback != nil {
		callback(key)
	}
}

func (et *EventTap) handlePassthroughCallback(callback PassthroughCallback) {
	if callback != nil {
		callback()
	}
}

func (et *EventTap) startDispatcher() {
	et.dispatchWg.Go(func() {
		for {
			select {
			case <-et.stopDispatch:
				// Drain any remaining events before exit
				for {
					event, ok := et.queue.next()
					if !ok {
						return
					}
					switch event.kind {
					case callbackEventKey:
						et.handleKeyCallback(event.key)
					case callbackEventPassthrough:
						et.handlePassthroughCallback(event.passthroughCallback)
					}
				}
			default:
			}

			event, ok := et.queue.next()
			if !ok {
				return
			}
			switch event.kind {
			case callbackEventKey:
				et.handleKeyCallback(event.key)
			case callbackEventPassthrough:
				et.handlePassthroughCallback(event.passthroughCallback)
			}
		}
	})
}

func (et *EventTap) stopDispatcher() {
	et.stopOnce.Do(func() {
		et.queue.close()
		close(et.stopDispatch)
	})

	et.dispatchWg.Wait()
}

func (et *EventTap) enqueueKey(key string) {
	select {
	case <-et.stopDispatch:
		return
	default:
	}

	et.queue.push(callbackEvent{
		kind: callbackEventKey,
		key:  key,
	})
}

func (et *EventTap) enqueuePassthrough(callback PassthroughCallback) {
	if callback == nil {
		return
	}

	select {
	case <-et.stopDispatch:
		return
	default:
	}

	et.queue.push(callbackEvent{
		kind:                callbackEventPassthrough,
		passthroughCallback: callback,
	})
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
		eventTap.enqueueKey(goKey)
	}
}

// eventTapPassthroughBridge is the C-to-Go callback bridge for passthrough
// notifications. It is invoked on the event tap thread when a modifier shortcut
// passes through to macOS. The callback snapshot is enqueued onto the existing
// dispatcher so delivery stays ordered with key events without blocking the
// CGEvent tap thread.
//
//export eventTapPassthroughBridge
func eventTapPassthroughBridge(_ unsafe.Pointer) {
	globalEventTapMu.RLock()
	eventTap := globalEventTap
	globalEventTapMu.RUnlock()

	if eventTap != nil {
		eventTap.callbackMu.RLock()
		cb := eventTap.passthroughCallback
		eventTap.callbackMu.RUnlock()

		if cb != nil {
			eventTap.enqueuePassthrough(cb)
		}
	}
}

// IsUinputScrollAvailable returns false on Darwin.
func IsUinputScrollAvailable() bool {
	return false
}
