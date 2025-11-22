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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, err := hint.NewHint(tt.label, tt.elem, tt.position)

			if tt.wantErr {
				if err == nil {
					t.Error("NewHint() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("NewHint() unexpected error: %v", err)
				return
			}

			if h.Label() != tt.label {
				t.Errorf("Label() = %v, want %v", h.Label(), tt.label)
			}

			if h.Position() != tt.position {
				t.Errorf("Position() = %v, want %v", h.Position(), tt.position)
			}
		})
	}
}

func TestHint_HasPrefix(t *testing.T) {
	elem, _ := element.NewElement("test", image.Rect(10, 10, 50, 50), element.RoleButton)
	h, _ := hint.NewHint("ASDF", elem, image.Point{})

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

	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			if got := h.HasPrefix(tt.prefix); got != tt.want {
				t.Errorf("HasPrefix(%q) = %v, want %v", tt.prefix, got, tt.want)
			}
		})
	}
}

func TestAlphabetGenerator_Generate(t *testing.T) {
	gen, err := hint.NewAlphabetGenerator("asdf")
	if err != nil {
		t.Fatalf("NewAlphabetGenerator() error: %v", err)
	}

	// Create test elements
	elements := make([]*element.Element, 10)
	for i := range elements {
		elements[i], _ = element.NewElement(
			element.ID("elem-"+string(rune('0'+i))),
			image.Rect(i*10, i*10, i*10+50, i*10+50),
			element.RoleButton,
		)
	}

	ctx := context.Background()
	hints, err := gen.Generate(ctx, elements)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if len(hints) != len(elements) {
		t.Errorf("Generate() returned %d hints, want %d", len(hints), len(elements))
	}

	// Check that all labels are unique
	seen := make(map[string]bool)
	for _, h := range hints {
		if seen[h.Label()] {
			t.Errorf("Duplicate label: %s", h.Label())
		}
		seen[h.Label()] = true
	}

	// Check that all labels are uppercase
	for _, h := range hints {
		label := h.Label()
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

	for _, tt := range tests {
		t.Run(tt.characters, func(t *testing.T) {
			gen, err := hint.NewAlphabetGenerator(tt.characters)
			if err != nil {
				t.Fatalf("NewAlphabetGenerator() error: %v", err)
			}

			got := gen.MaxHints()
			charCount := len(tt.characters)
			want := charCount * charCount * charCount

			if got != want {
				t.Errorf("MaxHints() = %d, want %d", got, want)
			}
		})
	}
}

func TestAlphabetGenerator_TooManyElements(t *testing.T) {
	gen, _ := hint.NewAlphabetGenerator("as") // Max 6 hints (2 + 2*2)

	// Try to generate 10 hints
	elements := make([]*element.Element, 10)
	for i := range elements {
		elements[i], _ = element.NewElement(
			element.ID("elem-"+string(rune('0'+i))),
			image.Rect(0, 0, 50, 50),
			element.RoleButton,
		)
	}

	ctx := context.Background()
	_, err := gen.Generate(ctx, elements)

	if err == nil {
		t.Error("Generate() expected error for too many elements, got nil")
	}
}

func TestNewCollection(t *testing.T) {
	elem, _ := element.NewElement("test", image.Rect(0, 0, 50, 50), element.RoleButton)

	hints := []*hint.Hint{
		mustNewHint("A", elem),
		mustNewHint("AS", elem),
		mustNewHint("AD", elem),
		mustNewHint("AF", elem),
		mustNewHint("S", elem),
	}

	collection := hint.NewCollection(hints)

	if collection.Count() != 5 {
		t.Errorf("Count() = %d, want 5", collection.Count())
	}

	// Test FindByLabel
	h := collection.FindByLabel("AS")
	if h == nil {
		t.Error("FindByLabel(\"AS\") returned nil")
	} else if h.Label() != "AS" {
		t.Errorf("FindByLabel(\"AS\") returned hint with label %q", h.Label())
	}

	// Test FilterByPrefix
	filtered := collection.FilterByPrefix("A")
	if len(filtered) != 4 { // A, AS, AD, AF
		t.Errorf("FilterByPrefix(\"A\") returned %d hints, want 4", len(filtered))
	}
}

func TestCollection_FilterByPrefix(t *testing.T) {
	elem, _ := element.NewElement("test", image.Rect(0, 0, 50, 50), element.RoleButton)

	hints := []*hint.Hint{
		mustNewHint("AA", elem),
		mustNewHint("AS", elem),
		mustNewHint("AD", elem),
		mustNewHint("SA", elem),
		mustNewHint("SS", elem),
		mustNewHint("A", elem),
		mustNewHint("S", elem),
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

	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			filtered := collection.FilterByPrefix(tt.prefix)
			if len(filtered) != tt.want {
				t.Errorf(
					"FilterByPrefix(%q) returned %d hints, want %d",
					tt.prefix,
					len(filtered),
					tt.want,
				)
			}
		})
	}
}

// Helper function for tests
func mustNewHint(label string, elem *element.Element) *hint.Hint {
	h, err := hint.NewHint(label, elem, image.Point{})
	if err != nil {
		panic(err)
	}
	return h
}

func BenchmarkAlphabetGenerator_Generate(b *testing.B) {
	gen, _ := hint.NewAlphabetGenerator("asdfghjkl")

	elements := make([]*element.Element, 50)
	for i := range elements {
		elements[i], _ = element.NewElement(
			element.ID("elem-"+string(rune('0'+i))),
			image.Rect(i*10, i*10, i*10+50, i*10+50),
			element.RoleButton,
		)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = gen.Generate(ctx, elements)
	}
}

func BenchmarkCollection_FilterByPrefix(b *testing.B) {
	elem, _ := element.NewElement("test", image.Rect(0, 0, 50, 50), element.RoleButton)

	hints := make([]*hint.Hint, 100)
	for i := range hints {
		label := string(rune('A'+i/26)) + string(rune('A'+i%26))
		hints[i] = mustNewHint(label, elem)
	}

	collection := hint.NewCollection(hints)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = collection.FilterByPrefix("A")
	}
}
