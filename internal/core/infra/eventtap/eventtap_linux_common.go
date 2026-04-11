//go:build linux

package eventtap

import (
	"os"
	"strings"
	"sync"

	"go.uber.org/zap"
)

type Callback func(key string)
type PassthroughCallback func()

type EventTap struct {
	logger *zap.Logger

	mu                   sync.RWMutex
	callback             Callback
	passthroughCallback  PassthroughCallback
	hotkeys              []string
	stickyModifierToggle bool
	enabled              bool

	stopCh chan struct{}
	doneCh chan struct{}
}

func NewEventTap(callback Callback, logger *zap.Logger) *EventTap {
	return &EventTap{
		logger:   logger,
		callback: callback,
	}
}

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

func (et *EventTap) run() {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		et.runWayland()
	} else {
		et.runX11()
	}
}

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

func (et *EventTap) Destroy() {
	et.Disable()
}

func (et *EventTap) SetHotkeys(hotkeys []string) {
	et.mu.Lock()
	defer et.mu.Unlock()
	et.hotkeys = append([]string(nil), hotkeys...)
}

func (et *EventTap) SetModifierPassthrough(_ bool, _ []string) {}

func (et *EventTap) SetInterceptedModifierKeys(_ []string) {}

func (et *EventTap) SetPassthroughCallback(cb PassthroughCallback) {
	et.mu.Lock()
	defer et.mu.Unlock()
	et.passthroughCallback = cb
}

func (et *EventTap) SetStickyModifierToggle(enabled bool) {
	et.mu.Lock()
	defer et.mu.Unlock()
	et.stickyModifierToggle = enabled
}

func (et *EventTap) PostModifierEvent(_ string, _ bool) {}

func (et *EventTap) SetKeyboardLayout(_ string) bool { return true }

func (et *EventTap) IsEnabled() bool {
	et.mu.RLock()
	defer et.mu.RUnlock()
	return et.enabled
}

func (et *EventTap) SetHandler(handler func(key string)) {
	et.mu.Lock()
	defer et.mu.Unlock()
	et.callback = handler
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
