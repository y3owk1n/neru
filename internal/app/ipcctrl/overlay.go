package ipcctrl

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
)

// The overlay command: hiding the overlay from screen sharing.

// OverlayHandler handles overlay-related IPC commands.
type OverlayHandler struct {
	appState *state.AppState
	logger   *zap.Logger
}

// NewOverlayHandler creates a new overlay command handler.
func NewOverlayHandler(appState *state.AppState, logger *zap.Logger) *OverlayHandler {
	return &OverlayHandler{
		appState: appState,
		logger:   logger,
	}
}

// RegisterHandlers registers overlay command handlers.
func (h *OverlayHandler) RegisterHandlers(
	handlers map[string]func(context.Context, ipc.Command) ipc.Response,
) {
	handlers[domain.CommandToggleScreenShare] = h.handleToggleScreenShare
}

func (h *OverlayHandler) handleToggleScreenShare(
	_ context.Context,
	cmd ipc.Command,
) ipc.Response {
	desired, errResponse := parseToggleState(domain.CommandToggleScreenShare, cmd.Args)
	if errResponse != nil {
		return *errResponse
	}

	// --state names the reported state, so "on" is hidden. Naming it after the
	// visibility instead would invert between the flag and the status field.
	newState := applyToggleState(
		desired,
		// Atomically toggle to avoid check-then-act race
		h.appState.ToggleHiddenForScreenShare,
		h.appState.SetHiddenForScreenShare,
	)

	status := "visible"
	if newState {
		status = "hidden"
	}

	return ipc.Response{
		Success: true,
		Message: "screen share visibility: " + status,
		Code:    ipc.CodeOK,
		Data:    map[string]bool{"hidden": newState},
	}
}
