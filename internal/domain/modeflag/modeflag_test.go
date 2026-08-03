package modeflag_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/domain/modeflag"
)

// The vocabulary is a compatibility contract, not an implementation detail.
// Users write these flags into hotkey bindings, macros and scripts, and the
// bindings outlive any one release. Both readers derive from the table, so a
// change there moves them together and no consistency check between them can
// notice — which is why the table itself is spelled out here.
//
// A failure means a user's existing binding stopped working. Update the case
// only when that is what was intended.
func TestAll_VocabularyIsWhatWasPublished(t *testing.T) {
	published := []modeflag.Spec{
		{Name: "action", Short: "a", TakesValue: true},
		{Name: "modifier", TakesValue: true},
		{Name: "on-exit", TakesValue: true},
		{Name: "repeat", Short: "r"},
		{Name: "toggle", Short: "t"},
		{Name: "search", Short: "s"},
		{Name: "hide-on-empty-search"},
		{Name: "role", TakesValue: true},
		{Name: "text", TakesValue: true},
		{Name: "strategy", TakesValue: true},
		{Name: "debug", Short: "d"},
		{Name: "label-direction", TakesValue: true},
		{Name: "split-word"},
		{Name: "zoom-to-depth", TakesValue: true},
		{Name: "cursor-selection-mode", TakesValue: true},
	}

	actual := modeflag.All()

	if len(actual) != len(published) {
		t.Fatalf("vocabulary has %d flags, want the %d published", len(actual), len(published))
	}

	for index, want := range published {
		got := actual[index]

		if got.Name != want.Name {
			t.Errorf("flag %d = %q, want %q", index, got.Name, want.Name)

			continue
		}

		if got.Short != want.Short {
			t.Errorf("--%s short form = %q, want %q; an existing -%s binding would stop working",
				want.Name, got.Short, want.Short, want.Short)
		}

		if got.TakesValue != want.TakesValue {
			t.Errorf("--%s takes a value = %v, want %v", want.Name, got.TakesValue, want.TakesValue)
		}
	}
}

func TestSpec_MatchAcceptsEverySpelling(t *testing.T) {
	spec, known := modeflag.Get(modeflag.Action)
	if !known {
		t.Fatal("action is missing from the vocabulary")
	}

	for _, arg := range []string{"--action", "--action=left_click", "-a", "-a=left_click"} {
		if !spec.Match(arg) {
			t.Errorf("Match(%q) = false, want it recognized as --action", arg)
		}
	}
}

// TestSpec_MatchRejectsNeighbouringNames guards the prefix comparison: "--role" must
// not swallow "--roles", or a flag added later would be eaten by an older one.
func TestSpec_MatchRejectsNeighbouringNames(t *testing.T) {
	spec, _ := modeflag.Get(modeflag.Role)

	for _, arg := range []string{"--roles", "--role-filter", "-role", "--text=OK", "role"} {
		if spec.Match(arg) {
			t.Errorf("Match(%q) = true, want only --role and its own spellings", arg)
		}
	}
}

// TestSpec_FlagsWithoutAShortFormDoNotClaimOne pins that a bare "-" spelling reaches
// nothing when the flag has no short form, rather than matching "-".
func TestSpec_FlagsWithoutAShortFormDoNotClaimOne(t *testing.T) {
	spec, _ := modeflag.Get(modeflag.SplitWord)

	if spec.Short != "" {
		t.Skip("split-word gained a short form; this case no longer applies")
	}

	for _, arg := range []string{"-", "-=x", "-s"} {
		if spec.Match(arg) {
			t.Errorf("Match(%q) = true for a flag with no short form", arg)
		}
	}
}

func TestName_AssignAndFlagProduceTheWireForms(t *testing.T) {
	if got := modeflag.Strategy.Flag(); got != "--strategy" {
		t.Errorf("Flag() = %q, want --strategy", got)
	}

	if got := modeflag.Strategy.Assign("vision"); got != "--strategy=vision" {
		t.Errorf("Assign() = %q, want --strategy=vision", got)
	}

	if got := modeflag.Strategy.String(); got != "strategy" {
		t.Errorf("String() = %q, want the bare name cobra registers", got)
	}
}

// TestAll_ReturnsACopy pins that a caller cannot rewrite the vocabulary for
// everyone else by editing what it was handed.
func TestAll_ReturnsACopy(t *testing.T) {
	modeflag.All()[0].Name = "tampered"

	if modeflag.All()[0].Name != modeflag.Action {
		t.Error("All() hands out the vocabulary itself; a caller can rewrite it")
	}
}
