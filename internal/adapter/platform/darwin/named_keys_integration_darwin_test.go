//go:build integration && darwin

package darwin_test

import (
	"slices"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/platform/darwin"
	"github.com/y3owk1n/neru/internal/domain/keyvocab"
)

// carbonKeyCodes maps each named key macOS can reach to its Carbon virtual key
// code (kVK_* in HIToolbox/Events.h, a frozen ABI). Each key is the spelling the
// event tap must emit.
var carbonKeyCodes = map[string]uint16{
	keyvocab.KeySpace:    49,
	keyvocab.KeyReturn:   36,
	keyvocab.KeyEscape:   53,
	keyvocab.KeyTab:      48,
	keyvocab.KeyDelete:   51,
	keyvocab.KeyLeft:     123,
	keyvocab.KeyRight:    124,
	keyvocab.KeyDown:     125,
	keyvocab.KeyUp:       126,
	keyvocab.KeyPageUp:   116,
	keyvocab.KeyPageDown: 121,
	keyvocab.KeyHome:     115,
	keyvocab.KeyEnd:      119,
	"F1":                 122,
	"F2":                 120,
	"F3":                 99,
	"F4":                 118,
	"F5":                 96,
	"F6":                 97,
	"F7":                 98,
	"F8":                 100,
	"F9":                 101,
	"F10":                109,
	"F11":                103,
	"F12":                111,
	"F13":                105,
	"F14":                107,
	"F15":                113,
	"F16":                106,
	"F17":                64,
	"F18":                79,
	"F19":                80,
	"F20":                90,
}

// darwinNamedKeyGaps are the named keys Carbon declares no virtual key code
// for, so a binding on one reaches a keystroke only on Linux and Windows.
var darwinNamedKeyGaps = []string{"F21", "F22", "F23", "F24", keyvocab.KeyInsert}

// TestParseKeyString_NamedKeysParseToTheirCarbonKeyCode checks that every
// named key a config can bind parses to the key it means on macOS. A name the
// keymap does not carry parses to nothing, and the binding never fires.
func TestParseKeyString_NamedKeysParseToTheirCarbonKeyCode(t *testing.T) {
	for _, name := range keyvocab.NamedKeys() {
		keyCode, _, parsed := darwin.ParseKeyString(name)

		if slices.Contains(darwinNamedKeyGaps, name) {
			if parsed {
				t.Errorf(
					"ParseKeyString(%q) = %d, but darwinNamedKeyGaps says macOS cannot reach it",
					name,
					keyCode,
				)
			}

			continue
		}

		means := name
		if resolved, isAlias := keyvocab.ResolveAlias(name); isAlias {
			means = resolved
		}

		want, known := carbonKeyCodes[means]
		if !known {
			t.Errorf(
				"%q has no Carbon key code in carbonKeyCodes and is not in darwinNamedKeyGaps",
				means,
			)

			continue
		}

		if !parsed || keyCode != int(want) {
			t.Errorf("ParseKeyString(%q) = %d, %t, want %d", name, keyCode, parsed, want)
		}
	}
}

// TestSpecialKeyName_AnswersTheVocabularySpelling checks the other direction.
// The event tap must name each keystroke with the vocabulary's spelling, never
// with an alias. Otherwise the keystroke does not equal the binding written for
// it. No other key code may get a name.
func TestSpecialKeyName_AnswersTheVocabularySpelling(t *testing.T) {
	for name, keyCode := range carbonKeyCodes {
		if got := darwin.SpecialKeyName(keyCode); got != name {
			t.Errorf("SpecialKeyName(%d) = %q, want %q", keyCode, got, name)
		}

		if got := darwin.KeyCodeToName(keyCode); got != name {
			t.Errorf("KeyCodeToName(%d) = %q, want %q", keyCode, got, name)
		}
	}

	named := make(map[uint16]bool, len(carbonKeyCodes))
	for _, keyCode := range carbonKeyCodes {
		named[keyCode] = true
	}

	for keyCode := range uint16(128) {
		if named[keyCode] {
			continue
		}

		if got := darwin.SpecialKeyName(keyCode); got != "" {
			t.Errorf("SpecialKeyName(%d) = %q, which carbonKeyCodes does not list", keyCode, got)
		}
	}
}
