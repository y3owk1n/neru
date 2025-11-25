package hint_test

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/domain/hint"
	"github.com/y3owk1n/neru/internal/infra/logger"
)

func TestManager_Filtering(t *testing.T) {
	// Setup hints
	element, _ := element.NewElement(element.ID("1"), image.Rect(0, 0, 10, 10), element.RoleButton)
	h1, _ := hint.NewHint("AA", element, image.Point{0, 0})
	h2, _ := hint.NewHint("AB", element, image.Point{0, 0})
	h3, _ := hint.NewHint("AC", element, image.Point{0, 0})

	collection := hint.NewCollection([]*hint.Interface{h1, h2, h3})
	manager := hint.NewManager(logger.Get())
	manager.SetHints(collection)

	tests := []struct {
		name        string
		input       string
		wantCount   int
		wantMatched string // Label of the exact match if any
	}{
		{"empty input", "", 3, ""},
		{"partial match A", "A", 3, ""},
		{"exact match AA", "AA", 1, "AA"},
		{"exact match AB", "AB", 1, "AB"},
		{"no match AD", "AD", 3, ""},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			manager.Reset()

			var (
				match *hint.Interface
				found bool
			)

			for _, char := range testCase.input {
				match, found = manager.HandleInput(string(char))
			}

			filtered := manager.GetFilteredHints()
			if len(filtered) != testCase.wantCount {
				t.Errorf(
					"GetFilteredHints() count = %d, want %d",
					len(filtered),
					testCase.wantCount,
				)
			}

			if testCase.wantMatched != "" {
				if !found || match == nil {
					t.Errorf("Expected exact match for %s, got nil", testCase.wantMatched)
				} else if match.Label() != testCase.wantMatched {
					t.Errorf("Expected exact match %s, got %s", testCase.wantMatched, match.Label())
				}
			} else if found {
				t.Errorf("Expected no exact match, got %s", match.Label())
			}
		})
	}
}

func TestManager_Backspace(t *testing.T) {
	element, _ := element.NewElement(element.ID("1"), image.Rect(0, 0, 10, 10), element.RoleButton)
	h1, _ := hint.NewHint("AA", element, image.Point{0, 0})
	collection := hint.NewCollection([]*hint.Interface{h1})
	manager := hint.NewManager(logger.Get())
	manager.SetHints(collection)

	// Type 'A'
	manager.HandleInput("A")

	if len(manager.GetFilteredHints()) != 1 {
		t.Error("Expected 1 hint after 'A'")
	}

	// Backspace
	manager.HandleInput("backspace")

	if len(manager.GetFilteredHints()) != 1 {
		t.Error("Expected 1 hint after Backspace")
	}

	if manager.GetInput() != "" {
		t.Errorf("Expected empty input, got %q", manager.GetInput())
	}
}
