package hint_test

import (
	"context"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/domain/hint"
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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hint, hintErr := hint.NewHint(test.label, test.elem, test.position)

			if test.wantErr {
				if hintErr == nil {
					t.Error("NewHint() expected error, got nil")
				}

				return
			}

			if hintErr != nil {
				t.Errorf("NewHint() unexpected error: %v", hintErr)

				return
			}

			if hint.Label() != test.label {
				t.Errorf("Label() = %v, want %v", hint.Label(), test.label)
			}

			if hint.Position() != test.position {
				t.Errorf("Position() = %v, want %v", hint.Position(), test.position)
			}
		})
	}
}

func TestHint_HasPrefix(t *testing.T) {
	elem, _ := element.NewElement("test", image.Rect(10, 10, 50, 50), element.RoleButton)
	hint, _ := hint.NewHint("ASDF", elem, image.Point{})

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

	for _, test := range tests {
		t.Run(test.prefix, func(t *testing.T) {
			got := hint.HasPrefix(test.prefix)
			if got != test.want {
				t.Errorf("HasPrefix(%q) = %v, want %v", test.prefix, got, test.want)
			}
		})
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

	context := context.Background()

	hints, hintsErr := generator.Generate(context, elements)
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

	for _, test := range tests {
		t.Run(test.characters, func(t *testing.T) {
			generator, generatorErr := hint.NewAlphabetGenerator(test.characters)
			if generatorErr != nil {
				t.Fatalf("NewAlphabetGenerator() error: %v", generatorErr)
			}

			got := generator.MaxHints()
			charCount := len(test.characters)
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

	context := context.Background()

	_, generateErr := generator.Generate(context, elements)
	if generateErr == nil {
		t.Error("Generate() expected error for too many elements, got nil")
	}
}

func TestNewCollection(t *testing.T) {
	element, _ := element.NewElement("test", image.Rect(0, 0, 50, 50), element.RoleButton)

	hints := []*hint.Hint{
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

	hints := []*hint.Hint{
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

	for _, test := range tests {
		t.Run(test.prefix, func(t *testing.T) {
			filtered := collection.FilterByPrefix(test.prefix)
			if len(filtered) != test.want {
				t.Errorf(
					"FilterByPrefix(%q) returned %d hints, want %d",
					test.prefix,
					len(filtered),
					test.want,
				)
			}
		})
	}
}

// Helper function for tests.
func mustNewHint(label string, elem *element.Element) *hint.Hint {
	hint, hintErr := hint.NewHint(label, elem, image.Point{})
	if hintErr != nil {
		panic(hintErr)
	}

	return hint
}

func BenchmarkAlphabetGenerator_Generate(b *testing.B) {
	generator, _ := hint.NewAlphabetGenerator("asdfghjkl")

	elements := make([]*element.Element, 50)
	for index := range elements {
		elements[index], _ = element.NewElement(
			element.ID("elem-"+string(rune('0'+index))),
			image.Rect(index*10, index*10, index*10+50, index*10+50),
			element.RoleButton,
		)
	}

	context := context.Background()

	for b.Loop() {
		_, _ = generator.Generate(context, elements)
	}
}

func BenchmarkCollection_FilterByPrefix(b *testing.B) {
	element, _ := element.NewElement("test", image.Rect(0, 0, 50, 50), element.RoleButton)

	hints := make([]*hint.Hint, 100)
	for index := range hints {
		label := string(rune('A'+index/26)) + string(rune('A'+index%26))
		hints[index] = mustNewHint(label, element)
	}

	collection := hint.NewCollection(hints)

	for b.Loop() {
		_ = collection.FilterByPrefix("A")
	}
}
