//go:build windows

package eventtap

import (
	"context"
	"strings"
	"sync"

	"go.uber.org/zap"

	winplatform "github.com/y3owk1n/neru/internal/core/infra/platform/windows"
)

const windowsKeyUpPrefix = "__keyup_"

// Callback defines the function signature for handling key press events.
type Callback func(key string)

// PassthroughCallback is invoked when a modifier shortcut passes through to the system.
type PassthroughCallback func()

// EventTap represents a keyboard event interceptor on Windows.
type EventTap struct {
	logger *zap.Logger

	mu                   sync.RWMutex
	callback             Callback
	passthroughCallback  PassthroughCallback
	hotkeys              []string
	stickyModifierToggle bool
	enabled              bool

	hook *winplatform.KeyboardHook
}

// NewEventTap initializes a new event tap.
func NewEventTap(callback Callback, logger *zap.Logger) *EventTap {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &EventTap{
		logger:   logger.Named("eventtap"),
		callback: callback,
	}
}

// Enable enables the event tap.
func (et *EventTap) Enable() {
	et.mu.Lock()
	if et.enabled {
		et.mu.Unlock()

		return
	}

	et.enabled = true
	et.mu.Unlock()

	hook, err := winplatform.StartKeyboardHook(et.handleKey)
	if err != nil {
		et.logger.Error("failed to start keyboard hook", zap.Error(err))
		et.mu.Lock()
		et.enabled = false
		et.mu.Unlock()

		return
	}

	et.mu.Lock()
	et.hook = hook
	et.mu.Unlock()
}

// Disable disables the event tap.
func (et *EventTap) Disable() {
	et.mu.Lock()
	if !et.enabled {
		et.mu.Unlock()

		return
	}

	hook := et.hook
	et.hook = nil
	et.enabled = false
	et.mu.Unlock()

	if hook != nil {
		hook.Stop()
	}
}

// Destroy destroys the event tap.
func (et *EventTap) Destroy() {
	et.Disable()
}

// SetHotkeys sets the hotkeys.
func (et *EventTap) SetHotkeys(hotkeys []string) {
	et.mu.Lock()
	defer et.mu.Unlock()

	et.hotkeys = append([]string(nil), hotkeys...)
}

// SetModifierPassthrough sets modifier passthrough.
func (et *EventTap) SetModifierPassthrough(_ bool, _ []string) {}

// SetInterceptedModifierKeys sets intercepted modifier keys.
func (et *EventTap) SetInterceptedModifierKeys(_ []string) {}

// SetPassthroughCallback sets the passthrough callback.
func (et *EventTap) SetPassthroughCallback(cb PassthroughCallback) {
	et.mu.Lock()
	defer et.mu.Unlock()

	et.passthroughCallback = cb
}

// SetStickyModifierToggle enables or disables sticky modifier toggle detection.
func (et *EventTap) SetStickyModifierToggle(enabled bool) {
	et.mu.Lock()
	defer et.mu.Unlock()

	et.stickyModifierToggle = enabled
}

// PostModifierEvent simulates a physical modifier key press or release.
func (et *EventTap) PostModifierEvent(_ string, _ bool) {}

// SetKeyboardLayout sets the keyboard layout.
func (et *EventTap) SetKeyboardLayout(_ string) bool { return true }

// IsEnabled returns whether the event tap is enabled.
func (et *EventTap) IsEnabled() bool {
	et.mu.RLock()
	defer et.mu.RUnlock()

	return et.enabled
}

// SetHandler sets the key handler.
func (et *EventTap) SetHandler(handler func(key string)) {
	et.mu.Lock()
	defer et.mu.Unlock()

	et.callback = handler
}

// EnableWithContext enables the event tap with context.
func (et *EventTap) EnableWithContext(_ context.Context) error {
	et.Enable()

	return nil
}

// DisableWithContext disables the event tap with context.
func (et *EventTap) DisableWithContext(_ context.Context) error {
	et.Disable()

	return nil
}

// IsUinputScrollAvailable returns false on Windows.
func IsUinputScrollAvailable() bool {
	return false
}

func (et *EventTap) handleKey(key string, isUp bool) {
	if key == "" {
		return
	}

	if mod := normalizeWindowsModifier(key); mod != "" {
		if et.stickyToggleEnabled() {
			et.dispatchKey(windowsModifierToggleEvent(mod, !isUp))
		}

		return
	}

	if isUp {
		if keyUp := windowsKeyUpEvent(key); keyUp != "" {
			et.dispatchKey(keyUp)
		}

		return
	}

	et.dispatchKey(normalizeWindowsKey(key))
}

func (et *EventTap) dispatchKey(key string) {
	et.mu.RLock()
	callback := et.callback
	et.mu.RUnlock()

	if callback != nil && key != "" {
		callback(key)
	}
}

func (et *EventTap) stickyToggleEnabled() bool {
	et.mu.RLock()
	defer et.mu.RUnlock()

	return et.stickyModifierToggle
}

func normalizeWindowsModifier(key string) string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "ctrl", "control":
		return "ctrl"
	case "alt", "option":
		return "alt"
	case "shift":
		return "shift"
	case "cmd", "command", "win", "super", "meta":
		return "cmd"
	default:
		return ""
	}
}

func windowsModifierToggleEvent(modifier string, isDown bool) string {
	suffix := "up"
	if isDown {
		suffix = "down"
	}

	return "__modifier_" + modifier + "_" + suffix
}

func windowsKeyUpEvent(key string) string {
	key = normalizeWindowsKey(key)
	if key == "" {
		return ""
	}

	parts := strings.Split(key, "+")
	baseKey := parts[len(parts)-1]

	return windowsKeyUpPrefix + baseKey
}

func normalizeWindowsKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}

	parts := strings.Split(key, "+")
	baseKey := parts[len(parts)-1]

	switch strings.ToLower(baseKey) {
	case "return", "enter":
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
