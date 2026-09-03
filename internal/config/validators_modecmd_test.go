package config_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

// stepGridSearch is a binding step that loads and does not do what it says:
// grid makes a selection by coordinates and has no search of its own.
const stepGridSearch = "grid --search"

// TestValidateModeCommands_WarnsWithoutRefusing pins the severity split where a
// user meets it: a binding that works minus one flag keeps loading, and what it
// will not do is reported instead.
//
// Refusing these would replace the whole configuration with the defaults, so
// one inert flag would cost the user every binding, theme and setting they
// wrote.
func TestValidateModeCommands_WarnsWithoutRefusing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		breakIt func(*config.Config)
		want    []string
	}{
		{
			name: "flag the mode does not accept",
			breakIt: func(cfg *config.Config) {
				cfg.Hotkeys.Bindings = map[string][]string{"g": {stepGridSearch}}
			},
			want: []string{"hotkeys.g: grid does not accept --search"},
		},
		{
			name: "unmet flag dependency",
			breakIt: func(cfg *config.Config) {
				cfg.Hotkeys.Bindings = map[string][]string{"h": {"hints --repeat"}}
			},
			want: []string{"hotkeys.h: --repeat requires --action"},
		},
		{
			// Every table a mode command can be written in is read, not only
			// the global one.
			name: "a mode's own hotkey table",
			breakIt: func(cfg *config.Config) {
				cfg.Hints.Hotkeys["q"] = config.StringOrStringArray{stepGridSearch}
			},
			want: []string{"hints.hotkeys.q: grid does not accept --search"},
		},
		{
			name: "a macro body",
			breakIt: func(cfg *config.Config) {
				cfg.Macros = map[string]config.StringOrStringArray{
					"click": {stepGridSearch},
				}
			},
			want: []string{"macros.click: grid does not accept --search"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := config.DefaultConfig()
			testCase.breakIt(cfg)

			warnings := &config.Warnings{}

			err := cfg.ValidateWithWarnings(warnings, config.WrittenConfig{})
			if err != nil {
				t.Fatalf("ValidateWithWarnings() refused the configuration: %v", err)
			}

			if got := warnings.Messages(); !slices.Equal(got, testCase.want) {
				t.Errorf("warnings = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestValidateModeCommands_RefusesAnUnreadableCommand pins the other tier: a
// binding naming a flag nothing knows never activated anything, so refusing it
// takes away nothing that worked. An unknown *command* in a binding already
// fails the whole load, so this extends a rule rather than inventing one.
func TestValidateModeCommands_RefusesAnUnreadableCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		step string
		want string
	}{
		{
			name: "mistyped flag",
			step: "hints --serach --action=bogus",
			want: "hotkeys.k: unknown flag: --serach",
		},
		{
			name: "unusable value",
			step: "hints --strategy=nonsense",
			want: "hotkeys.k: --strategy requires axtree, vision, or wl-kbptr",
		},
		{
			name: "action no mode can perform",
			step: "hints --action=move_mouse",
			want: `hotkeys.k: "move_mouse" cannot be used as a mode action`,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := config.DefaultConfig()
			cfg.Hotkeys.Bindings = map[string][]string{"k": {testCase.step}}

			warnings := &config.Warnings{}

			err := cfg.ValidateWithWarnings(warnings, config.WrittenConfig{})
			if err == nil {
				t.Fatal("ValidateWithWarnings() accepted a binding that cannot run")
			}

			if got := err.Error(); !strings.Contains(got, testCase.want) {
				t.Errorf("error = %q, want it to contain %q", got, testCase.want)
			}

			if got := warnings.Messages(); len(got) > 0 {
				t.Errorf("warnings = %q, want none alongside a refusal", got)
			}
		})
	}
}

// TestValidateModeCommands_LeavesMacroPlaceholdersAlone pins that a macro body
// is not judged on the arguments it has not been given yet. A placeholder only
// ever stands in for a flag's value, and what fills it is known when the macro
// is called, not when it is written.
func TestValidateModeCommands_LeavesMacroPlaceholdersAlone(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Macros = map[string]config.StringOrStringArray{"click": {"hints --action $1"}}
	cfg.Hotkeys.Bindings = map[string][]string{"m": {"macro click left_click"}}

	warnings := &config.Warnings{}

	err := cfg.ValidateWithWarnings(warnings, config.WrittenConfig{})
	if err != nil {
		t.Fatalf("ValidateWithWarnings() refused a macro body: %v", err)
	}

	if got := warnings.Messages(); len(got) > 0 {
		t.Errorf("warnings = %q, want none", got)
	}
}

// TestValidateModeCommands_AcceptsTheDefaults pins that a configuration nobody
// has broken reports as valid with nothing to say. The defaults are the one
// configuration every user starts from, so a warning here would reach everyone.
func TestValidateModeCommands_AcceptsTheDefaults(t *testing.T) {
	t.Parallel()

	warnings := &config.Warnings{}

	err := config.DefaultConfig().ValidateWithWarnings(warnings, config.WrittenConfig{})
	if err != nil {
		t.Fatalf("ValidateWithWarnings() refused the defaults: %v", err)
	}

	if got := warnings.Messages(); len(got) > 0 {
		t.Errorf("warnings = %q, want none", got)
	}
}
