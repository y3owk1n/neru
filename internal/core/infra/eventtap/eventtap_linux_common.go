//go:build linux

package eventtap

import (
	"os"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

type (
	// Callback is invoked when a key event is intercepted.
	Callback func(key string)
	// PassthroughCallback is invoked when a modifier key is in passthrough mode.
	PassthroughCallback func()
)

type pendingSyntheticModifierEvent struct {
	modifier  string
	isDown    bool
	expiresAt time.Time
}

const syntheticModifierSuppressionWindow = 250 * time.Millisecond

// EventTap intercepts keyboard events on Linux.
type EventTap struct {
	logger *zap.Logger

	mu                   sync.RWMutex
	callback             Callback
	passthroughCallback  PassthroughCallback
	hotkeys              []string
	stickyModifierToggle bool
	enabled              bool

	syntheticModifierEvents []pendingSyntheticModifierEvent

	stopCh chan struct{}
	doneCh chan struct{}
}

// NewEventTap creates a new EventTap instance.
func NewEventTap(callback Callback, logger *zap.Logger) *EventTap {
	return &EventTap{
		logger:   logger,
		callback: callback,
	}
}

// Enable starts intercepting keyboard events.
func (et *EventTap) Enable() {
	et.mu.Lock()
	if et.enabled {
		et.mu.Unlock()

		return
	}

	et.stopCh = make(chan struct{})
	et.doneCh = make(chan struct{})
	et.enabled = true
	et.mu.Unlock()

	go et.run()
}

// Disable stops intercepting keyboard events.
func (et *EventTap) Disable() {
	et.mu.Lock()
	if !et.enabled {
		et.mu.Unlock()

		return
	}

	stopCh := et.stopCh
	doneCh := et.doneCh
	et.enabled = false
	et.mu.Unlock()

	close(stopCh)
	<-doneCh
}

// Destroy stops and cleans up the EventTap.
func (et *EventTap) Destroy() {
	et.Disable()
}

// SetHandler sets the callback for key events.
func (et *EventTap) SetHandler(handler func(key string)) {
	et.mu.Lock()
	defer et.mu.Unlock()

	et.callback = handler
}

// SetHotkeys configures the hotkey list.
func (et *EventTap) SetHotkeys(hotkeys []string) {
	et.mu.Lock()
	defer et.mu.Unlock()

	et.hotkeys = append([]string(nil), hotkeys...)
}

// SetModifierPassthrough enables/disables modifier passthrough.
func (et *EventTap) SetModifierPassthrough(_ bool, _ []string) {}

// SetInterceptedModifierKeys sets which modifier keys to intercept.
func (et *EventTap) SetInterceptedModifierKeys(_ []string) {}

// SetPassthroughCallback sets the callback for passthrough mode.
func (et *EventTap) SetPassthroughCallback(cb PassthroughCallback) {
	et.mu.Lock()
	defer et.mu.Unlock()

	et.passthroughCallback = cb
}

// SetStickyModifierToggle enables/disables sticky modifier toggle.
func (et *EventTap) SetStickyModifierToggle(enabled bool) {
	et.mu.Lock()
	defer et.mu.Unlock()

	et.stickyModifierToggle = enabled
}

// PostModifierEvent posts a modifier key event.
func (et *EventTap) PostModifierEvent(modifier string, isDown bool) {
	modifier = canonicalLinuxModifier(modifier)
	if modifier == "" {
		return
	}

	et.rememberSyntheticModifierEvent(modifier, isDown)
	if !postLinuxModifierEvent(modifier, isDown) {
		et.consumeSyntheticModifierEvent(modifier, isDown)
	}
}

// SetKeyboardLayout sets the keyboard layout.
func (et *EventTap) SetKeyboardLayout(_ string) bool { return true }

// IsEnabled returns whether interception is active.
func (et *EventTap) IsEnabled() bool {
	et.mu.RLock()
	defer et.mu.RUnlock()

	return et.enabled
}

// run starts the event interception loop.
func (et *EventTap) run() {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		et.runWayland()
	} else {
		et.runX11()
	}
}

// dispatchKey dispatches a key event to the callback.
func (et *EventTap) dispatchKey(key string) {
	et.mu.RLock()
	callback := et.callback
	et.mu.RUnlock()

	if callback != nil && key != "" {
		callback(key)
	}
}

// stickyToggleEnabled returns whether sticky toggle is active.
func (et *EventTap) stickyToggleEnabled() bool {
	et.mu.RLock()
	defer et.mu.RUnlock()

	return et.stickyModifierToggle
}

func canonicalLinuxModifier(modifier string) string {
	switch strings.ToLower(strings.TrimSpace(modifier)) {
	case "cmd", "command", "super", "meta":
		return "cmd"
	case "shift":
		return "shift"
	case "alt", "option":
		return "alt"
	case "ctrl", "control":
		return "ctrl"
	default:
		return ""
	}
}

func linuxModifierToggleEvent(modifier string, isDown bool) string {
	modifier = canonicalLinuxModifier(modifier)
	if modifier == "" {
		return ""
	}

	suffix := "up"
	if isDown {
		suffix = "down"
	}

	return "__modifier_" + modifier + "_" + suffix
}

func (et *EventTap) rememberSyntheticModifierEvent(modifier string, isDown bool) {
	now := time.Now()

	et.mu.Lock()
	defer et.mu.Unlock()

	pending := et.syntheticModifierEvents[:0]
	for _, event := range et.syntheticModifierEvents {
		if now.Before(event.expiresAt) {
			pending = append(pending, event)
		}
	}

	et.syntheticModifierEvents = append(pending, pendingSyntheticModifierEvent{
		modifier:  modifier,
		isDown:    isDown,
		expiresAt: now.Add(syntheticModifierSuppressionWindow),
	})
}

func (et *EventTap) consumeSyntheticModifierEvent(modifier string, isDown bool) bool {
	now := time.Now()

	et.mu.Lock()
	defer et.mu.Unlock()

	pending := et.syntheticModifierEvents[:0]
	consumed := false

	for _, event := range et.syntheticModifierEvents {
		if !now.Before(event.expiresAt) {
			continue
		}

		if !consumed && event.modifier == modifier && event.isDown == isDown {
			consumed = true

			continue
		}

		pending = append(pending, event)
	}

	et.syntheticModifierEvents = pending

	return consumed
}

func normalizeLinuxKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}

	// Split modifiers from base key
	parts := strings.Split(key, "+")
	baseKey := parts[len(parts)-1]

	switch strings.ToLower(baseKey) {
	case "return":
		baseKey = "Return"
	case "space":
		baseKey = "Space"
	case "tab":
		baseKey = "Tab"
	case "escape", "esc":
		baseKey = "Escape"
	case "backspace":
		baseKey = "Delete"
	case "left":
		baseKey = "Left"
	case "right":
		baseKey = "Right"
	case "up":
		baseKey = "Up"
	case "down":
		baseKey = "Down"
	default:
		if len([]rune(baseKey)) == 1 {
			baseKey = strings.ToLower(baseKey)
		}
	}

	parts[len(parts)-1] = baseKey

	return strings.Join(parts, "+")
}
