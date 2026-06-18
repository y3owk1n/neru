package hint_test

import (
	"context"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/core/domain/element"
	"github.com/y3owk1n/neru/internal/core/domain/hint"
)

func TestNewHint(t *testing.T) {
	elem, _ := element.NewElement("test", image.Rect(10, 10, 50, 50), element.RoleButton)

	tests := []struct {
		name     string
		label    string
		elem     *element.Element
		position image.Point
		wantErr  bool
	}{
		{
			name:     "valid hint",
			label:    "AS",
			elem:     elem,
			position: image.Point{X: 30, Y: 30},
			wantErr:  false,
		},
		{
			name:     "empty label",
			label:    "",
			elem:     elem,
			position: image.Point{X: 30, Y: 30},
			wantErr:  true,
		},
		{
			name:     "nil element",
			label:    "AS",
			elem:     nil,
			position: image.Point{X: 30, Y: 30},
			wantErr:  true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			testHint, hintErr := hint.NewHint(testCase.label, testCase.elem, testCase.position)

			if testCase.wantErr {
				if hintErr == nil {
					t.Error("NewHint() expected error, got nil")
				}

				return
			}

			if hintErr != nil {
				t.Errorf("NewHint() unexpected error: %v", hintErr)

				return
			}

			if testHint.Label() != testCase.label {
				t.Errorf("Label() = %v, want %v", testHint.Label(), testCase.label)
			}

			if testHint.Position() != testCase.position {
				t.Errorf("Position() = %v, want %v", testHint.Position(), testCase.position)
			}
		})
	}
}

func TestHint_HasPrefix(t *testing.T) {
	elem, _ := element.NewElement("test", image.Rect(10, 10, 50, 50), element.RoleButton)
	testHint, _ := hint.NewHint("ASDF", elem, image.Point{})

	tests := []struct {
		prefix string
		want   bool
	}{
		{"A", true},
		{"AS", true},
		{"ASD", true},
		{"ASDF", true},
		{"ASDFF", false},
		{"B", false},
		{"", true}, // Empty prefix matches everything
	}

	for _, testCase := range tests {
		t.Run(testCase.prefix, func(t *testing.T) {
			got := testHint.HasPrefix(testCase.prefix)
			if got != testCase.want {
				t.Errorf("HasPrefix(%q) = %v, want %v", testCase.prefix, got, testCase.want)
			}
		})
	}
}

func TestHint_Methods(t *testing.T) {
	elem, _ := element.NewElement("test", image.Rect(10, 10, 50, 50), element.RoleButton)
	testHint, _ := hint.NewHint("AS", elem, image.Point{X: 30, Y: 30})

	// Test Element()
	if testHint.Element() != elem {
		t.Error("Element() returned wrong element")
	}

	// Test MatchedPrefix() - initially empty
	if testHint.MatchedPrefix() != "" {
		t.Errorf("MatchedPrefix() = %q, want empty string", testHint.MatchedPrefix())
	}

	// Test WithMatchedPrefix()
	hintWithPrefix := testHint.WithMatchedPrefix("A")
	if hintWithPrefix.MatchedPrefix() != "A" {
		t.Errorf(
			"WithMatchedPrefix() MatchedPrefix() = %q, want %q",
			hintWithPrefix.MatchedPrefix(),
			"A",
		)
	}

	// Test that WithMatchedPrefix returns a new instance
	if hintWithPrefix == testHint {
		t.Error("WithMatchedPrefix() should return a new instance")
	}

	// Test Bounds()
	bounds := testHint.Bounds()

	expectedBounds := image.Rect(10, 10, 50, 50)
	if bounds != expectedBounds {
		t.Errorf("Bounds() = %v, want %v", bounds, expectedBounds)
	}

	// Test IsVisible()
	screenBounds := image.Rect(0, 0, 100, 100)
	if !testHint.IsVisible(screenBounds) {
		t.Error("IsVisible() should return true for element within screen bounds")
	}

	smallScreen := image.Rect(60, 60, 100, 100)
	if testHint.IsVisible(smallScreen) {
		t.Error("IsVisible() should return false for element outside screen bounds")
	}

	// Test MatchesLabel()
	if !testHint.MatchesLabel("AS") {
		t.Error("MatchesLabel() should return true for exact match")
	}

	if testHint.MatchesLabel("A") {
		t.Error("MatchesLabel() should return false for partial match")
	}

	if testHint.MatchesLabel("ASDF") {
		t.Error("MatchesLabel() should return false for longer string")
	}
}

func TestCollection_FilterByText_Normalizes(t *testing.T) {
	cafeElem, _ := element.NewElement(
		"cafe",
		image.Rect(10, 10, 50, 50),
		element.RoleButton,
		element.WithTitle("Café"),
	)
	cafeHint, _ := hint.NewHint("AA", cafeElem, image.Point{X: 30, Y: 30})

	cjkElem, _ := element.NewElement(
		"cjk",
		image.Rect(10, 10, 50, 50),
		element.RoleButton,
		element.WithTitle("你好"), //nolint:gosmopolitan
	)
	cjkHint, _ := hint.NewHint("AB", cjkElem, image.Point{X: 30, Y: 30})

	collection := hint.NewCollection([]*hint.Interface{cafeHint, cjkHint})

	if got := collection.FilterByText("cafe").Count(); got != 1 {
		t.Fatalf("FilterByText(%q) = %d, want 1", "cafe", got)
	}

	if got := collection.FilterByText("CAFÉ").Count(); got != 1 {
		t.Fatalf("FilterByText(%q) = %d, want 1", "CAFÉ", got)
	}

	if got := collection.FilterByText("你好").Count(); got != 1 { //nolint:gosmopolitan
		t.Fatalf("FilterByText(%q) = %d, want 1", "你好", got) //nolint:gosmopolitan
	}
}

func TestCollection_FilterByText_SearchesAdditionalText(t *testing.T) {
	rowElem, _ := element.NewElement(
		"note-row",
		image.Rect(10, 10, 100, 50),
		element.Role("AXRow"),
		element.WithSearchText("Quarterly planning notes"),
	)
	rowHint, _ := hint.NewHint("AA", rowElem, image.Point{X: 30, Y: 30})

	collection := hint.NewCollection([]*hint.Interface{rowHint})

	if got := collection.FilterByText("planning").Count(); got != 1 {
		t.Fatalf("FilterByText(%q) = %d, want 1", "planning", got)
	}
}

func TestAlphabetGenerator_Generate(t *testing.T) {
	generator, generatorErr := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionReverse)
	if generatorErr != nil {
		t.Fatalf("NewAlphabetGenerator() error: %v", generatorErr)
	}

	// Create test elements
	elements := make([]*element.Element, 10)
	for index := range elements {
		elements[index], _ = element.NewElement(
			element.ID("elem-"+string(rune('0'+index))),
			image.Rect(index*10, index*10, index*10+50, index*10+50),
			element.RoleButton,
		)
	}

	ctx := context.Background()

	hints, hintsErr := generator.Generate(ctx, elements)
	if hintsErr != nil {
		t.Fatalf("Generate() error: %v", hintsErr)
	}

	if len(hints) != len(elements) {
		t.Errorf("Generate() returned %d hints, want %d", len(hints), len(elements))
	}

	// Check that all labels are unique
	seen := make(map[string]bool)
	for _, hint := range hints {
		if seen[hint.Label()] {
			t.Errorf("Duplicate label: %s", hint.Label())
		}

		seen[hint.Label()] = true
	}

	// Check that all labels are uppercase
	for _, hint := range hints {
		label := hint.Label()
		for _, r := range label {
			if r < 'A' || r > 'Z' {
				t.Errorf("Label %q contains non-uppercase character", label)
			}
		}
	}
}

func TestAlphabetGenerator_DeduplicatesCharacters(t *testing.T) {
	t.Run("dedup duplicate chars", func(t *testing.T) {
		generator, err := hint.NewAlphabetGenerator("aab", hint.LabelDirectionReverse)
		if err != nil {
			t.Fatalf("NewAlphabetGenerator() error: %v", err)
		}

		if got := generator.MaxHints(); got != 8 {
			t.Errorf("MaxHints() = %d, want 8 (deduped from \"aab\")", got)
		}
	})

	t.Run("no dedup needed", func(t *testing.T) {
		generator, err := hint.NewAlphabetGenerator("abc", hint.LabelDirectionReverse)
		if err != nil {
			t.Fatalf("NewAlphabetGenerator() error: %v", err)
		}

		if got := generator.MaxHints(); got != 27 {
			t.Errorf("MaxHints() = %d, want 27", got)
		}
	})

	t.Run("rejects below minimum after dedup", func(t *testing.T) {
		_, err := hint.NewAlphabetGenerator("aA", hint.LabelDirectionReverse)
		if err == nil {
			t.Fatal("NewAlphabetGenerator() expected error for \"aA\" (1 unique char after dedup)")
		}
	})
}

func TestAlphabetGenerator_DeduplicateProducesUniqueLabels(t *testing.T) {
	// With "aabc", deduped unique chars are "abc" (3 chars).
	// 3^3 = 27 max hints, so 10 elements should get unique 2-char labels.
	generator, err := hint.NewAlphabetGenerator("aabc", hint.LabelDirectionReverse)
	if err != nil {
		t.Fatalf("NewAlphabetGenerator() error: %v", err)
	}

	if got := generator.Characters(); got != "ABC" {
		t.Fatalf("Characters() = %q, want %q (deduped)", got, "ABC")
	}

	elements := make([]*element.Element, 10)
	for i := range elements {
		elements[i], _ = element.NewElement(
			element.ID(string(rune('0'+i))),
			image.Rect(i*10, i*10, i*10+50, i*10+50),
			element.RoleButton,
		)
	}

	hints, err := generator.Generate(context.Background(), elements)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	seen := make(map[string]struct{}, len(hints))
	for _, h := range hints {
		label := h.Label()
		if _, ok := seen[label]; ok {
			t.Errorf("Duplicate label generated: %q (characters=%q)", label, "aabc")
		}

		seen[label] = struct{}{}
	}

	if len(seen) != len(hints) {
		t.Errorf("Got %d unique labels for %d hints", len(seen), len(hints))
	}
}

func TestAlphabetGenerator_MaxHints(t *testing.T) {
	tests := []struct {
		characters string
		wantMax    int
	}{
		{"asdf", 64},       // 4^3 = 64
		{"asd", 27},        // 3^3 = 27
		{"asdfghjkl", 729}, // 9^3 = 729
	}

	for _, testCase := range tests {
		t.Run(testCase.characters, func(t *testing.T) {
			generator, generatorErr := hint.NewAlphabetGenerator(
				testCase.characters,
				hint.LabelDirectionReverse,
			)
			if generatorErr != nil {
				t.Fatalf("NewAlphabetGenerator() error: %v", generatorErr)
			}

			got := generator.MaxHints()
			charCount := len(testCase.characters)
			want := charCount * charCount * charCount

			if got != want {
				t.Errorf("MaxHints() = %d, want %d", got, want)
			}
		})
	}
}

func TestAlphabetGenerator_TooManyElements(t *testing.T) {
	generator, _ := hint.NewAlphabetGenerator("as", hint.LabelDirectionReverse) // Max 8 hints (2^3)

	// Try to generate more hints than the key combinations can label.
	elements := make([]*element.Element, 10)
	for index := range elements {
		elements[index], _ = element.NewElement(
			element.ID("elem-"+string(rune('0'+index))),
			image.Rect(index*10, index*10, index*10+50, index*10+50),
			element.RoleButton,
		)
	}

	ctx := context.Background()

	hints, generateErr := generator.Generate(ctx, elements)
	if generateErr != nil {
		t.Fatalf("Generate() unexpected error for too many elements: %v", generateErr)
	}

	if len(hints) != generator.MaxHints() {
		t.Fatalf("Generate() returned %d hints, want max %d", len(hints), generator.MaxHints())
	}

	for _, generatedHint := range hints {
		if generatedHint.Element().ID() == "elem-8" || generatedHint.Element().ID() == "elem-9" {
			t.Fatalf(
				"Generate() included an element beyond max hint capacity: %s",
				generatedHint.Element().ID(),
			)
		}
	}
}

func TestAlphabetGenerator_ReverseDirection(t *testing.T) {
	// Reverse direction emits fixed-length base-N labels. For "abc" with 9
	// labels (3 single-char capacity is exceeded) every label is 2 chars
	// and the output interleaves by leading character: AA, BA, CA, AB, BB,
	// CB, AC, BC, CC.
	generator, err := hint.NewAlphabetGenerator("abc", hint.LabelDirectionReverse)
	if err != nil {
		t.Fatalf("NewAlphabetGenerator() error: %v", err)
	}

	got := generator.LabelsForTesting(9)
	want := []string{
		"AA", "BA", "CA",
		"AB", "BB", "CB",
		"AC", "BC", "CC",
	}

	if !equalStringSlices(got, want) {
		t.Errorf("LabelsForTesting(9) = %v, want %v", got, want)
	}
}

func TestAlphabetGenerator_ReverseDirection_SingleCharTier(t *testing.T) {
	// When the count fits within the alphabet, reverse returns single-char
	// labels in alphabetical order.
	generator, err := hint.NewAlphabetGenerator("abc", hint.LabelDirectionReverse)
	if err != nil {
		t.Fatalf("NewAlphabetGenerator() error: %v", err)
	}

	got := generator.LabelsForTesting(3)
	want := []string{"A", "B", "C"}

	if !equalStringSlices(got, want) {
		t.Errorf("LabelsForTesting(3) = %v, want %v", got, want)
	}
}

func TestAlphabetGenerator_NormalDirection(t *testing.T) {
	// Normal direction uses the prefix-avoidance algorithm. For "abc" with 9
	// labels it expands all 3 single-char slots into 9 two-char labels.
	generator, err := hint.NewAlphabetGenerator("abc", hint.LabelDirectionNormal)
	if err != nil {
		t.Fatalf("NewAlphabetGenerator() error: %v", err)
	}

	got := generator.LabelsForTesting(9)
	want := []string{
		"AA", "AB", "AC",
		"BA", "BB", "BC",
		"CA", "CB", "CC",
	}

	if !equalStringSlices(got, want) {
		t.Errorf("LabelsForTesting(9) = %v, want %v", got, want)
	}
}

func TestAlphabetGenerator_NormalDirection_MixedTiers(t *testing.T) {
	// For "abcd" with 10 labels the algorithm keeps 2 single-char slots
	// (A, B) and emits the remaining 8 two-char labels starting from "C".
	generator, err := hint.NewAlphabetGenerator("abcd", hint.LabelDirectionNormal)
	if err != nil {
		t.Fatalf("NewAlphabetGenerator() error: %v", err)
	}

	got := generator.LabelsForTesting(10)
	want := []string{
		"A", "B",
		"CA", "CB", "CC", "CD",
		"DA", "DB", "DC", "DD",
	}

	if !equalStringSlices(got, want) {
		t.Errorf("LabelsForTesting(10) = %v, want %v", got, want)
	}
}

func TestAlphabetGenerator_NormalDirection_ExpandsToThreeChars(t *testing.T) {
	// For "abc" with 15 labels: tier 1 holds 0 (capacity 3 < 15), tier 2
	// holds 6 (capacity 9 >= 15 needs 6 of them), tier 3 holds 9. The 9
	// three-char labels continue from the base-N cursor [6,0,0] -> CAA,
	// CAB, CAC, CBA, CBB, CBC, CCA, CCB, CCC.
	generator, err := hint.NewAlphabetGenerator("abc", hint.LabelDirectionNormal)
	if err != nil {
		t.Fatalf("NewAlphabetGenerator() error: %v", err)
	}

	got := generator.LabelsForTesting(15)
	want := []string{
		"AA", "AB", "AC",
		"BA", "BB", "BC",
		"CAA", "CAB", "CAC",
		"CBA", "CBB", "CBC",
		"CCA", "CCB", "CCC",
	}

	if !equalStringSlices(got, want) {
		t.Errorf("LabelsForTesting(15) = %v, want %v", got, want)
	}
}

func TestAlphabetGenerator_NormalDirection_SingleCharTier(t *testing.T) {
	// When the count fits within the alphabet, normal returns single-char
	// labels in alphabetical order.
	generator, err := hint.NewAlphabetGenerator("abc", hint.LabelDirectionNormal)
	if err != nil {
		t.Fatalf("NewAlphabetGenerator() error: %v", err)
	}

	got := generator.LabelsForTesting(3)
	want := []string{"A", "B", "C"}

	if !equalStringSlices(got, want) {
		t.Errorf("LabelsForTesting(3) = %v, want %v", got, want)
	}
}

func TestAlphabetGenerator_DirectionsShareCacheKeys(t *testing.T) {
	// The cache key must include the direction so reverse and normal results
	// for the same (chars, count) never collide.
	reverseGen, err := hint.NewAlphabetGenerator("abc", hint.LabelDirectionReverse)
	if err != nil {
		t.Fatalf("NewAlphabetGenerator() reverse: %v", err)
	}

	normalGen, err := hint.NewAlphabetGenerator("abc", hint.LabelDirectionNormal)
	if err != nil {
		t.Fatalf("NewAlphabetGenerator() normal: %v", err)
	}

	reverseLabels := reverseGen.LabelsForTesting(9)
	normalLabels := normalGen.LabelsForTesting(9)

	if equalStringSlices(reverseLabels, normalLabels) {
		t.Fatalf(
			"reverse and normal directions produced identical labels: %v",
			reverseLabels,
		)
	}

	// Switching direction in place should yield the new direction's labels
	// without rebuilding the generator.
	reverseGen.UpdateLabelDirection(hint.LabelDirectionNormal)

	if got := reverseGen.LabelsForTesting(9); !equalStringSlices(got, normalLabels) {
		t.Errorf(
			"after UpdateLabelDirection(normal) LabelsForTesting(9) = %v, want %v",
			got, normalLabels,
		)
	}

	// And back again.
	reverseGen.UpdateLabelDirection(hint.LabelDirectionReverse)

	if got := reverseGen.LabelsForTesting(9); !equalStringSlices(got, reverseLabels) {
		t.Errorf(
			"after UpdateLabelDirection(reverse) LabelsForTesting(9) = %v, want %v",
			got, reverseLabels,
		)
	}
}

func TestAlphabetGenerator_UpdatePreservesDirection(t *testing.T) {
	// UpdateCharacters (the legacy call) must preserve the configured
	// direction rather than reset it.
	generator, err := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionNormal)
	if err != nil {
		t.Fatalf("NewAlphabetGenerator() error: %v", err)
	}

	err = generator.UpdateCharacters("qwer")
	if err != nil {
		t.Fatalf("UpdateCharacters() error: %v", err)
	}

	if got := generator.LabelDirection(); got != hint.LabelDirectionNormal {
		t.Errorf("LabelDirection() = %v, want %v", got, hint.LabelDirectionNormal)
	}
}

func TestLabelDirectionFromString(t *testing.T) {
	tests := []struct {
		input string
		want  hint.LabelDirection
	}{
		{"reverse", hint.LabelDirectionReverse},
		{"normal", hint.LabelDirectionNormal},
		{"", hint.LabelDirectionNormal},       // empty defaults to normal
		{"typo", hint.LabelDirectionNormal},   // unknown defaults to normal
		{"NORMAL", hint.LabelDirectionNormal}, // case-sensitive: unknown → default
	}

	for _, testCase := range tests {
		t.Run(testCase.input, func(t *testing.T) {
			if got := hint.LabelDirectionFromString(testCase.input); got != testCase.want {
				t.Errorf(
					"LabelDirectionFromString(%q) = %v, want %v",
					testCase.input,
					got,
					testCase.want,
				)
			}
		})
	}
}

func TestLabelDirection_String(t *testing.T) {
	if got := hint.LabelDirectionReverse.String(); got != "reverse" {
		t.Errorf("LabelDirectionReverse.String() = %q, want %q", got, "reverse")
	}

	if got := hint.LabelDirectionNormal.String(); got != "normal" {
		t.Errorf("LabelDirectionNormal.String() = %q, want %q", got, "normal")
	}

	// Unknown values fall back to the default.
	if got := hint.LabelDirection(99).String(); got != "normal" {
		t.Errorf("LabelDirection(99).String() = %q, want %q", got, "normal")
	}
}

func TestLabelDirection_Opposite(t *testing.T) {
	if got := hint.LabelDirectionReverse.Opposite(); got != hint.LabelDirectionNormal {
		t.Errorf(
			"LabelDirectionReverse.Opposite() = %v, want %v",
			got,
			hint.LabelDirectionNormal,
		)
	}

	if got := hint.LabelDirectionNormal.Opposite(); got != hint.LabelDirectionReverse {
		t.Errorf(
			"LabelDirectionNormal.Opposite() = %v, want %v",
			got,
			hint.LabelDirectionReverse,
		)
	}

	// Unknown values fall back to LabelDirectionNormal (the default), and
	// its opposite is LabelDirectionReverse.
	if got := hint.LabelDirection(99).Opposite(); got != hint.LabelDirectionReverse {
		t.Errorf(
			"LabelDirection(99).Opposite() = %v, want %v",
			got,
			hint.LabelDirectionReverse,
		)
	}
}

func equalStringSlices(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}

	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}

	return true
}

func TestNewCollection(t *testing.T) {
	element, _ := element.NewElement("test", image.Rect(0, 0, 50, 50), element.RoleButton)

	hints := []*hint.Interface{
		mustNewHint("A", element),
		mustNewHint("AS", element),
		mustNewHint("AD", element),
		mustNewHint("AF", element),
		mustNewHint("S", element),
	}

	collection := hint.NewCollection(hints)

	if collection.Count() != 5 {
		t.Errorf("Count() = %d, want 5", collection.Count())
	}

	// Test FindByLabel
	hint := collection.FindByLabel("AS")
	if hint == nil {
		t.Error("FindByLabel(\"AS\") returned nil")
	} else if hint.Label() != "AS" {
		t.Errorf("FindByLabel(\"AS\") returned hint with label %q", hint.Label())
	}

	// Test FilterByPrefix
	filtered := collection.FilterByPrefix("A")
	if len(filtered) != 4 { // A, AS, AD, AF
		t.Errorf("FilterByPrefix(\"A\") returned %d hints, want 4", len(filtered))
	}
}

func TestCollection_FilterByPrefix(t *testing.T) {
	element, _ := element.NewElement("test", image.Rect(0, 0, 50, 50), element.RoleButton)

	hints := []*hint.Interface{
		mustNewHint("AA", element),
		mustNewHint("AS", element),
		mustNewHint("AD", element),
		mustNewHint("SA", element),
		mustNewHint("SS", element),
		mustNewHint("A", element),
		mustNewHint("S", element),
	}

	collection := hint.NewCollection(hints)

	tests := []struct {
		prefix string
		want   int
	}{
		{"", 7},   // All hints
		{"A", 4},  // A, AA, AS, AD
		{"S", 3},  // S, SA, SS
		{"AA", 1}, // AA
		{"AS", 1}, // AS
		{"X", 0},  // No matches
	}

	for _, testCase := range tests {
		t.Run(testCase.prefix, func(t *testing.T) {
			filtered := collection.FilterByPrefix(testCase.prefix)
			if len(filtered) != testCase.want {
				t.Errorf(
					"FilterByPrefix(%q) returned %d hints, want %d",
					testCase.prefix,
					len(filtered),
					testCase.want,
				)
			}
		})
	}
}

func TestCollection_FilterByText(t *testing.T) {
	saveButton, _ := element.NewElement(
		"save",
		image.Rect(0, 0, 50, 50),
		element.RoleButton,
		element.WithTitle("Save Document"),
	)
	searchField, _ := element.NewElement(
		"search",
		image.Rect(0, 60, 50, 110),
		element.RoleTextField,
		element.WithDescription("Project finder"),
		element.WithValue("Neru"),
	)
	cancelButton, _ := element.NewElement(
		"cancel",
		image.Rect(0, 120, 50, 170),
		element.RoleButton,
		element.WithTitle("Cancel"),
	)

	collection := hint.NewCollection([]*hint.Interface{
		mustNewHint("AA", saveButton),
		mustNewHint("AS", searchField),
		mustNewHint("AD", cancelButton),
	})

	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{name: "matches title case-insensitive", query: "save", want: []string{"AA"}},
		{name: "matches description", query: "finder", want: []string{"AS"}},
		{name: "matches value", query: "NER", want: []string{"AS"}},
		{name: "empty query returns all", query: "", want: []string{"AA", "AS", "AD"}},
		{name: "no matches", query: "missing", want: []string{}},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			filtered := collection.FilterByText(testCase.query)
			if filtered.Count() != len(testCase.want) {
				t.Fatalf(
					"FilterByText(%q) count = %d, want %d",
					testCase.query,
					filtered.Count(),
					len(testCase.want),
				)
			}

			for _, label := range testCase.want {
				if filtered.FindByLabel(label) == nil {
					t.Fatalf("FilterByText(%q) missing label %q", testCase.query, label)
				}
			}
		})
	}
}

// Helper function for tests.
func mustNewHint(label string, elem *element.Element) *hint.Interface {
	hint, hintErr := hint.NewHint(label, elem, image.Point{})
	if hintErr != nil {
		panic(hintErr)
	}

	return hint
}
