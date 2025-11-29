//go:build unit

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

func TestAlphabetGenerator_Generate(t *testing.T) {
	generator, generatorErr := hint.NewAlphabetGenerator("asdf")
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
			generator, generatorErr := hint.NewAlphabetGenerator(testCase.characters)
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
	generator, _ := hint.NewAlphabetGenerator("as") // Max 6 hints (2 + 2*2)

	// Try to generate 10 hints
	elements := make([]*element.Element, 10)
	for index := range elements {
		elements[index], _ = element.NewElement(
			element.ID("elem-"+string(rune('0'+index))),
			image.Rect(0, 0, 50, 50),
			element.RoleButton,
		)
	}

	ctx := context.Background()

	_, generateErr := generator.Generate(ctx, elements)
	if generateErr == nil {
		t.Error("Generate() expected error for too many elements, got nil")
	}
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

// Helper function for tests.
func mustNewHint(label string, elem *element.Element) *hint.Interface {
	hint, hintErr := hint.NewHint(label, elem, image.Point{})
	if hintErr != nil {
		panic(hintErr)
	}

	return hint
}
