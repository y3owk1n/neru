//go:build linux

package linux

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/eventtap/tap"
	"github.com/y3owk1n/neru/internal/adapter/overlay"
	overlaymanager "github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	"github.com/y3owk1n/neru/internal/adapter/platform"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/keyvocab"
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
	callback             tap.Callback
	passthroughCallback  tap.PassthroughCallback
	hotkeys              []string
	stickyModifierToggle bool
	enabled              bool

	// Modifier passthrough state. Populated by SetModifierPassthrough /
	// SetInterceptedModifierKeys and consulted only by the Wayland evdev
	// backend (handleWaylandEvdevEvent): with a virtual-keyboard injection path
	// it can re-emit an unbound chord to the focused app. The X11 grab and the
	// wl-keyboard fallback cannot re-inject selectively, so this stays inert
	// there. Chords are stored canonicalized via canonicalChordForMatch.
	passthroughEnabled   bool
	passthroughBlacklist map[string]struct{}
	interceptedChords    map[string]struct{}

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

	// evdevWaylandCapture holds a *waylandEvdevCapture created once and
	// reused across Enable/Disable cycles. This avoids re-scanning
	// /dev/input/event* devices on every mode activation, which was the
	// source of a mild delay before the grid/hints mode accepted input.
	evdevWaylandCapture     any
	evdevWaylandCaptureInit sync.Mutex
}

// NewEventTap creates a new EventTap.
func NewEventTap(callback tap.Callback, logger *zap.Logger) *EventTap {
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
			if controller, ok := overlay.Get().(overlaymanager.KeyboardCaptureController); ok {
				controller.SetKeyboardCaptureEnabled(false)
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

	// Clean up the persistent evdev capture (closes file descriptors
	// and drains reader goroutines). The closeEvdevCapture method is
	// defined in the cgo build-tagged file for Wayland+cgo builds;
	// it is a no-op when the capture was never initialized.
	et.closeEvdevCapture()

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

// SetModifierPassthrough enables/disables modifier passthrough and records the
// blacklist of chords Neru must keep consuming even when they are otherwise
// unbound (the mode layer folds the active mode's own hotkeys into this list).
// Only the Wayland evdev backend acts on it; see the struct field comment.
func (et *EventTap) SetModifierPassthrough(enabled bool, blacklist []string) {
	set := canonicalChordSet(blacklist)

	et.mu.Lock()
	et.passthroughEnabled = enabled
	et.passthroughBlacklist = set
	et.mu.Unlock()
}

// SetInterceptedModifierKeys records the modifier chords the active mode still
// wants Neru to consume while passthrough is enabled.
func (et *EventTap) SetInterceptedModifierKeys(keys []string) {
	set := canonicalChordSet(keys)

	et.mu.Lock()
	et.interceptedChords = set
	et.mu.Unlock()
}

// SetPassthroughCallback sets the callback for passthrough mode.
func (et *EventTap) SetPassthroughCallback(cb tap.PassthroughCallback) {
	et.mu.Lock()
	defer et.mu.Unlock()

	et.passthroughCallback = cb
}

// canonicalChordSet builds a lookup set of chords normalized via
// canonicalChordForMatch so runtime lookups are independent of the order the
// user wrote their hotkeys in.
func canonicalChordSet(chords []string) map[string]struct{} {
	set := make(map[string]struct{}, len(chords))

	for _, chord := range chords {
		if canonical := canonicalChordForMatch(chord); canonical != "" {
			set[canonical] = struct{}{}
		}
	}

	return set
}

// canonicalChordForMatch normalizes a modifier chord to a stable,
// order-independent form for passthrough matching: aliases are resolved and
// tokens lowercased via config.NormalizeKeyForComparison, then the modifiers
// are re-emitted in the fixed shift+ctrl+alt+cmd order that
// evdevModifierState.prefix() produces, with the base key last. Applying it to
// both the configured blacklist/intercepted entries and the runtime evdev chord
// makes lookups agree regardless of modifier ordering.
func canonicalChordForMatch(chord string) string {
	normalized := config.NormalizeKeyForComparison(strings.TrimSpace(chord))
	if normalized == "" {
		return ""
	}

	parts := strings.Split(normalized, "+")
	base := strings.TrimSpace(parts[len(parts)-1])

	var hasShift, hasCtrl, hasAlt, hasCmd bool

	for _, part := range parts[:len(parts)-1] {
		switch keyvocab.CanonicalModifier(part) {
		case evdevModifierShift:
			hasShift = true
		case evdevModifierCtrl:
			hasCtrl = true
		case evdevModifierAlt:
			hasAlt = true
		case evdevModifierCmd:
			hasCmd = true
		}
	}

	var builder strings.Builder

	for _, mod := range []struct {
		held bool
		name string
	}{
		{hasShift, evdevModifierShift},
		{hasCtrl, evdevModifierCtrl},
		{hasAlt, evdevModifierAlt},
		{hasCmd, evdevModifierCmd},
	} {
		if mod.held {
			builder.WriteString(mod.name)
			builder.WriteByte('+')
		}
	}

	builder.WriteString(base)

	return builder.String()
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
	modifier = keyvocab.CanonicalModifier(modifier)
	if modifier == "" {
		return
	}

	// On X11, synthetic modifier events (from XTest) re-enter the event tap
	// loop and must be suppressed so they don't trigger __modifier_ events.
	// On Wayland, zwp_virtual_keyboard_v1_modifiers does not generate evdev
	// or wl_keyboard events, so the synthetic event never re-enters.
	// Remembering it would falsely suppress a genuine physical modifier
	// press within the suppression window.
	//
	// This must be the same answer postLinuxModifierEvent gives, or the
	// bookkeeping is kept for an injection that never happens; both ask the
	// one detector.
	onWayland := platform.DetectLinuxBackend().IsWayland()
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

// shouldPassthroughChord reports whether an unbound modifier chord should be
// passed through to the focused application instead of consumed by Neru. It
// mirrors the macOS event-tap decision: passthrough must be enabled, the chord
// must carry a Ctrl/Alt/Cmd modifier (shift-only chords stay usable inside
// modes), and it must be neither blacklisted nor in the mode's intercepted set.
func (et *EventTap) shouldPassthroughChord(chord string) bool {
	// Check the cheap enabled flag before the allocating normalization so the
	// common disabled case (the default) costs nothing on the key hot path.
	et.mu.RLock()
	enabled := et.passthroughEnabled
	et.mu.RUnlock()

	if !enabled || !config.HasPassthroughModifier(chord) {
		return false
	}

	canonical := canonicalChordForMatch(chord)

	et.mu.RLock()
	defer et.mu.RUnlock()

	if _, ok := et.passthroughBlacklist[canonical]; ok {
		return false
	}

	if _, ok := et.interceptedChords[canonical]; ok {
		return false
	}

	return true
}

// firePassthroughCallback invokes the registered passthrough callback (if any)
// on its own goroutine and without holding et.mu, so it can neither block the
// key-reader goroutine nor deadlock against the mode handler's own lock. This
// mirrors the async dispatch the macOS tap uses for the same callback.
func (et *EventTap) firePassthroughCallback() {
	et.mu.RLock()
	cb := et.passthroughCallback
	et.mu.RUnlock()

	if cb != nil {
		go cb()
	}
}

// run starts the event interception loop: evdev under a Wayland compositor, a
// keyboard grab on X11.
//
// The choice comes off platform.DetectLinuxBackend, the one detector for the
// compositor family, rather than a read of the environment here — this package
// has always documented it that way and did not do it (#1429). A backend the
// factory refuses to serve runs the X11 loop, which finds no DISPLAY and
// returns.
func (et *EventTap) run() {
	if platform.DetectLinuxBackend().IsWayland() {
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

// stickyToggleEnabled returns whether sticky toggle is active.
func (et *EventTap) stickyToggleEnabled() bool {
	et.mu.RLock()
	defer et.mu.RUnlock()

	return et.stickyModifierToggle
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
