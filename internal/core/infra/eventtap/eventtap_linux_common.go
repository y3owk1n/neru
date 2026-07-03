//go:build linux

package eventtap

import (
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/ui/overlay"
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

const dispatchChBufferSize = 256

// linuxModifierState tracks the reference count of each modifier key group.
// Counts may go transiently negative when a grab captures the keyboard after
// some modifiers were already held (the release arrives without a matching
// press). The <= 0 sentinel in allZero() handles this gracefully.
type linuxModifierState struct {
	shift int
	ctrl  int
	alt   int
	cmd   int
}

func (s *linuxModifierState) update(modifier string, isDown bool) {
	delta := 1
	if !isDown {
		delta = -1
	}

	switch modifier {
	case evdevModifierShift:
		s.shift += delta
	case evdevModifierCtrl:
		s.ctrl += delta
	case evdevModifierAlt:
		s.alt += delta
	case evdevModifierCmd:
		s.cmd += delta
	}
}

func (s *linuxModifierState) allZero() bool {
	return s.shift <= 0 && s.ctrl <= 0 && s.alt <= 0 && s.cmd <= 0
}

// EventTap intercepts keyboard events on Linux.
type EventTap struct {
	logger *zap.Logger

	mu                   sync.RWMutex
	callback             Callback
	passthroughCallback  PassthroughCallback
	hotkeys              []string
	stickyModifierToggle bool
	enabled              bool

	// Detection arming: sticky modifier events are only dispatched once all
	// initially-held modifiers have been released (matching macOS behavior).
	// SetStickyModifierToggle(true) disarms; the platform handler re-arms when
	// the modifier state reaches a clean slate.
	stickyModifierDetectionArmed bool

	syntheticModifierEvents []pendingSyntheticModifierEvent

	stopCh chan struct{}
	doneCh chan struct{}

	// dispatchCh decouples the event-tap goroutine from the callback
	// goroutine, preventing a deadlock when a key dispatch triggers a mode
	// exit that waits for the event-tap goroutine to stop.
	// The event-tap goroutine enqueues keys here; the dispatch goroutine
	// reads from this channel and invokes the callback. This matches the
	// macOS eventtap design.
	dispatchCh chan string
	dispatchWg sync.WaitGroup
	destroyed  bool

	// dispatchEpoch is incremented on every Disable(). dispatchLoop
	// snapshots the epoch before processing a key and verifies it
	// hasn't changed before invoking the callback. This prevents
	// stale buffered events from leaking across enable/disable cycles.
	dispatchEpoch atomic.Uint64
}

// NewEventTap creates a new EventTap instance.
func NewEventTap(callback Callback, logger *zap.Logger) *EventTap {
	tap := &EventTap{
		logger:   logger,
		callback: callback,
	}
	tap.dispatchCh = make(chan string, dispatchChBufferSize)
	tap.dispatchWg.Add(1)

	go tap.dispatchLoop()

	return tap
}

// Enable starts intercepting keyboard events.
func (et *EventTap) Enable() {
	et.mu.Lock()
	if et.enabled || et.destroyed {
		et.mu.Unlock()

		return
	}

	et.stopCh = make(chan struct{})
	et.doneCh = make(chan struct{})
	et.enabled = true
	et.mu.Unlock()

	// Initialize uinput scroll device on Enable for Wayland backends.
	// If successful, scroll events will go directly to applications
	// via the virtual device, bypassing the overlay.
	go func() {
		_, err := getUinputScrollFd()
		if err == nil {
			// Scroll device created - disable exclusive keyboard
			// so scroll events pass through to active application
			if m := overlay.Get(); m != nil {
				m.SetKeyboardCaptureEnabled(false)
			}
		}
	}()

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

	// Bump the dispatch epoch so any in-flight event that dispatchLoop
	// picked up before we drained will be discarded rather than delivered
	// to the callback.
	et.dispatchEpoch.Add(1)

	// Drain any stale events from the dispatch channel. After the evdev
	// goroutine has exited, no new events are being enqueued, so whatever
	// remains in the buffer was enqueued before the stop signal landed.
	// These stale events must be discarded to prevent them from being
	// misinterpreted by the next mode's handler after the event tap is
	// re-enabled.
	for {
		select {
		case <-et.dispatchCh:
		default:
			return
		}
	}
}

// Destroy stops and cleans up the EventTap.
// It is safe to call multiple times — subsequent calls are no-ops.
func (et *EventTap) Destroy() {
	et.mu.Lock()
	if et.destroyed {
		et.mu.Unlock()

		return
	}

	et.destroyed = true
	et.mu.Unlock()

	et.Disable()

	// Stop the dispatch goroutine and wait for it to finish.
	// The dispatchCh is created once in NewEventTap and lives for the
	// entire lifetime of the EventTap, so we close the channel to signal
	// the dispatch goroutine to exit.
	close(et.dispatchCh)
	et.dispatchWg.Wait()
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
	if enabled {
		// Disarm detection: the platform handler will re-arm once the
		// modifier state reaches a clean slate (all pre-held modifiers
		// released). This matches macOS behavior where modifier events
		// from the activation chord are not interpreted as sticky toggles.
		et.stickyModifierDetectionArmed = false
	} else {
		et.stickyModifierDetectionArmed = true
	}
}

// PostModifierEvent posts a modifier key event.
func (et *EventTap) PostModifierEvent(modifier string, isDown bool) {
	modifier = canonicalLinuxModifier(modifier)
	if modifier == "" {
		return
	}

	// On X11, synthetic modifier events (from XTest) re-enter the event tap
	// loop and must be suppressed so they don't trigger __modifier_ events.
	// On Wayland, zwp_virtual_keyboard_v1_modifiers does not generate evdev
	// or wl_keyboard events, so the synthetic event never re-enters.
	// Remembering it would falsely suppress a genuine physical modifier
	// press within the suppression window.
	onWayland := os.Getenv("WAYLAND_DISPLAY") != ""
	if !onWayland {
		et.rememberSyntheticModifierEvent(modifier, isDown)
	}

	if !postLinuxModifierEvent(modifier, isDown) {
		if !onWayland {
			et.consumeSyntheticModifierEvent(modifier, isDown)
		}
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

// stickyArmDetection arms sticky modifier detection. The platform handler
// calls this when it determines all pre-held modifiers have been released.
func (et *EventTap) stickyArmDetection() {
	et.mu.Lock()
	defer et.mu.Unlock()

	et.stickyModifierDetectionArmed = true
}

// stickyDetectionArmed returns whether sticky detection is armed.
func (et *EventTap) stickyDetectionArmed() bool {
	et.mu.RLock()
	defer et.mu.RUnlock()

	return et.stickyModifierDetectionArmed
}

// run starts the event interception loop.
func (et *EventTap) run() {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		et.runWayland()
	} else {
		et.runX11()
	}
}

// dispatchKey enqueues a key event for dispatch. The callback is invoked
// from a dedicated dispatch goroutine so that the event-tap goroutine never
// blocks on the callback (preventing a deadlock when the callback triggers
// a mode exit that waits for the event-tap goroutine to stop).
func (et *EventTap) dispatchKey(key string) {
	if key == "" {
		return
	}

	et.mu.RLock()
	destroyed := et.destroyed
	et.mu.RUnlock()

	if destroyed {
		return
	}

	select {
	case et.dispatchCh <- key:
	default:
		if et.logger != nil {
			et.logger.Warn("Dispatch channel full, dropping key", zap.String("key", key))
		}
	}
}

// dispatchLoop reads key events from the dispatch channel and invokes the
// registered callback. It runs in a dedicated goroutine that lives for the
// entire lifetime of the EventTap.
func (et *EventTap) dispatchLoop() {
	defer et.dispatchWg.Done()

	for key := range et.dispatchCh {
		epoch := et.dispatchEpoch.Load()

		et.mu.RLock()
		cb := et.callback
		et.mu.RUnlock()

		if cb != nil && et.dispatchEpoch.Load() == epoch {
			cb(key)
		}
	}
}

// linuxKeyUpPrefix matches modes.keyUpPrefix — signals key release to stop held-key repeat.
const linuxKeyUpPrefix = "__keyup_"

// linuxKeyUpEvent formats a key-up notification for held-repeat actions (scroll, page, etc.).
// Uses the base key (no modifier prefix) to match modes.Handler held-key tracking.
func linuxKeyUpEvent(key string) string {
	key = normalizeLinuxKey(key)
	if key == "" {
		return ""
	}

	parts := strings.Split(key, "+")
	baseKey := parts[len(parts)-1]

	return linuxKeyUpPrefix + baseKey
}

// stickyToggleEnabled returns whether sticky toggle is active.
func (et *EventTap) stickyToggleEnabled() bool {
	et.mu.RLock()
	defer et.mu.RUnlock()

	return et.stickyModifierToggle
}

func canonicalLinuxModifier(modifier string) string {
	switch strings.ToLower(strings.TrimSpace(modifier)) {
	case evdevModifierCmd, "command", evdevModifierAliasSuper, "meta":
		return evdevModifierCmd
	case evdevModifierShift:
		return evdevModifierShift
	case evdevModifierAlt, evdevModifierAliasOption:
		return evdevModifierAlt
	case evdevModifierCtrl, evdevModifierAliasControl:
		return evdevModifierCtrl
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

	pending := make([]pendingSyntheticModifierEvent, 0, len(et.syntheticModifierEvents))
	for _, event := range et.syntheticModifierEvents {
		if now.Before(event.expiresAt) {
			pending = append(pending, event)
		}
	}

	pending = append(pending, pendingSyntheticModifierEvent{
		modifier:  modifier,
		isDown:    isDown,
		expiresAt: now.Add(syntheticModifierSuppressionWindow),
	})
	et.syntheticModifierEvents = pending
}

func (et *EventTap) consumeSyntheticModifierEvent(modifier string, isDown bool) bool {
	now := time.Now()

	et.mu.Lock()
	defer et.mu.Unlock()

	pending := make([]pendingSyntheticModifierEvent, 0, len(et.syntheticModifierEvents))
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
		baseKey = evdevKeyNameReturn
	case "space":
		baseKey = "Space"
	case "tab":
		baseKey = "Tab"
	case "escape", "esc":
		baseKey = evdevKeyNameEscape
	case "backspace":
		baseKey = "Delete"
	case "left":
		baseKey = evdevKeyNameLeft
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
