package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
)

const (
	// vimExitChord is a chord vim does not use, bound to leave the mode.
	vimExitChord = "Ctrl+c"
	// vimPlainApp is an app config that sets no capture scope.
	vimPlainApp = "com.example.VimPlain"
)

// TestConfig_ValidateVim_RejectsBadAppConfigs pins what [[vim.app_configs]]
// takes: a capture scope from the vocabulary and hotkeys, and no field that
// belongs to hints or scroll.
func TestConfig_ValidateVim_RejectsBadAppConfigs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(cfg *config.Config)
	}{
		{"unknown capture scope", func(cfg *config.Config) {
			cfg.Vim.AppConfigs = []config.AppConfig{
				{BundleID: bundleExample, CaptureScope: unknownScope},
			}
		}},
		{"scroll field", func(cfg *config.Config) {
			step := 5
			cfg.Vim.AppConfigs = []config.AppConfig{{BundleID: bundleExample, ScrollStep: &step}}
		}},
		{"hints field", func(cfg *config.Config) {
			cfg.Vim.AppConfigs = []config.AppConfig{
				{BundleID: bundleExample, AdditionalClickable: []string{"button"}},
			}
		}},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := config.DefaultConfig()
			testCase.mutate(cfg)

			err := cfg.ValidateVim()
			if err == nil {
				t.Fatal("ValidateVim() accepted an invalid app config")
			}
		})
	}

	cfg := config.DefaultConfig()
	cfg.Vim.AppConfigs = []config.AppConfig{{
		BundleID:     bundleExample,
		CaptureScope: domain.CaptureScopeScreen,
		Hotkeys:      map[string]config.StringOrStringArray{vimExitChord: {config.CmdIdle}},
	}}

	err := cfg.ValidateVim()
	if err != nil {
		t.Fatalf("ValidateVim() rejected a capture scope and hotkeys: %v", err)
	}
}

// TestConfig_HotkeysForModeAndApp_ReadsVim pins the table a vim keystroke
// settles against. It ships empty, so every key is vim's, and the per-app
// table merges over the section's under the rules every mode follows.
func TestConfig_HotkeysForModeAndApp_ReadsVim(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()

	if got := cfg.HotkeysForModeAndApp(config.ModeNameVim, ""); len(got) != 0 {
		t.Fatalf("default vim hotkeys = %v, want none, since every key is vim's", got)
	}

	cfg.Vim.Hotkeys = map[string]config.StringOrStringArray{vimExitChord: {config.CmdIdle}}
	cfg.Vim.AppConfigs = []config.AppConfig{{
		BundleID: keymapTestApp,
		Hotkeys: map[string]config.StringOrStringArray{
			"Ctrl+h":     {config.ModeNameHints},
			vimExitChord: {config.DisabledSentinel},
		},
	}}

	if got := cfg.HotkeysForModeAndApp(config.ModeNameVim, ""); len(got[vimExitChord]) != 1 {
		t.Errorf("base Ctrl+c = %v, want the idle binding", got[vimExitChord])
	}

	overridden := cfg.HotkeysForModeAndApp(config.ModeNameVim, "com.apple.safari")
	if got := overridden["Ctrl+h"]; len(got) != 1 || got[0] != config.ModeNameHints {
		t.Errorf("overridden Ctrl+h = %v, want the per-app step", got)
	}

	if _, bound := overridden[vimExitChord]; bound {
		t.Error("overridden Ctrl+c is still bound; want __disabled__ to remove it")
	}

	if !cfg.Vim.HasAppHotkeyOverrides() {
		t.Error("HasAppHotkeyOverrides() = false with a per-app table declared")
	}
}

// TestVimConfig_CaptureScopeForApp_ShadowsTheSection pins that a per-app
// capture scope replaces the section's for that app alone.
func TestVimConfig_CaptureScopeForApp_ShadowsTheSection(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Vim.AppConfigs = []config.AppConfig{
		{BundleID: "com.example.Whole", CaptureScope: domain.CaptureScopeScreen},
		{BundleID: vimPlainApp},
	}

	if !cfg.Vim.HasAppCaptureScopeOverrides() {
		t.Fatal("an app config with a capture scope was not reported")
	}

	if got := cfg.Vim.CaptureScopeForApp("com.example.whole"); got != domain.CaptureScopeScreen {
		t.Fatalf("scope for the app that sets one = %q, want screen", got)
	}

	if got := cfg.Vim.CaptureScopeForApp(vimPlainApp); got != domain.CaptureScopeWindow {
		t.Fatalf("scope for an app without one = %q, want the section's window", got)
	}
}
