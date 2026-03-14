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

	t.Run("General Modifier Passthrough Defaults", func(t *testing.T) {
		if cfg.General.PassthroughUnboundedKeys {
			t.Error("Expected General.PassthroughUnboundedKeys to be false by default")
		}

		if cfg.General.ShouldExitAfterPassthrough {
			t.Error("Expected General.ShouldExitAfterPassthrough to be false by default")
		}

		if len(cfg.General.PassthroughUnboundedKeysBlacklist) != 0 {
			t.Errorf(
				"Expected General.PassthroughUnboundedKeysBlacklist to be empty by default, got %v",
				cfg.General.PassthroughUnboundedKeysBlacklist,
			)
		}
	})

	t.Run("Recursive Grid Defaults", func(t *testing.T) {
		if cfg.RecursiveGrid.UI.LabelBackground {
			t.Error("Expected RecursiveGrid.UI.LabelBackground to be false by default")
		}

		if cfg.RecursiveGrid.UI.LabelBackgroundColorLight != config.RecursiveGridLabelBackgroundColorLight {
			t.Errorf(
				"Expected RecursiveGrid.UI.LabelBackgroundColorLight %q, got %q",
				config.RecursiveGridLabelBackgroundColorLight,
				cfg.RecursiveGrid.UI.LabelBackgroundColorLight,
			)
		}

		if cfg.RecursiveGrid.UI.LabelBackgroundColorDark != config.RecursiveGridLabelBackgroundColorDark {
			t.Errorf(
				"Expected RecursiveGrid.UI.LabelBackgroundColorDark %q, got %q",
				config.RecursiveGridLabelBackgroundColorDark,
				cfg.RecursiveGrid.UI.LabelBackgroundColorDark,
			)
		}

		if cfg.RecursiveGrid.UI.LabelBackgroundPaddingX != config.DefaultRecursiveGridLabelBackgroundPaddingX {
			t.Errorf(
				"Expected RecursiveGrid.UI.LabelBackgroundPaddingX %d, got %d",
				config.DefaultRecursiveGridLabelBackgroundPaddingX,
				cfg.RecursiveGrid.UI.LabelBackgroundPaddingX,
			)
		}

		if cfg.RecursiveGrid.UI.LabelBackgroundPaddingY != config.DefaultRecursiveGridLabelBackgroundPaddingY {
			t.Errorf(
				"Expected RecursiveGrid.UI.LabelBackgroundPaddingY %d, got %d",
				config.DefaultRecursiveGridLabelBackgroundPaddingY,
				cfg.RecursiveGrid.UI.LabelBackgroundPaddingY,
			)
		}

		if cfg.RecursiveGrid.UI.LabelBackgroundBorderRadius != config.DefaultRecursiveGridLabelBackgroundBorderRadius {
			t.Errorf(
				"Expected RecursiveGrid.UI.LabelBackgroundBorderRadius %d, got %d",
				config.DefaultRecursiveGridLabelBackgroundBorderRadius,
				cfg.RecursiveGrid.UI.LabelBackgroundBorderRadius,
			)
		}

		if cfg.RecursiveGrid.UI.LabelBackgroundBorderWidth != config.DefaultRecursiveGridLabelBackgroundBorderWidth {
			t.Errorf(
				"Expected RecursiveGrid.UI.LabelBackgroundBorderWidth %d, got %d",
				config.DefaultRecursiveGridLabelBackgroundBorderWidth,
				cfg.RecursiveGrid.UI.LabelBackgroundBorderWidth,
			)
		}
	})
}
