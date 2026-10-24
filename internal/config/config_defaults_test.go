package config_test

import (
	"slices"
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
		if cfg.General.KBLayoutToUse != (config.PerPlatform[string]{}) {
			t.Errorf(
				"Expected General.KBLayoutToUse to be empty by default, got %+v",
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

	t.Run("General Shell Defaults", func(t *testing.T) {
		for goos, want := range map[string][]string{
			"darwin":      {TestShellBash, TestShellFlag},
			TestGOOSLinux: {TestShellBash, TestShellFlag},
			goosWindows:   {TestWindowsCmd, "/c"},
		} {
			if got := cfg.General.ShellCommand(goos); !slices.Equal(got, want) {
				t.Errorf("ShellCommand(%q) = %q, want %q", goos, got, want)
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
	})
}
