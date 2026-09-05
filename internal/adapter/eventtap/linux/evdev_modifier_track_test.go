//go:build linux && cgo

package linux

// Both consumers of the proxy track held modifiers through this one helper. What
// must never happen is a count going negative, because prefix() reads a modifier
// as held only above zero and a chord then stops matching for good; a release
// with no press behind it is reported, not counted.

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

// With no keymap to ask, the proxy falls back to the scan-code table rather than
// answering nothing — the names a us layout would give.
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
	// proxy feeds every key it reads and may never have got a keymap.
	capture.feedKey(evdevKeySemicolon, true)
}
