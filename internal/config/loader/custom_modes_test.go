package loader_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
)

const declaredModesConfig = `
[modes.window]
indicator = "Window"

[modes.window.hotkeys]
"h" = "exec yabai -m window --focus west"
"f" = ["exec yabai -m window --toggle zoom-fullscreen", "idle"]

[[modes.window.app_configs]]
bundle_id = "com.apple.Safari"
hotkeys = { "h" = "action scroll_left" }

[modes.silent]
indicator = ""

[modes.trapped]

[modes.trapped.hotkeys]
"Escape" = "__disabled__"
"q" = "idle"

[hotkeys]
"Primary+Shift+W" = "mode window"
`

// TestLoad_ReadsDeclaredModesFromTheRawTable pins the loader's half of a
// declaration: the hotkey table is tagged toml:"-" like every mode's, so it
// has to be read from the raw decode, merged over the default Escape binding,
// and given to a mode declared without a table at all.
func TestLoad_ReadsDeclaredModesFromTheRawTable(t *testing.T) {
	t.Parallel()

	result := loadWithOverride(t, declaredModesConfig, "")
	cfg := result.Config

	window, declared := cfg.Modes["window"]
	if !declared {
		t.Fatal("modes.window was not loaded")
	}

	if window.Indicator != "Window" {
		t.Errorf("modes.window.indicator = %q, want %q", window.Indicator, "Window")
	}

	if got := window.Hotkeys["h"]; len(got) != 1 || !strings.HasPrefix(got[0], "exec ") {
		t.Errorf("modes.window.hotkeys.h = %v, want the exec step", got)
	}

	if got := window.Hotkeys["f"]; len(got) != 2 {
		t.Errorf("modes.window.hotkeys.f = %v, want both steps", got)
	}

	if got := window.Hotkeys["Escape"]; len(got) != 1 || got[0] != config.CmdIdle {
		t.Errorf("modes.window.hotkeys.Escape = %v, want the default idle binding", got)
	}

	if len(window.AppConfigs) != 1 || window.AppConfigs[0].BundleID != "com.apple.Safari" {
		t.Errorf("modes.window.app_configs = %+v, want the one Safari entry", window.AppConfigs)
	}

	silent := cfg.Modes["silent"]
	if got := silent.Hotkeys["Escape"]; len(got) != 1 || got[0] != config.CmdIdle {
		t.Errorf("a mode declared without a table has Escape = %v, want the default", got)
	}

	trapped := cfg.Modes["trapped"]
	if _, bound := trapped.Hotkeys["Escape"]; bound {
		t.Error("__disabled__ left the default Escape binding in place")
	}

	if got := trapped.Hotkeys["q"]; len(got) != 1 || got[0] != config.CmdIdle {
		t.Errorf("modes.trapped.hotkeys.q = %v, want idle", got)
	}
}

// TestLoad_RefusesADeclaredModeItCannotLoad pins that the raw-table reading
// reports a fault the way the built-in tables do, naming the declaration.
func TestLoad_RefusesADeclaredModeItCannotLoad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		toml string
		want string
	}{
		{
			name: "two keys that normalize to one chord",
			toml: "[modes.window.hotkeys]\n\"escape\" = \"idle\"\n\"Escape\" = \"idle\"\n",
			want: "modes.window.hotkeys",
		},
		{
			name: "a binding into a mode nobody declared",
			toml: "[hotkeys]\n\"Primary+Shift+W\" = \"mode window\"\n",
			want: `mode "window" is not declared`,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			configPath := filepath.Join(t.TempDir(), "config.toml")

			err := os.WriteFile(configPath, []byte(testCase.toml), 0o600)
			if err != nil {
				t.Fatalf("failed to write config: %v", err)
			}

			service := loader.NewService(config.DefaultConfig(), configPath, zap.NewNop(), nil)

			result := service.LoadWithValidation(configPath)
			if result.ValidationError == nil {
				t.Fatal("LoadWithValidation() accepted a configuration it cannot run")
			}

			if got := result.ValidationError.Error(); !strings.Contains(got, testCase.want) {
				t.Errorf("error = %q, want it to contain %q", got, testCase.want)
			}
		})
	}
}

// TestSave_WritesDeclaredModeHotkeysTheLoaderReadsBack pins the persistence
// half: the table the encoder skips is written by hand, under the declaration
// it belongs to, in the shape the loader reads.
func TestSave_WritesDeclaredModeHotkeysTheLoaderReadsBack(t *testing.T) {
	t.Parallel()

	loaded := loadWithOverride(t, declaredModesConfig, "").Config

	savedPath := filepath.Join(t.TempDir(), "config.toml")

	err := loader.Save(loaded, savedPath)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	service := loader.NewService(config.DefaultConfig(), savedPath, zap.NewNop(), nil)

	result := service.LoadWithValidation(savedPath)
	if result.ValidationError != nil {
		t.Fatalf("the saved file does not load: %v", result.ValidationError)
	}

	for name, want := range loaded.Modes {
		got, declared := result.Config.Modes[name]
		if !declared {
			t.Errorf("modes.%s was not saved", name)

			continue
		}

		if len(got.Hotkeys) != len(want.Hotkeys) {
			t.Errorf("modes.%s.hotkeys = %v after a save, want %v", name, got.Hotkeys, want.Hotkeys)
		}

		for key, steps := range want.Hotkeys {
			if strings.Join(got.Hotkeys[key], "|") != strings.Join(steps, "|") {
				t.Errorf(
					"modes.%s.hotkeys.%s = %v after a save, want %v",
					name,
					key,
					got.Hotkeys[key],
					steps,
				)
			}
		}
	}
}

// TestLoad_ReadsDeclaredModeHotkeysFromTheOverrideFile pins the second file a
// declaration can come from: the override `neru config set` writes is decoded
// into the same toml:"-" field, so its tables have to be read from the raw
// decode too. A mode both files declare gets the override merged over the
// configuration's table, and a mode only the override declares is seeded with
// the default Escape binding like any other.
func TestLoad_ReadsDeclaredModeHotkeysFromTheOverrideFile(t *testing.T) {
	t.Parallel()

	override := `
[modes.window.hotkeys]
"h" = "action scroll_left"
"k" = "action scroll_up"

[modes.tabs]
indicator = "Tabs"

[modes.tabs.hotkeys]
"n" = "action feed ctrl+tab"
`

	cfg := loadWithOverride(t, declaredModesConfig, override).Config

	window := cfg.Modes["window"]
	if got := window.Hotkeys["h"]; len(got) != 1 || got[0] != "action scroll_left" {
		t.Errorf("modes.window.hotkeys.h = %v, want the override's step", got)
	}

	if got := window.Hotkeys["k"]; len(got) != 1 || got[0] != "action scroll_up" {
		t.Errorf("modes.window.hotkeys.k = %v, want the override's added step", got)
	}

	if got := window.Hotkeys["f"]; len(got) != 2 {
		t.Errorf("modes.window.hotkeys.f = %v, want the configuration's binding kept", got)
	}

	if got := window.Hotkeys["Escape"]; len(got) != 1 || got[0] != config.CmdIdle {
		t.Errorf("modes.window.hotkeys.Escape = %v, want the default kept", got)
	}

	tabs, declared := cfg.Modes["tabs"]
	if !declared {
		t.Fatal("a mode declared only in the override was not loaded")
	}

	if got := tabs.Hotkeys["n"]; len(got) != 1 || got[0] != "action feed ctrl+tab" {
		t.Errorf("modes.tabs.hotkeys.n = %v, want the override's step", got)
	}

	if got := tabs.Hotkeys["Escape"]; len(got) != 1 || got[0] != config.CmdIdle {
		t.Errorf("modes.tabs.hotkeys.Escape = %v, want the default seeded", got)
	}
}
