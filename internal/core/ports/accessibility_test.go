//go:build unit

package ports_test

import (
	"context"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/core/domain/element"
	"github.com/y3owk1n/neru/internal/core/ports"
)

func TestDefaultElementFilter(t *testing.T) {
	filter := ports.DefaultElementFilter()

	// Check default values
	if filter.IncludeOffscreen {
		t.Error("Expected IncludeOffscreen to be false by default")
	}

	expectedMinSize := image.Point{X: 1, Y: 1}
	if filter.MinSize != expectedMinSize {
		t.Errorf("Expected MinSize to be %v, got %v", expectedMinSize, filter.MinSize)
	}

	if filter.IncludeMenubar {
		t.Error("Expected IncludeMenubar to be false by default")
	}

	if len(filter.AdditionalMenubarTargets) != 0 {
		t.Errorf(
			"Expected AdditionalMenubarTargets to be empty, got %v",
			filter.AdditionalMenubarTargets,
		)
	}

	if filter.IncludeDock {
		t.Error("Expected IncludeDock to be false by default")
	}

	if filter.IncludeNotificationCenter {
		t.Error("Expected IncludeNotificationCenter to be false by default")
	}

	// Check that slices are initialized
	if filter.Roles != nil {
		t.Error("Expected Roles to be nil by default")
	}

	if filter.ExcludeRoles != nil {
		t.Error("Expected ExcludeRoles to be nil by default")
	}
}

func TestElementFilterStruct(t *testing.T) {
	// Test that we can create and modify an ElementFilter
	filter := ports.ElementFilter{
		Roles:                     []element.Role{element.RoleButton},
		IncludeOffscreen:          true,
		MinSize:                   image.Point{X: 10, Y: 10},
		ExcludeRoles:              []element.Role{element.RoleStaticText},
		IncludeMenubar:            true,
		AdditionalMenubarTargets:  []string{"com.example.app"},
		IncludeDock:               true,
		IncludeNotificationCenter: true,
	}

	if len(filter.Roles) != 1 || filter.Roles[0] != element.RoleButton {
		t.Errorf("Expected Roles to contain button role, got %v", filter.Roles)
	}

	if !filter.IncludeOffscreen {
		t.Error("Expected IncludeOffscreen to be true")
	}

	if filter.MinSize.X != 10 || filter.MinSize.Y != 10 {
		t.Errorf("Expected MinSize to be {10, 10}, got %v", filter.MinSize)
	}

	if len(filter.ExcludeRoles) != 1 || filter.ExcludeRoles[0] != element.RoleStaticText {
		t.Errorf("Expected ExcludeRoles to contain static text role, got %v", filter.ExcludeRoles)
	}

	if !filter.IncludeMenubar {
		t.Error("Expected IncludeMenubar to be true")
	}

	if len(filter.AdditionalMenubarTargets) != 1 ||
		filter.AdditionalMenubarTargets[0] != "com.example.app" {
		t.Errorf(
			"Expected AdditionalMenubarTargets to contain example app, got %v",
			filter.AdditionalMenubarTargets,
		)
	}

	if !filter.IncludeDock {
		t.Error("Expected IncludeDock to be true")
	}

	if !filter.IncludeNotificationCenter {
		t.Error("Expected IncludeNotificationCenter to be true")
	}
}

// TestInterfaceSegregation demonstrates how the segregated interfaces can be used independently.
// This test shows that consumers can depend on specific functionality rather than the entire port.
func TestInterfaceSegregation(t *testing.T) {
	// Create a mock that only implements ElementDiscovery
	elementDiscovery := &mockElementDiscovery{}

	// This function only needs element discovery, not the full accessibility port
	findClickableElements := func(discovery ports.ElementDiscovery) error {
		elements, err := discovery.ClickableElements(
			context.Background(),
			ports.DefaultElementFilter(),
		)
		if err != nil {
			return err
		}

		if len(elements) == 0 {
			t.Error("Expected to find some elements")
		}

		return nil
	}

	// Test that we can use just the ElementDiscovery interface
	err := findClickableElements(elementDiscovery)
	if err != nil {
		t.Errorf("Element discovery failed: %v", err)
	}

	// Create a mock that only implements ScreenManagement
	screenManager := &mockScreenManager{}

	// This function only needs screen management, not the full accessibility port
	getScreenInfo := func(manager ports.ScreenManagement) error {
		bounds, err := manager.ScreenBounds(context.Background())
		if err != nil {
			return err
		}

		if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
			t.Error("Expected valid screen bounds")
		}

		return nil
	}

	// Test that we can use just the ScreenManagement interface
	err = getScreenInfo(screenManager)
	if err != nil {
		t.Errorf("Screen management failed: %v", err)
	}
}

// mockElementDiscovery implements only the ElementDiscovery interface.
type mockElementDiscovery struct{}

func (m *mockElementDiscovery) ClickableElements(
	ctx context.Context,
	filter ports.ElementFilter,
) ([]*element.Element, error) {
	// Return a mock element
	elem, err := element.NewElement(
		element.ID("test-element"),
		image.Rect(100, 100, 150, 120), // bounds
		element.RoleButton,
		element.WithClickable(true),
		element.WithTitle("Test Button"),
	)
	if err != nil {
		return nil, err
	}

	return []*element.Element{elem}, nil
}

// mockScreenManager implements only the ScreenManagement interface.
type mockScreenManager struct{}

func (m *mockScreenManager) ScreenBounds(ctx context.Context) (image.Rectangle, error) {
	return image.Rect(0, 0, 1920, 1080), nil
}

func (m *mockScreenManager) MoveCursorToPoint(ctx context.Context, point image.Point) error {
	return nil
}

func (m *mockScreenManager) CursorPosition(ctx context.Context) (image.Point, error) {
	return image.Point{X: 960, Y: 540}, nil
}
