//go:build linux && cgo

package linux

import (
	"strconv"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/keyvocab"
)

func TestEvdevModifierName(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		code uint16
		want string
	}{
		{code: evdevKeyLeftShift, want: evdevModifierShift},
		{code: evdevKeyRightCtrl, want: evdevModifierCtrl},
		{code: evdevKeyLeftAlt, want: evdevModifierAlt},
		{code: evdevKeyRightMeta, want: evdevModifierCmd},
		{code: evdevKeyA, want: ""},
	}

	for _, testCase := range testCases {
		if got := evdevModifierName(testCase.code); got != testCase.want {
			t.Fatalf("evdevModifierName(%d) = %q, want %q", testCase.code, got, testCase.want)
		}
	}
}

func TestEvdevKeyName(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		code uint16
		want string
	}{
		{code: evdevKeyA, want: "a"},
		{code: evdevKeySlash, want: "/"},
		{code: evdevKeyEnter, want: evdevKeyNameReturn},
		{code: evdevKeyBackspace, want: "Backspace"},
		{code: evdevKeyLeft, want: evdevKeyNameLeft},
		{code: evdevKeyF1, want: "F1"},
		{code: evdevKeyF12, want: "F12"},
		{code: evdevKeyF13, want: "F13"},
		{code: evdevKeyF20, want: "F20"},
		{code: evdevKeyF21, want: "F21"},
		{code: evdevKeyF24, want: "F24"},
	}

	for _, testCase := range testCases {
		if got := evdevKeyName(testCase.code); got != testCase.want {
			t.Fatalf("evdevKeyName(%d) = %q, want %q", testCase.code, got, testCase.want)
		}
	}
}

// TestEvdevKeyNameFunctionKeysContiguous pins the F13-F24 evdev codes, which
// are not adjacent to the F1-F12 block (KEY_F13 is 183, not 89).
func TestEvdevKeyNameFunctionKeysContiguous(t *testing.T) {
	t.Parallel()

	const firstHighFunctionKeyCode = 183

	for index := 13; index <= 24; index++ {
		code := uint16(firstHighFunctionKeyCode + index - 13)
		want := "F" + strconv.Itoa(index)

		if got := evdevKeyName(code); got != want {
			t.Errorf("evdevKeyName(%d) = %q, want %q", code, got, want)
		}
	}
}

// keypadKeys lists every keypad key with the kernel code the evdev fallback
// reads (linux/input-event-codes.h) and the keysym the key reports with NumLock
// off. X11 and XKB use the same keysym numbers.
var keypadKeys = []struct {
	name   string
	code   uint16
	keysym uint32
}{
	{name: "KEY_KPASTERISK", code: 55, keysym: xkbKeysymKPMultiply},
	{name: "KEY_KPMINUS", code: 74, keysym: xkbKeysymKPSubtract},
	{name: "KEY_KPPLUS", code: 78, keysym: xkbKeysymKPAdd},
	{name: "KEY_KPENTER", code: 96, keysym: xkbKeysymKPEnter},
	{name: "KEY_KPSLASH", code: 98, keysym: xkbKeysymKPDivide},
	{name: "KEY_KP7", code: 71, keysym: x11KeysymKPHome},
	{name: "KEY_KP8", code: 72, keysym: x11KeysymKPUp},
	{name: "KEY_KP9", code: 73, keysym: x11KeysymKPPageUp},
	{name: "KEY_KP4", code: 75, keysym: x11KeysymKPLeft},
	{name: "KEY_KP5", code: 76, keysym: x11KeysymKPBegin},
	{name: "KEY_KP6", code: 77, keysym: x11KeysymKPRight},
	{name: "KEY_KP1", code: 79, keysym: x11KeysymKPEnd},
	{name: "KEY_KP2", code: 80, keysym: x11KeysymKPDown},
	{name: "KEY_KP3", code: 81, keysym: x11KeysymKPPageDown},
	{name: "KEY_KP0", code: 82, keysym: x11KeysymKPInsert},
	{name: "KEY_KPDOT", code: 83, keysym: x11KeysymKPDelete},
}

// TestEvdevKeyName_KeypadMatchesWayland checks the evdev fallback against the
// Wayland keymap for every keypad key. The evdev tap uses the fallback when xkb
// state creation fails, and a name that differs sends one physical key to a
// different binding.
func TestEvdevKeyName_KeypadMatchesWayland(t *testing.T) {
	t.Parallel()

	for _, testCase := range keypadKeys {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			want := xkbKeysymName(testCase.keysym)
			if want == "" {
				t.Fatalf("xkbKeysymName(%#x) names nothing", testCase.keysym)
			}

			if got := evdevKeyName(testCase.code); got != want {
				t.Fatalf("evdevKeyName(%d) = %q, want the Wayland name %q",
					testCase.code, got, want)
			}
		})
	}
}

// TestEvdevKeyName_KeypadNamesAreVocabularySpellings checks the other half of
// naming the keypad. The dispatch path runs every name through
// keyvocab.NormalizeKey, and a name that is neither a named key nor a single
// character survives it unchanged and then matches no binding. That is the
// silent failure ADR 0008 was written about, so it is pinned rather than
// assumed. It says nothing about whether a chord can be *written* with the
// name, since "+" is a single character and also the chord separator.
func TestEvdevKeyName_KeypadNamesAreVocabularySpellings(t *testing.T) {
	t.Parallel()

	for _, testCase := range keypadKeys {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			name := evdevKeyName(testCase.code)
			if got := keyvocab.NormalizeKey(name); got != name {
				t.Fatalf("NormalizeKey(%q) = %q, want it unchanged", name, got)
			}

			if !keyvocab.IsNamedKey(name) && len([]rune(name)) != 1 {
				t.Fatalf(
					"evdevKeyName(%d) = %q, which is neither a named key nor a single character",
					testCase.code,
					name,
				)
			}
		})
	}
}

func TestEvdevModifierStatePrefix(t *testing.T) {
	t.Parallel()

	state := evdevModifierState{}
	state.update("ctrl", true)
	state.update("shift", true)
	state.update("cmd", true)

	if got := state.prefix(); got != "Shift+Ctrl+Cmd+" {
		t.Fatalf("prefix() = %q, want %q", got, "Shift+Ctrl+Cmd+")
	}

	state.update("ctrl", false)

	if got := state.prefix(); got != "Shift+Cmd+" {
		t.Fatalf("prefix() after ctrl release = %q, want %q", got, "Shift+Cmd+")
	}
}
