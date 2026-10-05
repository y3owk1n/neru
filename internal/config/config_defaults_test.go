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
		if runtime.GOOS != goosWindows {
			t.Skip("only the Windows shell differs from the shared defaults table")
		}

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
	})
}
