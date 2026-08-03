package ipcctrl

import (
	"context"
	"image"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/ports/mocks"
)

// handleAction is the whole of `neru action ...`: it decides which of two dozen
// handlers a request reaches, and the order of its checks is load-bearing. A
// missing service is reported before a missing flag, for instance, so
// move_mouse_relative with no --dy says the service is unavailable rather than
// naming the flag.
//
// These cases pin the routing rather than the work: with no services wired, each
// request travels as far as the branch that reports what it needs, and that
// report identifies the branch. Reordering the checks changes these answers.
//
// The action names and messages these cases repeat.
const (
	middleClick       = "middle_click"
	moveMouseRelative = "move_mouse_relative"
	scrollDown        = "scroll_down"
	saveCursorPos     = "save_cursor_pos"
	cycleHint         = "cycle_hint"
	hideCursor        = "hide_cursor"
	deltaXOne         = "--dx=1"

	msgNoScrollService = "scroll service not available"

	msgMoveMouseNeedsATarget = "move_mouse requires --x and --y flags, " +
		"--center, --window, active selection, or --bare"
)

// routeCase is one request and the reply it has always produced.
type routeCase struct {
	args    []string
	success bool
	code    string
	message string
}

func routeCases() []routeCase {
	return []routeCase{
		{
			args:    []string{},
			success: false,
			code:    ipc.CodeInvalidInput,
			message: "action subcommand required (e.g., left_click, right_click)",
		},
		{
			args:    []string{leftClick},
			success: false,
			code:    ipc.CodeActionFailed,
			message: msgActionServiceNotAvailable,
		},
		{
			args:    []string{"right_click"},
			success: false,
			code:    ipc.CodeActionFailed,
			message: msgActionServiceNotAvailable,
		},
		{
			args:    []string{middleClick},
			success: false,
			code:    ipc.CodeActionFailed,
			message: msgActionServiceNotAvailable,
		},
		{
			args:    []string{leftClick, "--state=down"},
			success: false,
			code:    ipc.CodeActionFailed,
			message: msgActionServiceNotAvailable,
		},
		{
			args:    []string{leftClick, "--state=up"},
			success: false,
			code:    ipc.CodeActionFailed,
			message: msgActionServiceNotAvailable,
		},
		{
			args:    []string{leftClick, flagToggle},
			success: false,
			code:    ipc.CodeActionFailed,
			message: msgActionServiceNotAvailable,
		},
		{
			args:    []string{leftClick, "--modifier=cmd"},
			success: false,
			code:    ipc.CodeActionFailed,
			message: msgActionServiceNotAvailable,
		},
		{
			args:    []string{leftClick, "--modifier=bogus"},
			success: false,
			code:    ipc.CodeInvalidInput,
			message: "[INVALID_INPUT] unknown modifier \"bogus\" (valid: primary, cmd, super, meta, shift, alt, option, ctrl)",
		},
		{
			args:    []string{"left_click,left_click"},
			success: false,
			code:    ipc.CodeActionFailed,
			message: msgActionServiceNotAvailable,
		},
		{
			args:    []string{"left_click,scroll_up"},
			success: false,
			code:    ipc.CodeInvalidInput,
			message: "scroll sub-action \"scroll_up\" cannot be used in an action chain",
		},
		{
			args:    []string{moveMouse},
			success: false,
			code:    ipc.CodeInvalidInput,
			message: msgMoveMouseNeedsATarget,
		},
		{
			args:    []string{moveMouse, "--center"},
			success: false,
			code:    ipc.CodeActionFailed,
			message: msgActionServiceNotAvailable,
		},
		{
			args:    []string{moveMouse, "--window"},
			success: false,
			code:    ipc.CodeActionFailed,
			message: msgActionServiceNotAvailable,
		},
		{
			args:    []string{moveMouse, "--x=10", "--y=20"},
			success: false,
			code:    ipc.CodeActionFailed,
			message: msgActionServiceNotAvailable,
		},
		{
			args:    []string{moveMouseRelative},
			success: false,
			code:    ipc.CodeActionFailed,
			message: msgActionServiceNotAvailable,
		},
		{
			args:    []string{moveMouseRelative, deltaXOne, "--dy=2"},
			success: false,
			code:    ipc.CodeActionFailed,
			message: msgActionServiceNotAvailable,
		},
		{
			args:    []string{moveMouseRelative, deltaXOne},
			success: false,
			code:    ipc.CodeActionFailed,
			message: msgActionServiceNotAvailable,
		},
		{
			args:    []string{"scroll_up"},
			success: false,
			code:    ipc.CodeActionFailed,
			message: msgNoScrollService,
		},
		{
			args:    []string{scrollDown, "--steps=3"},
			success: false,
			code:    ipc.CodeActionFailed,
			message: msgNoScrollService,
		},
		{
			args:    []string{"page_down"},
			success: false,
			code:    ipc.CodeActionFailed,
			message: msgNoScrollService,
		},
		{
			args:    []string{"reset"},
			success: false,
			code:    ipc.CodeActionFailed,
			message: msgModesHandlerNotAvailable,
		},
		{
			args:    []string{"backspace"},
			success: false,
			code:    ipc.CodeActionFailed,
			message: msgModesHandlerNotAvailable,
		},
		{
			args:    []string{"move_cell", "--direction=left"},
			success: false,
			code:    ipc.CodeActionFailed,
			message: msgModesHandlerNotAvailable,
		},
		{
			args:    []string{"move_cell"},
			success: false,
			code:    ipc.CodeInvalidInput,
			message: "move_cell requires --direction (left, right, up, or down)",
		},
		{
			args:    []string{"wait_for_mode_exit"},
			success: false,
			code:    ipc.CodeActionFailed,
			message: "app state not available",
		},
		{
			args:    []string{saveCursorPos},
			success: false,
			code:    ipc.CodeActionFailed,
			message: msgActionServiceNotAvailable,
		},
		{
			args:    []string{"restore_cursor_pos"},
			success: false,
			code:    ipc.CodeActionFailed,
			message: msgActionServiceNotAvailable,
		},
		{
			args:    []string{saveCursorPos, "--slot=a"},
			success: false,
			code:    ipc.CodeActionFailed,
			message: msgActionServiceNotAvailable,
		},
		{
			args:    []string{"move_monitor", "--direction=left"},
			success: false,
			code:    ipc.CodeInvalidInput,
			message: "--direction is only supported with move_cell",
		},
		{
			args:    []string{cycleHint},
			success: false,
			code:    ipc.CodeActionFailed,
			message: msgModesHandlerNotAvailable,
		},
		{
			args:    []string{cycleHint, "--backward"},
			success: false,
			code:    ipc.CodeActionFailed,
			message: msgModesHandlerNotAvailable,
		},
		{
			args:    []string{"search_hints"},
			success: false,
			code:    ipc.CodeActionFailed,
			message: msgModesHandlerNotAvailable,
		},
		{
			args:    []string{hideCursor},
			success: false,
			code:    ipc.CodeActionFailed,
			message: msgModesHandlerNotAvailable,
		},
		{
			args:    []string{"show_cursor"},
			success: false,
			code:    ipc.CodeActionFailed,
			message: msgModesHandlerNotAvailable,
		},
		{
			args:    []string{"sleep", "1"},
			success: true,
			code:    ipc.CodeOK,
			message: "sleep performed",
		},
	}
}

func TestHandleAction_RoutesEveryRequest(t *testing.T) {
	handler := &ActionsHandler{logger: zap.NewNop()}

	for _, testCase := range routeCases() {
		name := "no arguments"
		if len(testCase.args) > 0 {
			name = testCase.args[0]

			var nameSb262 strings.Builder
			for _, arg := range testCase.args[1:] {
				nameSb262.WriteString(" " + arg)
			}

			name += nameSb262.String()
		}

		t.Run(name, func(t *testing.T) {
			resp := handler.handleAction(context.Background(), ipc.Command{
				Action: ActionCommand,
				Args:   testCase.args,
			})

			if resp.Success != testCase.success {
				t.Errorf("Success = %v, want %v", resp.Success, testCase.success)
			}

			if resp.Code != testCase.code {
				t.Errorf("Code = %s, want %s", resp.Code, testCase.code)
			}

			if resp.Message != testCase.message {
				t.Errorf("Message = %q, want %q", resp.Message, testCase.message)
			}
		})
	}
}

// servedHandler is a handler whose action service works, so a move can succeed.
func servedHandler() *ActionsHandler {
	screen := image.Rect(0, 0, 1000, 800)

	system := &mocks.MockSystemPort{
		ScreenBoundsFunc: func(context.Context) (image.Rectangle, error) {
			return screen, nil
		},
	}

	return &ActionsHandler{
		logger: zap.NewNop(),
		actionService: services.NewActionService(
			&mocks.MockAccessibilityPort{},
			&mocks.MockOverlayPort{},
			system,
			zap.NewNop(),
		),
	}
}

// The cases above run with no services wired, so each request travels only as
// far as the branch that reports what it needs. That pins the routing but never
// reaches a success path — and the success paths carry rules of their own: which
// name the reply quotes, and whether a landed move clears the selection point.
//
// These cases wire a real ActionService over mocked ports so those paths run.
func TestHandleAction_SucceedingMoveReportsTheRequestedName(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"to the screen center", []string{moveMouse, "--center"}, "move_mouse performed"},
		{
			"by an offset",
			[]string{moveMouseRelative, deltaXOne, "--dy=2"},
			"move_mouse_relative performed",
		},

		// --state and --toggle rewrite the action into the press, release or
		// toggle half of the click, but the reply quotes what the user asked
		// for. Quoting the rewritten name instead would report an action they
		// never typed.
		{"a click press", []string{leftClick, "--state=down"}, "left_click performed"},
		{"a click release", []string{leftClick, "--state=up"}, "left_click performed"},
		{"a click toggle", []string{"right_click", flagToggle}, "right_click performed"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			resp := servedHandler().handleAction(context.Background(), ipc.Command{
				Action: ActionCommand,
				Args:   testCase.args,
			})

			if !resp.Success {
				t.Fatalf("Success = false (%s); want the move to land", resp.Message)
			}

			if resp.Message != testCase.want {
				t.Errorf("Message = %q, want %q", resp.Message, testCase.want)
			}
		})
	}
}

// TestHandleAction_MissingServiceIsReportedBeforeAMissingFlag pins the order of the two
// checks. move_mouse_relative needs both a service and its --dx/--dy; a user
// with neither is told about the service, because that is the one they cannot
// fix by retyping the command.
func TestHandleAction_MissingServiceIsReportedBeforeAMissingFlag(t *testing.T) {
	unserved := &ActionsHandler{logger: zap.NewNop()}

	resp := unserved.handleAction(context.Background(), ipc.Command{
		Action: ActionCommand,
		Args:   []string{moveMouseRelative, deltaXOne},
	})

	if resp.Message != msgActionServiceNotAvailable {
		t.Errorf("Message = %q, want the missing service reported first", resp.Message)
	}

	// With the service present, the missing flag is what is left to report.
	served := servedHandler().handleAction(context.Background(), ipc.Command{
		Action: ActionCommand,
		Args:   []string{moveMouseRelative, deltaXOne},
	})

	if served.Message != "move_mouse_relative requires --dx and --dy flags" {
		t.Errorf("Message = %q, want the missing flag reported", served.Message)
	}
}
