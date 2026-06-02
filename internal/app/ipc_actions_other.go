//go:build !darwin

package app

import (
	"context"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

func (h *IPCControllerActions) handleFocusWindowAction(
	_ context.Context,
	_ parsedActionArgs,
) ipc.Response {
	return ipc.Response{
		Success: false,
		Message: derrors.New(
			derrors.CodeNotSupported,
			"focus_window is only supported on macOS",
		).Error(),
		Code: ipc.CodeActionFailed,
	}
}
