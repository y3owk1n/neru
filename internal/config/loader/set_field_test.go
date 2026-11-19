package loader_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
)

const (
	testHintChars = "qwerty"
	testColorHex  = "#FF0000AA"

	// The leaves the per-type tables below write, refuse and name.
	pathFontSize        = "hints.ui.font_size"
	pathPassthrough     = "general.passthrough_unbounded_keys"
	pathBackgroundColor = "hints.ui.background_color"
	valueTrue           = "true"
)

// TestSetField_WritesEachLeafType pins that `neru config set` writes each kind
// of leaf it can name, parsed the way the config file would carry it.
func TestSetField_WritesEachLeafType(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		value string
		got   func(*config.Config) any
		want  any
	}{
		{
			name:  "string",
			path:  "hints.hint_characters",
			value: testHintChars,
			got:   func(cfg *config.Config) any { return cfg.Hints.HintCharacters },
			want:  testHintChars,
		},
		{
			name:  "integer",
			path:  pathFontSize,
			value: "14",
			got:   func(cfg *config.Config) any { return cfg.Hints.UI.FontSize },
			want:  14,
		},
		{
			name:  "bool",
			path:  pathPassthrough,
			value: valueTrue,
			got:   func(cfg *config.Config) any { return cfg.General.PassthroughUnboundedKeys },
			want:  true,
		},
		{
			name:  "float",
			path:  "hints.vision.minimum_confidence",
			value: "0.5",
			got:   func(cfg *config.Config) any { return cfg.Hints.Vision.MinimumConfidence },
			want:  0.5,
		},
		{
			name:  "string slice",
			path:  "hints.clickable_roles",
			value: "AXButton,AXLink",
			got:   func(cfg *config.Config) any { return cfg.Hints.ClickableRoles },
			want:  []string{"AXButton", "AXLink"},
		},
		{
			name:  "color as one hex value",
			path:  pathBackgroundColor,
			value: testColorHex,
			got:   func(cfg *config.Config) any { return cfg.Hints.UI.BackgroundColor },
			want:  config.Color{Light: testColorHex, Dark: testColorHex},
		},
		{
			name:  "color as a JSON object",
			path:  pathBackgroundColor,
			value: `{"light":"#000","dark":"#FFF"}`,
			got:   func(cfg *config.Config) any { return cfg.Hints.UI.BackgroundColor },
			want:  config.Color{Light: "#000", Dark: "#FFF"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()

			err := loader.SetField(cfg, testCase.path, testCase.value)
			if err != nil {
				t.Fatalf("SetField(%s) error = %v", testCase.path, err)
			}

			if got := testCase.got(cfg); !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("%s = %#v, want %#v", testCase.path, got, testCase.want)
			}
		})
	}
}

// stepWithComma is a step whose command line holds a comma.
const stepWithComma = "exec echo a,b"

// TestSetField_StepsKeepTheirCommas pins that a list of binding steps is read
// as the config file reads it, one step or a TOML array, and never split on a
// comma inside a step.
func TestSetField_StepsKeepTheirCommas(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		value string
		got   func(*config.Config) config.StringOrStringArray
		want  config.StringOrStringArray
	}{
		{
			name:  "one step with a comma",
			path:  "hooks.on_mode_enter",
			value: stepWithComma,
			got:   func(cfg *config.Config) config.StringOrStringArray { return cfg.Hooks.OnModeEnter },
			want:  config.StringOrStringArray{stepWithComma},
		},
		{
			name:  "an array of steps",
			path:  "hooks.on_mode_exit",
			value: `["exec echo a,b", "idle"]`,
			got:   func(cfg *config.Config) config.StringOrStringArray { return cfg.Hooks.OnModeExit },
			want:  config.StringOrStringArray{stepWithComma, "idle"},
		},
		{
			name:  "a deprecated hint hook",
			path:  "hints.on_mission_control_activated",
			value: stepWithComma,
			got: func(cfg *config.Config) config.StringOrStringArray {
				return cfg.Hints.OnMissionControlActivated
			},
			want: config.StringOrStringArray{stepWithComma},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()

			err := loader.SetField(cfg, testCase.path, testCase.value)
			if err != nil {
				t.Fatalf("SetField(%s) error = %v", testCase.path, err)
			}

			if got := testCase.got(cfg); !slices.Equal(got, testCase.want) {
				t.Errorf("%s = %q, want %q", testCase.path, got, testCase.want)
			}
		})
	}
}

// TestSetField_RefusesAValueItCannotWrite pins that `neru config set` refuses a
// path or value it cannot write rather than leaving the field alone in silence.
func TestSetField_RefusesAValueItCannotWrite(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		value string
	}{
		{name: "unknown path", path: "hints.nonexistent_field", value: "value"},
		{name: "empty path", path: "", value: "value"},
		{name: "integer that is not a number", path: pathFontSize, value: "not-a-number"},
		{
			name:  "bool that is not a bool",
			path:  pathPassthrough,
			value: "not-a-bool",
		},
		{name: "unterminated step array", path: "hooks.on_mode_enter", value: `["exec echo a`},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := loader.SetField(config.DefaultConfig(), testCase.path, testCase.value)
			if err == nil {
				t.Errorf("SetField(%q, %q) accepted it", testCase.path, testCase.value)
			}
		})
	}
}

// TestValidateConfigSetField_LeavesSettingsThatDependOnOthersToTheDaemon covers
// a value that is only valid together with another setting. The defaults are
// not the user's config, so checking the rule against them would refuse a
// change the daemon accepts.
func TestValidateConfigSetField_LeavesSettingsThatDependOnOthersToTheDaemon(t *testing.T) {
	err := loader.ValidateConfigSetField("hints.detect_mission_control", "true")
	if err != nil {
		t.Fatalf("ValidateConfigSetField() refused a value the daemon may accept: %v", err)
	}
}

// TestConfigFieldType_NamesEachLeafType pins the type hint `neru config set`
// shows on a refused value, and the "unknown" ipcctrl keys off.
func TestConfigFieldType_NamesEachLeafType(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "hints.hint_characters", want: "string"},
		{path: pathFontSize, want: "integer"},
		{path: pathPassthrough, want: "boolean"},
		{path: "hints.vision.minimum_confidence", want: "float"},
		{path: "hints.clickable_roles", want: "array"},
		{path: pathBackgroundColor, want: "color (#RGB/#RRGGBB/#AARRGGBB or JSON object)"},
		{path: "nonexistent.path", want: "unknown"},
	}

	for _, testCase := range tests {
		t.Run(testCase.path, func(t *testing.T) {
			if got := loader.ConfigFieldType(testCase.path); got != testCase.want {
				t.Errorf("ConfigFieldType(%q) = %q, want %q", testCase.path, got, testCase.want)
			}
		})
	}
}
