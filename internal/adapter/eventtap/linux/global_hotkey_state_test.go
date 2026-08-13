//go:build linux && cgo

package linux

// The passive listener reads nothing while a mode holds the evdev grab, so its
// idea of which modifiers are down can only be as good as the events it was
// allowed to see. These are the two ways a grab window leaves it wrong, and the
// property that has to survive both: a chord that matched before the mode still
// matches after it.

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

// hotkeyStateChord is the binding these tests press, spelled as a user would.
const hotkeyStateChord = "Super+;"

// listenerJoinProof is how long a stopped reader stays busy in the join tests. It
// is what makes them prove the join rather than win a race with the scheduler: a
// stop that does not wait returns in microseconds, well inside this.
const listenerJoinProof = 80 * time.Millisecond

// keyEvent is one evdev key event, the shape the reader hands handleEvent.
func keyEvent(code uint16, value int32) waylandEvdevEvent {
	return waylandEvdevEvent{eventType: evdevEventKey, code: code, value: value}
}

// pressChord feeds the modifier and the base key of the test chord.
func pressChord(l *GlobalHotkeyListener, state *waylandEvdevKeyState) {
	l.handleEvent(nil, state, keyEvent(evdevKeyLeftMeta, evdevValuePress))
	l.handleEvent(nil, state, keyEvent(evdevKeySemicolon, evdevValuePress))
}

// newBoundListener returns a listener with the test chord bound, and the channel
// its callback reports on.
func newBoundListener(t *testing.T) (*GlobalHotkeyListener, chan struct{}) {
	t.Helper()

	listener := NewGlobalHotkeyListener(nil)
	fired := make(chan struct{}, 2)

	listener.SetBinding(hotkeyStateChord, func() { fired <- struct{}{} })

	return listener, fired
}

// waitFired reports whether the binding ran within a generous window. The
// callback is dispatched on a goroutine, so a miss has to be waited for.
func waitFired(t *testing.T, fired chan struct{}) bool {
	t.Helper()

	select {
	case <-fired:
		return true
	case <-time.After(2 * time.Second):
		return false
	}
}

// A release whose press the listener never saw must not be counted. It is what
// every toggle-off through the in-mode fallback leaves behind — the chord is
// pressed under the grab and released after the mode exits — and decrementing
// for it drove the count below zero, where prefix() can never report the
// modifier held again and the hotkey stopped matching for the rest of the
// session.
func TestGlobalHotkeyListener_UnmatchedReleaseKeepsTheChordMatching(t *testing.T) {
	t.Parallel()

	listener, fired := newBoundListener(t)
	state := waylandEvdevKeyState{pressed: make(map[uint16]bool)}

	// The mode had the grab: the press went there, only the release is seen.
	listener.handleEvent(nil, &state, keyEvent(evdevKeyLeftMeta, evdevValueRelease))

	if state.modifiers.cmd < 0 {
		t.Fatalf("cmd count is %d after an unmatched release, want it never below zero",
			state.modifiers.cmd)
	}

	// The next press of the same chord, with nothing grabbing any more.
	pressChord(listener, &state)

	if !waitFired(t, fired) {
		t.Fatal("the chord did not match after an unmatched release; the hotkey is dead")
	}
}

// The mirror case: a press seen, then its release swallowed by a grab that
// started underneath it. The modifier is physically up but still counted, so the
// next bare key would be read as the chord. Pressing the modifier again must not
// count a second time, or the release that follows leaves it held forever.
func TestGlobalHotkeyListener_RepeatedPressCountsOnce(t *testing.T) {
	t.Parallel()

	listener, _ := newBoundListener(t)
	state := waylandEvdevKeyState{pressed: make(map[uint16]bool)}

	listener.handleEvent(nil, &state, keyEvent(evdevKeyLeftMeta, evdevValuePress))
	listener.handleEvent(nil, &state, keyEvent(evdevKeyLeftMeta, evdevValuePress))
	listener.handleEvent(nil, &state, keyEvent(evdevKeyLeftMeta, evdevValueRelease))

	if state.modifiers.cmd != 0 {
		t.Errorf("cmd count is %d after press, release, want", state.modifiers.cmd)
	}
}

// The ordinary case, so the guards above cannot pass by silencing the listener:
// a chord pressed and released leaves nothing behind and matches every time.
func TestGlobalHotkeyListener_MatchesTheChordEveryTime(t *testing.T) {
	t.Parallel()

	listener, fired := newBoundListener(t)
	state := waylandEvdevKeyState{pressed: make(map[uint16]bool)}

	for attempt := 1; attempt <= 2; attempt++ {
		pressChord(listener, &state)

		listener.handleEvent(nil, &state, keyEvent(evdevKeySemicolon, evdevValueRelease))
		listener.handleEvent(nil, &state, keyEvent(evdevKeyLeftMeta, evdevValueRelease))

		if !waitFired(t, fired) {
			t.Fatalf("the chord did not match on attempt %d", attempt)
		}

		if state.modifiers.cmd != 0 {
			t.Fatalf("cmd count is %d after attempt %d, want 0", state.modifiers.cmd, attempt)
		}
	}
}

// Stopping the listener frees the capture, and resolving a chord dereferences
// that capture's xkb state — C memory the capture destroys on close. So a stop
// must not return until the goroutine reading events has, or the two race for a
// pointer one of them is freeing. The capture's own WaitGroup does not cover that
// goroutine and never has.
func TestGlobalHotkeyListener_StopJoinsTheReaderBeforeFreeingTheCapture(t *testing.T) {
	t.Parallel()

	listener := NewGlobalHotkeyListener(nil)

	// A capture with no devices: Close still runs its whole teardown, xkb state
	// included, which is the part a live reader must not be inside.
	capture := &waylandEvdevCapture{
		events: make(chan waylandEvdevEvent),
		logger: zap.NewNop(),
	}

	stopCh := make(chan struct{})
	runDone := make(chan struct{})
	reading := make(chan struct{})

	listener.mu.Lock()
	listener.capture = capture
	listener.stopCh = stopCh
	listener.runDone = runDone
	listener.running = true
	listener.mu.Unlock()

	go func() {
		close(reading)
		listener.run(capture, stopCh)

		// Stands for the work a reader can still be inside when the stop signal
		// lands: it is already past the select, resolving a chord against the
		// capture's xkb state, which the close about to run would free.
		time.Sleep(listenerJoinProof)
		close(runDone)
	}()

	<-reading

	started := time.Now()

	listener.Stop()

	if waited := time.Since(started); waited < listenerJoinProof {
		t.Fatalf(
			"Stop returned after %v, before the reader finished: the capture is freed "+
				"while a chord is still being resolved against its xkb state",
			waited,
		)
	}

	if listener.IsRunning() {
		t.Error("the listener still reports itself running after Stop")
	}
}

// The same for the bounded stop, which has to keep its deadline over the join as
// well as the close.
func TestGlobalHotkeyListener_StopWithTimeoutJoinsTheReader(t *testing.T) {
	t.Parallel()

	listener := NewGlobalHotkeyListener(nil)

	capture := &waylandEvdevCapture{
		events: make(chan waylandEvdevEvent),
		logger: zap.NewNop(),
	}

	stopCh := make(chan struct{})
	runDone := make(chan struct{})

	listener.mu.Lock()
	listener.capture = capture
	listener.stopCh = stopCh
	listener.runDone = runDone
	listener.running = true
	listener.mu.Unlock()

	go func() {
		listener.run(capture, stopCh)

		time.Sleep(listenerJoinProof)
		close(runDone)
	}()

	started := time.Now()

	if !listener.StopWithTimeout(2 * time.Second) {
		t.Fatal("the bounded stop timed out on a capture with no devices")
	}

	if waited := time.Since(started); waited < listenerJoinProof {
		t.Fatalf("StopWithTimeout returned after %v, before the reader finished", waited)
	}
}
