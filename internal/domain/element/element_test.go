package element_test

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/element"
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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			element, elementErr := element.NewElement(test.id, test.bounds, test.role, test.opts...)

			if test.wantErr {
				if elementErr == nil {
					t.Errorf("NewElement() expected error, got nil")
				}

				return
			}

			if elementErr != nil {
				t.Errorf("NewElement() unexpected error: %v", elementErr)

				return
			}

			if element.ID() != test.id {
				t.Errorf("ID() = %v, want %v", element.ID(), test.id)
			}

			if element.Bounds() != test.bounds {
				t.Errorf("Bounds() = %v, want %v", element.Bounds(), test.bounds)
			}

			if element.Role() != test.role {
				t.Errorf("Role() = %v, want %v", element.Role(), test.role)
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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := element.Contains(test.point)
			if got != test.want {
				t.Errorf("Contains(%v) = %v, want %v", test.point, got, test.want)
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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			elementB, _ := element.NewElement("elem2", test.bounds, element.RoleButton)

			got := elementA.Overlaps(elementB)
			if got != test.want {
				t.Errorf("Overlaps() = %v, want %v", got, test.want)
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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := element.IsVisible(test.screenBounds)
			if got != test.want {
				t.Errorf("IsVisible(%v) = %v, want %v", test.screenBounds, got, test.want)
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
