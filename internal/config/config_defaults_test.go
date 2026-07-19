package config_test

import (
	"path/filepath"
	"runtime"
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

	t.Run("General Exec Shell Defaults", func(t *testing.T) {
		if runtime.GOOS == goosWindows {
			if !filepath.IsAbs(cfg.General.ExecShell) {
				t.Errorf(
					"Expected General.ExecShell to be an absolute path, got %q",
					cfg.General.ExecShell,
				)
			}

			if len(cfg.General.ExecShellArgs) != 1 ||
				cfg.General.ExecShellArgs[0] != "/c" {
				t.Errorf(
					"Expected General.ExecShellArgs to be [%q], got %v",
					"/c",
					cfg.General.ExecShellArgs,
				)
			}
		} else {
			if cfg.General.ExecShell != config.DefaultExecShell {
				t.Errorf(
					"Expected General.ExecShell to be %q, got %q",
					config.DefaultExecShell,
					cfg.General.ExecShell,
				)
			}

			if len(cfg.General.ExecShellArgs) != 1 ||
				cfg.General.ExecShellArgs[0] != config.DefaultExecShellFlag {
				t.Errorf(
					"Expected General.ExecShellArgs to be [%q], got %v",
					config.DefaultExecShellFlag,
					cfg.General.ExecShellArgs,
				)
			}
		}
	})

	t.Run("Grid Defaults", func(t *testing.T) {
		if got := cfg.Grid.Hotkeys["`"]; len(got) != 1 ||
			got[0] != config.CmdToggleCursorFollowSelection {
			t.Fatalf("Expected Grid hotkey ` to toggle cursor-follow-selection, got %v", got)
		}
	})

	t.Run("Recursive Grid Defaults", func(t *testing.T) {
		if got := cfg.RecursiveGrid.Hotkeys["`"]; len(got) != 1 ||
			got[0] != config.CmdToggleCursorFollowSelection {
			t.Fatalf(
				"Expected RecursiveGrid hotkey ` to toggle cursor-follow-selection, got %v",
				got,
			)
		}

		if cfg.RecursiveGrid.UI.LabelBackground {
			t.Error("Expected RecursiveGrid.UI.LabelBackground to be false by default")
		}

		if cfg.RecursiveGrid.UI.LabelBackgroundColor.Light != config.RecursiveGridLabelBackgroundColorLight {
			t.Errorf(
				"Expected RecursiveGrid.UI.LabelBackgroundColor.Light %q, got %q",
				config.RecursiveGridLabelBackgroundColorLight,
				cfg.RecursiveGrid.UI.LabelBackgroundColor.Light,
			)
		}

		if cfg.RecursiveGrid.UI.LabelBackgroundColor.Dark != config.RecursiveGridLabelBackgroundColorDark {
			t.Errorf(
				"Expected RecursiveGrid.UI.LabelBackgroundColor.Dark %q, got %q",
				config.RecursiveGridLabelBackgroundColorDark,
				cfg.RecursiveGrid.UI.LabelBackgroundColor.Dark,
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

	t.Run("Virtual Pointer Defaults", func(t *testing.T) {
		if cfg.VirtualPointer.UI.Char != config.DefaultVirtualPointerChar {
			t.Errorf(
				"Expected VirtualPointer.UI.Char %q, got %q",
				config.DefaultVirtualPointerChar,
				cfg.VirtualPointer.UI.Char,
			)
		}

		if cfg.VirtualPointer.UI.TextColor.Light != config.VirtualPointerTextColorLight {
			t.Errorf(
				"Expected VirtualPointer.UI.TextColor.Light %q, got %q",
				config.VirtualPointerTextColorLight,
				cfg.VirtualPointer.UI.TextColor.Light,
			)
		}

		if cfg.VirtualPointer.UI.TextColor.Dark != config.VirtualPointerTextColorDark {
			t.Errorf(
				"Expected VirtualPointer.UI.TextColor.Dark %q, got %q",
				config.VirtualPointerTextColorDark,
				cfg.VirtualPointer.UI.TextColor.Dark,
			)
		}
	})
}
