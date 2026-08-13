//go:build linux && cgo

package linux

// Both readers of the devices track held modifiers through this one helper and
// part company only on the third answer: the tap expects an unmatched release
// (its grab has just started under a held key) and drops it, the listener treats
// it as proof it missed events and rebuilds from the kernel. What must never
// happen either way is a count going negative, because prefix() reads a modifier
// as held only above zero and a chord then stops matching for good.

import "testing"

func TestWaylandEvdevKeyState_TrackModifier(t *testing.T) {
	t.Parallel()

	const code = evdevKeyLeftMeta

	tests := []struct {
		name  string
		feed  []bool // press = true
		want  modifierTransition
		count int
	}{
		{
			name:  "press then release nets out",
			feed:  []bool{true, false},
			want:  modifierDropped,
			count: 0,
		},
		{
			name:  "press alone holds it",
			feed:  []bool{true},
			want:  modifierHeld,
			count: 1,
		},
		{
			name:  "a repeated press counts once",
			feed:  []bool{true, true},
			want:  modifierHeld,
			count: 1,
		},
		{
			name:  "a repeated press still nets out on one release",
			feed:  []bool{true, true, false},
			want:  modifierDropped,
			count: 0,
		},
		{
			name:  "a release with no press is reported, not counted",
			feed:  []bool{false},
			want:  modifierReleaseUnmatched,
			count: 0,
		},
		{
			name:  "and does not leave the count below zero",
			feed:  []bool{false, false, true},
			want:  modifierHeld,
			count: 1,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			state := &waylandEvdevKeyState{pressed: make(map[uint16]bool)}

			var got modifierTransition
			for _, isDown := range testCase.feed {
				got = state.trackModifier(code, evdevModifierCmd, isDown)
			}

			if got != testCase.want {
				t.Errorf("last transition = %d, want %d", got, testCase.want)
			}

			if state.modifiers.cmd != testCase.count {
				t.Errorf("cmd count = %d, want %d", state.modifiers.cmd, testCase.count)
			}

			if state.modifiers.cmd < 0 {
				t.Error("cmd count went below zero; prefix() can never report it held again")
			}
		})
	}
}

// With no keymap to ask, both readers fall back to the same scan-code table
// rather than answering nothing — the names a us layout would give.
func TestWaylandEvdevCapture_NamesFallBackToTheScanCodeTable(t *testing.T) {
	t.Parallel()

	var capture *waylandEvdevCapture

	if got := capture.keyName(evdevKeySemicolon); got != ";" {
		t.Errorf("keyName(semicolon) = %q, want %q", got, ";")
	}

	if got := capture.modifierName(evdevKeyLeftMeta); got != evdevModifierCmd {
		t.Errorf("modifierName(left meta) = %q, want %q", got, evdevModifierCmd)
	}

	if got := capture.modifierName(evdevKeySemicolon); got != "" {
		t.Errorf("modifierName(semicolon) = %q, want no modifier", got)
	}

	// Feeding a capture that has no keymap is a no-op rather than a crash: the
	// listener feeds every key it reads and may never have got a keymap.
	capture.feedKey(evdevKeySemicolon, true)
}

// A grab hands the passive listener no events, so it cannot see a mode come and
// go. The generation is how it finds out afterwards, and it has to fire once per
// window rather than once per event — the rebuild behind it reads the devices.
func TestWaylandEvdevKeyState_NeedsReconcileAfterGrab(t *testing.T) {
	t.Parallel()

	state := &waylandEvdevKeyState{pressed: make(map[uint16]bool)}

	if state.needsReconcileAfterGrab(0) {
		t.Error("a state level with the generation asked to reconcile")
	}

	if !state.needsReconcileAfterGrab(1) {
		t.Error("a state behind the generation did not ask to reconcile")
	}

	if state.needsReconcileAfterGrab(1) {
		t.Error("reconciled twice for one grab window")
	}

	if !state.needsReconcileAfterGrab(2) {
		t.Error("a second grab window did not ask to reconcile")
	}
}

// The listener's starting picture believes nothing and is already level with the
// generation, so its first event does not spend a device read on nothing.
func TestNewListenerKeyState_StartsLevelWithTheGeneration(t *testing.T) {
	state := newListenerKeyState()

	if state.needsReconcileAfterGrab(waylandEvdevGrabGeneration.Load()) {
		t.Error("a fresh listener state asked to reconcile before seeing any grab")
	}

	if state.pressed == nil {
		t.Error("a fresh listener state has no pressed map")
	}
}

// The flag and the counter are one statement: a grab that ended must never leave
// the flag set or the counter behind, or the overlay reclaims the keyboard while
// the listener still trusts a stale picture.
func TestEndEvdevGrabGeneration_ClearsTheFlagAndCountsTheGrabOut(t *testing.T) {
	before := waylandEvdevGrabGeneration.Load()

	waylandEvdevKeyboardActive.Store(true)
	endEvdevGrabGeneration()

	if IsWaylandEvdevKeyboardActive() {
		t.Error("the grab flag is still set after the grab ended")
	}

	if got := waylandEvdevGrabGeneration.Load(); got != before+1 {
		t.Errorf("generation = %d, want %d", got, before+1)
	}
}
