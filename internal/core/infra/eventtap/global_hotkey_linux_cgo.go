//go:build linux && cgo

// internal/core/infra/eventtap/global_hotkey_linux_cgo.go
// Passive evdev global-hotkey listener: watches keyboards (no grab) while Neru
// is idle and fires callbacks when a configured chord is pressed.
// Does NOT grab the keyboard or inject input; it only reads, so the focused app
// still receives the keys. While a mode is active the in-mode eventtap grabs the
// same devices, so this listener naturally goes quiet until the mode exits.

package eventtap

import (
	"sync"

	"go.uber.org/zap"
)

// GlobalHotkeyListener is the Wayland substitute for OS-level global hotkeys,
// which compositors do not expose to ordinary clients. It honors Neru's own
// config keybindings by reading evdev directly.
type GlobalHotkeyListener struct {
	logger *zap.Logger

	mu       sync.Mutex
	bindings map[string]func()
	capture  *waylandEvdevCapture
	stopCh   chan struct{}
	running  bool
}

// NewGlobalHotkeyListener creates an inactive listener. Call Start to begin
// reading the keyboard.
func NewGlobalHotkeyListener(logger *zap.Logger) *GlobalHotkeyListener {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &GlobalHotkeyListener{
		logger:   logger.Named("hotkeys.evdev"),
		bindings: make(map[string]func()),
	}
}

// SetBinding registers a callback for a chord string (e.g. "Ctrl+Shift+G").
// Safe to call before or after Start.
func (l *GlobalHotkeyListener) SetBinding(chord string, callback func()) {
	signature := canonicalChordSignature(chord)
	if signature == "" || callback == nil {
		return
	}

	l.mu.Lock()
	l.bindings[signature] = callback
	l.mu.Unlock()
}

// ClearBindings removes every chord binding without stopping the reader.
func (l *GlobalHotkeyListener) ClearBindings() {
	l.mu.Lock()
	l.bindings = make(map[string]func())
	l.mu.Unlock()
}

// Start opens the keyboards read-only and begins watching for chords. It is
// idempotent. An error is returned when no keyboard can be opened (typically a
// permissions problem: the user needs read access to /dev/input/event*).
func (l *GlobalHotkeyListener) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.running {
		return nil
	}

	capture, err := newWaylandEvdevCapture(l.logger)
	if err != nil {
		return err
	}

	// Intentionally no grabAll(): a passive read leaves keys flowing to the
	// focused application, which is what a global hotkey should do.
	capture.startReaders()

	l.capture = capture
	l.stopCh = make(chan struct{})
	l.running = true

	go l.run(capture, l.stopCh)

	l.logger.Info(
		"Wayland evdev global hotkey listener active",
		zap.Int("devices", len(capture.files)),
	)

	return nil
}

// Stop halts watching and releases the keyboards. Idempotent.
func (l *GlobalHotkeyListener) Stop() {
	l.mu.Lock()
	if !l.running {
		l.mu.Unlock()

		return
	}

	close(l.stopCh)
	capture := l.capture
	l.capture = nil
	l.running = false
	l.mu.Unlock()

	if capture != nil {
		capture.Close()
	}
}

func (l *GlobalHotkeyListener) run(capture *waylandEvdevCapture, stopCh chan struct{}) {
	state := waylandEvdevKeyState{pressed: make(map[uint16]bool)}

	for {
		select {
		case <-stopCh:
			return
		case event, ok := <-capture.events:
			if !ok {
				// Events channel closed: all reader goroutines exited. This can
				// happen when the evdev file descriptors become stale after
				// sleep/wake or device disconnection. Auto-restart the capture
				// unless Stop() was already called.
				if !l.tryRestartLocked(&capture, &stopCh, &state) {
					return
				}

				continue
			}

			l.handleEvent(&state, event)
		}
	}
}

// tryRestartLocked attempts to re-establish the evdev capture after a reader
// failure. The caller must NOT hold l.mu. Returns true when the capture was
// replaced successfully and the loop should continue; false when Stop was
// called concurrently or evdev remains unavailable and the loop must exit.
func (l *GlobalHotkeyListener) tryRestartLocked(
	capture **waylandEvdevCapture,
	stopCh *chan struct{},
	state *waylandEvdevKeyState,
) bool {
	l.mu.Lock()

	if !l.running {
		l.mu.Unlock()

		return false
	}

	newCapture, err := newWaylandEvdevCapture(l.logger)
	if err != nil {
		l.logger.Warn(
			"Evdev hotkey listener readers died and reconnection failed; "+
				"global hotkeys will stop working until neru is restarted",
			zap.Error(err),
		)
		l.running = false
		l.mu.Unlock()

		return false
	}

	newCapture.startReaders()

	// Close the old capture in the background so we don't hold l.mu across
	// the blocking ungrab/close calls.
	oldCapture := *capture

	*capture = newCapture
	*stopCh = make(chan struct{})
	*state = waylandEvdevKeyState{pressed: make(map[uint16]bool)}
	l.capture = newCapture
	l.stopCh = *stopCh
	l.mu.Unlock()

	if oldCapture != nil {
		go oldCapture.Close()
	}

	l.logger.Info(
		"Evdev hotkey listener reconnected after reader failure",
		zap.Int("devices", len(newCapture.files)),
	)

	return true
}

// IsRunning reports whether the listener is actively watching for chords.
func (l *GlobalHotkeyListener) IsRunning() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.running
}

func (l *GlobalHotkeyListener) handleEvent(
	state *waylandEvdevKeyState,
	event waylandEvdevEvent,
) {
	if event.eventType != evdevEventKey {
		return
	}

	if modifier := evdevModifierName(event.code); modifier != "" {
		if event.value == evdevValueRepeat {
			return
		}

		isDown := event.value == evdevValuePress
		state.trackKey(event.code, isDown)
		state.modifiers.update(modifier, isDown)

		return
	}

	// Only fire on the initial press so holding the chord does not re-trigger.
	if event.value != evdevValuePress {
		if event.value == evdevValueRelease {
			state.trackKey(event.code, false)
		}

		return
	}

	state.trackKey(event.code, true)

	key := evdevKeyName(event.code)
	if key == "" {
		return
	}

	signature := canonicalChordSignature(state.modifiers.prefix() + key)
	if signature == "" {
		return
	}

	l.mu.Lock()
	callback := l.bindings[signature]
	l.mu.Unlock()

	if callback == nil {
		return
	}

	l.logger.Debug("Global hotkey matched", zap.String("chord", signature))

	go callback()
}
