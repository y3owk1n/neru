package ipcctrl

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// handleSnapAction moves the cursor onto the nearest element hints would
// label, or with --direction onto the next one that way, inside the active
// mode's region when it has narrowed one. The reply carries where it landed.
//
// Finding nothing is a plain failure, so the rest of a binding such as
// ["action snap", "action left_click"] still runs. Inside a grid mode the
// click then lands on the grid's selection, which the failed snap left alone,
// and a grid keeps working when detection finds nothing. A binding that wants
// to stop writes --bail-on-error. A snap dropped because the user pressed a
// mode key during the scan answers with a chain bail, because the rest of the
// binding was aimed at a target they have left.
func (h *ActionsHandler) handleSnapAction(
	ctx context.Context,
	parsed parsedActionArgs,
) ipc.Response {
	opts, refusal := snapOptions(parsed)
	if refusal != nil {
		return *refusal
	}

	if h.modesHandler == nil {
		return failAction(msgModesHandlerNotAvailable)
	}

	target, err := h.modesHandler.Snap(ctx, opts)
	if err != nil {
		h.logger.Debug("Snap moved nothing", zap.Error(err))

		code := ipc.CodeActionFailed

		switch {
		case derrors.IsCode(err, derrors.CodeContextCanceled):
			code = ipc.CodeChainBail
		case derrors.IsNotSupported(err):
			code = ipc.CodeNotSupported
		}

		return ipc.Response{Success: false, Message: "snap: " + err.Error(), Code: code}
	}

	bounds := target.Element.Bounds()

	return ipc.Response{
		Success: true,
		Message: "snap performed",
		Code:    ipc.CodeOK,
		Data: ipc.SnapData{
			X:      target.Point.X,
			Y:      target.Point.Y,
			Role:   roleEntry(target.Element.Role()),
			Source: string(target.Element.Source()),
			Bounds: ipc.SnapBounds{
				X:      bounds.Min.X,
				Y:      bounds.Min.Y,
				Width:  bounds.Dx(),
				Height: bounds.Dy(),
			},
		},
	}
}

// snapOptions reads a snap request's flags, refusing a value it cannot use.
func snapOptions(parsed parsedActionArgs) (modes.SnapOptions, *ipc.Response) {
	opts := modes.SnapOptions{
		Roles: parsed.roles,
		Text:  parsed.texts,
		Bare:  parsed.useBare,
	}

	if parsed.strategy != "" {
		strategy, err := modecmd.ParseStrategy(parsed.strategy)
		if err != nil {
			refusal := refuseAction(valueMessage(modecmd.FlagStrategy))

			return opts, &refusal
		}

		opts.Strategy = strategy
	}

	if parsed.hasDirection {
		direction, err := domain.ParseDirection(parsed.directionStr)
		if err != nil {
			refusal := refuseAction(derrors.Message(err))

			return opts, &refusal
		}

		opts.Direction, opts.Directed = direction, true
	}

	return opts, nil
}
