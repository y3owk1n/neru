package config_test

import (
	"maps"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

// The spellings the declarations below share.
const (
	declaredModeName = "window"
	stepScroll       = "scroll"
	stepScrollLeft   = "action scroll_left"
)

// A declared mode as a test writes it: the shape the loader produces, with the
// default Escape binding already merged in.
func declaredMode(hotkeys map[string]config.StringOrStringArray) config.CustomModeConfig {
	table := config.DefaultCustomModeHotkeys()
	maps.Copy(table, hotkeys)

	return config.CustomModeConfig{Indicator: "Window", Hotkeys: table}
}

// TestValidateCustomModes_AcceptsADeclaredModeAndABindingIntoIt pins the
// happy path end to end: a declaration, a binding that enters it, a step in
// its table that leaves it, and a per-app override, all through the same
// ladder a load runs.
func TestValidateCustomModes_AcceptsADeclaredModeAndABindingIntoIt(t *testing.T) {
	t.Parallel()

	mode := declaredMode(map[string]config.StringOrStringArray{
		"h": {"exec yabai -m window --focus west"},
		"s": {stepScroll},
	})
	mode.AppConfigs = []config.AppConfig{{
		BundleID: keymapTestApp,
		Hotkeys:  map[string]config.StringOrStringArray{"h": {stepScrollLeft}},
	}}

	cfg := config.DefaultConfig()
	cfg.Modes = map[string]config.CustomModeConfig{declaredModeName: mode}
	cfg.Hotkeys.Bindings = map[string][]string{"Primary+Shift+W": {"mode window --toggle"}}

	warnings := &config.Warnings{}

	err := cfg.ValidateWithWarnings(warnings, config.WrittenConfig{})
	if err != nil {
		t.Fatalf("ValidateWithWarnings() refused a well-formed declaration: %v", err)
	}

	if got := warnings.Messages(); len(got) > 0 {
		t.Errorf("warnings = %q, want none", got)
	}
}

// TestValidateCustomModes_RefusesWhatCouldNeverBeEntered pins the load-time
// refusals: a name the grammar cannot carry, a name a built-in mode already
// answers to, a binding into a mode nobody declared, and a per-app field that
// belongs to another mode.
func TestValidateCustomModes_RefusesWhatCouldNeverBeEntered(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		breakIt func(*config.Config)
		want    string
	}{
		{
			name: "a name the grammar cannot carry",
			breakIt: func(cfg *config.Config) {
				cfg.Modes = map[string]config.CustomModeConfig{"my mode": declaredMode(nil)}
			},
			want: "modes.my mode: a mode name starts with a letter",
		},
		{
			name: "a built-in mode's name",
			breakIt: func(cfg *config.Config) {
				cfg.Modes = map[string]config.CustomModeConfig{stepScroll: declaredMode(nil)}
			},
			want: `modes.scroll: "scroll" is a built-in mode command`,
		},
		{
			name: "the command word itself",
			breakIt: func(cfg *config.Config) {
				cfg.Modes = map[string]config.CustomModeConfig{"mode": declaredMode(nil)}
			},
			want: `modes.mode: "mode" is a built-in mode command`,
		},
		{
			name: "a binding into a mode nobody declared",
			breakIt: func(cfg *config.Config) {
				cfg.Hotkeys.Bindings = map[string][]string{"w": {"mode window"}}
			},
			want: `hotkeys.w: mode "window" is not declared; declare it as [modes.window]`,
		},
		{
			name: "a step in a declared mode's own table into an undeclared one",
			breakIt: func(cfg *config.Config) {
				cfg.Modes = map[string]config.CustomModeConfig{
					declaredModeName: declaredMode(
						map[string]config.StringOrStringArray{"t": {"mode tabs"}},
					),
				}
			},
			want: `modes.window.hotkeys.t: mode "tabs" is not declared`,
		},
		{
			name: "a per-app field that belongs to hints",
			breakIt: func(cfg *config.Config) {
				mode := declaredMode(nil)
				mode.AppConfigs = []config.AppConfig{
					{BundleID: keymapTestApp, AdditionalClickable: []string{"button"}},
				}
				cfg.Modes = map[string]config.CustomModeConfig{declaredModeName: mode}
			},
			want: "modes.window.app_configs[0].additional_clickable_roles is only valid for hints mode",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := config.DefaultConfig()
			testCase.breakIt(cfg)

			warnings := &config.Warnings{}

			err := cfg.ValidateWithWarnings(warnings, config.WrittenConfig{})
			if err == nil {
				t.Fatal("ValidateWithWarnings() accepted a declaration that could never be entered")
			}

			if got := err.Error(); !strings.Contains(got, testCase.want) {
				t.Errorf("error = %q, want it to contain %q", got, testCase.want)
			}
		})
	}
}

// TestConfig_HotkeysForModeAndApp_ReadsADeclaredMode pins the lookup a
// keystroke settles against: the declared table by name, and the per-app
// override merged over it, under the same rules the built-in modes follow.
func TestConfig_HotkeysForModeAndApp_ReadsADeclaredMode(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	mode := declaredMode(map[string]config.StringOrStringArray{"h": {"exec left"}})
	mode.AppConfigs = []config.AppConfig{{
		BundleID: keymapTestApp,
		Hotkeys: map[string]config.StringOrStringArray{
			"h":      {stepScrollLeft},
			"Escape": {config.DisabledSentinel},
		},
	}}
	cfg.Modes = map[string]config.CustomModeConfig{declaredModeName: mode}

	base := cfg.HotkeysForModeAndApp(declaredModeName, "")
	if got := base["h"]; len(got) != 1 || got[0] != "exec left" {
		t.Errorf("base h = %v, want the declared step", got)
	}

	if got := base["Escape"]; len(got) != 1 || got[0] != config.CmdIdle {
		t.Errorf("base Escape = %v, want the default idle binding", got)
	}

	overridden := cfg.HotkeysForModeAndApp(declaredModeName, "com.apple.safari")
	if got := overridden["h"]; len(got) != 1 || got[0] != stepScrollLeft {
		t.Errorf("overridden h = %v, want the per-app step", got)
	}

	if _, bound := overridden["Escape"]; bound {
		t.Error("overridden Escape is still bound; want __disabled__ to remove it")
	}

	if !cfg.Modes[declaredModeName].HasAppHotkeyOverrides() {
		t.Error("HasAppHotkeyOverrides() = false with a per-app table declared")
	}

	if got := cfg.HotkeysForModeAndApp("nobody", ""); got != nil {
		t.Errorf("HotkeysForModeAndApp(nobody) = %v, want nil for an undeclared name", got)
	}
}
