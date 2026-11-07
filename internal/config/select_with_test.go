package config_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

const (
	selectWithTestApp = "com.example.browser"
	selectWithCtrl    = "ctrl"
	selectWithCmd     = "cmd"
	selectWithCtrlAlt = "ctrl+alt"
	selectWithAltShft = "alt+shift"
	stepIdle          = "idle"
	stepMiddleClick   = "action middle_click"
	stepCmdClick      = "action left_click --modifier cmd"
)

func TestConfig_ValidateSelectWith_AcceptsEntriesThatCanRun(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(cfg *config.Config)
	}{
		{"defaults", func(*config.Config) {}},
		{"modifier sets that no binding shadows", func(cfg *config.Config) {
			cfg.Hints.SelectWith = map[string]config.StringOrStringArray{
				selectWithCtrl:    {"action session_action", stepIdle},
				"Alt":             {rightClickStep},
				selectWithCmd:     {stepCmdClick},
				selectWithCtrlAlt: {stepIdle, stepMiddleClick},
			}
		}},
		{"shift once Shift+L is rebound", func(cfg *config.Config) {
			delete(cfg.Hints.Hotkeys, "Shift+L")
			cfg.Hints.SelectWith = map[string]config.StringOrStringArray{
				"shift": {"action session_action", stepIdle},
			}
		}},
		{"a per-app table that replaces and disables", func(cfg *config.Config) {
			cfg.Hints.SelectWith = map[string]config.StringOrStringArray{
				selectWithCtrl: {rightClickStep},
			}
			cfg.Hints.AppConfigs = []config.AppConfig{{
				BundleID: selectWithTestApp,
				SelectWith: map[string]config.StringOrStringArray{
					selectWithCtrl: {config.DisabledSentinel},
					selectWithCmd:  {stepCmdClick},
				},
			}}
		}},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			testCase.mutate(cfg)

			err := cfg.ValidateSelectWith()
			if err != nil {
				t.Fatalf("ValidateSelectWith() = %v, want nil", err)
			}
		})
	}
}

func TestConfig_ValidateSelectWith_RefusesEntriesThatCouldNeverRun(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(cfg *config.Config)
		want   string
	}{
		{
			"a key that is not a modifier",
			func(cfg *config.Config) {
				cfg.Hints.SelectWith = map[string]config.StringOrStringArray{"a": {stepIdle}}
			},
			"hints.select_with.a",
		},
		{
			"an unknown modifier",
			func(cfg *config.Config) {
				cfg.Hints.SelectWith = map[string]config.StringOrStringArray{
					"ctrl+hyper": {stepIdle},
				}
			},
			"hints.select_with.ctrl+hyper",
		},
		{
			"a modifier named twice",
			func(cfg *config.Config) {
				cfg.Hints.SelectWith = map[string]config.StringOrStringArray{
					"ctrl+control": {stepIdle},
				}
			},
			"hints.select_with.ctrl+control",
		},
		{
			"two keys naming the same set",
			func(cfg *config.Config) {
				cfg.Hints.SelectWith = map[string]config.StringOrStringArray{
					"alt+ctrl":        {stepIdle},
					selectWithCtrlAlt: {stepIdle},
				}
			},
			"names the same modifiers",
		},
		{
			"no steps",
			func(cfg *config.Config) {
				cfg.Hints.SelectWith = map[string]config.StringOrStringArray{selectWithCtrl: {}}
			},
			"hints.select_with.ctrl cannot be empty",
		},
		{
			"a blank step",
			func(cfg *config.Config) {
				cfg.Hints.SelectWith = map[string]config.StringOrStringArray{
					selectWithCtrl: {stepIdle, " "},
				}
			},
			"hints.select_with.ctrl[1]",
		},
		{
			"the disabled sentinel in the global table",
			func(cfg *config.Config) {
				cfg.Hints.SelectWith = map[string]config.StringOrStringArray{
					selectWithCtrl: {config.DisabledSentinel},
				}
			},
			"only removes a global entry",
		},
		{
			"shift while the default Shift+L binds a hint character",
			func(cfg *config.Config) {
				cfg.Hints.SelectWith = map[string]config.StringOrStringArray{"shift": {stepIdle}}
			},
			`"Shift+L" is bound in hints.hotkeys`,
		},
		{
			"a modifier set a global hotkey shadows",
			func(cfg *config.Config) {
				cfg.Hotkeys.Bindings["Ctrl+Alt+A"] = []string{config.ModeNameGrid}
				cfg.Hints.SelectWith = map[string]config.StringOrStringArray{
					selectWithCtrlAlt: {stepIdle},
				}
			},
			`"Ctrl+Alt+A" is bound in hotkeys`,
		},
		{
			"a per-app entry its own app's hotkeys shadow",
			func(cfg *config.Config) {
				cfg.Hints.AppConfigs = []config.AppConfig{{
					BundleID: selectWithTestApp,
					Hotkeys: map[string]config.StringOrStringArray{
						"Ctrl+S": {rightClickStep},
					},
					SelectWith: map[string]config.StringOrStringArray{selectWithCtrl: {stepIdle}},
				}}
			},
			"with " + selectWithTestApp + " focused",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			testCase.mutate(cfg)

			err := cfg.ValidateSelectWith()
			if err == nil {
				t.Fatal("ValidateSelectWith() accepted an entry that could never run")
			}

			if !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("ValidateSelectWith() = %q, want it to mention %q", err, testCase.want)
			}
		})
	}
}

func TestConfig_ValidateAppConfigs_RefusesSelectWithOutsideHints(t *testing.T) {
	selectWith := map[string]config.StringOrStringArray{selectWithCtrl: {stepIdle}}

	tests := []struct {
		name     string
		mutate   func(cfg *config.Config)
		validate func(cfg *config.Config) error
	}{
		{
			config.ModeNameGrid,
			func(cfg *config.Config) {
				cfg.Grid.AppConfigs = []config.AppConfig{
					{BundleID: selectWithTestApp, SelectWith: selectWith},
				}
			},
			func(cfg *config.Config) error { return cfg.Validate() },
		},
		{
			"the global app configs",
			func(cfg *config.Config) {
				cfg.AppConfigs = []config.AppConfig{
					{BundleID: selectWithTestApp, SelectWith: selectWith},
				}
			},
			func(cfg *config.Config) error { return cfg.Validate() },
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			testCase.mutate(cfg)

			err := testCase.validate(cfg)
			if err == nil ||
				!strings.Contains(err.Error(), "select_with is only valid for hints mode") {
				t.Fatalf("Validate() = %v, want select_with refused outside hints", err)
			}
		})
	}
}

func TestConfig_ResolveSelectWith_AppEntriesReplaceGlobalOnes(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.SelectWith = map[string]config.StringOrStringArray{
		selectWithCtrl: {rightClickStep},
		"Shift+Alt":    {stepMiddleClick},
	}
	cfg.Hints.AppConfigs = []config.AppConfig{{
		BundleID: selectWithTestApp,
		SelectWith: map[string]config.StringOrStringArray{
			"control":         {testActionLeftClick},
			selectWithAltShft: {config.DisabledSentinel},
			selectWithCmd:     {stepCmdClick},
		},
	}}

	tests := []struct {
		name string
		app  string
		want map[string][]string
	}{
		{
			"no app focused",
			"",
			map[string][]string{
				selectWithCtrl:    {rightClickStep},
				selectWithAltShft: {stepMiddleClick},
			},
		},
		{
			"an app without an entry",
			"com.example.other",
			map[string][]string{
				selectWithCtrl:    {rightClickStep},
				selectWithAltShft: {stepMiddleClick},
			},
		},
		{
			"the app, matched case-insensitively",
			strings.ToUpper(selectWithTestApp),
			map[string][]string{
				selectWithCtrl: {testActionLeftClick},
				selectWithCmd:  {stepCmdClick},
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			resolved := cfg.ResolveSelectWith(testCase.app)

			got := make(map[string][]string, resolved.Len())
			for _, modifiers := range resolved.ModifierSets() {
				entry, _ := resolved.Lookup(modifiers)
				got[modifiers] = entry.Steps
			}

			if len(got) != len(testCase.want) {
				t.Fatalf("ResolveSelectWith() = %v, want %v", got, testCase.want)
			}

			for modifiers, steps := range testCase.want {
				if !slices.Equal(got[modifiers], steps) {
					t.Fatalf(
						"ResolveSelectWith()[%q] = %v, want %v",
						modifiers,
						got[modifiers],
						steps,
					)
				}
			}
		})
	}
}
