package architecture_test

import (
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// This guardrail pins what `just --list` says about each recipe.
//
// `just` documents a recipe from the last contiguous comment line above it,
// and this justfile writes rationale paragraphs there, so for twenty-six of
// fifty-odd recipes the listed text was the tail of a sentence: `build`
// advertised itself as "X11/Wayland native backends). Windows currently builds
// with CGO disabled.", `list-foundation-packages` as "check could see.". Which
// line `just` happens to read is a function of where someone put a blank line,
// so the layout fix does not hold either — `build` had one and still rendered
// wrong.
//
// Nothing caught it. The justfile parses, every recipe runs, `just lint` is
// silent and every test passes with the list wrong, which is how it stayed
// wrong for months and is exactly the breach ADR 0011 says earns a guardrail.
// docs/DEVELOPMENT.md names `just --list` as how a newcomer sees what the
// project can do, so the orient step of the first hour was reading as noise.
//
// The fix is a declaration rather than a layout: every recipe `just --list`
// shows carries its own `[doc('…')]` attribute, which overrides the comment
// block entirely and leaves the prose above it exactly where it is.
//
// This reads the justfile as text and never runs `just`, per ADR 0011: the
// tests in this package hold on every CI leg regardless of what is installed.
const firstHourADR = "docs/adr/0012-the-first-hour-must-not-lie.md"

// maxSummaryColumns is the widest summary a recipe may declare.
//
// It caps the summary text, not the line `just --list` prints, and the
// difference is deliberate. `just` pads every signature out to the widest one
// it will align — signatures past 50 columns are dropped from the alignment
// and get their summary on a line of their own — which today is `generate-icons
// APP_ICON TRAY_ACTIVE TRAY_DISABLED` at 49. That puts the summary column at
// 56, so an 80-column terminal leaves 24 for the text. Summaries that short are
// labels rather than sentences, and they would be a downgrade for the entries
// that already read correctly today at 40 to 76 columns.
//
// So the rule this width states is that a summary is one line and not a
// paragraph. Past it the text is rationale, and rationale belongs in the
// comment block above the recipe — which the attribute deliberately leaves
// alone.
const maxSummaryColumns = 80

// minListedRecipes guards against a vacuous pass. The justfile lists fifty-odd
// recipes; a parser that matched none — a header pattern that stopped
// resolving, a renamed justfile — would satisfy every assertion below while
// checking nothing at all.
const minListedRecipes = 40

// docAttributePattern captures the text of a `[doc('…')]` attribute, in either
// quoting. Both are read: a single-quoted string is raw, so it cannot carry an
// apostrophe, and the double-quoted form is the only way to write a summary
// that needs one. The capture is the source text, so an escape in a
// double-quoted summary counts its backslash toward the width — a summary near
// enough the cap for that to matter is over it in spirit anyway.
var docAttributePattern = regexp.MustCompile(
	`\bdoc\(\s*(?:'([^']*)'|"((?:[^"\\]|\\.)*)")\s*\)`,
)

// privateAttributePattern matches the `[private]` attribute, which hides a
// recipe from `just --list`. It is matched as a whole word between the
// brackets and commas that delimit an attribute list.
var privateAttributePattern = regexp.MustCompile(`[\[,]\s*private\s*[,\]]`)

// TestEveryListedRecipeDeclaresItsSummary fails when a recipe `just --list`
// shows has no summary of its own, so the listed text falls back to whatever
// line of prose happens to sit above it.
func TestEveryListedRecipeDeclaresItsSummary(t *testing.T) {
	recipes := parseJustfile(t)

	listed := 0

	for _, name := range slices.Sorted(maps.Keys(recipes)) {
		if !recipeIsListed(name, recipes[name]) {
			continue
		}

		listed++

		summary, declared := recipeSummary(recipes[name])

		fault := summaryFault(summary, declared)
		if fault == "" {
			continue
		}

		t.Errorf(
			"the %s recipe in %s %s; declare a one-line [doc('…')] summary of "+
				"what the recipe does on every recipe `just --list` shows, and "+
				"leave the comment block above it alone (%s)",
			name, justfileName, fault, firstHourADR,
		)
	}

	assertWalkedAtLeast(t, "recipes `just --list` shows", listed, minListedRecipes)
}

// justfileSummaryFixture is a justfile holding one recipe of each shape the
// rule has an answer for. It is text rather than a temporary file because the
// parser reads text, and because a fixture `just` never runs cannot drift into
// depending on `just` being installed.
var justfileSummaryFixture = `# Prose about the good recipe, wrapped over
# more than one line, as this justfile writes it.
[doc('Build the development binary into bin/neru.')]
good:
    @echo good

# Prose whose tail is what ` + "`just --list`" + ` would show.
bare:
    @echo bare

[doc('')]
empty:
    @echo empty

[doc('` + overlongFixtureSummary + `')]
overlong:
    @echo overlong

[no-exit-message, doc('Carry the summary beside another attribute.')]
combined:
    @echo combined

[doc("Quote it the other way, which is how a summary carries an apostrophe.")]
doublequoted:
    @echo doublequoted

[private]
[doc('Hidden from just --list, so it owes no summary.')]
hidden:
    @echo hidden

_underscored:
    @echo underscored
`

// overlongFixtureSummary is one column past the cap, which is the only width
// that proves where the cap is.
var overlongFixtureSummary = strings.Repeat("x", maxSummaryColumns+1)

// TestJustfileSummaryRule_RejectsWhatItMustReject holds the rule to each shape
// it has to separate: a recipe with no attribute at all, one declaring an
// empty string, one declaring a paragraph, and — in the other direction — the
// recipes `just --list` never shows, which owe nothing.
//
// The real justfile can only ever show the rule passing. A rule that quietly
// stopped rejecting anything would pass there too, forever.
func TestJustfileSummaryRule_RejectsWhatItMustReject(t *testing.T) {
	recipes := parseJustfileText(justfileSummaryFixture)

	tests := []struct {
		recipe   string
		listed   bool
		rejected bool
	}{
		{recipe: "good", listed: true, rejected: false},
		{recipe: "combined", listed: true, rejected: false},
		{recipe: "doublequoted", listed: true, rejected: false},
		{recipe: "bare", listed: true, rejected: true},
		{recipe: "empty", listed: true, rejected: true},
		{recipe: "overlong", listed: true, rejected: true},
		{recipe: "hidden", listed: false, rejected: false},
		{recipe: "_underscored", listed: false, rejected: false},
	}

	// Exact, not a floor: the fixture declares one recipe per case below, so a
	// parser that merged two of them or invented one is a parser this test can
	// no longer speak for.
	if len(recipes) != len(tests) {
		t.Fatalf(
			"the fixture parsed to %d recipes, want %d; the parser is not reading "+
				"the fixture the way it reads the justfile",
			len(recipes), len(tests),
		)
	}

	for _, testCase := range tests {
		t.Run(testCase.recipe, func(t *testing.T) {
			recipe, defined := recipes[testCase.recipe]
			if !defined {
				t.Fatalf("the fixture defines no %s recipe", testCase.recipe)
			}

			if got := recipeIsListed(testCase.recipe, recipe); got != testCase.listed {
				t.Fatalf("recipeIsListed(%s) = %t, want %t", testCase.recipe, got, testCase.listed)
			}

			if !testCase.listed {
				return
			}

			summary, declared := recipeSummary(recipe)

			fault := summaryFault(summary, declared)
			if (fault != "") != testCase.rejected {
				t.Errorf(
					"summaryFault for %s = %q, want rejected = %t",
					testCase.recipe, fault, testCase.rejected,
				)
			}
		})
	}
}

// recipeIsListed reports whether `just --list` shows the recipe, which is the
// only place a declared summary is read and so the only place one is owed. A
// leading underscore and the `[private]` attribute both hide one.
func recipeIsListed(name string, recipe justRecipe) bool {
	if strings.HasPrefix(name, "_") {
		return false
	}

	return !privateAttributePattern.MatchString(recipe.attributes)
}

// recipeSummary returns the text of the recipe's `[doc('…')]` attribute and
// whether it declares one at all. The two are separate answers: a recipe with
// no attribute and one declaring an empty string are different mistakes, and
// the failure message says which.
func recipeSummary(recipe justRecipe) (string, bool) {
	match := docAttributePattern.FindStringSubmatch(recipe.attributes)
	if match == nil {
		return "", false
	}

	// One of the two quotings matched; the other capture is empty.
	return match[1] + match[2], true
}

// summaryFault returns why a declared summary fails the rule, phrased to slot
// into the failure message, or the empty string when it passes.
func summaryFault(summary string, declared bool) string {
	switch {
	case !declared:
		return "declares no [doc('…')] attribute, so `just --list` shows whatever " +
			"comment line happens to sit above it"
	case strings.TrimSpace(summary) == "":
		return "declares an empty summary, which lists as a bare `#`"
	case utf8.RuneCountInString(summary) > maxSummaryColumns:
		return "declares a summary of " +
			strconv.Itoa(utf8.RuneCountInString(summary)) +
			" columns, past the " + strconv.Itoa(maxSummaryColumns) +
			"-column line a summary has to fit"
	default:
		return ""
	}
}
