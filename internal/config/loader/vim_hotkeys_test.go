package loader_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

// TestLoad_ReadsTheVimHotkeysFromTheRawTable pins the loader's half of
// [vim.hotkeys]. The table is tagged toml:"-" like every mode's, so it has to
// be read from the raw decode, and the per-app table beside it too.
func TestLoad_ReadsTheVimHotkeysFromTheRawTable(t *testing.T) {
	t.Parallel()

	result := loadWithOverride(t, `
[vim.hotkeys]
"Ctrl+c" = "idle"

[[vim.app_configs]]
bundle_id = "com.apple.Safari"
capture_scope = "screen"
hotkeys = { "Ctrl+h" = "hints" }
`, "")
	cfg := result.Config

	if got := cfg.Vim.Hotkeys["Ctrl+c"]; len(got) != 1 || got[0] != config.CmdIdle {
		t.Errorf("vim.hotkeys Ctrl+c = %v, want idle", got)
	}

	if len(cfg.Vim.AppConfigs) != 1 {
		t.Fatalf("vim.app_configs = %v, want one entry", cfg.Vim.AppConfigs)
	}

	if got := cfg.Vim.CaptureScopeForApp("com.apple.Safari"); got != "screen" {
		t.Errorf("Safari's capture scope = %q, want screen", got)
	}

	merged := cfg.HotkeysForModeAndApp(config.ModeNameVim, "com.apple.Safari")
	if got := merged["Ctrl+h"]; len(got) != 1 || got[0] != "hints" {
		t.Errorf("Safari's Ctrl+h = %v, want hints", got)
	}
}
