//go:build !integration

package element_test

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/core/domain/element"
)

func TestNewElement(t *testing.T) {
	tests := []struct {
		name    string
		id      element.ID
		bounds  image.Rectangle
		role    element.Role
		opts    []element.Option
		wantErr bool
	}{
		{
			name:    "valid element",
			id:      "test-1",
			bounds:  image.Rect(10, 10, 100, 50),
			role:    element.RoleButton,
			opts:    []element.Option{element.WithClickable(true)},
			wantErr: false,
		},
		{
			name:    "empty ID",
			id:      "",
			bounds:  image.Rect(10, 10, 100, 50),
			role:    element.RoleButton,
			wantErr: true,
		},
		{
			name:    "empty bounds",
			id:      "test-2",
			bounds:  image.Rectangle{},
			role:    element.RoleButton,
			wantErr: true,
		},
		{
			name:   "with title and description",
			id:     "test-3",
			bounds: image.Rect(0, 0, 50, 30),
			role:   element.RoleLink,
			opts: []element.Option{
				element.WithTitle("Click me"),
				element.WithDescription("A clickable link"),
				element.WithClickable(true),
			},
			wantErr: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			element, elementErr := element.NewElement(
				testCase.id,
				testCase.bounds,
				testCase.role,
				testCase.opts...)

			if testCase.wantErr {
				if elementErr == nil {
					t.Errorf("NewElement() expected error, got nil")
				}

				return
			}

			if elementErr != nil {
				t.Errorf("NewElement() unexpected error: %v", elementErr)

				return
			}

			if element.ID() != testCase.id {
				t.Errorf("ID() = %v, want %v", element.ID(), testCase.id)
			}

			if element.Bounds() != testCase.bounds {
				t.Errorf("Bounds() = %v, want %v", element.Bounds(), testCase.bounds)
			}

			if element.Role() != testCase.role {
				t.Errorf("Role() = %v, want %v", element.Role(), testCase.role)
			}
		})
	}
}

func TestElement_Center(t *testing.T) {
	element, elementErr := element.NewElement(
		"test",
		image.Rect(10, 20, 110, 70),
		element.RoleButton,
	)
	if elementErr != nil {
		t.Fatalf("NewElement() error: %v", elementErr)
	}

	center := element.Center()
	want := image.Point{X: 60, Y: 45}

	if center != want {
		t.Errorf("Center() = %v, want %v", center, want)
	}
}

func TestElement_Contains(t *testing.T) {
	element, elementErr := element.NewElement(
		"test",
		image.Rect(10, 10, 100, 50),
		element.RoleButton,
	)
	if elementErr != nil {
		t.Fatalf("NewElement() error: %v", elementErr)
	}

	tests := []struct {
		name  string
		point image.Point
		want  bool
	}{
		{"inside", image.Point{X: 50, Y: 30}, true},
		{"on edge", image.Point{X: 10, Y: 10}, true},
		{"outside left", image.Point{X: 5, Y: 30}, false},
		{"outside right", image.Point{X: 105, Y: 30}, false},
		{"outside top", image.Point{X: 50, Y: 5}, false},
		{"outside bottom", image.Point{X: 50, Y: 55}, false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := element.Contains(testCase.point)
			if got != testCase.want {
				t.Errorf("Contains(%v) = %v, want %v", testCase.point, got, testCase.want)
			}
		})
	}
}

func TestElement_Overlaps(t *testing.T) {
	elementA, _ := element.NewElement("elem1", image.Rect(10, 10, 50, 50), element.RoleButton)

	tests := []struct {
		name   string
		bounds image.Rectangle
		want   bool
	}{
		{"completely overlapping", image.Rect(20, 20, 40, 40), true},
		{"partially overlapping", image.Rect(40, 40, 80, 80), true},
		{"touching edge", image.Rect(50, 10, 90, 50), false}, // Touching edges don't overlap
		{"completely separate", image.Rect(60, 60, 100, 100), false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			elementB, _ := element.NewElement("elem2", testCase.bounds, element.RoleButton)

			got := elementA.Overlaps(elementB)
			if got != testCase.want {
				t.Errorf("Overlaps() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestElement_IsVisible(t *testing.T) {
	element, _ := element.NewElement("test", image.Rect(10, 10, 50, 50), element.RoleButton)

	tests := []struct {
		name         string
		screenBounds image.Rectangle
		want         bool
	}{
		{"fully visible", image.Rect(0, 0, 100, 100), true},
		{"partially visible", image.Rect(30, 30, 100, 100), true},
		{"completely off screen", image.Rect(60, 60, 100, 100), false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := element.IsVisible(testCase.screenBounds)
			if got != testCase.want {
				t.Errorf("IsVisible(%v) = %v, want %v", testCase.screenBounds, got, testCase.want)
			}
		})
	}
}

func TestElement_Options(t *testing.T) {
	tests := []struct {
		name          string
		opts          []element.Option
		wantClickable bool
		wantTitle     string
		wantDesc      string
	}{
		{
			name:          "no options",
			opts:          nil,
			wantClickable: false,
			wantTitle:     "",
			wantDesc:      "",
		},
		{
			name:          "with clickable",
			opts:          []element.Option{element.WithClickable(true)},
			wantClickable: true,
			wantTitle:     "",
			wantDesc:      "",
		},
		{
			name:          "with title",
			opts:          []element.Option{element.WithTitle("Test Button")},
			wantClickable: false,
			wantTitle:     "Test Button",
			wantDesc:      "",
		},
		{
			name:          "with description",
			opts:          []element.Option{element.WithDescription("A test element")},
			wantClickable: false,
			wantTitle:     "",
			wantDesc:      "A test element",
		},
		{
			name: "with all options",
			opts: []element.Option{
				element.WithClickable(true),
				element.WithTitle("Click Me"),
				element.WithDescription("Clickable button"),
			},
			wantClickable: true,
			wantTitle:     "Click Me",
			wantDesc:      "Clickable button",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			elem, err := element.NewElement(
				"test",
				image.Rect(0, 0, 10, 10),
				element.RoleButton,
				testCase.opts...)
			if err != nil {
				t.Fatalf("NewElement() error: %v", err)
			}

			if elem.IsClickable() != testCase.wantClickable {
				t.Errorf("IsClickable() = %v, want %v", elem.IsClickable(), testCase.wantClickable)
			}

			if elem.Title() != testCase.wantTitle {
				t.Errorf("Title() = %q, want %q", elem.Title(), testCase.wantTitle)
			}

			if elem.Description() != testCase.wantDesc {
				t.Errorf("Description() = %q, want %q", elem.Description(), testCase.wantDesc)
			}
		})
	}
}

func TestElement_Immutability(t *testing.T) {
	// Test that elements are immutable
	element, _ := element.NewElement(
		"test",
		image.Rect(10, 10, 50, 50),
		element.RoleButton,
		element.WithClickable(true),
		element.WithTitle("Original"),
	)

	// Get values
	originalID := element.ID()
	originalBounds := element.Bounds()
	originalTitle := element.Title()

	// Modify returned values (should not affect element)
	originalBounds.Min.X = 999

	// Verify element unchanged
	if element.Bounds().Min.X == 999 {
		t.Error("Element bounds were modified - not immutable!")
	}

	if element.ID() != originalID {
		t.Error("Element ID changed")
	}

	if element.Title() != originalTitle {
		t.Error("Element title changed")
	}
}
