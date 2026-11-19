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
// is here for the same reason. macOS has no key code for it, and
// TestParseKeyString_NamedKeysParseToTheirCarbonKeyCode
// (internal/adapter/platform/darwin) checks that.
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

func TestNameToVirtualKeyPunctuationResolves(t *testing.T) {
	t.Parallel()

	// VK<->char for punctuation is keyboard-layout dependent (e.g. "`" is
	// VK_OEM_3 on US but VK_OEM_8 on UK), so the VK is not hardcoded. The VK a
	// name resolves to on the active layout must name that same key back, so
	// ParseHotkeyString and the hook agree. The default config's "`" and "/"
	// must resolve on any layout. The rest are checked where this layout types
	// them unshifted.
	tests := []struct {
		label    string
		name     string
		required bool
	}{
		{label: "backtick", name: "`", required: true},
		{label: "slash", name: "/", required: true},
		{label: "minus", name: "-"},
		{label: "equals", name: "="},
		{label: "semicolon", name: ";"},
		{label: "left bracket", name: "["},
		{label: "right bracket", name: "]"},
		{label: "quote", name: "'"},
	}

	for _, test := range tests {
		t.Run(test.label, func(t *testing.T) {
			t.Parallel()

			virtualKey, ok := nameToVirtualKey(test.name)
			if !ok || virtualKey == 0 {
				if test.required {
					t.Fatalf("nameToVirtualKey(%q) = %#x ok=%v, want a non-zero VK",
						test.name, virtualKey, ok)
				}

				t.Skipf("%q needs shift or is absent on this layout", test.name)
			}

			if got := KeyNameFromVirtualKey(virtualKey); got != test.name {
				t.Fatalf("nameToVirtualKey(%q) -> %#x -> KeyNameFromVirtualKey = %q, want %q",
					test.name, virtualKey, got, test.name)
			}
		})
	}
}
