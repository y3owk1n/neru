//go:build linux && cgo

package linux

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// GlobalHotkeyListener is the Wayland substitute for OS-level global hotkeys,
// which compositors do not expose to ordinary clients. It reads evdev
// passively — no grab, no injection, the focused app still receives the keys —
// and fires callbacks when a configured chord is pressed. While a mode is
// active the in-mode eventtap grabs the same devices, so this goes quiet
// until the mode exits — and the chords it would have fired are resolved by the
// mode handler instead, out of the same global table
// (internal/app/modes/keymap.go, settledKeymaps). That division is what
// keeps one press from running a binding twice: exactly one of the two can see
// any given press.
type GlobalHotkeyListener struct {
	logger *zap.Logger

	mu       sync.Mutex
	bindings map[string]func()
	capture  *waylandEvdevCapture
	stopCh   chan struct{}
	// runDone is closed by the goroutine reading events once it has returned. It
	// is what a stop joins before freeing the capture, because resolving a chord
	// dereferences that capture's xkb state (evdev_xkb_cgo.go) and Close destroys
	// it — the capture's own WaitGroup covers its reader goroutines and has never
	// covered this one. Per generation rather than a WaitGroup on the listener, so
	// a stop waits for the reader it is stopping and not for a later one.
	runDone chan struct{}
	running bool
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

	// Resolve chords through the compositor's keymap, exactly as the in-mode tap
	// does. A chord is written once and has to answer the same physical key
	// whether or not a mode is open, and only one of the two readers can see any
	// given press — so if they name keys differently, the same binding follows
	// the layout on one side of a mode boundary and the bare scan code on the
	// other (evdev_xkb_cgo.go).
	capture.refreshXkbState()

	// Intentionally no grabAll(): a passive read leaves keys flowing to the
	// focused application, which is what a global hotkey should do.
	capture.startReaders()

	stopCh := make(chan struct{})
	runDone := make(chan struct{})

	l.capture = capture
	l.stopCh = stopCh
	l.runDone = runDone
	l.running = true

	go func() {
		defer close(runDone)

		l.run(capture, stopCh)
	}()

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
	runDone := l.runDone
	l.capture = nil
	l.runDone = nil
	l.running = false
	l.mu.Unlock()

	// Joined outside the lock: the reader takes l.mu to look a chord up, so
	// waiting for it under the lock would wait on something that needs the lock.
	waitForListenerReader(runDone)

	if capture != nil {
		capture.Close()
	}
}

// waitForListenerReader blocks until the event reader of a stopped generation has
// returned, so nothing is still resolving keys against a capture about to be
// freed. A generation that never started one has nothing to wait for.
func waitForListenerReader(runDone chan struct{}) {
	if runDone == nil {
		return
	}

	<-runDone
}

// StopWithTimeout halts watching with a deadline. Returns true if the stop
// completed cleanly, false if the timeout expired and the old capture was
// abandoned. On timeout the stale reader goroutines are leaked but will
// eventually exit when the kernel finalizes the file descriptors.
func (l *GlobalHotkeyListener) StopWithTimeout(timeout time.Duration) bool {
	l.mu.Lock()
	if !l.running {
		l.mu.Unlock()

		return true
	}

	close(l.stopCh)
	capture := l.capture
	runDone := l.runDone
	l.capture = nil
	l.runDone = nil
	l.running = false
	l.mu.Unlock()

	if capture == nil {
		waitForListenerReader(runDone)

		return true
	}

	done := make(chan struct{})

	go func() {
		// Inside the timed goroutine, so the deadline bounds the join as well as
		// the close — a reader that will not return must not hang the caller.
		waitForListenerReader(runDone)
		capture.Close()
		close(done)
	}()

	select {
	case <-done:
		return true
	case <-time.After(timeout):
		l.logger.Warn(
			"Evdev capture close timed out; abandoning stale readers",
			zap.Duration("timeout", timeout),
		)

		return false
	}
}

// IsRunning reports whether the listener is actively watching for chords.
func (l *GlobalHotkeyListener) IsRunning() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.running
}

// DeviceCount returns the number of captured keyboard devices.
func (l *GlobalHotkeyListener) DeviceCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.capture == nil {
		return 0
	}

	l.capture.deviceMu.Lock()
	defer l.capture.deviceMu.Unlock()

	return len(l.capture.files)
}

func (l *GlobalHotkeyListener) run(capture *waylandEvdevCapture, stopCh chan struct{}) {
	state := newListenerKeyState()

	for {
		select {
		case <-stopCh:
			return
		case event, ok := <-capture.events:
			if !ok {
				if !l.tryRestartLocked(&capture, &stopCh, &state) {
					return
				}

				continue
			}

			l.handleEvent(capture, &state, event)
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

	// A fresh capture needs the keymap asked again, for the reason Start gives.
	newCapture.refreshXkbState()
	newCapture.startReaders()

	oldCapture := *capture

	*capture = newCapture
	*stopCh = make(chan struct{})
	*state = newListenerKeyState()
	l.capture = newCapture
	l.stopCh = *stopCh
	l.mu.Unlock()

	// The old capture is freed on a goroutine of its own, and this reader has
	// already stopped referencing it: it is the caller, so no chord is being
	// resolved against it right now, and it reads the new one from here on.

	if oldCapture != nil {
		go oldCapture.Close()
	}

	l.logger.Info(
		"Evdev hotkey listener reconnected after reader failure",
		zap.Int("devices", len(newCapture.files)),
	)

	return true
}

// resyncModifiersFromKernel replaces this state's idea of which modifiers are
// held with the kernel's, for when its own bookkeeping has provably missed
// events. It reports how many the kernel says are down.
//
// It is the same question newEvdevSessionState asks at grab time, for the same
// reason: a reader that was not being fed cannot know what happened while it was
// not, and EVIOCGKEY does. The modifier keys are re-read from the devices and the
// rest of the tracked keys left alone — a non-modifier key contributes to no
// count, so nothing about it can be stale in a way that matters.
//
// With no capture there is nothing to ask, and the state is left as it stands:
// keeping a count that may be wrong beats decrementing it into the range where
// prefix() can never report the modifier held again.
//
// Both directions of staleness reach here, and by two different routes. A release
// whose press was swallowed announces itself as an unmatched release. A press
// whose *release* was swallowed announces nothing at all — it just leaves a
// modifier counted as held, where a later bare key reads as a chord — so that one
// is caught by the grab generation instead (needsReconcileAfterGrab), which fires
// this at the first event after a mode is gone whether or not anything looked
// wrong.
func (state *waylandEvdevKeyState) resyncModifiersFromKernel(
	capture *waylandEvdevCapture,
) int {
	if capture == nil || state == nil {
		return 0
	}

	held := make(map[uint16]bool)
	state.modifiers.linuxModifierState = queryEvdevModifierState(capture, held)

	for code := range state.pressed {
		if capture.modifierName(code) != "" {
			delete(state.pressed, code)
		}
	}

	for code := range held {
		state.trackKey(code, true)
	}

	return len(held)
}

// resyncModifiers rebuilds the held-modifier picture and says so, for a reader
// that has provably missed events.
func (l *GlobalHotkeyListener) resyncModifiers(
	capture *waylandEvdevCapture,
	state *waylandEvdevKeyState,
	reason string,
) {
	held := state.resyncModifiersFromKernel(capture)

	l.logger.Debug(
		"Re-read the held modifiers from the kernel",
		zap.String("reason", reason),
		zap.Int("held", held),
	)
}

// newListenerKeyState is the listener's starting picture of the keyboard: nothing
// held, and already level with the current grab generation, since a state that
// believes nothing has nothing to reconcile.
func newListenerKeyState() waylandEvdevKeyState {
	return waylandEvdevKeyState{
		pressed:        make(map[uint16]bool),
		grabGeneration: waylandEvdevGrabGeneration.Load(),
	}
}

func (l *GlobalHotkeyListener) handleEvent(
	capture *waylandEvdevCapture,
	state *waylandEvdevKeyState,
	event waylandEvdevEvent,
) {
	if event.eventType != evdevEventKey {
		return
	}

	// A mode has held the devices since the last event this reader saw, so what it
	// believes is held describes a keyboard it was not watching. Ask the kernel
	// before doing arithmetic on top of it.
	if state.needsReconcileAfterGrab(waylandEvdevGrabGeneration.Load()) {
		l.resyncModifiers(capture, state, "a mode held the devices since the last event")
	}

	// Keep xkb's idea of the lock modifiers and the layout group current, so a
	// name resolved below is resolved under the layout in force now.
	switch event.value {
	case evdevValuePress:
		capture.feedKey(event.code, true)
	case evdevValueRelease:
		capture.feedKey(event.code, false)
	}

	if modifier := capture.modifierName(event.code); modifier != "" {
		if event.value == evdevValueRepeat {
			return
		}

		isDown := event.value == evdevValuePress

		if state.trackModifier(event.code, modifier, isDown) == modifierReleaseUnmatched {
			// A release with no press behind it. This listener reads nothing
			// while a mode holds the evdev grab, so it can watch a whole chord
			// go by unseen and then be handed the release when the mode exits
			// under the user's still-held modifier — which is what every
			// toggle-off through the in-mode fallback looks like from here. The
			// count is then not a count of anything, and decrementing it is what
			// breaks the next chord for good: prefix() reads a modifier as held
			// only above zero, so one unanswered release leaves every later
			// press one short and the hotkey silently stops matching. Ask the
			// kernel what is actually held instead.
			l.resyncModifiers(capture, state, "a release with no press behind it")
		}

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

	key := capture.keyName(event.code)
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
