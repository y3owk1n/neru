//go:build windows

package windows

import (
	"strconv"
	"strings"
	"testing"
)

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

func TestFunctionKeyRoundTrip(t *testing.T) {
	t.Parallel()

	// F1-F24 are contiguous from VK_F1, so the whole range must round-trip
	// between the hook's name lookup and hotkey parsing. F21-F24 in particular
	// exist only on Windows and Linux (macOS has no virtual keycode for them).
	for index := 1; index <= functionKeyCount; index++ {
		want := "F" + strconv.Itoa(index)
		wantVK := uint32(vkF1 + index - 1)

		if got := KeyNameFromVirtualKey(wantVK); got != want {
			t.Fatalf("KeyNameFromVirtualKey(%#x) = %q, want %q", wantVK, got, want)
		}

		// Config files and the CLI both accept lowercase spellings.
		for _, spelling := range []string{want, strings.ToLower(want)} {
			virtualKey, resolved := nameToVirtualKey(spelling)
			if !resolved || virtualKey != wantVK {
				t.Fatalf("nameToVirtualKey(%q) = %#x ok=%v, want %#x",
					spelling, virtualKey, resolved, wantVK)
			}
		}
	}

	// Function keys are not modifiers and must not be reported as such.
	if got := ModifierNameFromVirtualKey(vkF1); got != "" {
		t.Fatalf("ModifierNameFromVirtualKey(vkF1) = %q, want empty", got)
	}
}

// TestNavigationKeyRoundTrip pins the navigation keys on Windows. They are
// documented as valid on every platform and the other backends emit them, so
// the hook has to name them and a global hotkey has to register them rather
// than fail with errUnsupportedHotkeyKey. MapVirtualKey yields no character for
// these codes, so the explicit table is the only path that names them. Insert
// is here for the same reason; macOS carries no entry for it, which
// internal/architecture/named_key_tables_test.go states and pins.
func TestNavigationKeyRoundTrip(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		virtualKey uint32
		want       string
	}{
		{name: "page up", virtualKey: vkPrior, want: "PageUp"},
		{name: "page down", virtualKey: vkNext, want: "PageDown"},
		{name: "home", virtualKey: vkHome, want: "Home"},
		{name: "end", virtualKey: vkEnd, want: "End"},
		{name: "insert", virtualKey: vkInsert, want: "Insert"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := KeyNameFromVirtualKey(testCase.virtualKey); got != testCase.want {
				t.Fatalf("KeyNameFromVirtualKey(%#x) = %q, want %q",
					testCase.virtualKey, got, testCase.want)
			}

			// Config files and the CLI both accept lowercase spellings.
			for _, spelling := range []string{testCase.want, strings.ToLower(testCase.want)} {
				mods, virtualKey, err := ParseHotkeyString(spelling)
				if err != nil {
					t.Fatalf("ParseHotkeyString(%q) = %v, want no error", spelling, err)
				}

				if mods != 0 || virtualKey != testCase.virtualKey {
					t.Fatalf("ParseHotkeyString(%q) = mods %#x vk %#x, want mods 0 vk %#x",
						spelling, mods, virtualKey, testCase.virtualKey)
				}
			}
		})
	}
}

func TestFunctionKeyOutOfRange(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"f0", "f25", "f", "f1x", "fa"} {
		if virtualKey, resolved := functionKeyVirtualKey(name); resolved {
			t.Errorf("functionKeyVirtualKey(%q) = %#x ok=true, want false", name, virtualKey)
		}
	}

	if got := functionKeyName(vkF24 + 1); got != "" {
		t.Errorf("functionKeyName(%#x) = %q, want empty", vkF24+1, got)
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
