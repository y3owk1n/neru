package ipcctrl

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/domain"
)

const (
	moveCell      = "move_cell"
	dirLeftValue  = "left"
	directionLeft = flagDirection + "=" + dirLeftValue
)

func TestParseActionArgs_MoveCellFlags(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantParseErr  bool
		wantDirection string
		wantCount     int
		wantHasCount  bool
	}{
		{
			name:          "equals form",
			args:          []string{directionLeft, "--count=3"},
			wantDirection: dirLeftValue,
			wantCount:     3,
			wantHasCount:  true,
		},
		{
			name:          "space form",
			args:          []string{"--direction", "up", "--count", "2"},
			wantDirection: "up",
			wantCount:     2,
			wantHasCount:  true,
		},
		{
			name:          "count is optional",
			args:          []string{"--direction=right"},
			wantDirection: "right",
		},
		{name: "empty direction", args: []string{"--direction="}, wantParseErr: true},
		{name: "missing direction value", args: []string{"--direction"}, wantParseErr: true},
		{name: "non-numeric count", args: []string{"--count=many"}, wantParseErr: true},
		{name: "zero count", args: []string{"--count=0"}, wantParseErr: true},
		{name: "negative count", args: []string{"--count=-2"}, wantParseErr: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			parsed, parseErr := parseActionArgs(testCase.args)

			if parseErr != testCase.wantParseErr {
				t.Fatalf(
					"parseActionArgs(%v) parseErr = %v, want %v",
					testCase.args,
					parseErr,
					testCase.wantParseErr,
				)
			}

			if testCase.wantParseErr {
				return
			}

			if parsed.directionStr != testCase.wantDirection {
				t.Errorf("directionStr = %q, want %q", parsed.directionStr, testCase.wantDirection)
			}

			if parsed.hasCount != testCase.wantHasCount {
				t.Errorf("hasCount = %v, want %v", parsed.hasCount, testCase.wantHasCount)
			}

			if parsed.countVal != testCase.wantCount {
				t.Errorf("countVal = %d, want %d", parsed.countVal, testCase.wantCount)
			}
		})
	}
}

func TestHandleAction_MoveCellRequiresDirection(t *testing.T) {
	controller := &ActionsHandler{logger: zap.NewNop()}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: ActionCommand,
		Args:   []string{moveCell},
	})

	if resp.Success {
		t.Fatal("handleAction(move_cell) without --direction expected rejection, got success")
	}

	if resp.Code != ipc.CodeInvalidInput {
		t.Fatalf("code = %q, want %q", resp.Code, ipc.CodeInvalidInput)
	}
}

func TestHandleAction_MoveCellRejectsUnknownDirection(t *testing.T) {
	controller := &ActionsHandler{logger: zap.NewNop()}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: ActionCommand,
		Args:   []string{moveCell, "--direction=sideways"},
	})

	if resp.Success {
		t.Fatal("handleAction(move_cell --direction=sideways) expected rejection, got success")
	}

	if resp.Code != ipc.CodeInvalidInput {
		t.Fatalf("code = %q, want %q", resp.Code, ipc.CodeInvalidInput)
	}
}

func TestHandleAction_MoveCellRejectsUnsupportedFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "coordinates", args: []string{moveCell, directionLeft, "--x=10", "--y=10"}},
		{name: "modifier", args: []string{moveCell, directionLeft, "--modifier=shift"}},
		{name: "selection", args: []string{moveCell, directionLeft, flagSelection}},
		{name: "steps", args: []string{moveCell, directionLeft, stepsThree}},
		{name: "toggle", args: []string{moveCell, directionLeft, flagToggle}},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			controller := &ActionsHandler{logger: zap.NewNop()}

			resp := controller.handleAction(context.Background(), ipc.Command{
				Action: ActionCommand,
				Args:   testCase.args,
			})

			if resp.Success {
				t.Fatalf("handleAction(%v) expected rejection, got success", testCase.args)
			}

			if resp.Code != ipc.CodeInvalidInput {
				t.Fatalf("code = %q, want %q", resp.Code, ipc.CodeInvalidInput)
			}
		})
	}
}

func TestHandleAction_MoveCellFlagsRejectedOnOtherActions(t *testing.T) {
	controller := &ActionsHandler{logger: zap.NewNop()}

	for _, args := range [][]string{
		{leftClick, directionLeft},
		{moveMouse, "--x=1", "--y=1", "--count=2"},
	} {
		resp := controller.handleAction(context.Background(), ipc.Command{
			Action: ActionCommand,
			Args:   args,
		})

		if resp.Success {
			t.Errorf("handleAction(%v) expected rejection, got success", args)
		}

		if resp.Code != ipc.CodeInvalidInput {
			t.Errorf("handleAction(%v) code = %q, want %q", args, resp.Code, ipc.CodeInvalidInput)
		}
	}
}

func TestHandleAction_MoveCellWithoutModesHandler(t *testing.T) {
	controller := &ActionsHandler{logger: zap.NewNop()}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: ActionCommand,
		Args:   []string{moveCell, "--direction=right"},
	})

	if resp.Success {
		t.Fatal("handleAction(move_cell) with nil modes handler expected failure, got success")
	}

	if resp.Code != ipc.CodeActionFailed {
		t.Fatalf("code = %q, want %q", resp.Code, ipc.CodeActionFailed)
	}

	if resp.Message != msgModesHandlerNotAvailable {
		t.Fatalf("message = %q, want %q", resp.Message, msgModesHandlerNotAvailable)
	}
}

func TestMoveCellDirectionsAreParseable(t *testing.T) {
	// Every direction the CLI advertises has to survive the IPC round trip.
	for _, name := range []string{"left", "right", "up", "down"} {
		parsed, parseErr := parseActionArgs([]string{"--direction=" + name})
		if parseErr {
			t.Fatalf("parseActionArgs(--direction=%s) failed", name)
		}

		_, dirErr := domain.ParseDirection(parsed.directionStr)
		if dirErr != nil {
			t.Errorf("ParseDirection(%q) = %v, want no error", parsed.directionStr, dirErr)
		}
	}
}
