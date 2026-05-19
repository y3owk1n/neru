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

func TestValidateHints_MaxParallelDepth_WithinCap(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.MaxParallelDepth = 50

	err := cfg.ValidateHints()
	if err != nil {
		t.Fatalf("ValidateHints() expected no error for max_parallel_depth=50, got %v", err)
	}
}

func TestValidateHints_MaxParallelDepth_ExceedsCap(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.MaxParallelDepth = 51

	err := cfg.ValidateHints()
	if err == nil {
		t.Fatal("ValidateHints() expected error for max_parallel_depth=51 exceeding cap")
	}
}

func TestValidateHints_MaxParallelDepth_ExceedsMaxDepth(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.MaxDepth = 10
	cfg.Hints.MaxParallelDepth = 15

	err := cfg.ValidateHints()
	if err == nil {
		t.Fatal("ValidateHints() expected error for max_parallel_depth=15 exceeding max_depth=10")
	}
}

func TestValidateHints_MaxParallelDepth_ZeroMaxDepth(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.MaxDepth = 0
	cfg.Hints.MaxParallelDepth = 40

	err := cfg.ValidateHints()
	if err != nil {
		t.Fatalf(
			"ValidateHints() expected no error for max_parallel_depth=40 with unlimited max_depth, got %v",
			err,
		)
	}
}

func TestValidateHints_MaxParallelDepth_DefaultValid(t *testing.T) {
	cfg := config.DefaultConfig()

	err := cfg.ValidateHints()
	if err != nil {
		t.Fatalf(
			"ValidateHints() expected no error for default config (max_parallel_depth=10, max_depth=50), got %v",
			err,
		)
	}
}
