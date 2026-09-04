package tap

import "testing"

const (
	chordCtrlC     = "Ctrl+c"
	chordShiftCtrl = "shift+ctrl+t"
)

func TestCanonicalChord_OrderIndependent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "single ctrl lowercased", in: chordCtrlC, want: "ctrl+c"},
		{name: "cmd cased", in: "Cmd+C", want: "cmd+c"},
		{name: "ctrl+shift order a", in: "ctrl+shift+t", want: chordShiftCtrl},
		{name: "ctrl+shift order b", in: chordShiftCtrl, want: chordShiftCtrl},
		{name: "all mods reordered", in: "cmd+alt+ctrl+shift+k", want: "shift+ctrl+alt+cmd+k"},
		{name: "control alias", in: "control+a", want: "ctrl+a"},
		{name: "plain key", in: "c", want: "c"},
		{name: "empty", in: "", want: ""},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := CanonicalChord(testCase.in); got != testCase.want {
				t.Fatalf(
					"CanonicalChord(%q) = %q, want %q",
					testCase.in,
					got,
					testCase.want,
				)
			}
		})
	}
}
