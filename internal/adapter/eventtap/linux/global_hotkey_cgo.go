//go:build linux && cgo

package linux

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// GlobalHotkeyListener is the Wayland substitute for OS global hotkeys, which
// compositors do not expose. It is a set of chord bindings the evdev proxy
// matches while no mode is open, withholding a matched chord from the focused
// app. While a mode is open the mode handler resolves the same chords
// (internal/app/modes/keymap.go, settledKeymaps), and exactly one of the two
// sees any press.
type GlobalHotkeyListener struct {
	logger *zap.Logger

	mu       sync.Mutex
	bindings map[string]hotkeyBinding
	proxy    *evdevProxy
	running  bool
}

// NewGlobalHotkeyListener creates an inactive listener. Call Start to begin
// matching chords.
func NewGlobalHotkeyListener(logger *zap.Logger) *GlobalHotkeyListener {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &GlobalHotkeyListener{
		logger:   logger.Named("hotkeys.evdev"),
		bindings: make(map[string]hotkeyBinding),
	}
}

// SetBinding registers the callbacks for a chord string (e.g. "Ctrl+Shift+G"):
// press when the chord's key goes down, release (nil for none) when it comes
// up. Safe to call before or after Start.
func (l *GlobalHotkeyListener) SetBinding(chord string, press, release func()) {
	signature := canonicalBindingSignature(chord)
	if signature == "" || press == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.bindings[signature] = hotkeyBinding{press: press, release: release}
	l.publishLocked()
}

// ClearBindings removes every chord binding without stopping the listener.
func (l *GlobalHotkeyListener) ClearBindings() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.bindings = make(map[string]hotkeyBinding)
	l.publishLocked()
}

// Start attaches the bindings to the evdev proxy, building it on first use. It
// is idempotent. It fails when no keyboard can be opened, usually for lack of
// read access to /dev/input/event*.
func (l *GlobalHotkeyListener) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.running {
		return nil
	}

	proxy, err := acquireEvdevProxy(l.logger)
	if err != nil {
		return err
	}

	if proxy.deviceCount() == 0 {
		return errWaylandEvdevUnavailable
	}

	l.proxy = proxy
	l.running = true
	l.publishLocked()

	l.logger.Info(
		"Wayland evdev global hotkey listener active",
		zap.Int("devices", proxy.deviceCount()),
	)

	return nil
}

// Stop detaches the bindings. The proxy keeps forwarding keys for the
// daemon's lifetime. Idempotent.
func (l *GlobalHotkeyListener) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.running {
		return
	}

	l.running = false
	l.publishLocked()
}

// StopWithTimeout is Stop, a pointer swap. The deadline is kept for callers
// that pass one.
func (l *GlobalHotkeyListener) StopWithTimeout(_ time.Duration) bool {
	l.Stop()

	return true
}

// IsRunning reports whether the listener is actively matching chords.
func (l *GlobalHotkeyListener) IsRunning() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.running && l.proxy != nil && l.proxy.alive()
}

// DeviceCount returns the number of captured keyboard devices.
func (l *GlobalHotkeyListener) DeviceCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.proxy == nil {
		return 0
	}

	return l.proxy.deviceCount()
}

// publishLocked hands the proxy the bindings it matches, which is the whole
// set while running and none otherwise.
func (l *GlobalHotkeyListener) publishLocked() {
	if l.proxy == nil {
		return
	}

	if l.running {
		l.proxy.setBindings(l.bindings)
	} else {
		l.proxy.setBindings(nil)
	}
}
