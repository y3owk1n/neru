package ipcctrl

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
)

// The scroll command: inverting the scroll direction.

// ScrollHandler handles scroll-related IPC commands.
type ScrollHandler struct {
	appState      *state.AppState
	scrollService *services.ScrollService
	logger        *zap.Logger
}

// NewScrollHandler creates a new scroll command handler.
func NewScrollHandler(
	appState *state.AppState,
	scrollService *services.ScrollService,
	logger *zap.Logger,
) *ScrollHandler {
	return &ScrollHandler{
		appState:      appState,
		scrollService: scrollService,
		logger:        logger,
	}
}

// RegisterHandlers registers scroll command handlers.
func (h *ScrollHandler) RegisterHandlers(
	handlers map[string]func(context.Context, ipc.Command) ipc.Response,
) {
	handlers[domain.CommandToggleScrollInvert] = h.handleToggleScrollInvert
}

func (h *ScrollHandler) handleToggleScrollInvert(
	_ context.Context,
	cmd ipc.Command,
) ipc.Response {
	desired, errResponse := parseToggleState(domain.CommandToggleScrollInvert, cmd.Args)
	if errResponse != nil {
		return *errResponse
	}

	newState := applyToggleState(
		desired,
		// Atomically toggle to avoid check-then-act race
		h.appState.ToggleScrollInverted,
		h.appState.SetScrollInverted,
	)

	if h.scrollService != nil {
		h.scrollService.SetInvertScroll(newState)
	}

	status := "off"
	if newState {
		status = "on"
	}

	return ipc.Response{
		Success: true,
		Message: "scroll invert: " + status,
		Code:    ipc.CodeOK,
		Data:    map[string]bool{"inverted": newState},
	}
}
