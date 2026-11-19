package ipcctrl

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
)

const (
	moveCell      = "move_cell"
	dirLeftValue  = "left"
	directionLeft = flagDirection + "=" + dirLeftValue
)

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
