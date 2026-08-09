package keyvocab_test

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/keyvocab"
)

// Spellings the tests in this package assert against, written out rather than
// read from the table under test. The key* constants are display forms — the
// spelling a config file writes — and the name* constants are the lowercase
// inputs a caller may pass instead.
const (
	keyReturn    = "Return"
	keyEnter     = "Enter"
	keyEscape    = "Escape"
	keyDelete    = "Delete"
	keyLeft      = "Left"
	keyBackspace = "Backspace"
	keyInsert    = "Insert"
	keyPageDown  = "PageDown"

	keyMouseLeft   = "MouseLeft"
	keyMouseRight  = "MouseRight"
	keyMouseMiddle = "MouseMiddle"

	nameEnter     = "enter"
	nameBackspace = "backspace"
	nameInsert    = "insert"
	namePageDown  = "pagedown"
	nameEsc       = "esc"
)

// documentedNamedKeys is the vocabulary as docs/CONFIGURATION.md lists it,
// written out rather than derived from the table under test.
func documentedNamedKeys() []string {
	keys := []string{
		"Space", keyReturn, keyEnter, keyEscape, "Tab", keyDelete, keyBackspace,
		"Up", "Down", keyLeft, "Right", "Home", "End", "PageUp", keyPageDown,
		keyInsert,
		keyMouseLeft, keyMouseRight, keyMouseMiddle,
	}
	for index := 1; index <= 24; index++ {
		keys = append(keys, "F"+strconv.Itoa(index))
	}

	return keys
}

// TestNamedKeys_IsExactlyTheDocumentedSet is the pin: the declared vocabulary
// and the one docs/CONFIGURATION.md promises are the same set, so adding a key
// in one place without the other fails here rather than in a config file.
func TestNamedKeys_IsExactlyTheDocumentedSet(t *testing.T) {
	t.Parallel()

	want := documentedNamedKeys()
	slices.Sort(want)

	if got := keyvocab.NamedKeys(); !slices.Equal(got, want) {
		t.Errorf("NamedKeys() = %v, want %v", got, want)
	}
}

// TestIsNamedKey_TheDeclaredSet checks every documented spelling is recognized,
// case-insensitively.
func TestIsNamedKey_TheDeclaredSet(t *testing.T) {
	t.Parallel()

	for _, name := range documentedNamedKeys() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if !keyvocab.IsNamedKey(name) {
				t.Errorf("IsNamedKey(%q) = false, want true", name)
			}

			if !keyvocab.IsNamedKey(strings.ToLower(name)) {
				t.Errorf("IsNamedKey(%q) = false, want true", strings.ToLower(name))
			}
		})
	}
}

// TestIsNamedKey_OutsideTheSet pins the closed edges of the vocabulary: the
// function key range stops at F24, "esc" is a comparison shorthand rather than
// a spelling a binding may use, and CapsLock is declined (ADR 0008).
func TestIsNamedKey_OutsideTheSet(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"F0", "F25", nameEsc, "Esc", "CapsLock", "", "Meh"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if keyvocab.IsNamedKey(name) {
				t.Errorf("IsNamedKey(%q) = true, want false", name)
			}
		})
	}
}

// TestNamedKeyDisplay_KeepsTheSpellingWritten checks that an alias reports its
// own display form rather than the key it means: "Enter" is what a config file
// wrote and what a diagnostic should echo back.
func TestNamedKeyDisplay_KeepsTheSpellingWritten(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
	}{
		{in: namePageDown, want: keyPageDown},
		{in: "UP", want: "Up"},
		{in: "f1", want: "F1"},
		{in: nameEnter, want: keyEnter},
		{in: nameBackspace, want: keyBackspace},
		{in: nameInsert, want: keyInsert},
	}

	for _, testCase := range tests {
		t.Run(testCase.in, func(t *testing.T) {
			t.Parallel()

			got, ok := keyvocab.NamedKeyDisplay(testCase.in)
			if !ok || got != testCase.want {
				t.Errorf(
					"NamedKeyDisplay(%q) = (%q, %t), want (%q, true)",
					testCase.in, got, ok, testCase.want,
				)
			}
		})
	}

	if got, ok := keyvocab.NamedKeyDisplay("nope"); ok || got != "nope" {
		t.Errorf(`NamedKeyDisplay("nope") = (%q, %t), want ("nope", false)`, got, ok)
	}
}

// TestResolveAlias_MeansAnotherKey pins the two aliases the vocabulary carries
// plus the "esc" shorthand, and that a key meaning itself is not an alias.
func TestResolveAlias_MeansAnotherKey(t *testing.T) {
	t.Parallel()

	aliases := map[string]string{
		keyEnter:      keyReturn,
		nameEnter:     keyReturn,
		keyBackspace:  keyDelete,
		nameBackspace: keyDelete,
		nameEsc:       keyEscape,
		"ESC":         keyEscape,
	}

	for spelling, want := range aliases {
		t.Run(spelling, func(t *testing.T) {
			t.Parallel()

			got, ok := keyvocab.ResolveAlias(spelling)
			if !ok || got != want {
				t.Errorf("ResolveAlias(%q) = (%q, %t), want (%q, true)", spelling, got, ok, want)
			}
		})
	}

	for _, notAnAlias := range []string{keyReturn, keyDelete, keyEscape, "Up", "j", ""} {
		if got, ok := keyvocab.ResolveAlias(notAnAlias); ok {
			t.Errorf("ResolveAlias(%q) = (%q, true), want ok = false", notAnAlias, got)
		}
	}
}
