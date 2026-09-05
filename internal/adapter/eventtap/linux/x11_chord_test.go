//go:build linux && cgo

package linux

// The X11 tap has to spell a chord the way the evdev tap does, or a binding
// written once matches on one Linux backend and not the other. These pin the
// spelling, and the reason the base key comes from the keysym rather than from the
// string the server produced: with Ctrl held those two disagree.
//
// Keysym values are pinned as literals for the reason x11_keys_test.go gives — a
// test file cannot use cgo, and untyped constants convert at the call site.

import "testing"

// X11 keysyms from X11/keysym.h, all frozen protocol numbers.
const (
	x11KeysymSpace      = 0x020  // XK_space
	x11KeysymSemicolon  = 0x03b  // XK_semicolon
	x11KeysymColon      = 0x03a  // XK_colon
	x11KeysymUpperG     = 0x047  // XK_G
	x11KeysymLowerC     = 0x063  // XK_c
	x11KeysymLowerG     = 0x067  // XK_g
	x11KeysymReturnKey  = 0xFF0D // XK_Return
	x11KeysymCyrillicA  = 0x6C1  // XK_Cyrillic_a — outside Latin-1
	x11KeysymISOLeftTab = 0xFE20 // XK_ISO_Left_Tab — what Shift+Tab resolves to
)

func TestX11BaseKeyName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		keysym uint32
		want   string
	}{
		{name: "a letter is its character", keysym: x11KeysymLowerG, want: "g"},
		{name: "the shifted level is its own character", keysym: x11KeysymUpperG, want: "G"},
		{name: "punctuation is its character", keysym: x11KeysymSemicolon, want: ";"},
		{name: "so is the shifted punctuation", keysym: x11KeysymColon, want: ":"},
		// XK_space is 0x20, inside the printable range, and must still be named.
		{name: "a named key wins over its character", keysym: x11KeysymSpace, want: "Space"},
		{name: "a named key with no character", keysym: x11KeysymReturnKey, want: "Return"},
		{name: "a keysym outside Latin-1 is unnamed", keysym: x11KeysymCyrillicA, want: ""},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := x11BaseKeyName(testCase.keysym); got != testCase.want {
				t.Errorf("x11BaseKeyName(%#x) = %q, want %q", testCase.keysym, got, testCase.want)
			}
		})
	}
}

func TestX11ChordName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mods   linuxModifierState
		keysym uint32
		want   string
	}{
		{
			name:   "nothing held is the bare key",
			keysym: x11KeysymLowerG,
			want:   "g",
		},
		{
			// The case the string could never answer: XLookupString gives \x03.
			name:   "ctrl names the key, not the control character",
			mods:   linuxModifierState{ctrl: 1},
			keysym: x11KeysymLowerC,
			want:   "Ctrl+c",
		},
		{
			// The prefix carries Shift and NormalizeKey folds the case the level
			// chose, so this is "Shift+g" and not "Shift+G" — the same answer the
			// evdev reader gives, which is what a config "Shift+L" matches.
			name:   "shift is carried in the prefix, with the case folded",
			mods:   linuxModifierState{shift: 1},
			keysym: x11KeysymUpperG,
			want:   "Shift+g",
		},
		{
			// The reported chord: Super is cmd in Neru's vocabulary.
			name:   "super reaches the handler as a cmd chord",
			mods:   linuxModifierState{cmd: 1},
			keysym: x11KeysymSemicolon,
			want:   "Cmd+;",
		},
		{
			name:   "the modifiers keep their canonical order",
			mods:   linuxModifierState{shift: 1, ctrl: 1, alt: 1, cmd: 1},
			keysym: x11KeysymLowerG,
			want:   "Shift+Ctrl+Alt+Cmd+g",
		},
		{
			// The server resolves Shift+Tab to a keysym of its own, and the
			// chord has to be the one a config file writes.
			name:   "shifted tab is Shift+Tab",
			mods:   linuxModifierState{shift: 1},
			keysym: x11KeysymISOLeftTab,
			want:   "Shift+Tab",
		},
		{
			// Unnamed and no string to fall back to: nothing to dispatch.
			name:   "an unnamed keysym with no character is dropped",
			mods:   linuxModifierState{ctrl: 1},
			keysym: x11KeysymCyrillicA,
			want:   "",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			mods := testCase.mods

			got := x11ChordName(&mods, x11BaseKeyName(testCase.keysym))
			if got != testCase.want {
				t.Errorf(
					"chord for %#x = %q, want %q",
					testCase.keysym,
					got,
					testCase.want,
				)
			}
		})
	}
}
