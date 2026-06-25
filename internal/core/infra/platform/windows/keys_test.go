//go:build windows

package windows //nolint:testpackage // exercises unexported key translation helpers directly

import "testing"

func TestKeyComboFromBaseAndModifiers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		base string
		mods []string
		want string
	}{
		{
			name: "shift left click binding",
			base: "l",
			mods: []string{modNameShift},
			want: "shift+l",
		},
		{
			name: "shift right click binding",
			base: "r",
			mods: []string{modNameShift},
			want: "shift+r",
		},
		{
			name: "ctrl shift grid activation",
			base: "g",
			mods: []string{"ctrl", "shift"},
			want: "ctrl+shift+g",
		},
		{
			name: "plain key",
			base: "a",
			mods: nil,
			want: "a",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := KeyComboFromBaseAndModifiers(testCase.base, testCase.mods)
			if got != testCase.want {
				t.Fatalf("KeyComboFromBaseAndModifiers() = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestKeyNameFromVirtualKeyLetters(t *testing.T) {
	t.Parallel()

	if got := KeyNameFromVirtualKey(0x4C); got != "l" {
		t.Fatalf("KeyNameFromVirtualKey(0x4C) = %q, want l", got)
	}

	if got := ModifierNameFromVirtualKey(vkLShift); got != modNameShift {
		t.Fatalf("ModifierNameFromVirtualKey(vkLShift) = %q, want shift", got)
	}
}

func TestOEMPunctuationRoundTripLayoutAware(t *testing.T) {
	t.Parallel()

	// VK<->char for punctuation is keyboard-layout dependent (e.g. "`" is
	// VK_OEM_3 on US but VK_OEM_8 on UK). Rather than hardcode VK codes, verify
	// the layout-aware translation round-trips: the VK that produces a char must
	// map back to that same char. This keeps hotkeys like "`" and "/" working on
	// any layout. Chars that need shift on this layout are skipped.
	for _, keyChar := range []rune{'`', '/', '-', '=', ';', '[', ']', '\''} {
		virtualKey, ok := virtualKeyFromChar(keyChar)
		if !ok {
			continue
		}

		if got := KeyNameFromVirtualKey(virtualKey); got != string(keyChar) {
			t.Fatalf("round-trip for %q: KeyNameFromVirtualKey(%#x) = %q, want %q",
				keyChar, virtualKey, got, string(keyChar))
		}
	}
}

func TestNameToVirtualKeyPunctuationResolves(t *testing.T) {
	t.Parallel()

	// The literal hotkey strings used by the default config must resolve to a
	// virtual key on the active layout so ParseHotkeyString and the hook agree.
	for _, name := range []string{"`", "/"} {
		virtualKey, ok := nameToVirtualKey(name)
		if !ok || virtualKey == 0 {
			t.Fatalf("nameToVirtualKey(%q) = %#x ok=%v, want a non-zero VK", name, virtualKey, ok)
		}

		if got := KeyNameFromVirtualKey(virtualKey); got != name {
			t.Fatalf("nameToVirtualKey(%q) -> %#x -> KeyNameFromVirtualKey = %q, want %q",
				name, virtualKey, got, name)
		}
	}
}
