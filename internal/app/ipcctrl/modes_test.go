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
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

// What a mode command may say is the grammar's business, in
// internal/domain/modecmd, and that is where the rules are asserted. These
// cases cover what is left over here: that every mode command goes through the
// grammar, that a refusal reaches the caller with its own words, and that it
// carries the response code a script branches on.

// newModeTestController builds a controller over the real registration path, so
// these cases exercise the wiring rather than a handler called directly.
func newModeTestController() *ipcctrl.Controller {
	cfg := config.DefaultConfig()
	logger := zap.NewNop()
	appState := state.NewAppState()
	actionService := services.NewActionService(
		&portmocks.MockAccessibilityPort{},
		&portmocks.MockOverlayPort{},
		&portmocks.MockSystemPort{},
		logger,
	)

	return ipcctrl.New(ipcctrl.Deps{
		ActionService: actionService,
		ConfigService: loader.NewService(cfg, "", logger, nil),
		AppState:      appState,
		Config:        cfg,
		Modes:         newTestModesHandler(cfg, logger, appState, actionService),
		Logger:        logger,
	})
}

// TestHandleCommand_EveryModeCommandReadsTheGrammar pins that no mode is an
// exception. Each of the seven answers a command it cannot read with a refusal
// of its own rather than with "unknown command" or a silent success.
func TestHandleCommand_EveryModeCommandReadsTheGrammar(t *testing.T) {
	controller := newModeTestController()

	for _, mode := range []domain.Mode{
		domain.ModeHints,
		domain.ModeGrid,
		domain.ModeRecursiveGrid,
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
// --on-exit without an action is the one a user notices: it was refused when
// typed and accepted over the wire, where the steps were stored and never run.
func TestHandleCommand_RefusalsCarryTheGrammarsWording(t *testing.T) {
	controller := newModeTestController()

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
// delegation: a command the grammar accepts reaches the mode handler and is
// answered as done.
//
// Idle is the mode used here because leaving one needs nothing of the platform.
// That a flag reaches the mode it was written for is pinned end to end by the
// journeys in internal/app, which drive a real activation.
func TestHandleCommand_ModeCommandEntersItsMode(t *testing.T) {
	controller := newModeTestController()

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
