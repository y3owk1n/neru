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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elem, err := element.NewElement(tt.id, tt.bounds, tt.role, tt.opts...)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewElement() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("NewElement() unexpected error: %v", err)
				return
			}

			if elem.ID() != tt.id {
				t.Errorf("ID() = %v, want %v", elem.ID(), tt.id)
			}

			if elem.Bounds() != tt.bounds {
				t.Errorf("Bounds() = %v, want %v", elem.Bounds(), tt.bounds)
			}

			if elem.Role() != tt.role {
				t.Errorf("Role() = %v, want %v", elem.Role(), tt.role)
			}
		})
	}
}

func TestElement_Center(t *testing.T) {
	elem, err := element.NewElement(
		"test",
		image.Rect(10, 20, 110, 70),
		element.RoleButton,
	)
	if err != nil {
		t.Fatalf("NewElement() error: %v", err)
	}

	center := elem.Center()
	want := image.Point{X: 60, Y: 45}

	if center != want {
		t.Errorf("Center() = %v, want %v", center, want)
	}
}

func TestElement_Contains(t *testing.T) {
	elem, err := element.NewElement(
		"test",
		image.Rect(10, 10, 100, 50),
		element.RoleButton,
	)
	if err != nil {
		t.Fatalf("NewElement() error: %v", err)
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := elem.Contains(tt.point); got != tt.want {
				t.Errorf("Contains(%v) = %v, want %v", tt.point, got, tt.want)
			}
		})
	}
}

func TestElement_Overlaps(t *testing.T) {
	elem1, _ := element.NewElement("elem1", image.Rect(10, 10, 50, 50), element.RoleButton)

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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elem2, _ := element.NewElement("elem2", tt.bounds, element.RoleButton)
			if got := elem1.Overlaps(elem2); got != tt.want {
				t.Errorf("Overlaps() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestElement_IsVisible(t *testing.T) {
	elem, _ := element.NewElement("test", image.Rect(10, 10, 50, 50), element.RoleButton)

	tests := []struct {
		name         string
		screenBounds image.Rectangle
		want         bool
	}{
		{"fully visible", image.Rect(0, 0, 100, 100), true},
		{"partially visible", image.Rect(30, 30, 100, 100), true},
		{"completely off screen", image.Rect(60, 60, 100, 100), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := elem.IsVisible(tt.screenBounds); got != tt.want {
				t.Errorf("IsVisible(%v) = %v, want %v", tt.screenBounds, got, tt.want)
			}
		})
	}
}

func TestElement_Immutability(t *testing.T) {
	// Test that elements are immutable
	elem, _ := element.NewElement(
		"test",
		image.Rect(10, 10, 50, 50),
		element.RoleButton,
		element.WithClickable(true),
		element.WithTitle("Original"),
	)

	// Get values
	originalID := elem.ID()
	originalBounds := elem.Bounds()
	originalTitle := elem.Title()

	// Modify returned values (should not affect element)
	originalBounds.Min.X = 999

	// Verify element unchanged
	if elem.Bounds().Min.X == 999 {
		t.Error("Element bounds were modified - not immutable!")
	}

	if elem.ID() != originalID {
		t.Error("Element ID changed")
	}

	if elem.Title() != originalTitle {
		t.Error("Element title changed")
	}
}
