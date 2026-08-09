//go:build integration && darwin

package darwin_test

import (
	"os"
	"runtime"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/platform/darwin"
)

// Pin the main thread during package init so TestMain still runs on it.
func init() {
	runtime.LockOSThread()
}

// TestMain services the macOS main run loop while the tests run. Building the
// keyboard layout maps happens on the main queue, which nothing drains in a
// plain `go test` binary (see runloop.go).
func TestMain(m *testing.M) {
	os.Exit(darwin.RunMainLoopForTesting(m.Run))
}

// Virtual key codes under test, mirroring the KeyCode enum in keymap.h.
const (
	keyCodeReturn       = 36 // main-keyboard Return
	keyCodeDelete       = 51 // main-keyboard Delete (backspace position)
	keyCodeNumpadDot    = 65
	keyCodeNumpadClear  = 71
	keyCodeNumpadEnter  = 76
	keyCodeNumpadEquals = 81
	keyCodeNumpad0      = 82
	keyCodeNumpad5      = 87
	keyCodeNumpad9      = 92
	keyCodeNumpadDivide = 75
	keyCodeNumpadMinus  = 78
	keyCodeNumpadPlus   = 69
	keyCodeNumpadStar   = 67
)

// Named-key spellings the event tap emits on the wire.
const (
	namedKeyReturn = "Return"
	namedKeyClear  = "Clear"
)

// TestKeymap_KeyCodeToCharacter_NumpadFoldsToNamedKeys pins issue #1372: the
// numpad keys with a named-key equivalent must fold to it, the way the Wayland
// keymap folds KP_Enter -> Return, instead of surfacing raw control characters
// ("\x03" for numpad Enter, "\x7f" for numpad Clear) that either match no
// binding or impersonate the Delete key.
func TestKeymap_KeyCodeToCharacter_NumpadFoldsToNamedKeys(t *testing.T) {
	tests := []struct {
		name    string
		keyCode uint16
		want    string
	}{
		{"numpad Enter folds to Return", keyCodeNumpadEnter, namedKeyReturn},
		{"numpad Clear folds to Clear", keyCodeNumpadClear, namedKeyClear},
		{"numpad digit 0 stays a digit", keyCodeNumpad0, "0"},
		{"numpad digit 5 stays a digit", keyCodeNumpad5, "5"},
		{"numpad digit 9 stays a digit", keyCodeNumpad9, "9"},
		{"numpad dot stays a dot", keyCodeNumpadDot, "."},
		{"numpad plus stays an operator", keyCodeNumpadPlus, "+"},
		{"main-keyboard Return keeps its carriage-return character", keyCodeReturn, "\r"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := darwin.KeyCodeToCharacter(testCase.keyCode, 0)
			if got != testCase.want {
				t.Errorf(
					"KeyCodeToCharacter(%d, 0) = %q, want %q",
					testCase.keyCode,
					got,
					testCase.want,
				)
			}
		})
	}
}

// TestKeymap_KeyCodeToName_NumpadFoldsToNamedKeys pins the modifier-combo
// path of issue #1372: the event tap resolves "Cmd+..." / "Shift+..." combos
// through the keycode-to-name map, so numpad Enter must name "Return" there
// too or Cmd+NumpadEnter degrades to a bare character event.
func TestKeymap_KeyCodeToName_NumpadFoldsToNamedKeys(t *testing.T) {
	tests := []struct {
		name    string
		keyCode uint16
		want    string
	}{
		{"numpad Enter names Return", keyCodeNumpadEnter, namedKeyReturn},
		{"numpad Clear names Clear", keyCodeNumpadClear, namedKeyClear},
		{"main-keyboard Return still names Return", keyCodeReturn, namedKeyReturn},
		{"main-keyboard Delete still names Delete", keyCodeDelete, "Delete"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := darwin.KeyCodeToName(testCase.keyCode)
			if got != testCase.want {
				t.Errorf("KeyCodeToName(%d) = %q, want %q", testCase.keyCode, got, testCase.want)
			}
		})
	}
}

// TestKeymap_KeyCodeToCharacter_NumpadEmitsNoControlCharacters pins the
// acceptance criterion that no raw control character reaches key comparison
// from the numpad path.
func TestKeymap_KeyCodeToCharacter_NumpadEmitsNoControlCharacters(t *testing.T) {
	numpadKeyCodes := []uint16{
		keyCodeNumpadDot, keyCodeNumpadStar, keyCodeNumpadPlus, keyCodeNumpadClear,
		keyCodeNumpadDivide, keyCodeNumpadEnter, keyCodeNumpadMinus, keyCodeNumpadEquals,
		keyCodeNumpad0, 83, 84, 85, 86, keyCodeNumpad5, 88, 89, 91, keyCodeNumpad9,
	}

	for _, keyCode := range numpadKeyCodes {
		got := darwin.KeyCodeToCharacter(keyCode, 0)
		for _, r := range got {
			if r < 0x20 || r == 0x7f {
				t.Errorf(
					"KeyCodeToCharacter(%d, 0) = %q contains control character %U",
					keyCode, got, r,
				)
			}
		}
	}
}
