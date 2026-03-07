package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	t.Run("Systray Defaults", func(t *testing.T) {
		if !cfg.Systray.Enabled {
			t.Error("Expected Systray.Enabled to be true by default")
		}
	})

	t.Run("General Keyboard Layout Defaults", func(t *testing.T) {
		if cfg.General.KBLayoutToUse != "" {
			t.Errorf(
				"Expected General.KBLayoutToUse to be empty by default, got %q",
				cfg.General.KBLayoutToUse,
			)
		}
	})

	t.Run("Recursive Grid Defaults", func(t *testing.T) {
		if cfg.RecursiveGrid.LabelBackground {
			t.Error("Expected RecursiveGrid.LabelBackground to be false by default")
		}

		if cfg.RecursiveGrid.LabelBackgroundColorLight != config.RecursiveGridLabelBackgroundColorLight {
			t.Errorf(
				"Expected RecursiveGrid.LabelBackgroundColorLight %q, got %q",
				config.RecursiveGridLabelBackgroundColorLight,
				cfg.RecursiveGrid.LabelBackgroundColorLight,
			)
		}

		if cfg.RecursiveGrid.LabelBackgroundColorDark != config.RecursiveGridLabelBackgroundColorDark {
			t.Errorf(
				"Expected RecursiveGrid.LabelBackgroundColorDark %q, got %q",
				config.RecursiveGridLabelBackgroundColorDark,
				cfg.RecursiveGrid.LabelBackgroundColorDark,
			)
		}

		if cfg.RecursiveGrid.LabelBackgroundPaddingX != config.DefaultRecursiveGridLabelBackgroundPaddingX {
			t.Errorf(
				"Expected RecursiveGrid.LabelBackgroundPaddingX %d, got %d",
				config.DefaultRecursiveGridLabelBackgroundPaddingX,
				cfg.RecursiveGrid.LabelBackgroundPaddingX,
			)
		}

		if cfg.RecursiveGrid.LabelBackgroundPaddingY != config.DefaultRecursiveGridLabelBackgroundPaddingY {
			t.Errorf(
				"Expected RecursiveGrid.LabelBackgroundPaddingY %d, got %d",
				config.DefaultRecursiveGridLabelBackgroundPaddingY,
				cfg.RecursiveGrid.LabelBackgroundPaddingY,
			)
		}

		if cfg.RecursiveGrid.LabelBackgroundCornerRadius != config.DefaultRecursiveGridLabelBackgroundCornerRadius {
			t.Errorf(
				"Expected RecursiveGrid.LabelBackgroundCornerRadius %d, got %d",
				config.DefaultRecursiveGridLabelBackgroundCornerRadius,
				cfg.RecursiveGrid.LabelBackgroundCornerRadius,
			)
		}

		if cfg.RecursiveGrid.LabelBackgroundBorderWidth != config.DefaultRecursiveGridLabelBackgroundBorderWidth {
			t.Errorf(
				"Expected RecursiveGrid.LabelBackgroundBorderWidth %d, got %d",
				config.DefaultRecursiveGridLabelBackgroundBorderWidth,
				cfg.RecursiveGrid.LabelBackgroundBorderWidth,
			)
		}
	})
}
