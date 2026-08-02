package config_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

func TestMacroArity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		steps []string
		want  int
	}{
		{name: "no placeholders", steps: []string{testActionLeftClick}, want: 0},
		{name: "one placeholder", steps: []string{"action move_mouse --x $1"}, want: 1},
		{
			name:  "highest index across steps",
			steps: []string{"action move_mouse --x $2", "exec say $1"},
			want:  2,
		},
		{name: "repeated placeholder counts once", steps: []string{"exec say $1 $1"}, want: 1},
		{name: "escaped dollar is not a placeholder", steps: []string{"exec echo $$1"}, want: 0},
		{name: "lone dollar is text", steps: []string{"exec echo $ x"}, want: 0},
		{name: "zero names no argument", steps: []string{"exec echo $0"}, want: 0},
		{name: "multi-digit index", steps: []string{"exec echo $12"}, want: 12},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := config.MacroArity(testCase.steps); got != testCase.want {
				t.Fatalf("MacroArity(%v) = %d, want %d", testCase.steps, got, testCase.want)
			}
		})
	}
}

func TestExpandMacroSteps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		steps []string
		args  []string
		want  []string
	}{
		{
			name:  "positional substitution",
			steps: []string{"action move_mouse_relative --dx $1 --dy $2"},
			args:  []string{"100", "70"},
			want:  []string{"action move_mouse_relative --dx 100 --dy 70"},
		},
		{
			name:  "arguments may repeat and reorder",
			steps: []string{"exec echo $2 $1 $2"},
			args:  []string{"a", "b"},
			want:  []string{"exec echo b a b"},
		},
		{
			// Substitution is textual and happens before the step is split, so
			// quoting in the body is what keeps an argument with spaces whole.
			name:  "quoting in the body survives",
			steps: []string{`exec say "$1"`},
			args:  []string{"hello world"},
			want:  []string{`exec say "hello world"`},
		},
		{
			name:  "escaped dollar becomes a literal",
			steps: []string{"exec echo $$1"},
			args:  nil,
			want:  []string{"exec echo $1"},
		},
		{
			name:  "text without placeholders is untouched",
			steps: []string{testActionLeftClick},
			args:  []string{"unused"},
			want:  []string{testActionLeftClick},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := config.ExpandMacroSteps(testCase.steps, testCase.args)
			if !slices.Equal(got, testCase.want) {
				t.Fatalf("ExpandMacroSteps(%v, %v) = %v, want %v",
					testCase.steps, testCase.args, got, testCase.want)
			}
		})
	}
}

func TestParseMacroCall(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		step     string
		wantName string
		wantArgs []string
		wantCall bool
	}{
		{
			name:     "name and arguments",
			step:     "macro window_click 100 70",
			wantName: "window_click",
			wantArgs: []string{"100", "70"},
			wantCall: true,
		},
		{
			name:     "quoted argument stays whole",
			step:     "macro greet 'hello world'",
			wantName: "greet",
			wantArgs: []string{"hello world"},
			wantCall: true,
		},
		{name: "not a macro", step: testActionLeftClick},
		{name: "macro without a name", step: "macro", wantCall: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			gotName, gotArgs, gotCall := config.ParseMacroCall(testCase.step)

			if gotCall != testCase.wantCall || gotName != testCase.wantName {
				t.Fatalf("ParseMacroCall(%q) = (%q, _, %v), want (%q, _, %v)",
					testCase.step, gotName, gotCall, testCase.wantName, testCase.wantCall)
			}

			if !slices.Equal(gotArgs, testCase.wantArgs) {
				t.Fatalf("args = %v, want %v", gotArgs, testCase.wantArgs)
			}
		})
	}
}

// A mistyped macro name in a binding would otherwise do nothing at all at
// press time, so it has to fail when the config is loaded.
// errNoMacroNamed is the fragment reported for a call that names nothing.
const (
	errNoMacroNamed  = "no macro named"
	errUnknownAction = "unknown action subcommand"
)

func TestValidateMacros(t *testing.T) {
	t.Parallel()

	tests := []struct {
		build   func(*config.Config)
		name    string
		wantErr string
	}{
		{
			name: "valid definition and call",
			build: func(cfg *config.Config) {
				cfg.Macros = map[string]config.StringOrStringArray{
					"click_at": {"action move_mouse --x $1 --y $2", testActionLeftClick},
				}
				cfg.Hints.Hotkeys["Enter"] = config.StringOrStringArray{"macro click_at 10 20"}
			},
		},
		{
			name: "unknown macro name",
			build: func(cfg *config.Config) {
				cfg.Hints.Hotkeys["Enter"] = config.StringOrStringArray{"macro nope"}
			},
			wantErr: errNoMacroNamed,
		},
		{
			name: "wrong argument count",
			build: func(cfg *config.Config) {
				cfg.Macros = map[string]config.StringOrStringArray{
					"click_at": {"action move_mouse --x $1 --y $2"},
				}
				cfg.Hints.Hotkeys["Enter"] = config.StringOrStringArray{"macro click_at 10"}
			},
			wantErr: "takes 2 argument(s), got 1",
		},
		{
			name: "call from a macro body is checked too",
			build: func(cfg *config.Config) {
				cfg.Macros = map[string]config.StringOrStringArray{
					"outer": {"macro missing"},
				}
			},
			wantErr: errNoMacroNamed,
		},
		{
			// A step is not always a leaf: "run" carries its own steps, and a
			// mode command carries the steps of its --on-exit. A macro invoked
			// from either is still a macro this config will run.
			name: "call nested in a run",
			build: func(cfg *config.Config) {
				cfg.Hints.Hotkeys["Enter"] = config.StringOrStringArray{"run 'macro nope'"}
			},
			wantErr: errNoMacroNamed,
		},
		{
			name: "call nested two runs deep",
			build: func(cfg *config.Config) {
				cfg.Hints.Hotkeys["Enter"] = config.StringOrStringArray{`run 'run "macro nope"'`}
			},
			wantErr: errNoMacroNamed,
		},
		{
			name: "call in an --on-exit step",
			build: func(cfg *config.Config) {
				cfg.Hotkeys.Bindings = map[string][]string{
					"Primary+Shift+Y": {"hints --action left_click --on-exit 'macro nope'"},
				}
			},
			wantErr: errNoMacroNamed,
		},
		{
			name: "wrong arity nested in a run",
			build: func(cfg *config.Config) {
				cfg.Macros = map[string]config.StringOrStringArray{
					"needs_one": {"exec echo $1"},
				}
				cfg.Hints.Hotkeys["Enter"] = config.StringOrStringArray{"run 'macro needs_one a b'"}
			},
			wantErr: "takes 1 argument(s), got 2",
		},
		{
			name: "valid call nested in a run",
			build: func(cfg *config.Config) {
				cfg.Macros = map[string]config.StringOrStringArray{
					"real": {testActionLeftClick},
				}
				cfg.Hints.Hotkeys["Enter"] = config.StringOrStringArray{"run 'macro real'"}
			},
		},
		{
			name: "invalid macro name",
			build: func(cfg *config.Config) {
				cfg.Macros = map[string]config.StringOrStringArray{
					"has space": {testActionLeftClick},
				}
			},
			wantErr: "not a valid macro name",
		},
		{
			name: "empty body",
			build: func(cfg *config.Config) {
				cfg.Macros = map[string]config.StringOrStringArray{"empty": {}}
			},
			wantErr: "at least one step",
		},
		{
			name: "invalid step in a body",
			build: func(cfg *config.Config) {
				cfg.Macros = map[string]config.StringOrStringArray{
					"bad": {"action not_an_action"},
				}
			},
			wantErr: "unknown action subcommand",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := config.DefaultConfig()
			testCase.build(cfg)

			err := cfg.ValidateMacros()

			if testCase.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateMacros() unexpected error: %v", err)
				}

				return
			}

			if err == nil {
				t.Fatalf("ValidateMacros() = nil, want an error containing %q", testCase.wantErr)
			}

			if !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("error %q does not contain %q", err, testCase.wantErr)
			}
		})
	}
}

// Every action string the daemon can dispatch is checked when the config
// loads, at whatever depth and on whatever surface it is written. Execution
// has one path through the sequence executor; validation has to have the same
// reach or a step that names nothing real reaches a running daemon.
func TestValidate_CoversEveryActionSurface(t *testing.T) {
	t.Parallel()

	const unknownAction = "action bogus_thing"

	missionControl := func(cfg *config.Config) {
		cfg.Hints.DetectMissionControl = true
		cfg.Hints.IncludeDockHints = true
	}

	tests := []struct {
		build   func(*config.Config)
		name    string
		wantErr string
	}{
		{
			name: "monitor_select table",
			build: func(cfg *config.Config) {
				cfg.MonitorSelect.Hotkeys = map[string]config.StringOrStringArray{
					"x": {unknownAction},
				}
			},
			wantErr: errUnknownAction,
		},
		{
			name: "mission control activated hook",
			build: func(cfg *config.Config) {
				missionControl(cfg)
				cfg.Hints.OnMissionControlActivated = config.StringOrStringArray{unknownAction}
			},
			wantErr: errUnknownAction,
		},
		{
			name: "mission control deactivated hook",
			build: func(cfg *config.Config) {
				missionControl(cfg)
				cfg.Hints.OnMissionControlDeactivated = config.StringOrStringArray{unknownAction}
			},
			wantErr: errUnknownAction,
		},
		{
			name: "macro call in a mission control hook",
			build: func(cfg *config.Config) {
				missionControl(cfg)
				cfg.Hints.OnMissionControlActivated = config.StringOrStringArray{"macro nope"}
			},
			wantErr: errNoMacroNamed,
		},
		{
			name: "step nested in a run",
			build: func(cfg *config.Config) {
				cfg.Hints.Hotkeys["Enter"] = config.StringOrStringArray{
					"run '" + unknownAction + "'",
				}
			},
			wantErr: errUnknownAction,
		},
		{
			name: "step nested in an --on-exit",
			build: func(cfg *config.Config) {
				cfg.Hints.Hotkeys["Enter"] = config.StringOrStringArray{
					"hints --action left_click --on-exit '" + unknownAction + "'",
				}
			},
			wantErr: errUnknownAction,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := config.DefaultConfig()
			testCase.build(cfg)

			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want an error containing %q", testCase.wantErr)
			}

			if !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("error %q does not contain %q", err, testCase.wantErr)
			}
		})
	}
}

// The same surfaces must still accept what is valid, so the coverage above
// cannot be passing by rejecting everything.
func TestValidate_AcceptsValidActionsOnEverySurface(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Hints.DetectMissionControl = true
	cfg.Hints.IncludeDockHints = true
	cfg.Macros = map[string]config.StringOrStringArray{"real": {testActionLeftClick}}
	cfg.Hints.OnMissionControlActivated = config.StringOrStringArray{"macro real"}
	cfg.Hints.OnMissionControlDeactivated = config.StringOrStringArray{testActionLeftClick}
	cfg.MonitorSelect.Hotkeys = map[string]config.StringOrStringArray{"x": {config.CmdIdle}}
	cfg.Hints.Hotkeys["Enter"] = config.StringOrStringArray{
		"run 'macro real'",
		"hints --action left_click --on-exit 'macro real'",
	}

	err := cfg.Validate()
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
}
