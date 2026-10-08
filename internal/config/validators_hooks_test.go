package config_test

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/event"
)

// TestValidateHooks_ChecksTheHookOfEveryEvent pins the whole chain for every
// event name: the key a user writes is the event's name with "on_" in front,
// it decodes into the hook for that event, and its steps are checked at load
// like any binding's.
func TestValidateHooks_ChecksTheHookOfEveryEvent(t *testing.T) {
	t.Parallel()

	for _, name := range event.All() {
		key := "on_" + string(name)

		t.Run(key, func(t *testing.T) {
			t.Parallel()

			cfg := config.DefaultConfig()
			cfg.Hints.DetectMissionControl = true
			cfg.Hints.IncludeDockHints = true

			_, decodeErr := toml.Decode("[hooks]\n"+key+" = \""+"action bogus_thing\"\n", cfg)
			if decodeErr != nil {
				t.Fatalf("decoding %s: %v", key, decodeErr)
			}

			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Validate() accepted an unknown action in hooks.%s", key)
			}

			if !strings.Contains(err.Error(), "hooks."+key) {
				t.Errorf("error %q does not name hooks.%s", err, key)
			}
		})
	}
}

func TestValidateHooks_RejectsWhatABindingWould(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		steps   config.StringOrStringArray
		wantErr string
	}{
		{name: "empty step", steps: config.StringOrStringArray{"  "}, wantErr: "cannot be empty"},
		{
			name:    "undefined macro",
			steps:   config.StringOrStringArray{"macro missing"},
			wantErr: errNoMacroNamed,
		},
		{
			name:    "flag no mode has",
			steps:   config.StringOrStringArray{"hints --serach"},
			wantErr: "hooks.on_mode_enter",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := config.DefaultConfig()
			cfg.Hooks.OnModeEnter = testCase.steps

			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want an error containing %q", testCase.wantErr)
			}

			if !strings.Contains(err.Error(), testCase.wantErr) {
				t.Errorf("error %q does not contain %q", err, testCase.wantErr)
			}
		})
	}
}

func TestValidateHooks_AcceptsValidSteps(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Macros = map[string]config.StringOrStringArray{"known": {testActionLeftClick}}
	cfg.Hooks.OnModeEnter = config.StringOrStringArray{"exec echo \"$NERU_MODE\"", "macro known"}
	cfg.Hooks.OnConfigReload = config.StringOrStringArray{"grid --toggle"}

	err := cfg.Validate()
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
}

// TestValidateHooks_WarnsWhenMissionControlHooksCannotFire pins that a Mission
// Control hook with the detection off loads, and says it will never run.
func TestValidateHooks_WarnsWhenMissionControlHooksCannotFire(t *testing.T) {
	t.Parallel()

	for _, detect := range []bool{false, true} {
		cfg := config.DefaultConfig()
		cfg.Hints.DetectMissionControl = detect
		cfg.Hints.IncludeDockHints = true
		cfg.Hooks.OnMissionControlActivated = config.StringOrStringArray{"idle"}

		warnings := &config.Warnings{}

		err := cfg.ValidateWithWarnings(warnings, config.WrittenConfig{})
		if err != nil {
			t.Fatalf("detect=%v: ValidateWithWarnings() refused the hook: %v", detect, err)
		}

		warned := strings.Contains(
			strings.Join(warnings.Messages(), "\n"),
			"hooks.on_mission_control_activated",
		)
		if warned == detect {
			t.Errorf(
				"detect=%v: warned=%v, want a warning only while detection is off",
				detect,
				warned,
			)
		}
	}
}

// TestValidateHooks_WarnsThatTheHintsMissionControlKeysAreDeprecated pins the
// deprecation: the [hints] keys still load, and say where they moved.
func TestValidateHooks_WarnsThatTheHintsMissionControlKeysAreDeprecated(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Hints.DetectMissionControl = true
	cfg.Hints.IncludeDockHints = true
	cfg.Hints.OnMissionControlDeactivated = config.StringOrStringArray{"idle"}

	warnings := &config.Warnings{}

	err := cfg.ValidateWithWarnings(warnings, config.WrittenConfig{})
	if err != nil {
		t.Fatalf("ValidateWithWarnings() refused a deprecated key: %v", err)
	}

	messages := strings.Join(warnings.Messages(), "\n")
	if !strings.Contains(messages, "hooks.on_mission_control_deactivated") ||
		!strings.Contains(messages, "deprecated") {
		t.Errorf("warnings %q do not name the deprecation and its replacement", messages)
	}
}
