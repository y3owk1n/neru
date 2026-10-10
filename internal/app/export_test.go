package app

import (
	"context"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
)

// HandleCommand sends one IPC command through the controller the socket
// serves, so a journey can drive what a CLI command or script sends.
func (a *App) HandleCommand(ctx context.Context, cmd ipc.Command) ipc.Response {
	return a.ipcController.HandleCommand(ctx, cmd)
}
