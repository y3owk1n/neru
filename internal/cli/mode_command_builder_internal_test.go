package cli

import (
	"errors"
	"slices"
	"testing"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
)

// The values these cases repeat, spelled out once so a case still pins the
// exact text a user would type.
const (
	argActionLeftClick = "--action=left_click"
	argRoleButton      = "--role=AXButton"
	stepIdle           = "idle"
)

// strayArgument is what a user might type after a mode command, thinking it
// asks for an action.
const strayArgument = "left_click"

// modeCommands are the seven commands the builder produces, each against the mode
// it enters.
//
// Which flags each one offers is not asserted here: that the command line
// registers exactly what the grammar declares is a contract rather than a
// behavior, and it is pinned in internal/architecture alongside the published
// reference it has to agree with.
func modeCommands() map[domain.Mode]*cobra.Command {
	return map[domain.Mode]*cobra.Command{
		domain.ModeHints:         HintsCmd,
		domain.ModeGrid:          GridCmd,
		domain.ModeRecursiveGrid: RecursiveGridCmd,
		domain.ModeScroll:        ScrollCmd,
		domain.ModeMonitorSelect: MonitorSelectCmd,
		domain.ModeIdle:          IdleCmd,
		domain.ModeCustom:        ModeCmd,
	}
}

// TestModeCommands_RefuseStrayArguments pins that a mode command is its flags
// and nothing else. Idle used to forward whatever it was given to a daemon that
// ignored it entirely.
func TestModeCommands_RefuseStrayArguments(t *testing.T) {
	t.Parallel()

	for mode, cmd := range modeCommands() {
		t.Run(domain.ModeString(mode), func(t *testing.T) {
			t.Parallel()

			// The custom mode command takes the declared name and nothing
			// after it, so its stray argument is a second one.
			stray := []string{strayArgument}
			if mode == domain.ModeCustom {
				stray = []string{"window", strayArgument}
			}

			err := cmd.Args(cmd, stray)
			if err == nil {
				t.Error("a stray argument was accepted; want it refused rather than dropped")
			}
		})
	}
}

// TestModeCommands_OfferTheProbeOnHintsOnly covers the one flag the vocabulary
// does not hold: --debug asks for a probe rather than an activation, so it is
// the CLI's own spelling, and only hints has anything to probe.
func TestModeCommands_OfferTheProbeOnHintsOnly(t *testing.T) {
	t.Parallel()

	for mode, cmd := range modeCommands() {
		t.Run(domain.ModeString(mode), func(t *testing.T) {
			t.Parallel()

			flag := cmd.Flags().Lookup(flagDebug.String())
			if mode != domain.ModeHints {
				if flag != nil {
					t.Errorf("%s offers --debug, which probes hints", domain.ModeString(mode))
				}

				return
			}

			if flag == nil {
				t.Fatal("hints does not offer --debug")
			}

			if flag.Shorthand != flagDebugShort {
				t.Errorf("--debug shorthand = %q, want %q", flag.Shorthand, flagDebugShort)
			}
		})
	}
}

// TestReadModeCommand_SendsWhatWasTyped pins the trip a typed flag makes: read
// off the command, into an activation, and back out as the arguments the daemon
// reads.
//
// The mode name is not among them. It names the request already, and the CLI
// repeating it inside the arguments is the redundancy the wire is free of.
func TestReadModeCommand_SendsWhatWasTyped(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		config     ModeConfig
		argv       []string
		wantAction string
		wantArgs   []string
	}{
		{
			name:       "no flags at all",
			config:     ModeConfig{Mode: domain.ModeIdle},
			wantAction: domain.ModeNameIdle,
			wantArgs:   []string{},
		},
		{
			name:       "toggle on a mode that takes nothing else",
			config:     ModeConfig{Mode: domain.ModeScroll},
			argv:       []string{"-t"},
			wantAction: domain.ModeNameScroll,
			wantArgs:   []string{"--toggle"},
		},
		{
			name:       "a whole activation",
			config:     ModeConfig{Mode: domain.ModeHints},
			argv:       []string{argActionLeftClick, "--modifier=cmd", "-r", "-s"},
			wantAction: domain.ModeNameHints,
			wantArgs: []string{
				argActionLeftClick,
				"--modifier=cmd",
				"--repeat",
				"--search",
			},
		},
		{
			// A repeated flag keeps every step, in the order written. Reading
			// it as a plain value would silently come down to the last one.
			name:   "on-exit repeated keeps every step",
			config: ModeConfig{Mode: domain.ModeHints},
			argv: []string{
				argActionLeftClick,
				"--on-exit=action left_click",
				"--on-exit=" + stepIdle,
			},
			wantAction: domain.ModeNameHints,
			wantArgs: []string{
				argActionLeftClick,
				"--on-exit=action left_click",
				"--on-exit=" + stepIdle,
			},
		},
		{
			name:       "role repeated accumulates",
			config:     ModeConfig{Mode: domain.ModeHints},
			argv:       []string{argRoleButton, "--role=AXLink"},
			wantAction: domain.ModeNameHints,
			wantArgs:   []string{"--role=AXButton,AXLink"},
		},
		{
			name:       "a flag written off asks for nothing",
			config:     ModeConfig{Mode: domain.ModeGrid},
			argv:       []string{"--toggle=false"},
			wantAction: domain.ModeNameGrid,
			wantArgs:   []string{},
		},
		{
			name:       "zoom-to-depth travels as written",
			config:     ModeConfig{Mode: domain.ModeRecursiveGrid},
			argv:       []string{"--zoom-to-depth", "2"},
			wantAction: domain.ModeNameRecursiveGrid,
			wantArgs:   []string{"--zoom-to-depth=2"},
		},
		{
			// A probe is its own request, and takes only the flags that decide
			// which elements are collected.
			name:       "debug asks for a probe",
			config:     ModeConfig{Mode: domain.ModeHints, SupportDebug: true},
			argv:       []string{"-d", argRoleButton, "--strategy=vision"},
			wantAction: domain.CommandHintsProbe,
			wantArgs:   []string{argRoleButton, "--strategy=vision"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := readTyped(t, testCase.config, testCase.argv)

			if request.action != testCase.wantAction {
				t.Errorf("action = %q, want %q", request.action, testCase.wantAction)
			}

			if !slices.Equal(request.args, testCase.wantArgs) {
				t.Errorf("args = %v, want %v", request.args, testCase.wantArgs)
			}
		})
	}
}

// TestReadModeCommand_RefusesWhatTheGrammarRefuses pins that the rules reach a
// typed command too, in the grammar's own words, before the daemon is
// contacted. The rules themselves are the grammar's to test.
func TestReadModeCommand_RefusesWhatTheGrammarRefuses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config ModeConfig
		argv   []string
		want   string
	}{
		{
			name:   "repeat without action",
			config: ModeConfig{Mode: domain.ModeGrid},
			argv:   []string{"-r"},
			want:   "--repeat requires --action",
		},
		{
			name:   "an action no mode can perform",
			config: ModeConfig{Mode: domain.ModeGrid},
			argv:   []string{"--action=scroll_up"},
			want:   `scroll sub-action "scroll_up" cannot be used as a mode action; only mouse button actions can`,
		},
		{
			name:   "an unusable value",
			config: ModeConfig{Mode: domain.ModeRecursiveGrid},
			argv:   []string{"--zoom-to-depth=-1"},
			want:   "--zoom-to-depth requires a non-negative integer",
		},
		{
			name:   "a probe alongside an activation flag",
			config: ModeConfig{Mode: domain.ModeHints, SupportDebug: true},
			argv:   []string{"-d", argActionLeftClick},
			want: "--debug cannot be combined with --action: " +
				"a probe reports what would be targeted without entering the mode",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cmd := BuildModeCommand(testCase.config)

			parseErr := cmd.ParseFlags(testCase.argv)
			if parseErr != nil {
				t.Fatalf("ParseFlags(%v) error = %v", testCase.argv, parseErr)
			}

			_, err := readModeCommand(cmd, cmd.Flags().Args(), testCase.config)
			if err == nil {
				t.Fatalf("%v was accepted; want a refusal", testCase.argv)
			}

			if !derrors.IsCode(err, derrors.CodeInvalidInput) {
				t.Errorf("error = %v, want invalid input so a script can branch on it", err)
			}

			refusal, isDomainErr := errors.AsType[*derrors.Error](err)
			if !isDomainErr {
				t.Fatalf("error = %v, want the grammar's own refusal", err)
			}

			if refusal.Message() != testCase.want {
				t.Errorf("error = %q, want %q", refusal.Message(), testCase.want)
			}
		})
	}
}

// readTyped builds the command, gives it the arguments a user would type, and
// returns the request it would send.
func readTyped(t *testing.T, config ModeConfig, argv []string) modeRequest {
	t.Helper()

	cmd := BuildModeCommand(config)

	parseErr := cmd.ParseFlags(argv)
	if parseErr != nil {
		t.Fatalf("ParseFlags(%v) error = %v", argv, parseErr)
	}

	request, err := readModeCommand(cmd, cmd.Flags().Args(), config)
	if err != nil {
		t.Fatalf("readModeCommand(%v) error = %v", argv, err)
	}

	return request
}
