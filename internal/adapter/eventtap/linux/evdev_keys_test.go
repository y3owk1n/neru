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

// keypadKeyNameCases is the keypad half of the fallback table, written out.
// The codes are literals from linux/input-event-codes.h — frozen kernel ABI
// numbers, pinned here rather than read back from the declaration under test —
// and each `keysym` records which keysym the keypad reports for that key with
// NumLock off, so the `want` column can be checked against the fold table
// neru_normalize_xkb_name applies to it
// (internal/adapter/platform/linux/wayland_keymap.c). Every navigation name
// below is copied from that table, and every operator is the character its
// keysym types, which neru_xkb_keysym_name answers before the table; none is
// chosen here.
var keypadKeyNameCases = []struct {
	name   string
	keysym string
	code   uint16
	want   string
}{
	// NumLock-independent (see evdevKeyNames for why the split matters).
	{name: "KEY_KPASTERISK", keysym: "KP_Multiply", code: 55, want: "*"},
	{name: "KEY_KPMINUS", keysym: "KP_Subtract", code: 74, want: "-"},
	{name: "KEY_KPPLUS", keysym: "KP_Add", code: 78, want: "+"},
	{name: "KEY_KPENTER", keysym: "KP_Enter", code: 96, want: evdevKeyNameReturn},
	{name: "KEY_KPSLASH", keysym: "KP_Divide", code: 98, want: "/"},

	// Dual-function, named for what they do with NumLock off.
	{name: "KEY_KP7", keysym: "KP_Home", code: 71, want: evdevKeyNameHome},
	{name: "KEY_KP8", keysym: "KP_Up", code: 72, want: evdevKeyNameUp},
	{name: "KEY_KP9", keysym: "KP_Prior", code: 73, want: evdevKeyNamePageUp},
	{name: "KEY_KP4", keysym: "KP_Left", code: 75, want: evdevKeyNameLeft},
	{name: "KEY_KP6", keysym: "KP_Right", code: 77, want: evdevKeyNameRight},
	{name: "KEY_KP1", keysym: "KP_End", code: 79, want: evdevKeyNameEnd},
	{name: "KEY_KP2", keysym: "KP_Down", code: 80, want: evdevKeyNameDown},
	{name: "KEY_KP3", keysym: "KP_Next", code: 81, want: evdevKeyNamePageDown},
	{name: "KEY_KP0", keysym: "KP_Insert", code: 82, want: evdevKeyNameInsert},
	{name: "KEY_KPDOT", keysym: "KP_Delete", code: 83, want: evdevKeyNameDelete},

	// Dual-function and NumLock-independent anyway: KP_Begin folds to the digit
	// the key carries, which is what KP_5 gives.
	{name: "KEY_KP5", keysym: "KP_Begin", code: 76, want: "5"},
}

// TestEvdevKeyName_Keypad pins the keypad on the fallback the evdev tap uses
// when xkb state creation fails. Without these the keypad reaches the handler
// as nothing at all on that path, while the Wayland and X11 taps both name it.
func TestEvdevKeyName_Keypad(t *testing.T) {
	t.Parallel()

	for _, testCase := range keypadKeyNameCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := evdevKeyName(testCase.code); got != testCase.want {
				t.Fatalf("evdevKeyName(%d) = %q, want %q (%s folds to it)",
					testCase.code, got, testCase.want, testCase.keysym)
			}
		})
	}
}

// TestEvdevKeyName_KeypadNamesAreVocabularySpellings checks the other half of
// naming the keypad: the dispatch path runs every name through
// keyvocab.NormalizeKey, and a name that is neither a named key nor a single
// character survives it unchanged and then matches no binding. That is the
// silent failure ADR 0008 was written about, so it is pinned rather than
// assumed. It says nothing about whether a chord can be *written* with the
// name — "+" is a single character and also the chord separator.
func TestEvdevKeyName_KeypadNamesAreVocabularySpellings(t *testing.T) {
	t.Parallel()

	for _, testCase := range keypadKeyNameCases {
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
