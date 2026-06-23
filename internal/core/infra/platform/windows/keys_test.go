//go:build windows

package windows

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
			mods: []string{"shift"},
			want: "shift+l",
		},
		{
			name: "shift right click binding",
			base: "r",
			mods: []string{"shift"},
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := KeyComboFromBaseAndModifiers(tt.base, tt.mods)
			if got != tt.want {
				t.Fatalf("KeyComboFromBaseAndModifiers() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestKeyNameFromVirtualKeyLetters(t *testing.T) {
	t.Parallel()

	if got := KeyNameFromVirtualKey(0x4C); got != "l" {
		t.Fatalf("KeyNameFromVirtualKey(0x4C) = %q, want l", got)
	}

	if got := ModifierNameFromVirtualKey(vkLShift); got != "shift" {
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
	for _, ch := range []rune{'`', '/', '-', '=', ';', '[', ']', '\''} {
		vk, ok := virtualKeyFromChar(ch)
		if !ok {
			continue
		}

		if got := KeyNameFromVirtualKey(vk); got != string(ch) {
			t.Fatalf("round-trip for %q: KeyNameFromVirtualKey(%#x) = %q, want %q",
				ch, vk, got, string(ch))
		}
	}
}

func TestNameToVirtualKeyPunctuationResolves(t *testing.T) {
	t.Parallel()

	// The literal hotkey strings used by the default config must resolve to a
	// virtual key on the active layout so ParseHotkeyString and the hook agree.
	for _, name := range []string{"`", "/"} {
		vk, ok := nameToVirtualKey(name)
		if !ok || vk == 0 {
			t.Fatalf("nameToVirtualKey(%q) = %#x ok=%v, want a non-zero VK", name, vk, ok)
		}

		if got := KeyNameFromVirtualKey(vk); got != name {
			t.Fatalf("nameToVirtualKey(%q) -> %#x -> KeyNameFromVirtualKey = %q, want %q",
				name, vk, got, name)
		}
	}
}
