package ipcctrl

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
)

const bisectAction = "bisect"

func TestHandleAction_BisectRequiresDirection(t *testing.T) {
	controller := &ActionsHandler{logger: zap.NewNop()}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: ActionCommand,
		Args:   []string{bisectAction},
	})

	if resp.Success {
		t.Fatal("handleAction(bisect) without --direction expected rejection, got success")
	}

	if resp.Code != ipc.CodeInvalidInput {
		t.Fatalf("code = %q, want %q", resp.Code, ipc.CodeInvalidInput)
	}
}

func TestHandleAction_BisectRejectsUnknownDirectionAndForeignFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "unknown cut", args: []string{bisectAction, "--direction=sideways"}},
		{name: "zero count", args: []string{bisectAction, directionLeft, "--count=0"}},
		{name: "modifier", args: []string{bisectAction, directionLeft, "--modifier=shift"}},
		{name: "steps", args: []string{bisectAction, directionLeft, stepsThree}},
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
