//go:build linux && cgo

//nolint:testpackage // These tests validate unexported evdev passthrough helpers directly.
package eventtap

import (
	"slices"
	"testing"
)

func TestHeldPassthroughModifiers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(s *waylandEvdevKeyState)
		want  []string
	}{
		{
			name:  "none held",
			setup: func(_ *waylandEvdevKeyState) {},
			want:  []string{},
		},
		{
			name:  "ctrl only",
			setup: func(s *waylandEvdevKeyState) { s.modifiers.ctrl = 1 },
			want:  []string{evdevModifierCtrl},
		},
		{
			name: "shift and cmd emitted in fixed order",
			setup: func(s *waylandEvdevKeyState) {
				s.modifiers.cmd = 1
				s.modifiers.shift = 1
			},
			want: []string{evdevModifierShift, evdevModifierCmd},
		},
		{
			name: "all four in shift ctrl alt cmd order",
			setup: func(s *waylandEvdevKeyState) {
				s.modifiers.alt = 1
				s.modifiers.cmd = 1
				s.modifiers.ctrl = 1
				s.modifiers.shift = 1
			},
			want: []string{
				evdevModifierShift,
				evdevModifierCtrl,
				evdevModifierAlt,
				evdevModifierCmd,
			},
		},
		{
			name:  "refcount above one still counts once",
			setup: func(s *waylandEvdevKeyState) { s.modifiers.ctrl = 3 },
			want:  []string{evdevModifierCtrl},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			state := &waylandEvdevKeyState{}
			testCase.setup(state)

			if got := heldPassthroughModifiers(state); !slices.Equal(got, testCase.want) {
				t.Fatalf("heldPassthroughModifiers() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestHeldPassthroughModifiersNilState(t *testing.T) {
	t.Parallel()

	if got := heldPassthroughModifiers(nil); got != nil {
		t.Fatalf("heldPassthroughModifiers(nil) = %v, want nil", got)
	}
}
