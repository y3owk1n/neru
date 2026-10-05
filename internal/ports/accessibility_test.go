package ports_test

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/ports"
)

func TestDefaultElementFilter(t *testing.T) {
	filter := ports.DefaultElementFilter()

	// Check default values
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

	if filter.IncludeStageManager {
		t.Error("Expected IncludeStageManager to be false by default")
	}

	if filter.IncludePIP {
		t.Error("Expected IncludePIP to be false by default")
	}

	if filter.IncludeScreenCapture {
		t.Error("Expected IncludeScreenCapture to be false by default")
	}

	if filter.Roles != nil {
		t.Error("Expected Roles to be nil by default")
	}

	if filter.ExcludeRoles != nil {
		t.Error("Expected ExcludeRoles to be nil by default")
	}
}
