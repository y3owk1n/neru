package ipcctrl

import (
	"context"
	"image"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// handleScrollAction dispatches a scroll sub-action (scroll_up, page_down, etc.)
// to the ScrollService.
func (h *ActionsHandler) handleScrollAction(
	ctx context.Context,
	actionName string,
	parsed parsedActionArgs,
) ipc.Response {
	if h.scrollService == nil {
		return ipc.Response{
			Success: false,
			Message: "scroll service not available",
			Code:    ipc.CodeActionFailed,
		}
	}

	// Scroll never reaches performTargetedAction, so it validates the flag
	// combinations itself. Sticky modifiers are deliberately not merged in
	// here: they are a click affordance, and a scroll that silently picked one
	// up would zoom where the user asked to pan.
	flagErrResp := validateActionFlags(actionName, parsed)
	if flagErrResp != nil {
		return *flagErrResp
	}

	modifiers, modErr := action.ParseModifiers(parsed.modifierStr)
	if modErr != nil {
		return refuseAction(modErr.Error())
	}

	direction, amount, ok := scrollActionMapping(actionName)
	if !ok {
		return ipc.Response{
			Success: false,
			Message: "unknown scroll action: " + actionName,
			Code:    ipc.CodeInvalidInput,
		}
	}

	h.logger.Debug("Performing scroll action via IPC",
		zap.String("action", actionName),
		zap.Int("direction", int(direction)),
		zap.Int("amount", int(amount)),
		zap.String("modifiers", modifiers.String()),
	)

	targetsSelection := parsed.useSelection

	targetPoint := image.Point{}
	if parsed.useSelection {
		var pointErrResp *ipc.Response

		targetPoint, pointErrResp = h.resolveSelectionPoint()
		if pointErrResp != nil {
			return *pointErrResp
		}
	} else if !parsed.useBare {
		if selectionPoint, ok := h.currentSelectionPoint(); ok {
			targetPoint = selectionPoint
			targetsSelection = true
		}
	}

	if targetsSelection {
		if h.actionService == nil {
			return ipc.Response{
				Success: false,
				Message: msgActionServiceNotAvailable,
				Code:    ipc.CodeActionFailed,
			}
		}

		moveErr := h.actionService.MoveCursorToPointAndWait(ctx, targetPoint)
		if moveErr != nil {
			h.logger.Error("Failed to move cursor to scroll target", zap.Error(moveErr))

			return ipc.Response{
				Success: false,
				Message: "failed to perform scroll action: " + moveErr.Error(),
				Code:    ipc.CodeActionFailed,
			}
		}
	}

	scrollErr := h.scrollService.Scroll(ctx, direction, amount, parsed.stepsOverride, modifiers)
	if scrollErr != nil {
		h.logger.Error("Scroll action failed", zap.Error(scrollErr),
			zap.String("action", actionName))

		return ipc.Response{
			Success: false,
			Message: "failed to perform scroll action: " + scrollErr.Error(),
			Code:    ipc.CodeActionFailed,
		}
	}

	return ipc.Response{
		Success: true,
		Message: actionName + " performed",
		Code:    ipc.CodeOK,
	}
}

// scrollActionMapping returns the direction, default amount, and validity for a scroll action name.
func scrollActionMapping(name string) (services.ScrollDirection, services.ScrollAmount, bool) {
	switch name {
	case string(action.NameScrollUp):
		return services.ScrollDirectionUp, services.ScrollAmountChar, true
	case string(action.NameScrollDown):
		return services.ScrollDirectionDown, services.ScrollAmountChar, true
	case string(action.NameScrollLeft):
		return services.ScrollDirectionLeft, services.ScrollAmountChar, true
	case string(action.NameScrollRight):
		return services.ScrollDirectionRight, services.ScrollAmountChar, true
	case string(action.NameGoTop):
		return services.ScrollDirectionUp, services.ScrollAmountEnd, true
	case string(action.NameGoBottom):
		return services.ScrollDirectionDown, services.ScrollAmountEnd, true
	case string(action.NamePageUp):
		return services.ScrollDirectionUp, services.ScrollAmountHalfPage, true
	case string(action.NamePageDown):
		return services.ScrollDirectionDown, services.ScrollAmountHalfPage, true
	default:
		return 0, 0, false
	}
}
