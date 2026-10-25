package ipcctrl_test

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/app/ipcctrl"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
	"github.com/y3owk1n/neru/internal/ports"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

// What a mode command may say is the grammar's business, in
// internal/domain/modecmd, and that is where the rules are asserted. These
// cases cover what is left over here: that every mode command goes through the
// grammar, that a refusal reaches the caller with its own words, and that it
// carries the response code a script branches on.

// newModeTestController builds a controller over the real registration path, so
// these cases exercise the wiring rather than a handler called directly.
func newModeTestController(cfg *config.Config) *ipcctrl.Controller {
	return newModeTestControllerWith(
		cfg,
		&portmocks.MockAccessibilityPort{},
		&portmocks.MockSystemPort{},
		state.NewAppState(),
	)
}

// newModeTestControllerWith is newModeTestController over given ports and app
// state, for a case that needs the focused app, secure input or the screens,
// or reads the mode back.
func newModeTestControllerWith(
	cfg *config.Config,
	accessibility *portmocks.MockAccessibilityPort,
	system *portmocks.MockSystemPort,
	appState *state.AppState,
) *ipcctrl.Controller {
	logger := zap.NewNop()
	actionService := services.NewActionService(
		accessibility,
		&portmocks.MockOverlayPort{},
		system,
		logger,
	)

	return ipcctrl.New(ipcctrl.Deps{
		ActionService: actionService,
		ConfigService: loader.NewService(cfg, "", logger, nil),
		AppState:      appState,
		Config:        cfg,
		Modes:         newTestModesHandler(cfg, logger, appState, actionService, system),
		Logger:        logger,
	})
}

// TestHandleCommand_EveryModeCommandReadsTheGrammar pins that no mode is an
// exception. Each of the eight answers a command it cannot read with a refusal
// of its own rather than with "unknown command" or a silent success.
func TestHandleCommand_EveryModeCommandReadsTheGrammar(t *testing.T) {
	controller := newModeTestController(config.DefaultConfig())

	for _, mode := range []domain.Mode{
		domain.ModeHints,
		domain.ModeGrid,
		domain.ModeRecursiveGrid,
		domain.ModeBisect,
		domain.ModeScroll,
		domain.ModeMonitorSelect,
		domain.ModeIdle,
		domain.ModeCustom,
	} {
		name := domain.ModeString(mode)

		t.Run(name, func(t *testing.T) {
			resp := controller.HandleCommand(context.Background(), ipc.Command{
				Action: name,
				Args:   []string{"--serach"},
			})

			if resp.Success {
				t.Fatalf("%s --serach was accepted", name)
			}

			if resp.Code != ipc.CodeInvalidInput {
				t.Errorf("code = %q, want %q", resp.Code, ipc.CodeInvalidInput)
			}

			if want := "unknown flag: --serach"; resp.Message != want {
				t.Errorf("message = %q, want %q", resp.Message, want)
			}
		})
	}
}

// TestHandleCommand_RefusalsCarryTheGrammarsWording pins that the daemon passes
// the grammar's own sentence on, without the error code the domain error
// carries for callers and without a wording of its own.
//
// --on-exit without an action is the one a user notices. It was refused when
// typed and accepted over the wire, where the steps were stored and never run.
func TestHandleCommand_RefusalsCarryTheGrammarsWording(t *testing.T) {
	controller := newModeTestController(config.DefaultConfig())

	tests := []struct {
		name string
		cmd  ipc.Command
		want string
	}{
		{
			name: "on-exit without an action",
			cmd:  ipc.Command{Action: actionHints, Args: []string{"--on-exit=idle"}},
			want: "--on-exit requires --action (it runs only when the action is fulfilled)",
		},
		{
			name: "modifier without an action",
			cmd:  ipc.Command{Action: actionHints, Args: []string{"--modifier=shift"}},
			want: "--modifier requires --action",
		},
		{
			name: "a flag the mode does not accept",
			cmd:  ipc.Command{Action: actionGrid, Args: []string{"--search"}},
			want: "grid does not accept --search",
		},
		{
			name: "an unusable value",
			cmd: ipc.Command{
				Action: actionGrid,
				Args:   []string{actionGrid, "--cursor-selection-mode=invalid"},
			},
			want: "--cursor-selection-mode requires follow or hold",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			resp := controller.HandleCommand(context.Background(), testCase.cmd)

			if resp.Success {
				t.Fatalf("%v was accepted", testCase.cmd.Args)
			}

			if resp.Code != ipc.CodeInvalidInput {
				t.Errorf("code = %q, want %q", resp.Code, ipc.CodeInvalidInput)
			}

			if resp.Message != testCase.want {
				t.Errorf("message = %q, want %q", resp.Message, testCase.want)
			}
		})
	}
}

// TestHandleCommand_ModeCommandEntersItsMode pins the other half of the
// delegation. A command the grammar accepts reaches the mode handler and is
// answered as done.
//
// Idle is the mode used here because leaving one needs nothing of the platform.
// That a flag reaches the mode it was written for is pinned end to end by the
// journeys in internal/app, which drive a real activation.
func TestHandleCommand_ModeCommandEntersItsMode(t *testing.T) {
	controller := newModeTestController(config.DefaultConfig())

	tests := []struct {
		name string
		cmd  ipc.Command
		want string
	}{
		{
			name: "no arguments at all, as a binding sends them",
			cmd:  ipc.Command{Action: actionIdle},
			want: "idle mode activated",
		},
		{
			// A caller modeled on the CLI's own traffic repeats the mode name
			// inside the arguments. Anything written that way has to keep
			// working.
			name: "the mode name repeated in the arguments",
			cmd:  ipc.Command{Action: actionIdle, Args: []string{actionIdle}},
			want: "idle mode activated",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			resp := controller.HandleCommand(context.Background(), testCase.cmd)

			if !resp.Success {
				t.Fatalf("%v was refused: %s", testCase.cmd.Args, resp.Message)
			}

			if resp.Code != ipc.CodeOK {
				t.Errorf("code = %q, want %q", resp.Code, ipc.CodeOK)
			}

			if resp.Message != testCase.want {
				t.Errorf("message = %q, want %q", resp.Message, testCase.want)
			}
		})
	}
}

// TestHandleCommand_RefusesADisabledMode pins the answer a script gets for a
// mode switched off in the configuration. The daemon used to answer
// "activated" with code OK while the activation did nothing, so a script read
// a no-op as success.
func TestHandleCommand_RefusesADisabledMode(t *testing.T) {
	tests := []struct {
		mode    domain.Mode
		disable func(*config.Config)
	}{
		{domain.ModeHints, func(c *config.Config) { c.Hints.Enabled = false }},
		{domain.ModeGrid, func(c *config.Config) { c.Grid.Enabled = false }},
		{domain.ModeRecursiveGrid, func(c *config.Config) { c.RecursiveGrid.Enabled = false }},
		{domain.ModeBisect, func(c *config.Config) { c.Bisect.Enabled = false }},
		{domain.ModeMonitorSelect, func(c *config.Config) { c.MonitorSelect.Enabled = false }},
	}

	for _, testCase := range tests {
		name := domain.ModeString(testCase.mode)

		t.Run(name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			testCase.disable(cfg)

			resp := newModeTestController(cfg).HandleCommand(
				context.Background(),
				ipc.Command{Action: name},
			)

			if resp.Success {
				t.Fatalf("%s was accepted while disabled: %s", name, resp.Message)
			}

			if resp.Code != ipc.CodeModeDisabled {
				t.Errorf("code = %q, want %q", resp.Code, ipc.CodeModeDisabled)
			}

			if want := name + " mode is disabled in the configuration"; resp.Message != want {
				t.Errorf("message = %q, want %q", resp.Message, want)
			}
		})
	}
}

// TestHandleCommand_ReportsWhyAModeDidNotOpen pins the answer for a mode that
// is enabled but refuses or fails to open. The daemon used to answer
// "activated" with code OK while nothing opened, so a script read the refusal
// as success.
func TestHandleCommand_ReportsWhyAModeDidNotOpen(t *testing.T) {
	const excludedApp = "com.example.excluded"

	tests := []struct {
		name        string
		action      string
		focusedApp  string
		secureInput bool
		stopped     bool
		screensErr  error
		wantMessage string
		wantCode    string
	}{
		{
			name:        "focused app is excluded",
			action:      actionGrid,
			focusedApp:  excludedApp,
			wantMessage: "focused app is excluded",
			wantCode:    ipc.CodeInvalidInput,
		},
		{
			name:        "secure input is on",
			action:      actionGrid,
			secureInput: true,
			wantMessage: "secure input is enabled - a password field may be focused",
			wantCode:    ipc.CodeActionFailed,
		},
		{
			name:        "neru is stopped",
			action:      actionGrid,
			stopped:     true,
			wantMessage: "neru is stopped; run `neru start` first",
			wantCode:    ipc.CodeNotRunning,
		},
		{
			name:        "the monitor picker cannot list the screens",
			action:      domain.ModeNameMonitorSelect,
			screensErr:  derrors.New(derrors.CodeNotSupported, "screens unavailable"),
			wantMessage: "screens unavailable",
			wantCode:    ipc.CodeNotSupported,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.General.ExcludedApps = []string{excludedApp}
			cfg.MonitorSelect.Enabled = true

			appState := state.NewAppState()
			if testCase.stopped {
				appState.Disable()
			}

			controller := newModeTestControllerWith(cfg, &portmocks.MockAccessibilityPort{
				FocusedAppBundleIDFunc: func(context.Context) (string, error) {
					return testCase.focusedApp, nil
				},
			}, &portmocks.MockSystemPort{
				IsSecureInputEnabledFunc: func() bool { return testCase.secureInput },
				ScreensFunc: func(context.Context) ([]ports.Screen, error) {
					return nil, testCase.screensErr
				},
			}, appState)

			resp := controller.HandleCommand(
				context.Background(),
				ipc.Command{Action: testCase.action},
			)

			if resp.Success {
				t.Fatalf("%s was reported as %q with success", testCase.action, resp.Message)
			}

			if resp.Message != testCase.wantMessage {
				t.Errorf("message = %q, want %q", resp.Message, testCase.wantMessage)
			}

			if resp.Code != testCase.wantCode {
				t.Errorf("code = %q, want %q", resp.Code, testCase.wantCode)
			}

			if got := appState.CurrentMode(); got != domain.ModeIdle {
				t.Errorf("mode = %s, want idle", domain.ModeString(got))
			}
		})
	}
}

// TestHandleCommand_ARefusalDoesNotCarryOver pins that the reason one command
// did not open is not reported again for the next command, which opens.
func TestHandleCommand_ARefusalDoesNotCarryOver(t *testing.T) {
	const excludedApp = "com.example.excluded"

	cfg := config.DefaultConfig()
	cfg.General.ExcludedApps = []string{excludedApp}

	controller := newModeTestControllerWith(cfg, &portmocks.MockAccessibilityPort{
		FocusedAppBundleIDFunc: func(context.Context) (string, error) { return excludedApp, nil },
	}, &portmocks.MockSystemPort{}, state.NewAppState())

	refused := controller.HandleCommand(context.Background(), ipc.Command{Action: actionGrid})
	if refused.Success {
		t.Fatalf("grid over an excluded app was reported as %q with success", refused.Message)
	}

	resp := controller.HandleCommand(context.Background(), ipc.Command{Action: actionIdle})

	if !resp.Success {
		t.Errorf("idle after a refused grid was refused: %s", resp.Message)
	}
}
