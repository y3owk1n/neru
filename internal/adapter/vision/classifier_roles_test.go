package vision

import (
	"reflect"
	"slices"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/element"
)

// The heuristic classifier hands back a *native* role name, because that is
// what the hint pipeline filters on: ports.ElementFilter.Roles holds the
// configured clickable roles already resolved to the running platform's
// vocabulary. A classifier that answered "AXButton" on Linux would be compared
// against a list holding "push button", match nothing, and produce a vision
// strategy that finds text and hints none of it.

// classifiableRoles pairs each field of classifierRoles that names a real
// semantic role with that role, so the two tests below can walk them together.
// Generic and Unknown are absent on purpose: neither is a semantic role a user
// can write, and both exist for regions the classifier could not identify.
var classifiableRoles = []struct {
	field    string
	semantic element.SemanticRole
	name     func(classifierRoles) string
}{
	{"Button", element.SemanticButton, func(r classifierRoles) string { return r.Button }},
	{"Link", element.SemanticLink, func(r classifierRoles) string { return r.Link }},
	{"CheckBox", element.SemanticCheckbox, func(r classifierRoles) string { return r.CheckBox }},
	{"Image", element.SemanticImage, func(r classifierRoles) string { return r.Image }},
	{
		"StaticText",
		element.SemanticStaticText,
		func(r classifierRoles) string { return r.StaticText },
	},
}

// TestClassifierRolesFor_CoversEveryAccessibilityVocabulary is the guardrail
// for that failure. Any platform whose accessibility backend has a vocabulary
// can grow a vision backend, and the day it does the role table must already
// answer in that vocabulary rather than silently borrowing macOS's.
func TestClassifierRolesFor_CoversEveryAccessibilityVocabulary(t *testing.T) {
	for _, goos := range []string{"darwin", "linux", "windows"} {
		vocab, hasVocabulary := element.VocabularyForGOOS(goos)
		if !hasVocabulary {
			t.Fatalf("%s reports no accessibility vocabulary", goos)
		}

		roles, hasRoles := classifierRolesFor(vocab)
		if !hasRoles {
			t.Errorf("no classifier role set for vocabulary %q (%s)", vocab, goos)

			continue
		}

		value := reflect.ValueOf(roles)
		for i := range value.NumField() {
			if value.Field(i).String() == "" {
				t.Errorf("%s classifier roles leave %s empty",
					vocab, value.Type().Field(i).Name)
			}
		}
	}
}

// TestClassifierRolesFor_AnswersInTheVocabularyItWasAskedFor pins the names
// themselves against the role vocabulary they have to match, rather than
// against a copy of the table under test. Every name a classifier emits must be
// one the platform's own semantic role expands to, or the configured
// clickable_roles will never contain it.
func TestClassifierRolesFor_AnswersInTheVocabularyItWasAskedFor(t *testing.T) {
	vocabularies := []element.NativeVocabulary{
		element.VocabularyAX,
		element.VocabularyATSPI,
		element.VocabularyUIA,
	}

	for _, vocab := range vocabularies {
		roles, found := classifierRolesFor(vocab)
		if !found {
			t.Errorf("no classifier role set for vocabulary %q", vocab)

			continue
		}

		for _, subject := range classifiableRoles {
			t.Run(string(vocab)+"/"+subject.field, func(t *testing.T) {
				mapping, known := element.LookupSemantic(subject.semantic)
				if !known {
					t.Fatalf("semantic role %q is not in the vocabulary", subject.semantic)
				}

				native := mapping.Native(vocab)

				got := subject.name(roles)
				if !slices.Contains(native, got) {
					t.Errorf("classifier emits %q for %s, which %q does not expand to (%v)",
						got, subject.field, subject.semantic, native)
				}
			})
		}
	}
}
