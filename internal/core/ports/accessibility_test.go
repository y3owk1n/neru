//go:build unit

package ports_test

import (
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
