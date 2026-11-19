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

func TestElement_Text_PrefersValueThenTitleThenDescription(t *testing.T) {
	tests := []struct {
		name string
		opts []element.Option
		want string
	}{
		{
			"a field's value wins over its label",
			[]element.Option{
				element.WithTitle("Name"),
				element.WithValue("Ada"),
				element.WithDescription("Your name"),
			},
			"Ada",
		},
		{
			"a button is named by its title",
			[]element.Option{element.WithTitle("Save"), element.WithDescription("Saves the file")},
			"Save",
		},
		{
			"an icon button may have only a description",
			[]element.Option{element.WithDescription("Back")},
			"Back",
		},
		{"an element with no text", nil, ""},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			elem, err := element.NewElement(
				"id",
				image.Rect(0, 0, 10, 10),
				element.RoleButton,
				testCase.opts...)
			if err != nil {
				t.Fatalf("NewElement() = %v", err)
			}

			if got := elem.Text(); got != testCase.want {
				t.Fatalf("Text() = %q, want %q", got, testCase.want)
			}
		})
	}
}
