package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

func TestValidateHints_EnabledRequiresClickableRoles(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.Enabled = true
	cfg.Hints.ClickableRoles = nil

	err := cfg.ValidateHints()
	if err == nil {
		t.Fatal("ValidateHints() expected error when enabled and clickable_roles is empty")
	}
}

func TestValidateHints_BoundaryHighlightGeometry(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.BoundaryHighlight.BorderWidth = -1

	err := cfg.ValidateHints()
	if err == nil {
		t.Fatal("ValidateHints() expected error for negative boundary border width")
	}

	cfg = config.DefaultConfig()
	cfg.Hints.BoundaryHighlight.BorderRadius = -1

	err = cfg.ValidateHints()
	if err != nil {
		t.Fatalf("ValidateHints() expected no error for -1 (auto) border radius, got %v", err)
	}
}

func TestValidateHints_UIPlacement(t *testing.T) {
	validPlacements := []string{
		"top",
		"center",
		"bottom",
	}

	for _, placement := range validPlacements {
		cfg := config.DefaultConfig()
		cfg.Hints.UI.Placement = placement

		err := cfg.ValidateHints()
		if err != nil {
			t.Fatalf("ValidateHints() expected placement %q to be valid, got %v", placement, err)
		}
	}

	cfg := config.DefaultConfig()

	cfg.Hints.UI.Placement = "floating"

	err := cfg.ValidateHints()
	if err == nil {
		t.Fatal("ValidateHints() expected error for invalid hints.ui.placement")
	}
}

func TestValidateHints_PositiveUnitFloat(t *testing.T) {
	// merge_iou_threshold cannot be 0
	cfg := config.DefaultConfig()
	cfg.Hints.Vision.MergeIOUThreshold = 0.0

	err := cfg.ValidateHints()
	if err == nil {
		t.Fatal("ValidateHints() expected error for 0.0 merge_iou_threshold")
	}

	// button_min_confidence cannot be 0
	cfg = config.DefaultConfig()
	cfg.Hints.Vision.ButtonMinConfidence = 0.0

	err = cfg.ValidateHints()
	if err == nil {
		t.Fatal("ValidateHints() expected error for 0.0 button_min_confidence")
	}

	// generic_clickable_min_confidence cannot be 0
	cfg = config.DefaultConfig()
	cfg.Hints.Vision.GenericClickableMinConfidence = 0.0

	err = cfg.ValidateHints()
	if err == nil {
		t.Fatal("ValidateHints() expected error for 0.0 generic_clickable_min_confidence")
	}

	// but minimum_confidence CAN be 0
	cfg = config.DefaultConfig()
	cfg.Hints.Vision.MinimumConfidence = 0.0

	err = cfg.ValidateHints()
	if err != nil {
		t.Fatalf("ValidateHints() expected no error for 0.0 minimum_confidence, got %v", err)
	}
}
